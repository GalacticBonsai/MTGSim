package simulation

import (
	"sort"
	"strings"

	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/internal/logger"
)

// StackAwareHandler implements PriorityHandler by running full APNAP
// priority rounds backed by the ability package's Stack and
// PriorityManager. During each priority window players get a real
// opportunity to cast instants or activate abilities; these actions go
// through the stack so that other players can respond before they
// resolve.
//
// Each priority round continues until all players pass with an empty
// stack (CR 117), matching the real MTG turn structure within the
// simulation's phase loop.
type StackAwareHandler struct {
	g              *game.Game
	engine         *abil.ExecutionEngine
	spellCasting   *abil.SpellCastingEngine
	ai             *abil.AIDecisionMaker
	gameState      *bridge.AbilityGameState
	activePlayer   abil.AbilityPlayer
	processedRound bool
	lastTurn       int
	lastPhase      game.Phase
	log            *EDHEventLog
}

// NewStackAwareHandler creates a handler backed by a real Stack and
// PriorityManager. log may be nil.
func NewStackAwareHandler(g *game.Game, logOpt *EDHEventLog) *StackAwareHandler {
	gs := bridge.NewAbilityGameState(g)
	engine := abil.NewExecutionEngine(gs)
	spellCasting := abil.NewSpellCastingEngine(gs, engine)

	players := make([]abil.AbilityPlayer, 0, len(g.GetPlayersRaw()))
	for _, p := range g.GetPlayersRaw() {
		if ap := gs.GetPlayer(p.GetName()); ap != nil {
			players = append(players, ap)
		}
	}
	spellCasting.SetPlayers(players)

	ai := abil.NewAIDecisionMaker(engine)

	h := &StackAwareHandler{
		g:            g,
		engine:       engine,
		spellCasting: spellCasting,
		ai:           ai,
		gameState:    gs,
		log:          logOpt,
	}

	spellCasting.GetPriorityManager().DecisionFunc = h.aiDecision

	// Wire stack callbacks: log resolution events and apply state-based actions.
	spellCasting.GetStack().OnResolve = func(item *abil.StackItem) {
		if h.log == nil {
			return
		}
		if item.Type == abil.StackItemSpell && item.Spell != nil {
			if item.Countered {
				h.log.Append(EDHEvent{
					Turn:   h.g.GetTurnNumber(),
					Phase:  phaseLabel(h.g.GetCurrentPhase()),
					Kind:   EventSpellCountered,
					Actor:  item.Controller.GetName(),
					Detail: item.Spell.Name,
				})
			} else {
				h.log.Append(EDHEvent{
					Turn:   h.g.GetTurnNumber(),
					Phase:  phaseLabel(h.g.GetCurrentPhase()),
					Kind:   EventSpellResolved,
					Actor:  item.Controller.GetName(),
					Detail: item.Spell.Name,
				})
			}
		}
	}
	spellCasting.GetStack().OnAfterResolve = func() {
		h.g.ApplyStateBasedActions()
	}

	return h
}

// OnOpponentPriority implements PriorityHandler. On the first call per
// (turn, phase) it runs a full ProcessPriorityRound. Subsequent calls
// for the same turn+phase are no-ops.
func (h *StackAwareHandler) OnOpponentPriority(g *game.Game, active *game.Player, opp *game.Player, phase game.Phase) {
	if g.GetTurnNumber() != h.lastTurn || phase != h.lastPhase {
		h.processedRound = false
		h.lastTurn = g.GetTurnNumber()
		h.lastPhase = phase
	}
	if h.processedRound {
		return
	}
	h.processedRound = true

	ap := h.gameState.GetPlayer(active.GetName())
	if ap == nil {
		return
	}
	h.activePlayer = ap
	h.spellCasting.SetActivePlayer(ap)
	h.spellCasting.SetPhase(phaseLabel(phase))

	logger.LogCard("Starting priority round for %s in %s", active.GetName(), phaseLabel(phase))
	if err := h.spellCasting.ProcessPriority(); err != nil {
		logger.LogCard("Priority round error: %v", err)
	}
}

// CastSpellThroughStack routes a spell through the stack during the main
// phase. It removes the card from hand, puts it on the stack, runs
// ProcessPriority to allow opponents to respond, resolves the stack, and
// moves the card to the graveyard. The caller MUST have already paid
// mana for the card.
func (h *StackAwareHandler) CastSpellThroughStack(ap *game.Player, c game.SimpleCard, casterName string) bool {
	gs := h.gameState
	playerAdapter := gs.GetPlayer(casterName)
	if playerAdapter == nil {
		return false
	}

	// Remove card from hand
	cidx := -1
	for i, cc := range ap.Hand {
		if cc.Name == c.Name {
			cidx = i
			break
		}
	}
	if cidx < 0 {
		return false
	}
	ap.Hand = append(ap.Hand[:cidx], ap.Hand[cidx+1:]...)

	abilities, err := h.engine.ParseAndRegisterAbilities(c.OracleText, c)
	if err != nil || len(abilities) == 0 {
		ap.Graveyard = append(ap.Graveyard, c)
		return true
	}

	var effects []abil.Effect
	for _, ab := range abilities {
		effects = append(effects, ab.Effects...)
	}

	spell := &abil.Spell{
		Name:       c.Name,
		ManaCost:   c.ManaCost,
		CMC:        int(c.GetMinManaCost().Total()),
		TypeLine:   c.TypeLine,
		OracleText: c.OracleText,
		Effects:    effects,
		Source:     c,
	}

	// Set up priority for main phase
	h.spellCasting.SetActivePlayer(playerAdapter)
	h.spellCasting.SetPhase("Main Phase")

	pm := h.spellCasting.GetPriorityManager()

	// Cast the spell (puts on stack via priority manager, resets priority)
	if err := pm.CastSpell(playerAdapter, spell, nil); err != nil {
		logger.LogPlayer("Failed to cast %s through stack: %v", c.Name, err)
		ap.Graveyard = append(ap.Graveyard, c)
		return false
	}

	logger.LogPlayer("%s casts %s through stack", ap.GetName(), c.Name)

	// Run full priority round — opponents can respond, then stack resolves
	if err := h.spellCasting.ProcessPriority(); err != nil {
		logger.LogCard("Priority round error for %s: %v", c.Name, err)
	}

	ap.Graveyard = append(ap.Graveyard, c)
	return true
}

// CastPermanentThroughStack routes a permanent spell through the stack.
// It removes the card from hand, puts it on the stack as a spell, runs
// priority rounds (opponents can respond/counter), and on successful
// resolution creates the permanent on the battlefield and fires ETB
// triggers. Returns true if the permanent resolved and entered the
// battlefield.
func (h *StackAwareHandler) CastPermanentThroughStack(ap *game.Player, c game.SimpleCard, casterName string) bool {
	gs := h.gameState
	playerAdapter := gs.GetPlayer(casterName)
	if playerAdapter == nil {
		return false
	}

	// Remove card from hand
	cidx := -1
	for i, cc := range ap.Hand {
		if cc.Name == c.Name {
			cidx = i
			break
		}
	}
	if cidx < 0 {
		return false
	}
	ap.Hand = append(ap.Hand[:cidx], ap.Hand[cidx+1:]...)

	abilities, err := h.engine.ParseAndRegisterAbilities(c.OracleText, c)
	if err != nil {
		ap.Graveyard = append(ap.Graveyard, c)
		return false
	}

	var effects []abil.Effect
	isPermanent := c.IsCreature() || c.IsArtifact() || c.IsEnchantment() || c.IsPlaneswalker()
	for _, ab := range abilities {
		if isPermanent && (ab.Type == abil.Triggered || ab.Type == abil.Activated || ab.Type == abil.Mana) {
			continue
		}
		effects = append(effects, ab.Effects...)
	}

	spell := &abil.Spell{
		Name:       c.Name,
		ManaCost:   c.ManaCost,
		CMC:        int(c.GetMinManaCost().Total()),
		TypeLine:   c.TypeLine,
		OracleText: c.OracleText,
		Effects:    effects,
		Source:     c,
	}

	// Set up priority for main phase
	h.spellCasting.SetActivePlayer(playerAdapter)
	h.spellCasting.SetPhase("Main Phase")

	pm := h.spellCasting.GetPriorityManager()

	// Cast the spell (puts on stack via priority manager)
	if err := pm.CastSpell(playerAdapter, spell, nil); err != nil {
		logger.LogPlayer("Failed to cast %s through stack: %v", c.Name, err)
		ap.Graveyard = append(ap.Graveyard, c)
		return false
	}

	logger.LogPlayer("%s casts %s through stack", ap.GetName(), c.Name)

	// Save the StackItem reference so we can check if it was countered after priority resolves.
	candidate := h.spellCasting.GetStack().LastCastItem()

	// Run full priority round — opponents can respond, then stack resolves
	if err := h.spellCasting.ProcessPriority(); err != nil {
		logger.LogCard("Priority round error for %s: %v", c.Name, err)
	}

	// If the spell was countered, the card goes to graveyard.
	if candidate != nil && candidate.Countered {
		ap.Graveyard = append(ap.Graveyard, c)
		logger.LogPlayer("%s was countered — permanent not created", c.Name)
		return false
	}

	// Spell resolved — create permanent on battlefield
	perm, err := castPermanentCard(h.g, ap, c)
	if err != nil || perm == nil {
		ap.Graveyard = append(ap.Graveyard, c)
		return false
	}
	perm.SetEnteredTurn(h.g.GetTurnNumber())

	// Route ETB triggers through the stack instead of firing directly
	h.processETBTriggers(ap, perm, c)

	return true
}

// processETBTriggers puts ETB triggered abilities for a resolved permanent
// onto the ability stack instead of executing them directly (CR 603.3).
// It runs a priority round so opponents can respond to each trigger.
func (h *StackAwareHandler) processETBTriggers(ap *game.Player, perm *game.Permanent, c game.SimpleCard) {
	gs := bridge.NewAbilityGameState(h.g)
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
	stack := h.spellCasting.GetStack()
	hasTriggers := false
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
		// Put the triggered ability on the stack
		stack.AddAbility(ab, playerAdapter, targets)
		hasTriggers = true
		if h.log != nil {
			targetStr := ""
			for _, t := range targets {
				if s, ok := t.(game.SimpleCard); ok {
					targetStr = s.Name
					break
				}
			}
			h.log.Append(EDHEvent{
				Turn:   h.g.GetTurnNumber(),
				Phase:  phaseLabel(h.g.GetCurrentPhase()),
				Kind:   EventTriggerResolved,
				Actor:  ap.GetName(),
				Detail: src.Name + " ETB: " + ab.Name,
				Target: targetStr,
			})
		}
	}
	if hasTriggers {
		// Run a priority round so opponents can respond to triggers
		h.spellCasting.SetActivePlayer(playerAdapter)
		h.spellCasting.SetPhase("Main Phase")
		if err := h.spellCasting.ProcessPriority(); err != nil {
			logger.LogCard("Priority round error during ETB triggers for %s: %v", c.Name, err)
		}
	}
}

// phaseLabel converts a game.Phase to the string label used by the
// ability engine's priority-manager and AI systems.
func phaseLabel(p game.Phase) string {
	switch p {
	case game.PhaseUpkeep:
		return "Upkeep"
	case game.PhaseDraw:
		return "Draw"
	case game.PhaseMain1:
		return "Main Phase"
	case game.PhaseCombat:
		return "Combat Phase"
	case game.PhaseMain2:
		return "Main Phase"
	case game.PhaseEnd:
		return "End Step"
	default:
		return "Main Phase"
	}
}

// aiDecision is called by the PriorityManager for each player when they
// have priority. It uses the AIDecisionMaker to decide whether to
// activate abilities or cast instants — now stack-aware.
func (h *StackAwareHandler) aiDecision(player abil.AbilityPlayer) *abil.PriorityDecision {
	opponents := h.getOpponents(player)
	context := h.ai.BuildDecisionContext(player, opponents, h.spellCasting.GetPriorityManager().GetPhase())

	// If there's an opponent's spell on the stack, consider countering it
	stack := h.spellCasting.GetStack()
	if top := stack.Peek(); top != nil && top.Type == abil.StackItemSpell && top.Controller.GetName() != player.GetName() {
		spell := top.Spell
		if counterDecision := h.tryCounterSpell(player, spell.Name, spell.CMC); counterDecision != nil {
			return counterDecision
		}
	}

	if h.ai.ShouldActivateAbilities(context) {
		abilities := h.engine.GetActivatableAbilities(player)
		chosen := h.ai.ChooseAbilitiesToActivate(abilities, context)
		if len(chosen) > 0 {
			logger.LogPlayer("%s activates %s during priority", player.GetName(), chosen[0].Name)
			targets := h.ai.ChooseTargetsFor(chosen[0], context)
			return &abil.PriorityDecision{
				Action:  abil.PriorityActionActivateAbility,
				Ability: chosen[0],
				Targets: targets,
				Player:  player,
			}
		}
	}

	decision := h.decideInstantSpell(player, context)
	if decision != nil {
		return decision
	}

	return &abil.PriorityDecision{
		Action: abil.PriorityActionPass,
		Player: player,
	}
}

// tryCounterSpell checks whether the player can counter the given
// opponent spell. Returns a CastSpell decision if a suitable counterspell
// is found, or nil.
func (h *StackAwareHandler) tryCounterSpell(player abil.AbilityPlayer, opponentSpellName string, opponentSpellCMC int) *abil.PriorityDecision {
	// Find the underlying game.Player for mana checking
	var gp *game.Player
	for _, p := range h.g.GetPlayersRaw() {
		if p.GetName() == player.GetName() && !p.HasLost() {
			gp = p
			break
		}
	}
	if gp == nil {
		return nil
	}

	strategy := NewCounterspellStrategy(gp)
	shouldCounter, counter := strategy.ShouldCounterSpell(opponentSpellName, opponentSpellCMC)
	if !shouldCounter {
		return nil
	}

	// Pay mana for the counterspell
	if !gp.PayForCard(counter) {
		return nil
	}

	abilities, err := h.engine.ParseAndRegisterAbilities(counter.OracleText, counter)
	if err != nil || len(abilities) == 0 {
		return nil
	}

	var effects []abil.Effect
	for _, ab := range abilities {
		effects = append(effects, ab.Effects...)
	}

	spell := &abil.Spell{
		Name:       counter.Name,
		ManaCost:   counter.ManaCost,
		CMC:        int(counter.GetMinManaCost().Total()),
		TypeLine:   counter.TypeLine,
		OracleText: counter.OracleText,
		Effects:    effects,
		Source:     counter,
	}

	logger.LogPlayer("%s counters %s with %s", player.GetName(), opponentSpellName, counter.Name)
	return &abil.PriorityDecision{
		Action: abil.PriorityActionCastSpell,
		Spell:  spell,
		Player: player,
	}
}

// instantScore returns a priority score for casting an instant in the
// current context. Higher values are better. The scorer considers:
//   - The top spell on the stack (opponent's threat we could respond to)
//   - Board state (opponent creatures, our life total)
//   - Hand size (low hand size → value draw spells)
func instantScore(card game.SimpleCard, gp *game.Player, stackTop *abil.StackItem, opponentHasCreatures bool) int {
	lower := strings.ToLower(card.TypeLine + " " + card.OracleText)
	score := 1

	// Counterspell: highest priority when there's an opponent spell on the stack
	if card.IsCounterspell() {
		if stackTop != nil && stackTop.Type == abil.StackItemSpell {
			return 100
		}
		return 5
	}

	// Removal (destroy, exile, damage): valuable when opponents have creatures
	if strings.Contains(lower, "destroy") || strings.Contains(lower, "exile") || strings.Contains(lower, "damage") {
		if opponentHasCreatures {
			score += 10
		}
	}

	// Protection/hexproof/indestructible: valuable when our creature is threatened
	if strings.Contains(lower, "hexproof") || strings.Contains(lower, "indestructible") || strings.Contains(lower, "protection") {
		score += 3
	}

	// Card draw: more valuable with fewer cards in hand
	if strings.Contains(lower, "draw") {
		if len(gp.Hand) <= 2 {
			score += 8
		} else {
			score += 2
		}
	}

	// Life gain: more valuable at low life
	if strings.Contains(lower, "gain") && strings.Contains(lower, "life") {
		if gp.GetLifeTotal() <= 10 {
			score += 6
		}
	}

	// Bounce: valuable when opponent has expensive permanents
	if strings.Contains(lower, "return") && strings.Contains(lower, "hand") {
		score += 4
	}

	return score
}

// decideInstantSpell checks the player's hand for castable instants,
// evaluates them with a context-aware scorer, and casts the best option.
// pays their mana cost immediately (as CR 601.2h requires), and returns
// a CastSpell decision. Returns nil if no instant is worth casting.
func (h *StackAwareHandler) decideInstantSpell(player abil.AbilityPlayer, context abil.DecisionContext) *abil.PriorityDecision {
	stackTop := h.spellCasting.GetStack().Peek()

	var gp *game.Player
	for _, p := range h.g.GetPlayersRaw() {
		if p.GetName() == player.GetName() && !p.HasLost() {
			gp = p
			break
		}
	}
	if gp == nil {
		return nil
	}

	opponentHasCreatures := false
	for _, p := range h.g.GetPlayersRaw() {
		if p.GetName() != player.GetName() && !p.HasLost() && len(p.GetCreatures()) > 0 {
			opponentHasCreatures = true
			break
		}
	}

	type scoredCard struct {
		card  game.SimpleCard
		score int
	}

	var candidates []scoredCard
	for _, raw := range player.GetHand() {
		card, ok := raw.(game.SimpleCard)
		if !ok || !card.IsInstant() || card.OracleText == "" {
			continue
		}
		if !gp.CanPayForCard(card) {
			continue
		}
		score := instantScore(card, gp, stackTop, opponentHasCreatures)
		candidates = append(candidates, scoredCard{card: card, score: score})
	}

	if len(candidates) == 0 {
		return nil
	}

	// Pick the highest-scoring instant
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	best := candidates[0]

	if !gp.PayForCard(best.card) {
		return nil
	}

	abilities, err := h.engine.ParseAndRegisterAbilities(best.card.OracleText, best.card)
	if err != nil || len(abilities) == 0 {
		return nil
	}

	var effects []abil.Effect
	for _, ab := range abilities {
		effects = append(effects, ab.Effects...)
	}

	spell := &abil.Spell{
		Name:       best.card.Name,
		ManaCost:   best.card.ManaCost,
		CMC:        int(best.card.GetMinManaCost().Total()),
		TypeLine:   best.card.TypeLine,
		OracleText: best.card.OracleText,
		Effects:    effects,
		Source:     best.card,
	}

	logger.LogPlayer("%s casts %s at instant speed (score=%d)", player.GetName(), best.card.Name, best.score)
	return &abil.PriorityDecision{
		Action: abil.PriorityActionCastSpell,
		Spell:  spell,
		Player: player,
	}
}

// CastCommanderThroughStack routes a commander cast from the command zone
// through the stack so opponents can respond/counter. Returns the permanent
// on successful resolution, or nil if countered/failed.
func (h *StackAwareHandler) CastCommanderThroughStack(ap *game.Player, c game.SimpleCard, casterName string) *game.Permanent {
	gs := h.gameState
	playerAdapter := gs.GetPlayer(casterName)
	if playerAdapter == nil {
		return nil
	}

	abilities, err := h.engine.ParseAndRegisterAbilities(c.OracleText, c)
	if err != nil {
		return nil
	}

	var effects []abil.Effect
	isPermanent := c.IsCreature() || c.IsArtifact() || c.IsEnchantment() || c.IsPlaneswalker()
	for _, ab := range abilities {
		if isPermanent && (ab.Type == abil.Triggered || ab.Type == abil.Activated || ab.Type == abil.Mana) {
			continue
		}
		effects = append(effects, ab.Effects...)
	}

	spell := &abil.Spell{
		Name:       c.Name,
		ManaCost:   c.ManaCost,
		CMC:        int(c.GetMinManaCost().Total()),
		TypeLine:   c.TypeLine,
		OracleText: c.OracleText,
		Effects:    effects,
		Source:     c,
	}

	h.spellCasting.SetActivePlayer(playerAdapter)
	h.spellCasting.SetPhase("Main Phase")

	pm := h.spellCasting.GetPriorityManager()

	if err := pm.CastSpell(playerAdapter, spell, nil); err != nil {
		logger.LogPlayer("Failed to cast commander %s through stack: %v", c.Name, err)
		return nil
	}

	logger.LogPlayer("%s casts commander %s through stack", ap.GetName(), c.Name)

	candidate := h.spellCasting.GetStack().LastCastItem()

	if err := h.spellCasting.ProcessPriority(); err != nil {
		logger.LogCard("Priority round error for commander %s: %v", c.Name, err)
	}

	if candidate != nil && candidate.Countered {
		ap.Graveyard = append(ap.Graveyard, c)
		logger.LogPlayer("Commander %s was countered", c.Name)
		return nil
	}

	perm := ap.CastCommander(c.Name)
	if perm == nil {
		return nil
	}
	perm.SetEnteredTurn(h.g.GetTurnNumber())

	h.processETBTriggers(ap, perm, c)
	return perm
}

// ProcessPendingGameTriggers fires any triggers queued by the game engine.
// Triggers were already collected in APNAP order by handleTriggers. When a
// StackAwareHandler is active, each trigger is logged as going through the
// stack and the Action callback is invoked at the controlled point in the
// turn loop (after SBA). A priority round is offered so opponents can
// respond before the next phase action.
func (h *StackAwareHandler) ProcessPendingGameTriggers() {
	if !h.g.HasPendingTriggers() {
		return
	}
	pending := h.g.DrainPendingTriggers()

	for _, pt := range pending {
		if pt.Trigger == nil || pt.Trigger.Action == nil {
			continue
		}
		actor := ""
		if pt.Trigger.Controller != nil {
			actor = pt.Trigger.Controller.GetName()
		}
		if h.log != nil {
			h.log.Append(EDHEvent{
				Turn:   h.g.GetTurnNumber(),
				Phase:  phaseLabel(h.g.GetCurrentPhase()),
				Kind:   EventTriggerResolved,
				Actor:  actor,
				Detail: "trigger",
			})
		}
		pt.Trigger.Action(h.g, pt.Event)
	}

	// Run a priority round so opponents can respond
	if h.activePlayer != nil {
		h.spellCasting.SetActivePlayer(h.activePlayer)
		if err := h.spellCasting.ProcessPriority(); err != nil {
			logger.LogCard("Priority round error after trigger resolution: %v", err)
		}
	}
	h.g.ApplyStateBasedActions()
}

func (h *StackAwareHandler) getOpponents(player abil.AbilityPlayer) []abil.AbilityPlayer {
	var opponents []abil.AbilityPlayer
	for _, p := range h.g.GetPlayersRaw() {
		if !p.HasLost() && p.GetName() != player.GetName() {
			if ap := h.gameState.GetPlayer(p.GetName()); ap != nil {
				opponents = append(opponents, ap)
			}
		}
	}
	return opponents
}
