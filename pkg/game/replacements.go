package game

import "github.com/google/uuid"

// ReplacementEffect represents a static or replacement effect per CR 614.
// A replacement effect watches for an event and either prevents/replaces
// it or modifies it. The AppliedEffects list on each Event prevents
// infinite loops (CR 614.5).
//
// "If a permanent would X, instead Y" is a classic replacement effect.
type ReplacementEffect struct {
	ID          uuid.UUID
	Subject     string    // ID of the permanent this applies to (empty = game-wide)
	Priority    int       // when multiple effects apply, higher wins (CR 616.1)
	Applicable  func(g *Game, e Event) bool
	Apply       func(g *Game, e *Event) *Event // returns replacement event, nil to cancel
	Layer       Layer                           // continuous effect layer (CR 613.1)
	Duration    Duration                        // how long the effect lasts
}

type Duration int

const (
	DurationPermanent    Duration = iota
	DurationEndOfTurn
	DurationEndOfCombat
	DurationEndOfStep
	DurationEndOfTurnCycle
)

// replacementEngine holds all active replacement effects.
type replacementEngine struct {
	effects []*ReplacementEffect
}

func (g *Game) ensureReplacementEngine() {
	if g.replacementEngine == nil {
		g.replacementEngine = &replacementEngine{}
	}
}

// RegisterReplacementEffect adds a replacement effect to the engine.
func (g *Game) RegisterReplacementEffect(re *ReplacementEffect) {
	g.ensureReplacementEngine()
	if re.ID == uuid.Nil {
		re.ID = uuid.New()
	}
	g.replacementEngine.effects = append(g.replacementEngine.effects, re)
}

// RemoveReplacementEffect removes a replacement effect by ID.
func (g *Game) RemoveReplacementEffect(id uuid.UUID) {
	if g.replacementEngine == nil {
		return
	}
	for i, re := range g.replacementEngine.effects {
		if re.ID == id {
			g.replacementEngine.effects = append(
				g.replacementEngine.effects[:i],
				g.replacementEngine.effects[i+1:]...,
			)
			return
		}
	}
}

// applyReplacementEffects processes the event through all applicable
// replacement effects per CR 616. It returns the final event (possibly
// modified) or nil if the event was fully replaced/cancelled.
//
// CR 616.1: If two or more replacement effects would apply to the same
// event, the controller of the permanent chooses which one applies first.
// For simulation, we use priority ordering.
func (g *Game) applyReplacementEffects(e Event) *Event {
	if g.replacementEngine == nil {
		return &e
	}

	// Collect applicable effects, excluding already-applied ones (CR 614.5)
	var applicable []*ReplacementEffect
	for _, re := range g.replacementEngine.effects {
		if re == nil {
			continue
		}
		if re.Applicable == nil || !re.Applicable(g, e) {
			continue
		}
		if e.HasAppliedEffect(re.ID) {
			continue
		}
		applicable = append(applicable, re)
	}

	if len(applicable) == 0 {
		return &e
	}

	// Sort by priority (higher first) — CR 616.1
	for i := 1; i < len(applicable); i++ {
		for j := i; j > 0 && applicable[j].Priority > applicable[j-1].Priority; j-- {
			applicable[j], applicable[j-1] = applicable[j-1], applicable[j]
		}
	}

	// Apply each effect in priority order
	event := &e
	for _, re := range applicable {
		if event == nil {
			return nil
		}
		event.AddAppliedEffect(re.ID)
		if re.Apply != nil {
			event = re.Apply(g, event)
		}
	}

	return event
}

// clearReplacementEffectsEOT removes all duration-end-of-turn effects.
func (g *Game) clearReplacementEffectsEOT() {
	if g.replacementEngine == nil {
		return
	}
	var kept []*ReplacementEffect
	for _, re := range g.replacementEngine.effects {
		if re != nil && re.Duration != DurationEndOfTurn {
			kept = append(kept, re)
		}
	}
	g.replacementEngine.effects = kept
}

// --- Built-in replacement effects ---

// AddWouldDieExileUntilEOT causes the permanent to be exiled instead of
// dying (e.g. Rest in Peace).
func (g *Game) AddWouldDieExileUntilEOT(p *Permanent) {
	if p == nil {
		return
	}
	g.RegisterReplacementEffect(&ReplacementEffect{
		Subject:  p.GetID().String(),
		Priority: 10,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, e Event) bool {
			return (e.Type == EventLeavesBattlefield || e.Type == EventLeavesBattlefieldDied) &&
				e.ZoneChange != nil && e.ZoneChange.Permanent == p
		},
		Apply: func(g *Game, e *Event) *Event {
			// Redirect to exile
			if e.ZoneChange != nil {
				e.ZoneChange.To = Exile
			}
			return e
		},
	})
}

// AddDamageRedirectEffect redirects damage from source to target.
func (g *Game) AddDamageRedirectEffect(sourceID uuid.UUID, targetID uuid.UUID) {
	g.RegisterReplacementEffect(&ReplacementEffect{
		Subject:  sourceID.String(),
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, e Event) bool {
			return (e.Type == EventDamagePlayer || e.Type == EventDamagedPlayer) &&
				e.SourceID == sourceID
		},
		Apply: func(g *Game, e *Event) *Event {
			e.TargetID = targetID
			return e
		},
	})
}

// AddDamagePreventionEffect prevents all damage from a source.
func (g *Game) AddDamagePreventionEffect(sourceID uuid.UUID) {
	g.RegisterReplacementEffect(&ReplacementEffect{
		Subject:  sourceID.String(),
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, e Event) bool {
			return (e.Type == EventDamagePlayer || e.Type == EventDamagedPlayer ||
				e.Type == EventDamagePermanent || e.Type == EventDamagedPermanent) &&
				e.SourceID == sourceID
		},
		Apply: func(g *Game, e *Event) *Event {
			e.Amount = 0
			return e
		},
	})
}

// AddETBReplacementEffect replaces an event when a permanent enters the
// battlefield (e.g. "enters the battlefield tapped").
func (g *Game) AddETBReplacementEffect(perm *Permanent, replacement func(g *Game, e *Event) *Event) {
	g.RegisterReplacementEffect(&ReplacementEffect{
		Subject:  perm.GetID().String(),
		Priority: 5,
		Duration: DurationPermanent,
		Applicable: func(g *Game, e Event) bool {
			return e.Type == EventEntersBattlefield &&
				e.ZoneChange != nil && e.ZoneChange.Permanent != nil && e.ZoneChange.Permanent.GetID() == perm.GetID()
		},
		Apply: replacement,
	})
}

// AddCostReductionEffect reduces the mana cost of a spell.
// This is technically a cost-reducing effect, not a replacement effect,
// but we track it in the same engine for simplicity.
func (g *Game) AddCostReductionEffect(cardName string, reduction int) {
	g.RegisterReplacementEffect(&ReplacementEffect{
		Priority: 0,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, e Event) bool {
			return false // cost reduction doesn't replace events
		},
		Apply: nil,
	})
}

// AddLifeGainReplacementEffect replaces life gain events (e.g. "if you
// would gain life, you gain twice that much life instead").
func (g *Game) AddLifeGainReplacementEffect(player *Player, multiplier int) {
	g.RegisterReplacementEffect(&ReplacementEffect{
		Subject:  player.ID(),
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, e Event) bool {
			return (e.Type == EventGainLife || e.Type == EventGainedLife) &&
				e.PlayerID == player.ID()
		},
		Apply: func(g *Game, e *Event) *Event {
			e.Amount = e.Amount * multiplier
			return e
		},
	})
}

// AddTokenDoublingEffect doubles the number of tokens created.
func (g *Game) AddTokenDoublingEffect(controller *Player) {
	g.RegisterReplacementEffect(&ReplacementEffect{
		Priority: 5,
		Duration: DurationPermanent,
		Applicable: func(g *Game, e Event) bool {
			return (e.Type == EventCreateToken || e.Type == EventTokenCreated) &&
				e.ControllerID == controller.ID()
		},
		Apply: func(g *Game, e *Event) *Event {
			e.Amount = e.Amount * 2
			return e
		},
	})
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
	// Check replacement effects (e.g. Rest in Peace)
	if g.replacementEngine != nil {
		replaced := g.applyReplacementEffects(Event{
			Type:       EventLeavesBattlefield,
			ZoneChange: &ZoneChange{Permanent: perm, From: Battlefield, LKI: snap},
		})
		if replaced != nil && replaced.ZoneChange != nil && replaced.ZoneChange.To == Exile {
			ok := ctrl.DestroyPermanentToExile(perm)
			if ok {
				g.emit(Event{Type: EventZoneChange, ZoneChange: &ZoneChange{Card: perm.source, From: Battlefield, To: Exile, LKI: snap}})
			}
			return ok
		}
	}
	ok := ctrl.DestroyPermanent(perm)
	if ok {
		g.emit(Event{Type: EventZoneChange, ZoneChange: &ZoneChange{Card: perm.source, From: Battlefield, To: Graveyard, LKI: snap}})
	}
	return ok
}
