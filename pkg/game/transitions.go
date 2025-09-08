package game

// SummonCreature wraps Player.SummonCreature and emits ETB event.
func (g *Game) SummonCreature(p *Player, name string) (*Permanent, error) {
	perm, err := p.SummonCreature(name)
	if err != nil {
		return nil, err
	}
	g.emit(Event{Type: EventEntersBattlefield, ZoneChange: &ZoneChange{Permanent: perm, To: Battlefield}})
	return perm, nil
}

// PlayLand wraps Player.PlayLand and emits ETB event.
func (g *Game) PlayLand(p *Player, name string) (*Permanent, error) {
	perm, err := p.PlayLand(name)
	if err != nil {
		return nil, err
	}
	g.emit(Event{Type: EventEntersBattlefield, ZoneChange: &ZoneChange{Permanent: perm, To: Battlefield}})
	return perm, nil
}

// DestroyPermanent destroys a permanent; applies replacement if present.
func (g *Game) DestroyPermanent(perm *Permanent) bool {
	return g.handleDies(perm)
}

// ExileFromGraveyard wraps Player.ExileFromGraveyard and emits zone change event.
func (g *Game) ExileFromGraveyard(p *Player, name string) bool {
	// find card in graveyard
	var card *SimpleCard
	for i := len(p.Graveyard) - 1; i >= 0; i-- {
		if p.Graveyard[i].Name == name || name == "" {
			c := p.Graveyard[i]
			card = &c
			break
		}
	}
	ok := p.ExileFromGraveyard(name)
	if ok && card != nil {
		g.emit(Event{Type: EventZoneChange, ZoneChange: &ZoneChange{Card: *card, From: Graveyard, To: Exile}})
	}
	return ok
}
