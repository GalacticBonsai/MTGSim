package game

import "sort"

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
		if m.trigger.Action != nil {
			m.trigger.Action(g, e)
		}
	}
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
