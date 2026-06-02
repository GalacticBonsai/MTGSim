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

// aiDecision is called by the PriorityManager for each player when they
// have priority. It uses the AIDecisionMaker to decide whether to
// activate abilities or cast instants.
func (h *StackAwareHandler) aiDecision(player abil.AbilityPlayer) *abil.PriorityDecision {
	// Build decision context for this player
	opponents := h.getOpponents(player)
	context := h.ai.BuildDecisionContext(player, opponents, h.spellCasting.GetPriorityManager().GetPhase())

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

// decideInstantSpell checks the player's hand for castable instants,
// pays their mana cost immediately (as CR 601.2h requires), and returns
// a CastSpell decision. Returns nil if no instant is worth casting.
func (h *StackAwareHandler) decideInstantSpell(player abil.AbilityPlayer, context abil.DecisionContext) *abil.PriorityDecision {
	for _, raw := range player.GetHand() {
		card, ok := raw.(game.SimpleCard)
		if !ok || !card.IsInstant() || card.OracleText == "" {
			continue
		}
		// Find the underlying game.Player to check and pay costs. The
		// ability package's PayCost expects an ability.Cost, not a
		// SimpleCard, so we go directly to game.Player.PayForCard.
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


