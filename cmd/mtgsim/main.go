package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/combo"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/pkg/scryfall"
	"github.com/mtgsim/mtgsim/pkg/stats"
)

// CLI flags
var (
	gamesFlag      = flag.Int("games", 100, "Number of games to simulate")
	decksDirFlag   = flag.String("decks", "decks/1v1", "Directory containing .deck files (searched recursively)")
	swapNFlag      = flag.Int("swap", 0, "Number of sideboard cards to swap into the main deck each game")
	verbosityFlag  = flag.Int("v", 1, "Verbosity: 0=minimal, 1=summary, 2=per-game details")
	logLevelFlag   = flag.String("log", "META", "Log level (META, GAME, PLAYER, CARD)")
	cardStatsFlag  = flag.String("card-stats", "card_library.json", "Path to a JSON file for persistent global card stats (loads existing, merges new, saves on exit)")
)

// Stats accumulators
type deckStats struct {
	wins, losses int
}

type cardPerf struct {
	casts int
	wins  int
}

func recordCast(tracker map[string]map[string]int, ctrl *game.Player, name string) {
	if tracker == nil || ctrl == nil {
		return
	}
	if tracker[ctrl.GetName()] == nil {
		tracker[ctrl.GetName()] = map[string]int{}
	}
	tracker[ctrl.GetName()][name]++
}

type aggregateStats struct {
	turns          []int
	p1Life         []int
	p2Life         []int
	p1CreaturesEnd []int
	p2CreaturesEnd []int
	p1PermsEnd     []int
	p2PermsEnd     []int
	p1HandEnd      []int
	p2HandEnd      []int
	durations      []time.Duration
}

func (a *aggregateStats) add(turns int, p1, p2 *game.Player, dur time.Duration) {
	a.turns = append(a.turns, turns)
	a.p1Life = append(a.p1Life, p1.GetLifeTotal())
	a.p2Life = append(a.p2Life, p2.GetLifeTotal())
	p1c, p1p := boardCounts(p1)
	p2c, p2p := boardCounts(p2)
	a.p1CreaturesEnd = append(a.p1CreaturesEnd, p1c)
	a.p2CreaturesEnd = append(a.p2CreaturesEnd, p2c)
	a.p1PermsEnd = append(a.p1PermsEnd, p1p)
	a.p2PermsEnd = append(a.p2PermsEnd, p2p)
	a.p1HandEnd = append(a.p1HandEnd, len(p1.Hand))
	a.p2HandEnd = append(a.p2HandEnd, len(p2.Hand))
	a.durations = append(a.durations, dur)
}

func (a *aggregateStats) meanInt(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, v := range xs {
		sum += v
	}
	return float64(sum) / float64(len(xs))
}

func (a *aggregateStats) minMax(xs []int) (int, int) {
	if len(xs) == 0 {
		return 0, 0
	}
	mn, mx := xs[0], xs[0]
	for _, v := range xs[1:] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}

func (a *aggregateStats) meanDur(xs []time.Duration) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range xs {
		sum += d
	}
	return time.Duration(int64(sum) / int64(len(xs)))
}

func boardCounts(p *game.Player) (creatures, permanents int) {
	for _, perm := range p.Battlefield {
		permanents++
		if perm.IsCreature() {
			creatures++
		}
	}
	return
}

// Convert card.Card to game.SimpleCard
func toSimple(c card.Card) game.SimpleCard {
	return game.SimpleCard{
		Name:       c.Name,
		TypeLine:   c.TypeLine,
		Power:      c.Power,
		Toughness:  c.Toughness,
		OracleText: c.OracleText,
		Colors:     c.Colors,
	}
}

// Shuffle slice in place
func shuffle[T any](s []T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

// Random sample without replacement of k indices from [0..n)
func sampleIndices(n, k int) []int {
	if k > n {
		k = n
	}
	idx := rand.New(rand.NewSource(time.Now().UnixNano()))
	perm := idx.Perm(n)
	return perm[:k]
}

// Clear all mana from a player's pool
func clearManaPool(p *game.Player) {
	mp := p.GetManaPool()
	for k := range mp {
		delete(mp, k)
	}
}

// Clear all temporary EOT effects (e.g., pump) on a player's permanents
func clearTempEffects(p *game.Player) {
	for _, perm := range p.Battlefield {
		perm.ClearTempBuffs()
	}
}

// Untap all permanents a player controls
func untapAll(p *game.Player) {
	for _, perm := range p.Battlefield {
		perm.Untap()
	}
}

// Parse a mana cost string like "{2}{G}{G}" into game.Mana
func parseCostToGameMana(cost string) game.Mana {
	cm := card.ParseManaCost(cost)
	gm := game.Mana{}
	// Specific colors and colorless
	gm.Add(game.White, cm.Get(game.White))
	gm.Add(game.Blue, cm.Get(game.Blue))
	gm.Add(game.Black, cm.Get(game.Black))
	gm.Add(game.Red, cm.Get(game.Red))
	gm.Add(game.Green, cm.Get(game.Green))
	gm.Add(game.Colorless, cm.Get(game.Colorless))
	// Generic (Any) numeric
	gm.Add(game.Any, cm.Get(game.Any))
	return gm
}

// taxedCost returns the mana cost of a card plus any generic taxes from
// static effects like Thalia, Guardian of Thraben.
func taxedCost(p *game.Player, cardData card.Card, g *game.Game) game.Mana {
	cost := parseCostToGameMana(cardData.ManaCost)
	if g != nil {
		registry := g.GetStaticEffects()
		additional := registry.TotalAdditionalCost(p, cardData.TypeLine)
		if additional > 0 {
			cost.Add(game.Any, additional)
		}
	}
	return cost
}

// Convert game.Mana to ability.Cost
func toAbilityCostFromGameMana(gm game.Mana) abil.Cost {
	mc := map[game.ManaType]int{}
	for _, t := range []game.ManaType{game.White, game.Blue, game.Black, game.Red, game.Green, game.Colorless, game.Any} {
		mc[t] = gm.Get(t)
	}
	return abil.Cost{ManaCost: mc}
}

// Convert ability.Cost back to game.Mana
func abilityCostToGameMana(c abil.Cost) game.Mana {
	gm := game.Mana{}
	for k, v := range c.ManaCost {
		gm.Add(k, v)
	}
	return gm
}

// manaToString converts a game.Mana cost back into a Scryfall-style mana cost string.
func manaToString(m game.Mana) string {
	var s strings.Builder
	if n := m.Get(game.Any); n > 0 {
		fmt.Fprintf(&s, "{%d}", n)
	}
	colors := []struct {
		t game.ManaType
		c string
	}{
		{game.White, "W"},
		{game.Blue, "U"},
		{game.Black, "B"},
		{game.Red, "R"},
		{game.Green, "G"},
		{game.Colorless, "C"},
	}
	for _, col := range colors {
		for i := 0; i < m.Get(col.t); i++ {
			fmt.Fprintf(&s, "{%s}", col.c)
		}
	}
	return s.String()
}

// Check if a mana pool can pay a cost (colored first, Any from remaining total)
func poolCanPay(pool map[game.ManaType]int, cost game.Mana) bool {
	if cost == nil {
		return true
	}
	temp := map[game.ManaType]int{}
	for k, v := range pool {
		temp[k] = v
	}
	specific := []game.ManaType{game.White, game.Blue, game.Black, game.Red, game.Green, game.Colorless}
	for _, t := range specific {
		need := cost.Get(t)
		if need == 0 {
			continue
		}
		if temp[t] < need {
			return false
		}
		temp[t] -= need
	}
	anyNeed := cost.Get(game.Any)
	if anyNeed > 0 {
		remain := 0
		for _, v := range temp {
			remain += v
		}
		if remain < anyNeed {
			return false
		}
	}
	return true
}

// Determine mana types a permanent can produce
func producerTypes(perm *game.Permanent) []game.ManaType {
	is, types := card.CheckManaProducer(perm.GetSource().OracleText)
	if is {
		return types
	}
	if perm.IsLand() {
		return []game.ManaType{game.Colorless}
	}
	return nil
}

// Choose one mana type to produce from a set (prefer colored WUBRG, else Colorless)
func chooseProduction(types []game.ManaType) game.ManaType {
	order := []game.ManaType{game.White, game.Blue, game.Black, game.Red, game.Green, game.Colorless}
	for _, pref := range order {
		for _, t := range types {
			if t == pref || t == game.Any {
				if t == game.Any {
					return game.Colorless
				}
				return pref
			}
		}
	}
	if len(types) > 0 {
		return types[0]
	}
	return game.Colorless
}

// abilityStackAdapter adapts the ability spell casting engine to game.SimpleStack
// so we can call g.CastSimpleSpell without importing ability everywhere.
// CR 601: Casting Spells; CR 117: Priority (casting is only allowed when you have priority).
type abilityStackAdapter struct {
	sce    *abil.SpellCastingEngine
	gs     *bridge.AbilityGameState
	cardDB *card.CardDB
}

func (a *abilityStackAdapter) EnqueueSpell(name string, _ int, manaCost string, typeLine string, controller any, targets []any) error {
	// Look up the real card from the database to include oracle/effects
	cc, ok := a.cardDB.GetCardByName(name)
	if !ok {
		return fmt.Errorf("card not found: %s", name)
	}
	// Use the taxed mana cost passed from the caller if available
	if manaCost != "" {
		cc.ManaCost = manaCost
	}
	c, ok := controller.(abil.AbilityPlayer)
	if !ok {
		// try to map by player name if we received a *game.Player
		if gp, ok2 := controller.(*game.Player); ok2 {
			c = a.gs.GetPlayer(gp.GetName())
		} else {
			return fmt.Errorf("invalid controller type for %s", name)
		}
	}
	// Static-effect constraint check (e.g., Rule of Law)
	if gp, ok2 := controller.(*game.Player); ok2 {
		g := a.gs.G
		if g != nil {
			registry := g.GetStaticEffects()
			spellsCastThisTurn := 0 // TODO: track per-player spells cast per turn accurately
			if !registry.CanCastSpell(gp, spellsCastThisTurn) {
				return fmt.Errorf("cannot cast %s: cast constraint active", name)
			}
		}
	}
	return a.sce.CastSpell(cc, c, targets)
}

func (a *abilityStackAdapter) Size() int { return a.sce.GetStack().Size() }

// napResponseWindow gives the non-active player (dp) a chance to cast
// instants or activate abilities in response to the active player's actions.
// After NAP actions, the stack is resolved.
func napResponseWindow(g *game.Game, nap *game.Player, ap *game.Player, cardDB *card.CardDB, gs *bridge.AbilityGameState, ai *abil.AIDecisionMaker, exec *abil.ExecutionEngine, casts map[string]map[string]int, sce *abil.SpellCastingEngine) {
	// NAP produces mana and casts instants if any
	produceAllAvailableMana(nap)
	castInstants(g, nap, ap, cardDB, nap, gs, ai, exec, casts)
	// Resolve any items added by NAP before returning to AP resolution
	resolveStackWithPermanents(sce, g, gs, exec)
}

// attachAbilitiesAndExecuteETB parses oracle text abilities onto a permanent and
// immediately executes any EntersTheBattlefield triggers.
func attachAbilitiesAndExecuteETB(perm *game.Permanent, oracleText string, exec *abil.ExecutionEngine, gs *bridge.AbilityGameState) {
	if perm == nil || oracleText == "" {
		return
	}
	abs, err := exec.ParseAndRegisterAbilities(oracleText, perm)
	if err != nil || len(abs) == 0 {
		return
	}
	var wrapped []any
	for _, a := range abs {
		a.Source = perm
		wrapped = append(wrapped, a)
	}
	perm.SetAbilities(wrapped)

	ctrlName := perm.GetControllerName()
	if ctrlName == "" {
		return
	}
	ctrlAdapter := gs.GetPlayer(ctrlName)
	if ctrlAdapter == nil {
		return
	}
	for _, a := range abs {
		if a.Type == abil.Triggered && a.TriggerCondition == abil.EntersTheBattlefield {
			for _, effect := range a.Effects {
				_ = exec.ApplyEffect(effect, ctrlAdapter, nil)
			}
		}
	}
}

// resolveStackWithPermanents resolves the ability stack one item at a time.
// After each resolution, if the item was a permanent spell, we create the
// corresponding permanent on the battlefield by moving the card from hand.
// CR 117.4b: When all players pass in succession, the top object on the stack resolves.
func resolveStackWithPermanents(sce *abil.SpellCastingEngine, g *game.Game, gs *bridge.AbilityGameState, exec *abil.ExecutionEngine) {
	st := sce.GetStack()
	for !st.IsEmpty() {
		item := st.Peek()
		// Resolve through the engine so effects (instants/sorceries) apply
		_ = st.ResolveTop()
		if item == nil || item.Spell == nil {
			continue
		}
		name := item.Spell.Name
		tl := item.Spell.TypeLine
		ctrlName := item.Controller.GetName()
		var ctrl *game.Player
		for _, pl := range g.GetPlayersRaw() {
			if pl.GetName() == ctrlName {
				ctrl = pl
				break
			}
		}
		if ctrl == nil {
			continue
		}
		// Move permanent spells onto the battlefield
		var perm *game.Permanent
		if strings.Contains(tl, "Creature") {
			perm, _ = g.SummonCreature(ctrl, name)
		} else if strings.Contains(tl, "Enchantment") || strings.Contains(tl, "Artifact") || strings.Contains(tl, "Planeswalker") {
			perm, _ = g.CastPermanent(ctrl, name)
		} else if strings.Contains(tl, "Instant") || strings.Contains(tl, "Sorcery") {
			// Non-permanent spells move from hand to graveyard on resolution
			idx := ctrl.FindCardInHand(name)
			if idx >= 0 {
				card := ctrl.Hand[idx]
				ctrl.Hand = append(ctrl.Hand[:idx], ctrl.Hand[idx+1:]...)
				ctrl.Graveyard = append(ctrl.Graveyard, card)
			}
		}
		if perm != nil && item.Spell.OracleText != "" {
			attachAbilitiesAndExecuteETB(perm, item.Spell.OracleText, exec, gs)
		}
		g.ApplyStateBasedActions()
	}
}

// Tap all available producers to generate mana (simple heuristic)
func produceAllAvailableMana(p *game.Player) {
	for _, perm := range p.Battlefield {
		if perm.IsTapped() {
			continue
		}
		types := producerTypes(perm)
		if len(types) == 0 {
			continue
		}
		mt := chooseProduction(types)
		perm.Tap()
		p.AddManaToPool(mt, 1)
	}
}

// Compute aggregate colored and generic requirements from castable cards in hand,
// accounting for static-effect mana taxes so the AI produces enough mana.
func handCostTotals(p *game.Player, cardDB *card.CardDB, g *game.Game) (totals game.Mana) {
	totals = game.Mana{}
	for _, c := range p.Hand {
		if c.IsLand() || c.IsInstant() || c.IsSorcery() {
			continue
		}
		if cd, ok := cardDB.GetCardByName(c.Name); ok {
			mc := taxedCost(p, cd, g)
			for _, t := range []game.ManaType{game.White, game.Blue, game.Black, game.Red, game.Green, game.Colorless, game.Any} {
				totals.Add(t, mc.Get(t))
			}
		}
	}
	return
}

func clonePool(mp map[game.ManaType]int) map[game.ManaType]int {
	cp := map[game.ManaType]int{}
	for k, v := range mp {
		cp[k] = v
	}
	return cp
}

// Produce mana tailored to hand needs using a greedy allocation over producers.
// Accounts for taxed costs so the AI does not fall short when stax is present.
func produceSmartMana(p *game.Player, cardDB *card.CardDB, g *game.Game) {
	// Current pool and targets
	pool := clonePool(p.GetManaPool())
	tot := handCostTotals(p, cardDB, g)
	need := map[game.ManaType]int{}
	for _, t := range []game.ManaType{game.White, game.Blue, game.Black, game.Red, game.Green} {
		v := tot.Get(t) - pool[t]
		if v < 0 {
			v = 0
		}
		need[t] = v
	}
	anyNeed := tot.Get(game.Any)
	if anyNeed < 0 {
		anyNeed = 0
	}

	// Collect producers
	type prod struct {
		idx   int
		perm  *game.Permanent
		types []game.ManaType
	}
	var fixed []prod     // single-color producers (WUBRG)
	var flexible []prod  // multi-choice or Any producers
	var colorless []prod // colorless-only producers
	for i, perm := range p.Battlefield {
		if perm.IsTapped() {
			continue
		}
		types := producerTypes(perm)
		if len(types) == 0 {
			continue
		}
		pr := prod{idx: i, perm: perm, types: types}
		// classify
		hasColor := false
		hasAny := false
		for _, t := range types {
			if t == game.Any {
				hasAny = true
			}
			if t == game.White || t == game.Blue || t == game.Black || t == game.Red || t == game.Green {
				hasColor = true
			}
		}
		if hasAny || len(types) > 1 {
			flexible = append(flexible, pr)
		} else if hasColor {
			fixed = append(fixed, pr)
		} else {
			colorless = append(colorless, pr)
		}
	}

	// Pass 1: use fixed producers to satisfy color deficits
	for _, pr := range fixed {
		if len(pr.types) != 1 {
			continue
		}
		t := pr.types[0]
		if need[t] > 0 {
			pr.perm.Tap()
			p.AddManaToPool(t, 1)
			need[t]--
		}
	}

	// Pass 2: flexible producers fill remaining color deficits (largest need first)
	for _, pr := range flexible {
		// pick the color with highest need that this producer can make
		bestT := game.Colorless
		bestVal := 0
		for _, t := range []game.ManaType{game.White, game.Blue, game.Black, game.Red, game.Green} {
			if need[t] <= bestVal {
				continue
			}
			can := false
			for _, pt := range pr.types {
				if pt == t || pt == game.Any {
					can = true
					break
				}
			}
			if can {
				bestT, bestVal = t, need[t]
			}
		}
		if bestVal > 0 && bestT != game.Colorless {
			pr.perm.Tap()
			p.AddManaToPool(bestT, 1)
			need[bestT]--
			continue
		}
		// otherwise assign to generic if anyNeed remains
		if anyNeed > 0 {
			pr.perm.Tap()
			// choose a produced type; prefer Colorless for generic to save colored sources
			p.AddManaToPool(game.Colorless, 1)
			anyNeed--
		}
	}

	// Pass 3: colorless producers for remaining generic need
	for _, pr := range colorless {
		if anyNeed <= 0 {
			break
		}
		pr.perm.Tap()
		p.AddManaToPool(game.Colorless, 1)
		anyNeed--
	}
}

// Cast as many creatures as possible with current pool (descending power)
// Spells are cast through the stack (SimpleStack adapter) and then resolved.
func castAllPossibleCreatures(g *game.Game, p *game.Player, cardDB *card.CardDB, controller any, tracker map[string]map[string]int) {
	for {
		bestIdx := -1
		bestPow := -1
		var bestCard card.Card
		for i, c := range p.Hand {
			if !c.IsCreature() {
				continue
			}
			if cardData, ok := cardDB.GetCardByName(c.Name); ok {
				cost := taxedCost(p, cardData, g)
				if poolCanPay(p.GetManaPool(), cost) {
					pow := atoiSafe(c.Power)
					if pow > bestPow {
						bestPow = pow
						bestIdx = i
						bestCard = cardData
					}
				}
			}
		}
		if bestIdx < 0 {
			break
		}
		cost := taxedCost(p, bestCard, g)
		if err := g.CastSimpleSpell(bestCard.Name, int(bestCard.CMC), manaToString(cost), bestCard.TypeLine, controller, nil); err != nil {
			break
		}
		if ctrl, ok := controller.(*game.Player); ok {
			recordCast(tracker, ctrl, bestCard.Name)
		}
	}
}

// Cast artifacts/enchantments/planeswalkers in main phase (non-creature permanents)
func castNonCreaturePermanents(g *game.Game, p *game.Player, cardDB *card.CardDB, controller any, tracker map[string]map[string]int) {
	for {
		castSomething := false
		for _, c := range p.Hand {
			if !(c.IsArtifact() || c.IsEnchantment() || c.IsPlaneswalker()) {
				continue
			}
			cardData, ok := cardDB.GetCardByName(c.Name)
			if !ok {
				continue
			}
			cost := taxedCost(p, cardData, g)
			if !poolCanPay(p.GetManaPool(), cost) {
				continue
			}
			if err := g.CastSimpleSpell(cardData.Name, int(cardData.CMC), manaToString(cost), cardData.TypeLine, controller, nil); err == nil {
				castSomething = true
				if ctrl, ok := controller.(*game.Player); ok {
					recordCast(tracker, ctrl, cardData.Name)
				}
			}
		}
		if !castSomething {
			break
		}
	}
}

// AI-driven sorcery casting in main phase
func castSorceries(g *game.Game, p *game.Player, dp *game.Player, cardDB *card.CardDB, controller any, gs *bridge.AbilityGameState, ai *abil.AIDecisionMaker, exec *abil.ExecutionEngine, tracker map[string]map[string]int) {
	// Build candidate abilities from sorceries in hand
	var abilities []*abil.Ability
	cardByAbility := map[*abil.Ability]card.Card{}
	for _, sc := range p.Hand {
		if !sc.IsSorcery() {
			continue
		}
		cd, ok := cardDB.GetCardByName(sc.Name)
		if !ok {
			continue
		}
		abs, err := exec.ParseAndRegisterAbilities(cd.OracleText, cd)
		if err != nil || len(abs) == 0 {
			continue
		}
		// Use first parsed ability as proxy and attach mana cost (with taxes)
		ab := *abs[0]
		cost := toAbilityCostFromGameMana(taxedCost(p, cd, g))
		ab.Cost = cost
		abilities = append(abilities, &ab)
		cardByAbility[&ab] = cd
	}
	if len(abilities) == 0 {
		return
	}
	// Decision context
	ap := gs.GetPlayer(p.GetName())
	op := gs.GetPlayer(dp.GetName())
	ctx := ai.BuildDecisionContext(ap, []abil.AbilityPlayer{op}, "Main")
	selected := ai.ChooseAbilitiesToActivate(abilities, ctx)
	for _, ab := range selected {
		cd := cardByAbility[ab]
		gm := abilityCostToGameMana(ab.Cost)
		if !poolCanPay(p.GetManaPool(), gm) {
			continue
		}
		// Choose targets via AI
		ts := ai.ChooseTargetsFor(ab, ctx)
		_ = g.CastSimpleSpell(cd.Name, int(cd.CMC), manaToString(gm), cd.TypeLine, controller, ts)
		if ctrl, ok := controller.(*game.Player); ok {
			recordCast(tracker, ctrl, cd.Name)
		}
	}
}

// AI-driven instant casting for a single window (casts best available one or few)
func castInstants(g *game.Game, p *game.Player, dp *game.Player, cardDB *card.CardDB, controller any, gs *bridge.AbilityGameState, ai *abil.AIDecisionMaker, exec *abil.ExecutionEngine, tracker map[string]map[string]int) {
	var abilities []*abil.Ability
	cardByAbility := map[*abil.Ability]card.Card{}
	for _, sc := range p.Hand {
		if !sc.IsInstant() {
			continue
		}
		cd, ok := cardDB.GetCardByName(sc.Name)
		if !ok {
			continue
		}
		abs, err := exec.ParseAndRegisterAbilities(cd.OracleText, cd)
		if err != nil || len(abs) == 0 {
			continue
		}
		ab := *abs[0]
		ab.Cost = toAbilityCostFromGameMana(taxedCost(p, cd, g))
		abilities = append(abilities, &ab)
		cardByAbility[&ab] = cd
	}
	if len(abilities) == 0 {
		return
	}
	phaseLabel := "Combat"
	if g.IsMainPhase() {
		phaseLabel = "Main"
	}
	if g.GetCurrentPhase() == game.PhaseEnd {
		phaseLabel = "End"
	}
	ap := gs.GetPlayer(p.GetName())
	op := gs.GetPlayer(dp.GetName())
	ctx := ai.BuildDecisionContext(ap, []abil.AbilityPlayer{op}, phaseLabel)
	selected := ai.ChooseAbilitiesToActivate(abilities, ctx)
	for _, ab := range selected {
		cd := cardByAbility[ab]
		gm := abilityCostToGameMana(ab.Cost)
		if !poolCanPay(p.GetManaPool(), gm) {
			continue
		}
		ts := ai.ChooseTargetsFor(ab, ctx)
		_ = g.CastSimpleSpell(cd.Name, int(cd.CMC), manaToString(gm), cd.TypeLine, controller, ts)
		if ctrl, ok := controller.(*game.Player); ok {
			recordCast(tracker, ctrl, cd.Name)
		}
	}
}

// Perform sideboard swap of N cards; keeps main size constant.
func applySideboardSwap(main deck.Deck, side deck.Deck, n int) (deck.Deck, deck.Deck) {
	if n <= 0 || len(side.Cards) == 0 || len(main.Cards) == 0 {
		return main, side
	}
	n = min(n, len(side.Cards))
	n = min(n, len(main.Cards))
	mi := sampleIndices(len(main.Cards), n)
	si := sampleIndices(len(side.Cards), n)
	// sort descending so we can remove by index safely
	sort.Sort(sort.Reverse(sort.IntSlice(mi)))
	sort.Sort(sort.Reverse(sort.IntSlice(si)))
	removedMain := []card.Card{}
	for _, i := range mi {
		removedMain = append(removedMain, main.Cards[i])
		main.Cards = append(main.Cards[:i], main.Cards[i+1:]...)
	}
	addedFromSide := []card.Card{}
	for _, i := range si {
		addedFromSide = append(addedFromSide, side.Cards[i])
		side.Cards = append(side.Cards[:i], side.Cards[i+1:]...)
	}
	// add from side to main
	main.Cards = append(main.Cards, addedFromSide...)
	// put removed main into sideboard to keep total constant
	side.Cards = append(side.Cards, removedMain...)
	return main, side
}

// Build a game using pkg/game with libraries populated from deck cards.
func buildGameFromDecks(d1, d2 deck.Deck) (*game.Game, *game.Player, *game.Player) {
	p1 := game.NewPlayer(d1.Name, 20)
	p2 := game.NewPlayer(d2.Name, 20)

	// Convert decks to libraries
	for _, c := range d1.Cards {
		p1.Library = append(p1.Library, toSimple(c))
	}
	for _, c := range d2.Cards {
		p2.Library = append(p2.Library, toSimple(c))
	}
	// Shuffle libraries
	shuffle(p1.Library)
	shuffle(p2.Library)
	// Draw opening hands
	p1.Draw(7)
	p2.Draw(7)

	g := game.NewGame(p1, p2)
	return g, p1, p2
}

// gameStateSnapshot returns a string representation of the current game state
// excluding turn and phase numbers so it can be used to detect meaningful progress
// across individual phase actions.
func gameStateSnapshot(g *game.Game) string {
	var b strings.Builder
	for _, p := range g.GetPlayersRaw() {
		fmt.Fprintf(&b, "%s:L%d,lost:%v,Lib%d,Hand%d,BF%d,G%d,E%d;",
			p.GetName(), p.GetLifeTotal(), p.HasLost(),
			len(p.Library), len(p.Hand), len(p.Battlefield),
			len(p.Graveyard), len(p.Exile))
	}
	return b.String()
}

// Play a single game with phases, mana, and costs enforced.
// Returns the winner, loser, a draw flag, turn count, duration, and per-player card casts.
func playOneGame(g *game.Game, p1, p2 *game.Player, verbosity int, cardDB *card.CardDB, p1Combo, p2Combo *combo.Index) (winner *game.Player, loser *game.Player, isDraw bool, turns int, dur time.Duration, casts map[string]map[string]int) {
	_ = verbosity
	start := time.Now()
	landDropUsed := map[*game.Player]bool{}
	maxTurns := 30
	casts = map[string]map[string]int{}

	// Wire ability stack + priority engine
	gs := bridge.NewAbilityGameState(g)
	exec := abil.NewExecutionEngine(gs)
	sce := abil.NewSpellCastingEngine(gs, exec)
	ai := abil.NewAIDecisionMaker(exec)
	if p1Combo != nil {
		ai.SetComboIndex(p1.GetName(), p1Combo)
	}
	if p2Combo != nil {
		ai.SetComboIndex(p2.GetName(), p2Combo)
	}
	adapter := &abilityStackAdapter{sce: sce, gs: gs, cardDB: cardDB}
	g.SetStack(adapter)

	lastState := ""
	unchangedPhases := 0
	const maxUnchangedPhases = 12

	for g.GetTurnNumber() <= maxTurns {
		// Detect stale game state before each phase action
		currentState := gameStateSnapshot(g)
		if currentState == lastState {
			unchangedPhases++
			if unchangedPhases >= maxUnchangedPhases {
				logger.LogMeta("STUCK LOOP: no state change for %d consecutive phases, breaking. snapshot=%s", unchangedPhases, currentState)
				break
			}
		} else {
			unchangedPhases = 0
			lastState = currentState
		}

		ap := g.GetActivePlayerRaw()
		dp := opponentOf(g, ap)
		phase := g.GetCurrentPhase()

		// Keep ability engine in sync with game state.
		// CR 117.1a: A player gets priority at specific times; CR 117.3b: sorcery timing requires main phase and empty stack.
		sce.SetPlayers(gs.GetAllPlayers())
		sce.SetActivePlayer(gs.GetActivePlayer())
		var phaseLabel string
		switch phase {
		case game.PhaseMain1, game.PhaseMain2:
			phaseLabel = "Main Phase"
		case game.PhaseCombat:
			phaseLabel = "Combat Phase"
		case game.PhaseEnd:
			phaseLabel = "End Step"
		default:
			phaseLabel = ""
		}
		sce.SetPhase(phaseLabel)

		switch phase {
		case game.PhaseUntap:
			untapAll(ap)
			landDropUsed[ap] = false
			clearManaPool(ap)
		case game.PhaseUpkeep:
			// no-op (hooks for triggers would go here)
		case game.PhaseDraw:
			if ap.Draw(1) == 0 && len(ap.Library) == 0 {
				ap.Lose("deckout")
			}
		case game.PhaseMain1, game.PhaseMain2:
			// One land per turn per player
			if !landDropUsed[ap] {
				landIdx := -1
				for i, c := range ap.Hand {
					if c.IsLand() {
						landIdx = i
						break
					}
				}
				if landIdx >= 0 {
					c := ap.Hand[landIdx]
					// CR 305.2: A player may play one land during their main phase when they have priority and the stack is empty.
					// Do not remove from hand here; Player.PlayLand handles the zone change (single removal).
					if perm, err := g.PlayLand(ap, c.Name); err == nil {
						landDropUsed[ap] = true
						recordCast(casts, ap, c.Name)
						if c.OracleText != "" {
							attachAbilitiesAndExecuteETB(perm, c.OracleText, exec, gs)
						}
					}
				}
			}
			// Generate mana by tapping producers and cast spells via stack
			produceSmartMana(ap, cardDB, g)
			// Pre-sorcery instant-speed window (AP then NAP priority)
			produceAllAvailableMana(ap)
			castInstants(g, ap, dp, cardDB, ap, gs, ai, exec, casts)
			napResponseWindow(g, dp, ap, cardDB, gs, ai, exec, casts, sce)
			// Cast non-creature permanents first, then sorceries, then creatures
			castNonCreaturePermanents(g, ap, cardDB, ap, casts)
			castSorceries(g, ap, dp, cardDB, ap, gs, ai, exec, casts)
			castAllPossibleCreatures(g, ap, cardDB, ap, casts)
			// Post-sorcery instant-speed window (AP then NAP priority)
			produceAllAvailableMana(ap)
			castInstants(g, ap, dp, cardDB, ap, gs, ai, exec, casts)
			napResponseWindow(g, dp, ap, cardDB, gs, ai, exec, casts, sce)
			// CR 106.4: Any unused mana in a player's mana pool empties as steps and phases end.
			clearManaPool(ap)
		case game.PhaseCombat:
			g.BeginCombat()
			// Attack with all untapped creatures
			for _, perm := range ap.GetCreatures() {
				if !perm.IsTapped() {
					_ = g.DeclareAttacker(perm, dp)
				}
			}
			// Attacker instant-speed window (after attackers declared, before blocks)
			produceAllAvailableMana(ap)
			castInstants(g, ap, dp, cardDB, ap, gs, ai, exec, casts)
			napResponseWindow(g, dp, ap, cardDB, gs, ai, exec, casts, sce)
			// Blockers: only block sometimes; prefer survival blocks, then trades
			for _, blocker := range dp.GetCreatures() {
				if blocker.IsTapped() {
					continue
				}
				// 50% chance to skip blocking to allow damage through
				if rand.Intn(2) == 0 {
					continue
				}
				blocked := false
				// Prefer blocks where blocker survives
				for _, cand := range ap.GetCreatures() {
					if blocker.GetToughness() > cand.GetPower() {
						if err := g.DeclareBlocker(blocker, cand); err == nil {
							blocked = true
							break
						}
					}
				}
				if !blocked {
					// Try to trade if possible
					for _, cand := range ap.GetCreatures() {
						if cand.GetToughness() <= blocker.GetPower() {
							if err := g.DeclareBlocker(blocker, cand); err == nil {
								break
							}
						}
					}
				}
			}
			// Defender instant-speed window (after blocks declared, before damage)
			produceAllAvailableMana(dp)
			castInstants(g, dp, ap, cardDB, dp, gs, ai, exec, casts)
			// NAP in combat is the attacker when defender acts; give AP a chance to respond
			napResponseWindow(g, ap, dp, cardDB, gs, ai, exec, casts, sce)
			g.ResolveCombatDamage()
		case game.PhaseEnd:
			// End step instant windows (active then non-active player)
			produceAllAvailableMana(ap)
			castInstants(g, ap, dp, cardDB, ap, gs, ai, exec, casts)
			napResponseWindow(g, dp, ap, cardDB, gs, ai, exec, casts, sce)
			produceAllAvailableMana(dp)
			castInstants(g, dp, ap, cardDB, dp, gs, ai, exec, casts)
			napResponseWindow(g, ap, dp, cardDB, gs, ai, exec, casts, sce)
			// Discard down to 7
			for len(ap.Hand) > 7 {
				i := rand.Intn(len(ap.Hand))
				c := ap.Hand[i]
				ap.Hand = append(ap.Hand[:i], ap.Hand[i+1:]...)
				ap.Graveyard = append(ap.Graveyard, c)
			}
			// Cleanup EOT temporary effects and empty pools
			clearTempEffects(ap)
			clearTempEffects(dp)
			clearManaPool(ap)
			clearManaPool(dp)
		}

		// Advance
		g.AdvancePhase()
		g.ApplyStateBasedActions()

		// Check for loss
		if p1.HasLost() || p2.HasLost() || p1.GetLifeTotal() <= 0 || p2.GetLifeTotal() <= 0 {
			break
		}
	}
	turns = g.GetTurnNumber()
	dur = time.Since(start)

	// Apply final SBAs to ensure loss reasons are set
	g.ApplyStateBasedActions()

	p1Lost := p1.HasLost() || p1.GetLifeTotal() <= 0
	p2Lost := p2.HasLost() || p2.GetLifeTotal() <= 0

	// Draw: both players eliminated simultaneously
	if p1Lost && p2Lost {
		return nil, nil, true, turns, dur, casts
	}
	if !p1Lost && p2Lost {
		return p1, p2, false, turns, dur, casts
	}
	if p1Lost && !p2Lost {
		return p2, p1, false, turns, dur, casts
	}

	// No one has lost yet: use life total as tie-breaker
	if p1.GetLifeTotal() > p2.GetLifeTotal() {
		return p1, p2, false, turns, dur, casts
	}
	if p2.GetLifeTotal() > p1.GetLifeTotal() {
		return p2, p1, false, turns, dur, casts
	}
	// tie-breaker: active player loses
	if g.GetActivePlayerRaw() == p1 {
		return p2, p1, false, turns, dur, casts
	}
	return p1, p2, false, turns, dur, casts
}

func atoiSafe(s string) int { var v int; _, _ = fmt.Sscanf(strings.TrimSpace(s), "%d", &v); return v }

func opponentOf(g *game.Game, p *game.Player) *game.Player {
	for _, o := range g.GetPlayersRaw() {
		if o != p {
			return o
		}
	}
	return nil
}

// Wilson score interval (95%) for win rate
func wilson95(wins, total int) (float64, float64) {
	if total == 0 {
		return 0, 0
	}
	z := 1.96
	p := float64(wins) / float64(total)
	n := float64(total)
	den := 1 + z*z/n
	center := (p + (z*z)/(2*n)) / den
	radius := (z * math.Sqrt((p*(1-p)+z*z/(4*n))/n)) / den
	return center - radius, center + radius
}

// deckCardNames extracts unique card names from a deck.
func deckCardNames(d deck.Deck) []string {
	seen := make(map[string]bool, len(d.Cards))
	out := make([]string, 0, len(d.Cards))
	for _, c := range d.Cards {
		if !seen[c.Name] {
			seen[c.Name] = true
			out = append(out, c.Name)
		}
	}
	return out
}

// buildComboIndex lazily builds (or returns a cached) combo index for a deck.
func buildComboIndex(d deck.Deck, client *combo.Client, cache map[string]*combo.Index) *combo.Index {
	if ci, ok := cache[d.Name]; ok {
		return ci
	}
	names := deckCardNames(d)
	result, err := client.FindMyCombos(names, nil)
	if err != nil {
		return nil
	}
	ci := combo.NewIndex(result, names)
	cache[d.Name] = ci
	return ci
}

// enrichCardLibraryImages backfills image URLs from the card database and downloads them to cache.
func enrichCardLibraryImages(lib *stats.CardLibrary, db *card.CardDB, sfClient *scryfall.Client, cache map[string]string) {
	for name := range lib.Cards {
		var imageURL string

		if url, ok := cache[name]; ok && url != "" {
			imageURL = url
		} else if c, ok := db.GetCardByName(name); ok && c.ImageURIs != nil && c.ImageURIs.Normal != "" {
			imageURL = c.ImageURIs.Normal
			cache[name] = imageURL
		} else {
			cd, err := sfClient.GetCardByName(name)
			if err == nil && cd.ImageURIs != nil && cd.ImageURIs.Normal != "" {
				imageURL = cd.ImageURIs.Normal
				cache[name] = imageURL
			}
		}

		// Store URL for web display and download to cache
		if imageURL != "" {
			lib.SetImageURL(name, imageURL)
			// Download and cache the image file for offline use
			if _, err := sfClient.DownloadAndCacheImage(imageURL); err != nil {
				logger.LogMeta("Failed to cache image for %s: %v", name, err)
			}
		}
	}
}

func main() {
	flag.Parse()
	logger.SetLogLevel(logger.ParseLogLevel(*logLevelFlag))

	cardLib, err := stats.LoadCardLibrary(*cardStatsFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading card stats library: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := cardLib.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving card stats library: %v\n", err)
		}
	}()

	// Load card DB
	logger.LogMeta("Loading card database...")
	cardDB, err := card.LoadCardDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading card database: %v\n", err)
		os.Exit(1)
	}
	logger.LogMeta("Card database loaded with %d cards", cardDB.Size())

	// Initialize combo and scryfall clients.
	comboClient := combo.NewClient()
	scryfallClient := scryfall.NewClient()

	// Cache combo indices and image URLs by deck path to avoid repeated work.
	deckComboCache := map[string]*combo.Index{}
	deckImageCache := map[string]string{}

	// Discover deck files
	deckFiles := []string{}
	if err := filepath.WalkDir(*decksDirFlag, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".deck") {
			deckFiles = append(deckFiles, path)
		}
		return nil
	}); err != nil {
		logger.LogMeta("Error walking deck directory: %v", err)
	}
	if len(deckFiles) < 2 {
		fmt.Fprintln(os.Stderr, "Error: need at least two .deck files in the specified directory")
		os.Exit(1)
	}
	logger.LogMeta("Found %d deck files", len(deckFiles))

	// Warn about unimplemented cards in discovered decks
	implTracker := abil.NewImplementationTracker()
	warnedDecks := map[string]bool{}
	for _, dPath := range deckFiles {
		if warnedDecks[dPath] {
			continue
		}
		warnedDecks[dPath] = true
		m, _, err := deck.ImportDeckfile(dPath, cardDB)
		if err != nil {
			continue
		}
		unimpl := implTracker.CheckDeck(m.Cards, cardDB)
		for _, name := range unimpl {
			logger.LogMeta("Warning: unimplemented card in %s: %s", dPath, name)
		}
	}
	if report, err := card.ComputeImplementationStatus(cardDB, implTracker); err == nil {
		logger.LogMeta("Implementation status: %d/%d cards (%.1f%%)", report.ImplementedCount, report.TotalCards, report.Percentage)
		_ = implTracker.Save()
	}

	// Per-deck stats
	perDeck := map[string]*deckStats{}
	perCard := map[string]map[string]*cardPerf{}
	agg := &aggregateStats{}

	startAll := time.Now()
	for i := 0; i < *gamesFlag; i++ {
		// pick two distinct decks at random
		d1 := deckFiles[rand.Intn(len(deckFiles))]
		d2 := deckFiles[rand.Intn(len(deckFiles))]
		for d2 == d1 && len(deckFiles) > 1 {
			d2 = deckFiles[rand.Intn(len(deckFiles))]
		}

		// import decks (main+side)
		m1, s1, err1 := deck.ImportDeckfile(d1, cardDB)
		m2, s2, err2 := deck.ImportDeckfile(d2, cardDB)
		if err1 != nil || err2 != nil || m1.Size() == 0 || m2.Size() == 0 {
			if *verbosityFlag >= 1 {
				fmt.Printf("Skipping game %d due to deck import error or empty deck\n", i+1)
			}
			continue
		}

		// sideboard swap
		m1, s1 = applySideboardSwap(m1, s1, *swapNFlag)
		m2, s2 = applySideboardSwap(m2, s2, *swapNFlag)
		_ = s1
		_ = s2 // sideboards not used in-game further

		// build and play
		g, p1, p2 := buildGameFromDecks(m1, m2)
		p1Combo := buildComboIndex(m1, comboClient, deckComboCache)
		p2Combo := buildComboIndex(m2, comboClient, deckComboCache)
		winner, loser, isDraw, turns, dur, gameCasts := playOneGame(g, p1, p2, *verbosityFlag, cardDB, p1Combo, p2Combo)

		if !isDraw {
			// stats update
			if perDeck[winner.GetName()] == nil {
				perDeck[winner.GetName()] = &deckStats{}
			}
			if perDeck[loser.GetName()] == nil {
				perDeck[loser.GetName()] = &deckStats{}
			}
			perDeck[winner.GetName()].wins++
			perDeck[loser.GetName()].losses++
		}
		agg.add(turns, p1, p2, dur)

		// Aggregate per-card stats
		for pName, pCasts := range gameCasts {
			if perCard[pName] == nil {
				perCard[pName] = map[string]*cardPerf{}
			}
			for cName, n := range pCasts {
				if perCard[pName][cName] == nil {
					perCard[pName][cName] = &cardPerf{}
				}
				perCard[pName][cName].casts += n
				wins := 0
				if !isDraw && winner != nil && winner.GetName() == pName {
					perCard[pName][cName].wins += n
					wins = n
				}
				cardLib.RecordCounts(cName, n, wins)
			}
		}

		if *verbosityFlag >= 2 {
			winnerName := "DRAW"
			if winner != nil {
				winnerName = winner.GetName()
			}
			fmt.Printf("Game %3d: %-30s vs %-30s | Winner: %-30s | Turns: %2d | Dur: %s | Final Life: [%d, %d]\n",
				i+1, p1.GetName(), p2.GetName(), winnerName, turns, dur.Truncate(time.Millisecond), p1.GetLifeTotal(), p2.GetLifeTotal())
		}
	}
	// Backfill image URLs into the persistent card library.
	enrichCardLibraryImages(cardLib, cardDB, scryfallClient, deckImageCache)

	elapsed := time.Since(startAll)

	// Summary output
	totalGames := 0
	for _, ds := range perDeck {
		totalGames += ds.wins + ds.losses
	}
	if *verbosityFlag >= 0 {
		fmt.Println()
		fmt.Println("== Simulation Summary ==")
		fmt.Printf("Games: %d | Total Time: %s | Games/sec: %d\n", totalGames, elapsed.Truncate(time.Millisecond), int(float64(totalGames)/elapsed.Seconds()+0.5))
	}

	// Per-deck table
	if *verbosityFlag >= 1 {
		fmt.Println()
		fmt.Println("Deck Performance (95% CI)")
		fmt.Printf("%-35s %8s %8s %9s %20s\n", "Deck", "Wins", "Losses", "Win%", "95% CI")
		// stable order
		names := make([]string, 0, len(perDeck))
		for n := range perDeck {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			ds := perDeck[n]
			t := ds.wins + ds.losses
			winRate := 0.0
			if t > 0 {
				winRate = 100 * float64(ds.wins) / float64(t)
			}
			l, u := wilson95(ds.wins, t)
			fmt.Printf("%-35s %8d %8d %8.1f%%   [%5.1f%%, %5.1f%%]\n", truncate(n, 35), ds.wins, ds.losses, winRate, 100*l, 100*u)
		}
	}

	// Aggregates
	if *verbosityFlag >= 0 {
		fmt.Println()
		fmt.Println("Aggregate Game Metrics")
		fmt.Printf("Avg Turns: %.2f\n", (&aggregateStats{turns: agg.turns}).meanInt(agg.turns))
		p1Avg, p2Avg := (&aggregateStats{}).meanInt(agg.p1Life), (&aggregateStats{}).meanInt(agg.p2Life)
		p1Min, p1Max := agg.minMax(agg.p1Life)
		p2Min, p2Max := agg.minMax(agg.p2Life)
		fmt.Printf("Final Life P1: avg=%.2f min=%d max=%d | P2: avg=%.2f min=%d max=%d\n", p1Avg, p1Min, p1Max, p2Avg, p2Min, p2Max)
		fmt.Printf("End Board P1: creatures=%.2f perms=%.2f | P2: creatures=%.2f perms=%.2f\n",
			(&aggregateStats{}).meanInt(agg.p1CreaturesEnd), (&aggregateStats{}).meanInt(agg.p1PermsEnd),
			(&aggregateStats{}).meanInt(agg.p2CreaturesEnd), (&aggregateStats{}).meanInt(agg.p2PermsEnd))
		fmt.Printf("End Hand P1: avg=%.2f | P2: avg=%.2f\n",
			(&aggregateStats{}).meanInt(agg.p1HandEnd), (&aggregateStats{}).meanInt(agg.p2HandEnd))
		fmt.Printf("Avg Game Duration: %s\n", agg.meanDur(agg.durations).Truncate(time.Millisecond))
	}

	// Top Cards by Win Rate When Cast
	if *verbosityFlag >= 1 {
		fmt.Println()
		fmt.Println("Top Cards by Win Rate When Cast (min 5 casts)")
		fmt.Printf("%-35s %-30s %8s %8s %9s\n", "Deck", "Card", "Casts", "Wins", "Win%")
		type cardEntry struct {
			deck    string
			name    string
			casts   int
			wins    int
			winRate float64
		}
		var entries []cardEntry
		for dName, cards := range perCard {
			for cName, cp := range cards {
				if cp.casts >= 5 {
					entries = append(entries, cardEntry{
						deck: dName, name: cName, casts: cp.casts, wins: cp.wins,
						winRate: 100 * float64(cp.wins) / float64(cp.casts),
					})
				}
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].winRate != entries[j].winRate {
				return entries[i].winRate > entries[j].winRate
			}
			return entries[i].casts > entries[j].casts
		})
		for _, e := range entries {
			fmt.Printf("%-35s %-30s %8d %8d %8.1f%%\n", truncate(e.deck, 35), truncate(e.name, 30), e.casts, e.wins, e.winRate)
		}
	}

	// Global Card Library (suppressed when running more than 100 games)
	if *verbosityFlag >= 1 && *cardStatsFlag != "" && *gamesFlag <= 100 {
		fmt.Println()
		fmt.Println("Global Card Library (All Runs)")
		fmt.Printf("%-40s %8s %8s %9s\n", "Card", "Casts", "Wins", "Win%")
		for _, e := range cardLib.TopCards(5, 50) {
			fmt.Printf("%-40s %8d %8d %8.1f%%\n", truncate(e.Name, 40), e.Casts, e.Wins, e.WinRate)
		}
	}

	// Report unimplemented/unsupported cards encountered
	um := abil.GetUnimplementedCards()
	if len(um) > 0 {
		fmt.Println()
		fmt.Println("Unimplemented/Unsupported Cards Encountered")
		names := make([]string, 0, len(um))
		for n := range um {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Printf("- %s: %s\n", n, um[n])
		}
	}

}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
