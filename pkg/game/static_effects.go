package game

// StaticEffectType categorizes continuous static effects that alter game rules.
type StaticEffectType int

const (
	// CostModifier increases or decreases mana costs of spells/abilities.
	CostModifier StaticEffectType = iota
	// CastConstraint imposes a "can't" restriction (e.g., Rule of Law).
	CastConstraint
	// AbilityRestriction prevents certain abilities from being activated.
	AbilityRestriction
)

// StaticEffect represents a continuous static effect on the game.
type StaticEffect struct {
	Type        StaticEffectType
	Source      *Permanent // permanent generating the effect
	Controller  *Player    // player who controls the source
	Description string

	// For CostModifier: additional generic mana to pay per spell
	AdditionalGenericCost int

	// For CastConstraint: max spells a player may cast per turn (0 = unlimited)
	MaxSpellsPerTurn int

	// TargetFilter restricts which cards/players are affected.
	// Empty means "all".
	AffectsCardTypes []string // e.g., "Creature", "Instant"
}

// StaticEffectRegistry holds all active static effects in a game.
type StaticEffectRegistry struct {
	effects []*StaticEffect
}

// NewStaticEffectRegistry creates an empty registry.
func NewStaticEffectRegistry() *StaticEffectRegistry {
	return &StaticEffectRegistry{}
}

// Register adds a static effect to the registry.
func (r *StaticEffectRegistry) Register(e *StaticEffect) {
	r.effects = append(r.effects, e)
}

// Unregister removes all effects originating from a given permanent.
func (r *StaticEffectRegistry) Unregister(source *Permanent) {
	var kept []*StaticEffect
	for _, e := range r.effects {
		if e.Source != source {
			kept = append(kept, e)
		}
	}
	r.effects = kept
}

// All returns a snapshot of current effects.
func (r *StaticEffectRegistry) All() []*StaticEffect {
	out := make([]*StaticEffect, len(r.effects))
	copy(out, r.effects)
	return out
}

// TotalAdditionalCost computes the sum of all generic mana taxes
// that apply to a spell cast by the given controller with the given type line.
func (r *StaticEffectRegistry) TotalAdditionalCost(controller *Player, typeLine string) int {
	total := 0
	for _, e := range r.effects {
		if e.Type != CostModifier {
			continue
		}
		if e.Controller == controller {
			continue // self-tax is rare; skip own controller for simplicity
		}
		if !e.affectsType(typeLine) {
			continue
		}
		total += e.AdditionalGenericCost
	}
	return total
}

// CanCastSpell checks whether the given player is allowed to cast another
// spell this turn under all active CastConstraint effects.
func (r *StaticEffectRegistry) CanCastSpell(controller *Player, spellsCastThisTurn int) bool {
	for _, e := range r.effects {
		if e.Type != CastConstraint {
			continue
		}
		if e.Controller == controller {
			continue
		}
		if e.MaxSpellsPerTurn > 0 && spellsCastThisTurn >= e.MaxSpellsPerTurn {
			return false
		}
	}
	return true
}

// affectsType returns true if the effect applies to the given card type line.
func (e *StaticEffect) affectsType(typeLine string) bool {
	if len(e.AffectsCardTypes) == 0 {
		return true
	}
	for _, t := range e.AffectsCardTypes {
		if contains(typeLine, t) {
			return true
		}
	}
	return false
}

// Game-level helpers

// RegisterStaticEffect registers a static effect on the game.
func (g *Game) RegisterStaticEffect(e *StaticEffect) {
	if g.continuous == nil {
		g.continuous = &continuous{}
	}
	if g.continuous.staticEffects == nil {
		g.continuous.staticEffects = NewStaticEffectRegistry()
	}
	g.continuous.staticEffects.Register(e)
}

// RemoveStaticEffects removes all static effects originating from a permanent.
func (g *Game) RemoveStaticEffects(source *Permanent) {
	if g.continuous != nil && g.continuous.staticEffects != nil {
		g.continuous.staticEffects.Unregister(source)
	}
}

// GetStaticEffects returns the current static effect registry.
func (g *Game) GetStaticEffects() *StaticEffectRegistry {
	if g.continuous == nil || g.continuous.staticEffects == nil {
		return NewStaticEffectRegistry()
	}
	return g.continuous.staticEffects
}
