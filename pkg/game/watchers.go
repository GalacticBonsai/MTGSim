package game

// Watcher observes events and maintains derived state. Reset each end step.
type Watcher interface {
	OnEvent(e Event)
	ResetEOT()
}

func (g *Game) AddWatcher(w Watcher) { g.watchers = append(g.watchers, w) }
func (g *Game) ClearWatchers()       { g.watchers = nil }

func (g *Game) handleWatchers(e Event) {
	for _, w := range g.watchers {
		if w != nil {
			w.OnEvent(e)
		}
	}
}

func (g *Game) resetWatchersEOT() {
	for _, w := range g.watchers {
		if w != nil {
			w.ResetEOT()
		}
	}
}

// CreatureETBWatcher counts creature ETB events this turn.
type CreatureETBWatcher struct{ Count int }

func (w *CreatureETBWatcher) OnEvent(e Event) {
	if e.Type == EventEntersBattlefield && e.ZoneChange != nil && e.ZoneChange.Permanent != nil {
		if e.ZoneChange.Permanent.IsCreature() {
			w.Count++
		}
	}
}

func (w *CreatureETBWatcher) ResetEOT() { w.Count = 0 }
