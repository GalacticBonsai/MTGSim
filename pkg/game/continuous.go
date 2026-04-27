package game

import "sort"

// Continuous-effect engine implementing CR 613 (Interaction of Continuous
// Effects). Effects are evaluated in seven layers; layer 7 (P/T) further
// splits into five sublayers. Within a layer, dependencies are ignored for
// now and effects are applied in timestamp order (oldest first).
//
// Reference: XMage org.mage.abilities.effects.ContinuousEffects, which
// implements the same layered pipeline in Java.

// Layer enumerates CR 613.1 layers.
type Layer int

const (
	Layer1Copy    Layer = 1 // 613.1a copy effects
	Layer2Control Layer = 2 // 613.1b control-changing effects
	Layer3Text    Layer = 3 // 613.1c text-changing effects
	Layer4Type    Layer = 4 // 613.1d type-changing effects
	Layer5Color   Layer = 5 // 613.1e color-changing effects
	Layer6Ability Layer = 6 // 613.1f ability-adding/removing effects
	Layer7PT      Layer = 7 // 613.1g power/toughness effects
)

// Sublayer enumerates CR 613.1g sublayers (only meaningful for Layer 7).
type Sublayer int

const (
	SublayerNone Sublayer = 0
	Sublayer7A   Sublayer = 1 // characteristic-defining abilities
	Sublayer7B   Sublayer = 2 // set base power/toughness
	Sublayer7C   Sublayer = 3 // modifications (anthems, +N/+N pumps)
	Sublayer7D   Sublayer = 4 // counters (+1/+1, -1/-1)
	Sublayer7E   Sublayer = 5 // switch power and toughness
)

// PermanentView is the mutable view of a permanent's characteristics that
// each layered effect transforms in turn. The final view is what
// Permanent.GetPower / GetToughness read from.
type PermanentView struct {
	Power     int
	Toughness int
	// SwapPT is toggled by Layer 7E; the engine flips Power and Toughness
	// once after all other Layer 7 sublayers have been applied.
	SwapPT bool
}

// LayeredEffect is one continuous effect registered with the engine.
// Apply mutates view in place; Affects gates which permanents the effect
// applies to. Source may be nil for global / game-level effects.
type LayeredEffect struct {
	ID         uint64
	Layer      Layer
	Sublayer   Sublayer
	Source     *Permanent
	Affects    func(p *Permanent) bool
	Apply      func(p *Permanent, v *PermanentView)
	ExpiresEOT bool
	Timestamp  uint64
}

// continuous holds the active layered effects and bookkeeping for the
// recompute pipeline.
type continuous struct {
	effects   []*LayeredEffect
	nextID    uint64
	timestamp uint64
}

func (g *Game) ensureContinuous() {
	if g.continuous == nil {
		g.continuous = &continuous{}
	}
}

// AddLayeredEffect registers a continuous effect and returns its id.
// Recompute is invoked so cached views reflect the new effect immediately.
func (g *Game) AddLayeredEffect(eff *LayeredEffect) uint64 {
	if eff == nil {
		return 0
	}
	g.ensureContinuous()
	g.continuous.nextID++
	g.continuous.timestamp++
	eff.ID = g.continuous.nextID
	eff.Timestamp = g.continuous.timestamp
	g.continuous.effects = append(g.continuous.effects, eff)
	g.RecomputeContinuous()
	return eff.ID
}

// RemoveLayeredEffect drops the effect with the given id, if any.
func (g *Game) RemoveLayeredEffect(id uint64) {
	if g.continuous == nil || id == 0 {
		return
	}
	out := g.continuous.effects[:0]
	for _, e := range g.continuous.effects {
		if e.ID != id {
			out = append(out, e)
		}
	}
	g.continuous.effects = out
	g.RecomputeContinuous()
}

// RecomputeContinuous walks every battlefield permanent, resets its
// effective view to its printed values, and applies all active layered
// effects in (Layer, Sublayer, Timestamp) order.
func (g *Game) RecomputeContinuous() {
	if g == nil {
		return
	}
	for _, pl := range g.players {
		for _, p := range pl.Battlefield {
			p.effPower = p.printedPower
			p.effToughness = p.printedToughness
			p.effSwapPT = false
		}
	}
	if g.continuous == nil || len(g.continuous.effects) == 0 {
		return
	}
	ordered := make([]*LayeredEffect, len(g.continuous.effects))
	copy(ordered, g.continuous.effects)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Layer != ordered[j].Layer {
			return ordered[i].Layer < ordered[j].Layer
		}
		if ordered[i].Sublayer != ordered[j].Sublayer {
			return ordered[i].Sublayer < ordered[j].Sublayer
		}
		return ordered[i].Timestamp < ordered[j].Timestamp
	})
	for _, eff := range ordered {
		for _, pl := range g.players {
			for _, p := range pl.Battlefield {
				if eff.Affects != nil && !eff.Affects(p) {
					continue
				}
				v := &PermanentView{Power: p.effPower, Toughness: p.effToughness, SwapPT: p.effSwapPT}
				eff.Apply(p, v)
				p.effPower = v.Power
				p.effToughness = v.Toughness
				p.effSwapPT = v.SwapPT
			}
		}
	}
	for _, pl := range g.players {
		for _, p := range pl.Battlefield {
			if p.effSwapPT {
				p.effPower, p.effToughness = p.effToughness, p.effPower
				p.effSwapPT = false
			}
		}
	}
}

// clearLayeredEffectsEOT removes all effects flagged as EOT-duration.
func (g *Game) clearLayeredEffectsEOT() {
	if g.continuous == nil {
		return
	}
	out := g.continuous.effects[:0]
	for _, e := range g.continuous.effects {
		if !e.ExpiresEOT {
			out = append(out, e)
		}
	}
	g.continuous.effects = out
}

// ApplySetPTUntilEOT sets a creature's base power/toughness until end of turn.
// Last-applied wins by virtue of higher timestamp in Layer 7B ordering.
func (g *Game) ApplySetPTUntilEOT(p *Permanent, power, toughness int) {
	if p == nil {
		return
	}
	target := p
	g.AddLayeredEffect(&LayeredEffect{
		Layer:    Layer7PT,
		Sublayer: Sublayer7B,
		Source:   target,
		Affects:  func(q *Permanent) bool { return q == target },
		Apply: func(_ *Permanent, v *PermanentView) {
			v.Power = power
			v.Toughness = toughness
		},
		ExpiresEOT: true,
	})
}
