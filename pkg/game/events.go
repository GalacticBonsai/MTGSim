package game

type EventType int

const (
	EventZoneChange EventType = iota
	EventEntersBattlefield
	EventLeavesBattlefield
)

type PermanentSnapshot struct {
	Name      string
	Power     int
	Toughness int
	Damage    int
}

type ZoneChange struct {
	Card      SimpleCard
	Permanent *Permanent
	From      Zone
	To        Zone
	LKI       *PermanentSnapshot
}

type Event struct {
	Type       EventType
	ZoneChange *ZoneChange
}

// Listener registration
func (g *Game) AddListener(l func(Event)) { g.listeners = append(g.listeners, l) }

func (g *Game) emit(e Event) {
	// Notify listeners first
	for _, l := range g.listeners {
		l(e)
	}
	// Then process triggers and watchers
	g.handleTriggers(e)
	g.handleWatchers(e)
}

func snapshotPermanent(p *Permanent) *PermanentSnapshot {
	if p == nil {
		return nil
	}
	return &PermanentSnapshot{
		Name:      p.GetName(),
		Power:     p.GetPower(),
		Toughness: p.GetToughness(),
		Damage:    p.GetDamageCounters(),
	}
}
