package game

import (
	"github.com/google/uuid"
)

// CopyEffect represents a Layer 1 copy effect (CR 613.1f).
// A copy effect creates a copy of an object, potentially with modifications.
type CopyEffect struct {
	ID       uuid.UUID
	Priority int
	Source   uuid.UUID // ID of the object being copied
	Target   uuid.UUID // ID of the object being created (the copy)
	Duration Duration

	// CopyApplier can modify the copied object (e.g., "except it loses all
	// abilities" or "except its name is X").
	CopyApplier func(copy *Permanent, original *Permanent)

	// Layer is always LayerCopyEffects (1) for copy effects
	Layer Layer
}

// copyEngine holds all active copy effects.
type copyEngine struct {
	effects []*CopyEffect
}

func (g *Game) ensureCopyEngine() {
	if g.copyEngine == nil {
		g.copyEngine = &copyEngine{}
	}
}

// RegisterCopyEffect adds a copy effect to the engine.
func (g *Game) RegisterCopyEffect(ce *CopyEffect) {
	g.ensureCopyEngine()
	ce.ID = uuid.New()
	ce.Layer = Layer1Copy
	g.copyEngine.effects = append(g.copyEngine.effects, ce)
}

// RemoveCopyEffect removes a copy effect by ID.
func (g *Game) RemoveCopyEffect(id uuid.UUID) {
	if g.copyEngine == nil {
		return
	}
	for i, ce := range g.copyEngine.effects {
		if ce.ID == id {
			g.copyEngine.effects = append(g.copyEngine.effects[:i], g.copyEngine.effects[i+1:]...)
			return
		}
	}
}

// copyPermanent creates a copy of a permanent, applying any active copy
// effects and the provided CopyApplier (CR 706).
func (g *Game) copyPermanent(source *Permanent, newController *Player) *Permanent {
	if source == nil {
		return nil
	}

	// Create the base copy
	copy := NewPermanent(source.source, source.GetOwner(), newController)
	copy.printedPower = source.printedPower
	copy.printedToughness = source.printedToughness
	copy.effPower = source.effPower
	copy.effToughness = source.effToughness
	copy.tapped = source.tapped
	copy.enteredTurn = g.turnNumber // CR 706.2: a copy has summoning sickness

	// Copy counters (CR 706.2)
	if source.counters != nil {
		copy.counters = make(map[string]int)
		for k, v := range source.counters {
			copy.counters[k] = v
		}
	}

	// Copy keywords
	if source.printedKeywords != nil {
		copy.printedKeywords = make(map[Keyword]bool)
		for k, v := range source.printedKeywords {
			copy.printedKeywords[k] = v
		}
	}
	if source.grantedKeywords != nil {
		copy.grantedKeywords = make(map[Keyword]bool)
		for k, v := range source.grantedKeywords {
			copy.grantedKeywords[k] = v
		}
	}

	// Apply copy applier if provided
	if g.copyEngine != nil {
		for _, ce := range g.copyEngine.effects {
			if ce == nil {
				continue
			}
			if ce.Source == source.GetID() {
				if ce.CopyApplier != nil {
					ce.CopyApplier(copy, source)
				}
			}
		}
	}

	return copy
}

// CopySpell creates a copy of a spell on the stack (CR 706.10).
// The copy is put onto the stack as a new spell.
func (g *Game) CopySpell(source *GameStackItem) *GameStackItem {
	if source == nil {
		return nil
	}

	copy := &GameStackItem{
		ID:          uuid.New(),
		Type:        source.Type,
		Name:        source.Name,
		Controller:  source.Controller,
		Source:      source.Source,
		Targets:     make([]interface{}, len(source.Targets)),
		Description: source.Name + " (copy)",
		Resolve:     source.Resolve,
	}

	// Copy targets
	for i, t := range source.Targets {
		copy.Targets[i] = t
	}

	return copy
}

// ClearCopyEffectsEOT removes end-of-turn duration copy effects.
func (g *Game) ClearCopyEffectsEOT() {
	if g.copyEngine == nil {
		return
	}
	var kept []*CopyEffect
	for _, ce := range g.copyEngine.effects {
		if ce != nil && ce.Duration != DurationEndOfTurn {
			kept = append(kept, ce)
		}
	}
	g.copyEngine.effects = kept
}

// --- Built-in copy effect helpers ---

// AddBecomeCopyEffect makes a permanent become a copy of another permanent.
// The permanent keeps its counters and has summoning sickness (CR 706.2).
func (g *Game) AddBecomeCopyEffect(target *Permanent, source *Permanent) {
	g.RegisterCopyEffect(&CopyEffect{
		Source:   source.GetID(),
		Target:   target.GetID(),
		Duration: DurationPermanent,
		CopyApplier: func(copy *Permanent, original *Permanent) {
			// Copy the source's effective stats
			copy.effPower = original.GetPower()
			copy.effToughness = original.GetToughness()
			copy.printedPower = original.printedPower
			copy.printedToughness = original.printedToughness
		},
	})
}

// AddTokenCopyEffect creates a copy of a permanent as a token.
func (g *Game) AddTokenCopyEffect(source *Permanent, newController *Player) *Permanent {
	copy := g.copyPermanent(source, newController)
	if copy != nil {
		newController.Battlefield = append(newController.Battlefield, copy)
		g.emit(Event{
			Type:       EventTokenCreated,
			ControllerID: newController.GetName(),
			Permanent:  copy,
		})
	}
	return copy
}
