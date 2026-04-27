package game

// replacements holds simple replacement effects state.
type replacements struct {
	wouldDieExile map[*Permanent]bool // until EOT flags
}

func (g *Game) ensureReplacements() {
	if g.replacements == nil {
		g.replacements = &replacements{wouldDieExile: map[*Permanent]bool{}}
	}
}

// AddWouldDieExileUntilEOT causes the permanent to be exiled if it would die this turn.
func (g *Game) AddWouldDieExileUntilEOT(p *Permanent) {
	if p == nil {
		return
	}
	g.ensureReplacements()
	g.replacements.wouldDieExile[p] = true
}

func (g *Game) clearReplacementsEOT() {
	if g.replacements != nil {
		g.replacements.wouldDieExile = map[*Permanent]bool{}
	}
}

func (g *Game) wouldDieExile(p *Permanent) bool {
	return g.replacements != nil && g.replacements.wouldDieExile[p]
}

// handleDies moves a permanent from battlefield to GY (or Exile if replacement) and emits events.
func (g *Game) handleDies(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	snap := snapshotPermanent(perm)
	g.emit(Event{Type: EventLeavesBattlefield, ZoneChange: &ZoneChange{Permanent: perm, From: Battlefield, LKI: snap}})
	ctrl := perm.GetController()
	if ctrl == nil {
		return false
	}
	// CR 903.9: if a commander would be put into a graveyard or exile from
	// anywhere, its owner may put it into the command zone instead. The
	// automated player always elects to do so.
	if perm.IsCommander() {
		owner := perm.GetOwner()
		if owner != nil && owner.SendCommanderToCommandZone(perm) {
			g.emit(Event{Type: EventZoneChange, ZoneChange: &ZoneChange{Card: perm.source, From: Battlefield, To: Command, LKI: snap}})
			return true
		}
	}
	if g.wouldDieExile(perm) {
		ok := ctrl.DestroyPermanentToExile(perm)
		if ok {
			g.emit(Event{Type: EventZoneChange, ZoneChange: &ZoneChange{Card: perm.source, From: Battlefield, To: Exile, LKI: snap}})
		}
		return ok
	}
	ok := ctrl.DestroyPermanent(perm)
	if ok {
		g.emit(Event{Type: EventZoneChange, ZoneChange: &ZoneChange{Card: perm.source, From: Battlefield, To: Graveyard, LKI: snap}})
	}
	return ok
}
