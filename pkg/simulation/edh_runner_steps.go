package simulation

import (
	"strings"

	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// stepOneEDHTurn drives the active player through a complete turn using
// the simplified runner AI. Returns (anyAlive, stuck) where anyAlive is
// false if no players are alive after the turn finishes, and stuck is
// true if the game state did not progress for maxUnchangedActions
// consecutive phase actions. priority is invoked at instant-speed windows
// so future AI can respond on opponents' turns; log is optional.
func stepOneEDHTurn(g *game.Game, casts []int, priority PriorityHandler, log *EDHEventLog, metrics *edhMetrics) (bool, bool) {
	startTurn := g.GetTurnNumber()
	milledThisTurn := false
	if metrics != nil {
		metrics.resetTurn()
	}

	// Extract the stack handler if available for stack-based casting in the main phase.
	var stackHandler *StackAwareHandler
	if h, ok := priority.(*StackAwareHandler); ok {
		stackHandler = h
	}

	lastState := ""
	unchangedActions := 0
	const maxUnchangedActions = 12

	for {
		// Detect stale game state before each phase action
		currentState := edhActionStateSnapshot(g)
		if currentState == lastState {
			unchangedActions++
			if unchangedActions >= maxUnchangedActions {
				logger.LogMeta("STUCK LOOP: no state change for %d consecutive actions, breaking. snapshot=%s", unchangedActions, currentState)
				return survivors(g) >= 1, true
			}
		} else {
			unchangedActions = 0
			lastState = currentState
		}

		ap := g.GetActivePlayerRaw()
		switch g.GetCurrentPhase() {
		case game.PhaseUntap:
			for _, perm := range ap.Battlefield {
				perm.Untap()
			}
		case game.PhaseUpkeep:
			offerOpponentPriority(g, ap, priority)
		case game.PhaseDraw:
			if g.GetTurnNumber() > 1 || ap != g.GetPlayerByIndex(0) {
				if ap.Draw(1) == 0 && len(ap.Library) == 0 {
					ap.Lose("deckout") // CR 704.5b: empty-library draw loss
					milledThisTurn = true
				}
			}
			offerOpponentPriority(g, ap, priority)
		case game.PhaseMain1:
			if log != nil {
				log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventTurnStart, Actor: ap.GetName()})
			}
			runMainPhase(g, ap, casts, log, metrics, stackHandler)
			offerOpponentPriority(g, ap, priority)
		case game.PhaseCombat:
			runCombatPhase(g, ap, log, metrics)
			offerOpponentPriority(g, ap, priority)
		case game.PhaseEnd:
			offerOpponentPriority(g, ap, priority)
		}
		g.ApplyStateBasedActions()
		if milledThisTurn {
			markMillIfApplicable(ap)
		}
		recordEliminations(g, log, ap, metrics)
		g.AdvancePhase()
		if survivors(g) <= 1 {
			return survivors(g) >= 1, false
		}
		if g.GetTurnNumber() != startTurn {
			break
		}
	}
	return survivors(g) >= 1, false
}

// offerOpponentPriority walks each living non-active opponent in APNAP
// order and invokes the priority handler. Default Noop short-circuits.
func offerOpponentPriority(g *game.Game, ap *game.Player, h PriorityHandler) {
	if h == nil {
		return
	}
	if _, ok := h.(NoopPriorityHandler); ok {
		return
	}
	players := g.GetPlayersRaw()
	n := len(players)
	start := indexOfPlayer(g, ap)
	for i := 1; i < n; i++ {
		opp := players[(start+i)%n]
		if opp == ap || opp.HasLost() {
			continue
		}
		h.OnOpponentPriority(g, ap, opp, g.GetCurrentPhase())
	}
}

// recordEliminations emits a player_eliminated event the first time a
// player is observed as lost. Tracking lives in the closure-free way:
// we just inspect HasLost and avoid duplicate emission by checking the
// last event of the matching kind for that actor.
func recordEliminations(g *game.Game, log *EDHEventLog, ap *game.Player, metrics *edhMetrics) {
	if log == nil && metrics == nil {
		return
	}
	var existing []EDHEvent
	if log != nil {
		existing = log.Events()
	}
	already := map[string]bool{}
	for _, e := range existing {
		if e.Kind == EventPlayerEliminated {
			already[e.Actor] = true
		}
	}
	for _, p := range g.GetPlayersRaw() {
		if !p.HasLost() || already[p.GetName()] {
			continue
		}
		loserIdx := indexOfPlayer(g, p)
		if metrics != nil && !metrics.recordPlayerLost(loserIdx) {
			continue
		}
		if log != nil {
			log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(g.GetCurrentPhase()), Kind: EventPlayerEliminated, Actor: p.GetName()})
		}
		if p != ap && metrics != nil {
			metrics.recordElimination(indexOfPlayer(g, ap))
		}
	}
}

// runMainPhase plays one land, casts the commander when possible, and
// summons creatures in hand if mana allows. Optional log
// records every public action so a replay can be reproduced.
// stackHandler, when non-nil, routes instants and sorceries through the
// ability package's stack so opponents can respond before resolution.
func runMainPhase(g *game.Game, ap *game.Player, casts []int, log *EDHEventLog, metrics *edhMetrics, stackHandler *StackAwareHandler) {
	idx := indexOfPlayer(g, ap)
	landsPlayed := 0
	for _, c := range ap.Hand {
		if c.IsLand() {
			if landsPlayed >= ap.LandPlaysAvailable() {
				break
			}
			if _, err := g.PlayLand(ap, c.Name); err != nil {
				continue
			}
			if log != nil {
				log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventLandPlay, Actor: ap.GetName(), Detail: c.Name})
			}
			if metrics != nil {
				metrics.recordLand(idx, c.Name)
			}
			landsPlayed++
		}
	}
	ap.ResetLandPlays()

	activateSearchAbilities(g, ap, log)

	tapManaSourcesForMainPhaseMana(g, ap, idx, metrics)

	if idx >= 0 && len(ap.CommandZone) > 0 {
		name := ap.CommandZone[0].Name
		cmdrCard := ap.CommandZone[0]
		cost := cmdrCard.GetManaCost()
		if cost.Total() == 0 && cmdrCard.ManaCost == "" {
			goto skipCommander
		}
		if ap.CanPayForCommander(cmdrCard) {
			if ap.PayForCommander(cmdrCard) {
				manaSpent := manaSpentForCommander(ap, cmdrCard)
				perm := ap.CastCommander(name)
				if perm == nil {
					return
				}
				perm.SetEnteredTurn(g.GetTurnNumber())
				casts[idx]++
				storm := 0
				if metrics != nil {
					storm = metrics.recordSpell(idx, manaSpent, true, name)
				}
				if log != nil {
					log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventCommanderCast, Actor: ap.GetName(), Detail: eventDetail(name, manaSpent, storm)})
				}
			}
		}
	}
skipCommander:
	if attemptCEDHComboFinish(g, ap, log, metrics) {
		return
	}

	again := true
	for again {
		again = false
		tapManaSourcesForMainPhaseMana(g, ap, idx, metrics)
		for _, c := range ap.Hand {
			if !isCastableSpell(c) || c.IsCounterspell() || !ap.CanPayForCard(c) {
				continue
			}
			if c.GetManaCost().Total() == 0 && c.ManaCost == "" {
				continue
			}
			if !ap.PayForCard(c) {
				continue
			}
			manaSpent := manaSpentForCard(c)
			if c.IsInstant() || c.IsSorcery() {
				var resolved bool
				if stackHandler != nil {
					resolved = stackHandler.CastSpellThroughStack(ap, c, ap.GetName())
				} else {
					resolved = castNonPermanentSpell(g, ap, c, log, metrics)
				}
				storm := 0
				if metrics != nil && resolved {
					storm = metrics.recordSpell(idx, manaSpent, c.IsCreature(), c.Name)
				}
				if log != nil && resolved {
					countered := manaSpent == 0 && checkVexingBauble(g, ap, c, log)
					if !countered {
						log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventPermanentCast, Actor: ap.GetName(), Detail: eventDetail(c.Name, manaSpent, storm)})
					}
				}
				if resolved {
					again = true
					break
				}
				continue
			}
			if manaSpent == 0 && checkVexingBauble(g, ap, c, log) {
				again = true
				break
			}
			perm, err := castPermanentCard(g, ap, c)
			if err != nil || perm == nil {
				continue
			}
		perm.SetEnteredTurn(g.GetTurnNumber())
		resolvePermanentETB(g, perm, ap, log)
		storm := 0
		if metrics != nil {
			storm = metrics.recordSpell(idx, manaSpent, c.IsCreature(), c.Name)
		}
		if log != nil {
			kind := EventPermanentCast
			if c.IsCreature() {
				kind = EventCreatureSummon
			}
			log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: kind, Actor: ap.GetName(), Detail: eventDetail(c.Name, manaSpent, storm)})
		}
		again = true
		break
		}
	}

	activateSearchAbilities(g, ap, log)
	bridge.AutoActivateMainPhaseAbilitiesWithLog(g, func(cardName, detail string) {
		if log != nil {
			actor := ap.GetName()
			log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventFetchActivated, Actor: actor, Detail: cardName + " -> " + detail})
		}
	})
	attemptCEDHComboFinish(g, ap, log, metrics)
}

func isCastableSpell(c game.SimpleCard) bool {
	return !c.IsLand() && (c.IsCreature() || c.IsArtifact() || c.IsEnchantment() || c.IsPlaneswalker() || c.IsInstant() || c.IsSorcery())
}

func castPermanentCard(g *game.Game, ap *game.Player, c game.SimpleCard) (*game.Permanent, error) {
	if c.IsCreature() {
		return g.SummonCreature(ap, c.Name)
	}
	return g.CastPermanent(ap, c.Name)
}

func castNonPermanentSpell(g *game.Game, ap *game.Player, c game.SimpleCard, log *EDHEventLog, metrics *edhMetrics) bool {
	if c.OracleText == "" {
		return false
	}
	idx := -1
	for i, cc := range ap.Hand {
		if cc.Name == c.Name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	ap.Hand = append(ap.Hand[:idx], ap.Hand[idx+1:]...)
	gs := bridge.NewAbilityGameState(g)
	engine := abil.NewExecutionEngine(gs)
	abilities, err := engine.ParseAndRegisterAbilities(c.OracleText, c)
	if err != nil || len(abilities) == 0 {
		ap.Graveyard = append(ap.Graveyard, c)
		return true
	}
	playerAdapter := gs.GetPlayer(ap.GetName())
	if playerAdapter == nil {
		ap.Graveyard = append(ap.Graveyard, c)
		return true
	}
	for _, ab := range abilities {
		var targets []any
		for _, eff := range ab.Effects {
			for _, tgt := range eff.Targets {
				if tgt.Required {
					potentials := engine.GetPotentialTargets(tgt.Type, nil)
					if len(potentials) > 0 {
						targets = append(targets, potentials[0])
					}
				}
			}
		}
		_ = engine.ExecuteAbility(ab, playerAdapter, targets)
	}
	ap.Graveyard = append(ap.Graveyard, c)
	return true
}

func resolvePermanentETB(g *game.Game, perm *game.Permanent, ap *game.Player, log *EDHEventLog) {
	gs := bridge.NewAbilityGameState(g)
	engine := abil.NewExecutionEngine(gs)
	playerAdapter := gs.GetPlayer(ap.GetName())
	if playerAdapter == nil {
		return
	}
	src := perm.GetSource()
	if src.OracleText == "" {
		return
	}
	abilities, err := engine.ParseAndRegisterAbilities(src.OracleText, src)
	if err != nil || len(abilities) == 0 {
		return
	}
	for _, ab := range abilities {
		if ab.Type != abil.Triggered || ab.TriggerCondition != abil.EntersTheBattlefield {
			continue
		}
		var targets []any
		for _, eff := range ab.Effects {
			for _, tgt := range eff.Targets {
				if tgt.Required {
					potentials := engine.GetPotentialTargets(tgt.Type, nil)
					if len(potentials) > 0 {
						targets = append(targets, potentials[0])
					}
				}
			}
		}
		_ = engine.ExecuteAbility(ab, playerAdapter, targets)
	}
}

func eventDetail(name string, manaSpent, storm int) string {
	return name + " | mana=" + intString(manaSpent) + " storm=" + intString(storm)
}

//nolint:unused
func isValidSpellCost(c game.SimpleCard) bool {
	if c.IsLand() {
		return false
	}
	if cost := c.GetManaCost(); cost.Total() == 0 && c.ManaCost == "" {
		return false
	}
	return true
}

func activateSearchAbilities(g *game.Game, ap *game.Player, log *EDHEventLog) {
	gs := bridge.NewAbilityGameState(g)
	engine := abil.NewExecutionEngine(gs)
	playerAdapter := gs.GetPlayer(ap.GetName())
	if playerAdapter == nil {
		return
	}
	var foundCard string
	gs.OnSearchResult = func(name string) {
		foundCard = name
	}
	abilities := engine.GetActivatableAbilities(playerAdapter)
	for _, ability := range abilities {
		if ability.Type != abil.Activated {
			continue
		}
		hasSearch := false
		for _, effect := range ability.Effects {
			if effect.Type == abil.SearchLibrary {
				hasSearch = true
				break
			}
		}
		if !hasSearch || !ability.Cost.SacrificeCost {
			continue
		}
		foundCard = ""
		if err := engine.ExecuteAbility(ability, playerAdapter, nil); err != nil {
			continue
		}
		srcName := "unknown"
		if src, ok := ability.Source.(game.SimpleCard); ok {
			srcName = src.Name
		}
		detail := "searched"
		if foundCard != "" {
			detail = foundCard
		}
		if log != nil {
			log.Append(EDHEvent{
				Turn:   g.GetTurnNumber(),
				Phase:  phaseName(game.PhaseMain1),
				Kind:   EventFetchActivated,
				Actor:  ap.GetName(),
				Detail: srcName + " -> " + detail,
			})
		}
	}
}

func checkVexingBauble(g *game.Game, caster *game.Player, c game.SimpleCard, log *EDHEventLog) bool {
	for _, opp := range g.GetPlayersRaw() {
		if opp == caster || opp.HasLost() {
			continue
		}
		for _, perm := range opp.Battlefield {
			if strings.EqualFold(perm.GetName(), "Vexing Bauble") {
				if log != nil {
					log.Append(EDHEvent{
						Turn:   g.GetTurnNumber(),
						Phase:  phaseName(g.GetCurrentPhase()),
						Kind:   EventSpellCountered,
						Actor:  caster.GetName(),
						Target: opp.GetName(),
						Detail: c.Name + " (countered by Vexing Bauble)",
					})
				}
				return true
			}
		}
	}
	return false
}

func intString(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func tapManaSourcesForMainPhaseMana(g *game.Game, ap *game.Player, idx int, metrics *edhMetrics) {
	demand := aggregateMainPhaseManaDemand(ap)
	totalProduced := 0
	hasUrborg := permanentOnBattlefield(ap, "Urborg, Tomb of Yawgmoth")
	hasYavimaya := permanentOnBattlefield(ap, "Yavimaya, Cradle of Growth")
	for _, perm := range ap.Battlefield {
		if perm.IsTapped() {
			continue
		}
		produced := chooseManaProduction(perm.GetSource(), demand)
		if len(produced) == 0 {
			continue
		}
		if perm.IsCreature() && perm.GetEnteredTurn() == g.GetTurnNumber() && !perm.HasKeyword(game.KWHaste) {
			continue
		}
		if hasUrborg && perm.GetSource().IsLand() {
			hasBlack := false
			for mt := range produced {
				if mt == game.Black { hasBlack = true; break }
			}
			if !hasBlack { produced[game.Black] = produced[game.Black] + 1 }
		}
		if hasYavimaya && perm.GetSource().IsLand() {
			hasGreen := false
			for mt := range produced {
				if mt == game.Green { hasGreen = true; break }
			}
			if !hasGreen { produced[game.Green] = produced[game.Green] + 1 }
		}
		perm.Tap()
		for mt, n := range produced {
			ap.AddManaToPool(mt, n)
			totalProduced += n
		}
	}
	if metrics != nil && idx >= 0 {
		metrics.recordManaProduced(idx, totalProduced)
	}
}

func permanentOnBattlefield(ap *game.Player, name string) bool {
	for _, perm := range ap.Battlefield {
		if strings.EqualFold(perm.GetName(), name) {
			return true
		}
	}
	return false
}

func aggregateMainPhaseManaDemand(ap *game.Player) game.Mana {
	demand := game.Mana{}
	for _, c := range ap.CommandZone {
		for mt, n := range c.GetManaCost() {
			demand.Add(mt, n)
		}
		demand.Add(game.Any, ap.CommanderTax(c.Name))
	}
	for _, c := range ap.Hand {
		if !isCastableSpell(c) {
			continue
		}
		for mt, n := range c.GetManaCost() {
			demand.Add(mt, n)
		}
	}

	// Add mana for holding up counterspells if the player has any in hand
	counterspellHoldUpMana := calculateCounterspellHoldUpMana(ap)
	for mt, n := range counterspellHoldUpMana {
		demand.Add(mt, n)
	}

	return demand
}

// calculateCounterspellHoldUpMana determines mana to hold up for counterspells.
// Players with counterspells in hand should hold up mana to be able to counter
// opponent spells. The typical hold-up is the cost of the cheapest counterspell.
func calculateCounterspellHoldUpMana(ap *game.Player) game.Mana {
	holdUp := game.Mana{}
	
	// Find counterspells in hand
	minCounterspellCost := 999
	var cheapestCounterspell game.SimpleCard
	hasCounterspell := false
	
	for _, c := range ap.Hand {
		if c.IsCounterspell() {
			hasCounterspell = true
			cost := c.GetManaCost().Total()
			if cost < minCounterspellCost {
				minCounterspellCost = cost
				cheapestCounterspell = c
			}
		}
	}
	
	// If player has counterspells, hold up mana equal to the cheapest one
	if hasCounterspell && minCounterspellCost < 999 {
		for mt, n := range cheapestCounterspell.GetManaCost() {
			holdUp.Add(mt, n)
		}
	}
	
	return holdUp
}

func chooseManaProduction(c game.SimpleCard, demand game.Mana) game.Mana {
	options := manaProductionOptions(c)
	if len(options) == 0 {
		return nil
	}
	best := options[0]
	bestNeed := manaDemandScore(best, demand)
	for _, opt := range options[1:] {
		if score := manaDemandScore(opt, demand); score > bestNeed {
			best = opt
			bestNeed = score
		}
	}
	return best
}

func manaDemandScore(produced game.Mana, demand game.Mana) int {
	score := 0
	for mt, n := range produced {
		if demand[mt] > 0 {
			score += n * (demand[mt] + 1)
		} else if demand[game.Any] > 0 {
			score += n
		}
	}
	return score
}

func manaProductionOptions(c game.SimpleCard) []game.Mana {
	text := strings.ToLower(c.Name + " " + c.TypeLine + " " + c.OracleText)
	if !c.IsLand() && !strings.Contains(text, "{t}") && !strings.Contains(text, "tap") {
		return nil
	}
	var out []game.Mana
	add := func(mt game.ManaType, needles ...string) {
		for _, needle := range needles {
			if strings.Contains(text, strings.ToLower(needle)) {
				out = append(out, game.Mana{mt: 1})
				return
			}
		}
	}
	// Chrome Mox-style: "any of the exiled card's colors"
	if strings.Contains(text, "one mana of any color") || strings.Contains(text, "one mana of any type") ||
		strings.Contains(text, "any of the exiled card") {
		return []game.Mana{{game.White: 1}, {game.Blue: 1}, {game.Black: 1}, {game.Red: 1}, {game.Green: 1}}
	}
	// Bloom Tender-style: produce one mana of each color among permanents (approximate as all 5)
	if strings.Contains(text, "for each color among permanents you control") {
		return []game.Mana{{game.White: 1, game.Blue: 1, game.Black: 1, game.Red: 1, game.Green: 1}}
	}
	// Mox-style: "Mox" cards that produce any color
	name := strings.ToLower(c.Name)
	if strings.Contains(name, "mox") && !strings.Contains(name, "moxfield") {
		return []game.Mana{{game.White: 1}, {game.Blue: 1}, {game.Black: 1}, {game.Red: 1}, {game.Green: 1}}
	}
	if strings.Contains(text, "{c}{c}") {
		out = append(out, game.Mana{game.Colorless: 2})
	}
	add(game.White, "plains", "{w}")
	add(game.Blue, "island", "{u}")
	add(game.Black, "swamp", "{b}")
	add(game.Red, "mountain", "{r}")
	add(game.Green, "forest", "{g}")
	add(game.Colorless, "wastes", "{c}")
	return out
}

// runCombatPhase declares all eligible attackers against the most
// threatening living opponent (Phase 4) and resolves combat damage.
// Falls back to seat-order rotation when threat scores are tied.
func runCombatPhase(g *game.Game, ap *game.Player, log *EDHEventLog, metrics *edhMetrics) {
	defender := chooseAttackTarget(g, ap)
	if defender == nil {
		return
	}
	beforeLife := defender.GetLifeTotal()
	declared := 0
	for _, perm := range ap.GetCreatures() {
		if perm.IsTapped() {
			continue
		}
		if err := g.DeclareAttacker(perm, defender); err == nil {
			declared++
			if log != nil {
				log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseCombat), Kind: EventAttackDeclared, Actor: ap.GetName(), Target: defender.GetName(), Detail: perm.GetName()})
			}
		}
	}
	g.ResolveCombatDamage()
	damage := max(0, beforeLife-defender.GetLifeTotal())
	if metrics != nil {
		metrics.recordCombatDamage(indexOfPlayer(g, ap), damage)
	}
	if log != nil && declared > 0 {
		log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseCombat), Kind: EventCombatResolved, Actor: ap.GetName(), Target: defender.GetName(), Detail: "damage=" + intString(damage)})
	}
}

func indexOfPlayer(g *game.Game, p *game.Player) int {
	for i, q := range g.GetPlayersRaw() {
		if q == p {
			return i
		}
	}
	return -1
}

// markMillIfApplicable lets classifyElimination distinguish mill from
// life-loss. We tag the player by zeroing life and rely on a sentinel
// (empty library + lost) at finalize time.
func markMillIfApplicable(p *game.Player) {
	// no-op; classifyElimination uses Library size as the heuristic
	_ = p
}
