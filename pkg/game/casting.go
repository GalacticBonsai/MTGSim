package game

// SimpleStack is a minimal interface to enqueue spells/abilities without
// importing the ability package (avoids cycles). It is intended to be
// implemented by an adapter that wraps the real ability stack.
type SimpleStack interface {
	EnqueueSpell(name string, cmc int, manaCost string, typeLine string, controller any, targets []any) error
	Size() int
}

type casting struct {
	stack SimpleStack
}

// SetStack injects a SimpleStack implementation for casting/activating.
func (g *Game) SetStack(s SimpleStack) { g.ensureCasting().stack = s }

// CastSimpleSpell enqueues a simplistic spell onto the injected stack.
func (g *Game) CastSimpleSpell(name string, cmc int, manaCost string, typeLine string, controller any, targets []any) error {
	if g.casting == nil || g.casting.stack == nil {
		return ErrNoStack
	}
	return g.casting.stack.EnqueueSpell(name, cmc, manaCost, typeLine, controller, targets)
}

// helper to ensure casting sub-struct is initialized
func (g *Game) ensureCasting() *casting {
	if g.casting == nil {
		g.casting = &casting{}
	}
	return g.casting
}
