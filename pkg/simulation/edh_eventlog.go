package simulation

import (
	"sync"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// EDHEventKind tags an entry in the per-pod event log so consumers can
// filter for replay or post-game analysis.
type EDHEventKind string

const (
	EventGameStart       EDHEventKind = "game_start"
	EventTurnStart       EDHEventKind = "turn_start"
	EventLandPlay        EDHEventKind = "land_play"
	EventCreatureSummon  EDHEventKind = "creature_summon"
	EventCommanderCast   EDHEventKind = "commander_cast"
	EventAttackDeclared  EDHEventKind = "attack_declared"
	EventCombatResolved  EDHEventKind = "combat_resolved"
	EventPlayerEliminated EDHEventKind = "player_eliminated"
	EventGameEnd         EDHEventKind = "game_end"
)

// EDHEvent is a single structured entry in a pod's event log. Designed
// to be JSON-friendly so dashboards / replays can ingest it directly.
type EDHEvent struct {
	Turn   int          `json:"turn"`
	Phase  string       `json:"phase"`
	Kind   EDHEventKind `json:"kind"`
	Actor  string       `json:"actor,omitempty"`
	Target string       `json:"target,omitempty"`
	Detail string       `json:"detail,omitempty"`
}

// EDHEventLog is a thread-safe append-only log scoped to a single pod.
// Workers each get their own log; aggregation happens by attaching the
// log to an EDHGameRecord at finalize time.
type EDHEventLog struct {
	mu     sync.Mutex
	events []EDHEvent
}

// NewEDHEventLog allocates an empty log.
func NewEDHEventLog() *EDHEventLog { return &EDHEventLog{} }

// Append appends an event. Safe for concurrent use though the runner
// today is single-threaded per pod.
func (l *EDHEventLog) Append(e EDHEvent) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

// Events returns a snapshot copy of the log.
func (l *EDHEventLog) Events() []EDHEvent {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]EDHEvent, len(l.events))
	copy(out, l.events)
	return out
}

// PriorityHandler is the extension hook the runner calls before each
// step where opponents would normally receive priority (CR 117). The
// default implementation is a no-op so the runner remains
// sorcery-speed; advanced AI can register a handler that casts instants
// or activates abilities here.
type PriorityHandler interface {
	OnOpponentPriority(g *game.Game, active *game.Player, opp *game.Player, phase game.Phase)
}

// NoopPriorityHandler is the default. It does nothing so games run at
// the existing speed. Provided so the runner can always invoke its
// hook unconditionally.
type NoopPriorityHandler struct{}

// OnOpponentPriority is a no-op.
func (NoopPriorityHandler) OnOpponentPriority(g *game.Game, active *game.Player, opp *game.Player, phase game.Phase) {
}

// phaseName converts a Phase enum into a JSON-friendly label.
func phaseName(p game.Phase) string {
	switch p {
	case game.PhaseUntap:
		return "untap"
	case game.PhaseUpkeep:
		return "upkeep"
	case game.PhaseDraw:
		return "draw"
	case game.PhaseMain1:
		return "main1"
	case game.PhaseCombat:
		return "combat"
	case game.PhaseMain2:
		return "main2"
	case game.PhaseEnd:
		return "end"
	default:
		return "unknown"
	}
}
