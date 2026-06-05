package simulation

import (
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

	// Spell resolved — create permanent on battlefield and fire ETB
	perm, err := castPermanentCard(h.g, ap, c)
	if err != nil || perm == nil {
		ap.Graveyard = append(ap.Graveyard, c)
		return false
	}
	perm.SetEnteredTurn(h.g.GetTurnNumber())
	resolvePermanentETB(h.g, perm, ap, h.log)

	return true
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

// decideInstantSpell checks the player's hand for castable instants,
// pays their mana cost immediately (as CR 601.2h requires), and returns
// a CastSpell decision. Returns nil if no instant is worth casting.
func (h *StackAwareHandler) decideInstantSpell(player abil.AbilityPlayer, context abil.DecisionContext) *abil.PriorityDecision {
	for _, raw := range player.GetHand() {
		card, ok := raw.(game.SimpleCard)
		if !ok || !card.IsInstant() || card.OracleText == "" {
			continue
		}
		var gp *game.Player
		for _, p := range h.g.GetPlayersRaw() {
			if p.GetName() == player.GetName() && !p.HasLost() {
				gp = p
				break
			}
		}
		if gp == nil || !gp.CanPayForCard(card) {
			continue
		}
		if !gp.PayForCard(card) {
			continue
		}

		abilities, err := h.engine.ParseAndRegisterAbilities(card.OracleText, card)
		if err != nil || len(abilities) == 0 {
			continue
		}

		var effects []abil.Effect
		for _, ab := range abilities {
			effects = append(effects, ab.Effects...)
		}

		spell := &abil.Spell{
			Name:       card.Name,
			ManaCost:   card.ManaCost,
			CMC:        int(card.GetMinManaCost().Total()),
			TypeLine:   card.TypeLine,
			OracleText: card.OracleText,
			Effects:    effects,
			Source:     card,
		}

		logger.LogPlayer("%s casts %s at instant speed", player.GetName(), card.Name)
		return &abil.PriorityDecision{
			Action: abil.PriorityActionCastSpell,
			Spell:  spell,
			Player: player,
		}
	}
	return nil
}

// getGamePlayer finds the game.Player by name, skipping eliminated players.
func (h *StackAwareHandler) getGamePlayer(name string) *game.Player {
	for _, p := range h.g.GetPlayersRaw() {
		if p.GetName() == name && !p.HasLost() {
			return p
		}
	}
	return nil
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
