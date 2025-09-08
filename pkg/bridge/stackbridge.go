package bridge

import (
    abil "github.com/mtgsim/mtgsim/pkg/ability"
)

// StackBridge is a thin holder around ability.Stack to avoid package cycles.
// It is constructed with an ability.GameState implementation.
type StackBridge struct {
    stack *abil.Stack
}

// NewStackBridge constructs a new StackBridge using the provided GameState.
func NewStackBridge(gs abil.GameState) *StackBridge {
    ee := abil.NewExecutionEngine(gs)
    st := abil.NewStack(gs, ee)
    return &StackBridge{stack: st}
}

func (b *StackBridge) Stack() *abil.Stack { return b.stack }

// Convenience helpers
func (b *StackBridge) Size() int { return b.stack.Size() }
func (b *StackBridge) IsEmpty() bool { return b.stack.IsEmpty() }

