package game

import (
	"sort"

	"github.com/google/uuid"
)

// SubStep represents a sub-step within a phase (CR 500).
type SubStep int

const (
	// Untap step sub-steps
	StepUntapNone SubStep = iota
	// Upkeep step sub-steps
	StepUpkeepNone
	// Draw step sub-steps
	StepDrawNone
	// Main phase (no sub-steps)
	StepMainNone
	// Combat phase sub-steps (CR 507-510)
	StepBeginCombat
	StepDeclareAttackers
	StepDeclareBlockers
	StepCombatDamage
	StepEndOfCombat
	// End step sub-steps
	StepEndStep
	// Cleanup step sub-steps (CR 514)
	StepCleanupDiscard
	StepCleanupClear
)

// Trigger is a simple event-based action.
//
// Controller (optional) is the player whose triggered ability this is.
// When multiple triggers fire from the same event in a multiplayer game,
// CR 603.3b requires they be put on the stack in APNAP order starting
// with the active player's controller. Triggers with a nil Controller
// (game-wide system effects) are processed after all controlled
// triggers in their original registration order.
type Trigger struct {
	On         EventType
	Controller *Player
	Condition  func(Event) bool
	Action     func(g *Game, e Event)
}

func (g *Game) AddTrigger(t *Trigger) { g.triggers = append(g.triggers, t) }
func (g *Game) ClearTriggers()        { g.triggers = nil }

// PendingTrigger records a triggered action and the event that caused it
// for later resolution via the ability stack (CR 603.3).
type PendingTrigger struct {
	Trigger *Trigger
	Event   Event
}

// handleTriggers collects all triggers matching the given event into a pending
// queue sorted in APNAP order (CR 603.3b). It does NOT fire them immediately —
// callers must drain the queue via DrainPendingTriggers and resolve through
// the ability stack with a priority round.
func (g *Game) handleTriggers(e Event) {
	type pending struct {
		idx     int // registration order — used as a stable tiebreak
		apnap   int // 0 = active player, 1..n-1 = NAP rotating, n = nil controller
		trigger *Trigger
	}
	matches := make([]pending, 0, len(g.triggers))
	for i, t := range g.triggers {
		if t == nil || t.On != e.Type {
			continue
		}
		if t.Condition != nil && !t.Condition(e) {
			continue
		}
		matches = append(matches, pending{idx: i, apnap: g.apnapPosition(t.Controller), trigger: t})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].apnap != matches[j].apnap {
			return matches[i].apnap < matches[j].apnap
		}
		return matches[i].idx < matches[j].idx
	})
	for _, m := range matches {
		g.pendingTriggers = append(g.pendingTriggers, PendingTrigger{Trigger: m.trigger, Event: e})
	}
}

// DrainPendingTriggers returns all queued triggers and clears the queue.
// Callers should process these through the ability stack with a priority
// round per CR 603.3.
func (g *Game) DrainPendingTriggers() []PendingTrigger {
	out := g.pendingTriggers
	g.pendingTriggers = nil
	return out
}

// HasPendingTriggers returns true if there are unprocessed triggers in the queue.
func (g *Game) HasPendingTriggers() bool {
	return len(g.pendingTriggers) > 0
}

// ProcessPendingTriggers fires all queued triggers immediately (legacy path).
// New code should use DrainPendingTriggers and route through the ability stack.
func (g *Game) ProcessPendingTriggers() {
	for _, pt := range g.pendingTriggers {
		if pt.Trigger != nil && pt.Trigger.Action != nil {
			pt.Trigger.Action(g, pt.Event)
		}
	}
	g.pendingTriggers = nil
}

// apnapPosition returns 0 for the active player, then 1..n-1 walking
// forward through the seat order. Triggers with no controller are sent
// to the back of the queue.
func (g *Game) apnapPosition(p *Player) int {
	if p == nil {
		return len(g.players)
	}
	for i, pl := range g.players {
		if pl == p {
			d := i - g.activeIdx
			if d < 0 {
				d += len(g.players)
			}
			return d
		}
	}
	return len(g.players)
}

// --- Delayed Triggered Abilities (CR 603.7) ---

// DelayedTriggeredAbility represents a triggered ability that is created
// by a resolving spell/ability and will trigger once in the future (e.g.
// "at the beginning of your next end step, return it to your hand").
type DelayedTriggeredAbility struct {
	ID         uuid.UUID
	Controller *Player
	Event      Event // the event that caused this delayed trigger
	Condition  func(g *Game, e Event) bool
	Action     func(g *Game, e Event)
	// Duration controls when the delayed trigger is removed
	Duration    Duration
	// Replacement stores what event to look for (nil = system will
	// automatically check the condition on matching event types)
	TriggerType EventType
	// Zone stores which zone the trigger watches (0 = any)
	Zone Zone
	// Once triggers only once then self-removes
	Once bool
}

// RegisterDelayedTrigger adds a delayed triggered ability to the game.
func (g *Game) RegisterDelayedTrigger(dt *DelayedTriggeredAbility) {
	dt.ID = uuid.New()
	g.delayedTriggers = append(g.delayedTriggers, dt)
}

// RemoveDelayedTrigger removes a delayed triggered ability by ID.
func (g *Game) RemoveDelayedTrigger(id uuid.UUID) {
	for i, dt := range g.delayedTriggers {
		if dt.ID == id {
			g.delayedTriggers = append(g.delayedTriggers[:i], g.delayedTriggers[i+1:]...)
			return
		}
	}
}

// handleDelayedTriggers checks all delayed triggers against an event and
// collects those that match into pending triggers for the ability stack.
func (g *Game) handleDelayedTriggers(e Event) {
	for i := len(g.delayedTriggers) - 1; i >= 0; i-- {
		dt := g.delayedTriggers[i]
		if dt == nil {
			continue
		}
		// Check if the trigger type matches
		if dt.TriggerType != 0 && dt.TriggerType != e.Type {
			continue
		}
		// Check condition
		if dt.Condition != nil && !dt.Condition(g, e) {
			continue
		}
		// Check zone if specified
		if dt.Zone != 0 && e.ZoneChange != nil && e.ZoneChange.To != dt.Zone {
			continue
		}
		// Wrap the delayed trigger's condition to match Trigger's signature
		cond := dt.Condition
		var wrappedCondition func(Event) bool
		if cond != nil {
			wrappedCondition = func(e Event) bool {
				return cond(g, e)
			}
		}
		// Add to pending triggers
		g.pendingTriggers = append(g.pendingTriggers, PendingTrigger{
			Trigger: &Trigger{
				On:         e.Type,
				Controller: dt.Controller,
				Condition:  wrappedCondition,
				Action:     dt.Action,
			},
			Event: e,
		})
		// Remove if one-shot
		if dt.Once {
			g.delayedTriggers = append(g.delayedTriggers[:i], g.delayedTriggers[i+1:]...)
		}
	}
}

// clearDelayedTriggersEOT removes end-of-turn duration triggers.
func (g *Game) clearDelayedTriggersEOT() {
	var kept []*DelayedTriggeredAbility
	for _, dt := range g.delayedTriggers {
		if dt != nil && dt.Duration != DurationEndOfTurn {
			kept = append(kept, dt)
		}
	}
	g.delayedTriggers = kept
}

// --- Reflexive Triggered Abilities (CR 603.12) ---

// ReflexiveTriggeredAbility represents a triggered ability that is created
// as part of a resolving spell/ability and fires immediately when its
// trigger condition is met within that resolution (CR 603.12).
// Example: "When ~ enters the battlefield, you may return target card
// from your graveyard to your hand. When you do, draw a card."
type ReflexiveTriggeredAbility struct {
	ID          uuid.UUID
	Controller  *Player
	TriggerType EventType
	Condition   func(g *Game, e Event) bool
	Action      func(g *Game, e Event)
	// FiringEvent stores the event that would trigger the reflexive ability
	FiringEvent *Event
	// Once triggers only once then self-removes
	Once bool
}

// RegisterReflexiveTrigger adds a reflexive triggered ability.
func (g *Game) RegisterReflexiveTrigger(rt *ReflexiveTriggeredAbility) {
	rt.ID = uuid.New()
	g.reflexiveTriggers = append(g.reflexiveTriggers, rt)
}

// RemoveReflexiveTrigger removes a reflexive triggered ability by ID.
func (g *Game) RemoveReflexiveTrigger(id uuid.UUID) {
	for i, rt := range g.reflexiveTriggers {
		if rt.ID == id {
			g.reflexiveTriggers = append(g.reflexiveTriggers[:i], g.reflexiveTriggers[i+1:]...)
			return
		}
	}
}

// handleReflexiveTriggers checks all reflexive triggers against an event.
func (g *Game) handleReflexiveTriggers(e Event) {
	for i := len(g.reflexiveTriggers) - 1; i >= 0; i-- {
		rt := g.reflexiveTriggers[i]
		if rt == nil {
			continue
		}
		if rt.TriggerType != 0 && rt.TriggerType != e.Type {
			continue
		}
		if rt.Condition != nil && !rt.Condition(g, e) {
			continue
		}
		// Fire the reflexive trigger
		if rt.Action != nil {
			rt.Action(g, e)
		}
		if rt.Once {
			g.reflexiveTriggers = append(g.reflexiveTriggers[:i], g.reflexiveTriggers[i+1:]...)
		}
	}
}

// --- Zone Change Tracking (CR 400) ---

// ZoneChangeEvent tracks zone changes for "from anywhere" triggers.
type ZoneChangeEvent struct {
	CardName string
	From     Zone
	To       Zone
	Player   *Player
	Turn     int
}

// TrackZoneChange records a zone change for triggers that care about
// "from any zone" events.
func (g *Game) TrackZoneChange(cardName string, from, to Zone, player *Player) {
	// This is used by triggers that care about "whenever a card leaves
	// your graveyard" or "whenever a creature enters the battlefield
	// from your graveyard" etc.
	_ = ZoneChangeEvent{
		CardName: cardName,
		From:     from,
		To:       to,
		Player:   player,
		Turn:     g.turnNumber,
	}
}
