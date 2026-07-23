package game

// SummonCreature wraps Player.SummonCreature and emits ETB event.
func (g *Game) SummonCreature(p *Player, name string) (*Permanent, error) {
	perm, err := p.SummonCreature(name)
	if err != nil {
		return nil, err
	}
	// Track summoning sickness (CR 302.6): remember the turn a creature entered
	perm.SetEnteredTurn(g.turnNumber)
	g.emit(Event{Type: EventEntersBattlefield, ZoneChange: &ZoneChange{Permanent: perm, To: Battlefield}})
	return perm, nil
}

// CastPermanent wraps Player.CastPermanent and emits ETB event.
func (g *Game) CastPermanent(p *Player, name string) (*Permanent, error) {
	perm, err := p.CastPermanent(name)
	if err != nil {
		return nil, err
	}
	// Set turn entered for summoning sickness relevance (only matters if it's a creature)
	perm.SetEnteredTurn(g.turnNumber)
	g.emit(Event{Type: EventEntersBattlefield, ZoneChange: &ZoneChange{Permanent: perm, To: Battlefield}})
	return perm, nil
}

// PlayLand wraps Player.PlayLand and emits ETB event.
func (g *Game) PlayLand(p *Player, name string) (*Permanent, error) {
	perm, err := p.PlayLand(name)
	if err != nil {
		return nil, err
	}
	// Lands also "entered the battlefield this turn" but don't have summoning sickness.
	perm.SetEnteredTurn(g.turnNumber)
	g.emit(Event{Type: EventEntersBattlefield, ZoneChange: &ZoneChange{Permanent: perm, To: Battlefield}})
	return perm, nil
}

// DestroyPermanent destroys a permanent; applies replacement if present.
func (g *Game) DestroyPermanent(perm *Permanent) bool {
	return g.handleDies(perm)
}

// Sacrifice sacrifices a permanent (CR 701.16). The permanent goes to its
// owner's graveyard. Returns true if the sacrifice occurred.
func (g *Game) Sacrifice(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	owner := perm.GetOwner()
	if owner == nil {
		return false
	}
	// Remove from controller's battlefield
	ctrl := perm.GetController()
	if ctrl != nil {
		ctrl.RemoveFromBattlefield(perm)
	}
	// Put into owner's graveyard
	src := perm.GetSource()
	owner.Graveyard = append(owner.Graveyard, src)
	g.emit(Event{Type: EventSacrificed, Player: ctrl, Card: &src})
	g.emit(Event{Type: EventZoneChange, ZoneChange: &ZoneChange{Card: src, From: Battlefield, To: Graveyard}})
	g.ApplyStateBasedActions()
	return true
}

// ChangeController moves a permanent from one player's battlefield to another's
// (CR 903.16). Returns true if the change occurred.
func (g *Game) ChangeController(perm *Permanent, newController *Player) bool {
	if perm == nil || newController == nil {
		return false
	}
	oldController := perm.GetController()
	if oldController == nil {
		return false
	}
	if oldController == newController {
		return true // already controlled by new controller
	}
	// Remove from old controller's battlefield
	oldController.RemoveFromBattlefield(perm)
	// Add to new controller's battlefield
	perm.SetController(newController)
	newController.Battlefield = append(newController.Battlefield, perm)
	g.emit(Event{Type: EventControllerChanged, Player: newController, Permanent: perm})
	return true
}

// DestroyPermanentToExile exiles a permanent, sending it to its owner's exile
// zone directly (bypasses graveyard). Returns true if the exile occurred.
func (g *Game) DestroyPermanentToExile(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	owner := perm.GetOwner()
	ctrl := perm.GetController()
	if ctrl != nil {
		ctrl.RemoveFromBattlefield(perm)
	}
	src := perm.GetSource()
	if owner != nil {
		owner.Exile = append(owner.Exile, src)
	} else if ctrl != nil {
		ctrl.Exile = append(ctrl.Exile, src)
	}
	g.emit(Event{Type: EventZoneChange, ZoneChange: &ZoneChange{Card: src, From: Battlefield, To: Exile}})
	g.ApplyStateBasedActions()
	return true
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
