package simulation

import (
	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/internal/logger"
)

// StackAwareHandler implements PriorityHandler by running full priority
// rounds with AI decision-making. During each phase's priority window it
// gives all living non-active players an opportunity to activate
// instant-speed abilities or cast instants from hand.
//
// The handler is the concrete bridge between the ability engine (which
// knows how to parse oracle text and resolve effects) and the
// simulation loop (which knows the turn structure). Each priority round
// fires once per unique (turn, phase) pair.
type StackAwareHandler struct {
	g              *game.Game
	engine         *abil.ExecutionEngine
	ai             *abil.AIDecisionMaker
	gameState      *bridge.AbilityGameState
	processedRound bool
	lastTurn       int
	lastPhase      game.Phase
	log            *EDHEventLog
}

// NewStackAwareHandler creates a handler for the given game.
// log may be nil.
func NewStackAwareHandler(g *game.Game, logOpt *EDHEventLog) *StackAwareHandler {
	gs := bridge.NewAbilityGameState(g)
	engine := abil.NewExecutionEngine(gs)
	ai := abil.NewAIDecisionMaker(engine)
	return &StackAwareHandler{
		g:         g,
		engine:    engine,
		ai:        ai,
		gameState: gs,
		log:       logOpt,
	}
}

// OnOpponentPriority implements PriorityHandler. On the first call per
// (turn, phase) it runs the full priority round for all opponents.
// Subsequent calls for the same turn+phase are no-ops.
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
	h.processPriorityForPhase(active, phase)
}

// processPriorityForPhase iterates over all living non-active players in
// APNAP order and gives each one a chance to act.
func (h *StackAwareHandler) processPriorityForPhase(active *game.Player, phase game.Phase) {
	players := h.g.GetPlayersRaw()
	n := len(players)
	start := indexOfPlayer(h.g, active)
	if start < 0 {
		return
	}
	pLabel := phaseLabel(phase)

	for i := 1; i < n; i++ {
		opp := players[(start+i)%n]
		if opp == active || opp.HasLost() {
			continue
		}
		h.offerPlayer(opp, pLabel)
	}
}

// offerPlayer gives a single opponent the chance to act:
//  1. Activate instant-speed abilities via the AI decision-maker
//  2. Cast one instant from hand
func (h *StackAwareHandler) offerPlayer(opp *game.Player, phaseLabel string) {
	ap := h.gameState.GetPlayer(opp.GetName())
	if ap == nil {
		return
	}

	opponents := h.getOpponents(ap)
	context := h.ai.BuildDecisionContext(ap, opponents, phaseLabel)

	h.activateInstantAbilities(ap, context)
	h.castPriorityInstants(opp, ap)
}

// activateInstantAbilities uses the AI to find and activate abilities
// that can be activated at instant speed (AnyTime timing).
func (h *StackAwareHandler) activateInstantAbilities(ap abil.AbilityPlayer, context abil.DecisionContext) {
	if !h.ai.ShouldActivateAbilities(context) {
		return
	}
	abilities := h.engine.GetActivatableAbilities(ap)
	if len(abilities) == 0 {
		return
	}
	chosen := h.ai.ChooseAbilitiesToActivate(abilities, context)
	for _, ab := range chosen {
		targets := h.ai.ChooseTargetsFor(ab, context)
		if err := h.engine.ExecuteAbility(ab, ap, targets); err == nil {
			logger.LogPlayer("%s activates %s during priority", ap.GetName(), ab.Name)
		}
	}
}

// castPriorityInstants scans the opponent's hand for instant cards the AI
// should cast during this priority window. Only the first castable
// instant is cast per window to keep the simulation decisive.
func (h *StackAwareHandler) castPriorityInstants(opp *game.Player, ap abil.AbilityPlayer) {
	for _, card := range opp.Hand {
		if !card.IsInstant() || card.OracleText == "" {
			continue
		}
		if !opp.CanPayForCard(card) {
			continue
		}
		abilities, err := h.engine.ParseAndRegisterAbilities(card.OracleText, card)
		if err != nil || len(abilities) == 0 {
			continue
		}
		if !opp.PayForCard(card) {
			continue
		}

		for _, ab := range abilities {
			_ = h.engine.ExecuteAbility(ab, ap, nil)
		}

		h.removeFromHand(opp, card)
		opp.Graveyard = append(opp.Graveyard, card)

		logger.LogCard("%s casts %s at instant speed during priority",
			opp.GetName(), card.Name)
		if h.log != nil {
			h.log.Append(EDHEvent{
				Turn:   h.g.GetTurnNumber(),
				Phase:  phaseName(h.g.GetCurrentPhase()),
				Kind:   EventPermanentCast,
				Actor:  opp.GetName(),
				Detail: card.Name + " (instant)",
			})
		}
		return
	}
}

func (h *StackAwareHandler) removeFromHand(p *game.Player, card game.SimpleCard) {
	for i, c := range p.Hand {
		if c.Name == card.Name {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			return
		}
	}
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
