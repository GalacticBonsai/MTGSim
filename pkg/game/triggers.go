package game

// Trigger is a simple event-based action.
type Trigger struct {
	On        EventType
	Condition func(Event) bool
	Action    func(g *Game, e Event)
}

func (g *Game) AddTrigger(t *Trigger) { g.triggers = append(g.triggers, t) }
func (g *Game) ClearTriggers()        { g.triggers = nil }

func (g *Game) handleTriggers(e Event) {
	for _, t := range g.triggers {
		if t == nil {
			continue
		}
		if t.On != e.Type {
			continue
		}
		if t.Condition != nil && !t.Condition(e) {
			continue
		}
		if t.Action != nil {
			t.Action(g, e)
		}
	}
}
