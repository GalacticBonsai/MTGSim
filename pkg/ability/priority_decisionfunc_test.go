package ability

import (
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

// TestDecisionFunc_CalledDuringRound verifies that when DecisionFunc is
// set, ProcessPriorityRound calls it for each player that receives
// priority instead of the default always-pass stub.
func TestDecisionFunc_CalledDuringRound(t *testing.T) {
	gs := &mockStackGameState{}
	ee := NewExecutionEngine(gs)
	stack := NewStack(gs, ee)
	pm := NewPriorityManager(stack, gs)

	p1 := &mockStackPlayer{name: "P1"}
	p2 := &mockStackPlayer{name: "P2"}
	pm.SetPlayers([]AbilityPlayer{p1, p2})
	pm.SetActivePlayer(p1)
	pm.SetPhase("Main Phase")

	var callCount int32
	pm.DecisionFunc = func(player AbilityPlayer) *PriorityDecision {
		atomic.AddInt32(&callCount, 1)
		return &PriorityDecision{Action: PriorityActionPass, Player: player}
	}

	err := pm.ProcessPriorityRound(50)
	if err != nil {
		t.Fatalf("ProcessPriorityRound: %v", err)
	}

	// Both players must have been called at least once
	if n := atomic.LoadInt32(&callCount); n < 2 {
		t.Errorf("expected at least 2 decision calls (one per player), got %d", n)
	}
}

// TestDecisionFunc_CastSpell verifies that returning a CastSpell action
// from DecisionFunc properly puts a spell on the stack and resets the
// priority round.
func TestDecisionFunc_CastSpell(t *testing.T) {
	gs := &mockStackGameState{isMainPhase: true}
	ee := NewExecutionEngine(gs)
	stack := NewStack(gs, ee)
	pm := NewPriorityManager(stack, gs)

	p1 := &mockStackPlayer{name: "P1"}
	p2 := &mockStackPlayer{name: "P2"}
	pm.SetPlayers([]AbilityPlayer{p1, p2})
	pm.SetActivePlayer(p1)
	pm.SetPhase("Main Phase")

	spell := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
		Effects:  []Effect{{Type: DealDamage, Value: 3}},
	}

	var castCount atomic.Int32
	pm.DecisionFunc = func(player AbilityPlayer) *PriorityDecision {
		// P1 casts a spell once, then both pass
		if player.GetName() == "P1" && castCount.Load() == 0 {
			castCount.Add(1)
			return &PriorityDecision{
				Action: PriorityActionCastSpell,
				Spell:  spell,
				Player: player,
			}
		}
		return &PriorityDecision{Action: PriorityActionPass, Player: player}
	}

	err := pm.ProcessPriorityRound(50)
	if err != nil {
		t.Fatalf("ProcessPriorityRound: %v", err)
	}

	if stack.Size() != 0 {
		t.Errorf("expected stack empty after full priority round, got %d items", stack.Size())
	}
	if n := castCount.Load(); n != 1 {
		t.Errorf("expected exactly 1 cast, got %d", n)
	}
}

// TestDecisionFunc_ActivateAbility verifies that returning an
// ActivateAbility action works correctly through the priority loop.
func TestDecisionFunc_ActivateAbility(t *testing.T) {
	gs := &mockStackGameState{isMainPhase: true}
	ee := NewExecutionEngine(gs)
	stack := NewStack(gs, ee)
	pm := NewPriorityManager(stack, gs)

	p1 := &mockStackPlayer{name: "P1"}
	p2 := &mockStackPlayer{name: "P2"}
	pm.SetPlayers([]AbilityPlayer{p1, p2})
	pm.SetActivePlayer(p1)
	pm.SetPhase("Main Phase")

	ability := &Ability{
		ID:                uuid.New(),
		Name:              "Tap to Bolt",
		Type:              Activated,
		TimingRestriction: AnyTime,
	}

	var activateCount atomic.Int32
	pm.DecisionFunc = func(player AbilityPlayer) *PriorityDecision {
		if player.GetName() == "P1" && activateCount.Load() == 0 {
			activateCount.Add(1)
			return &PriorityDecision{
				Action:  PriorityActionActivateAbility,
				Ability: ability,
				Player:  player,
			}
		}
		return &PriorityDecision{Action: PriorityActionPass, Player: player}
	}

	err := pm.ProcessPriorityRound(50)
	if err != nil {
		t.Fatalf("ProcessPriorityRound: %v", err)
	}

	if n := activateCount.Load(); n != 1 {
		t.Errorf("expected exactly 1 activation, got %d", n)
	}
}

// TestDecisionFunc_MultiplePlayersCasting tests that all players get a
// chance to cast during a single priority round when the stack has
// items.
func TestDecisionFunc_MultiplePlayersCasting(t *testing.T) {
	gs := &mockStackGameState{isMainPhase: true}
	ee := NewExecutionEngine(gs)
	stack := NewStack(gs, ee)
	pm := NewPriorityManager(stack, gs)

	p1 := &mockStackPlayer{name: "P1"}
	p2 := &mockStackPlayer{name: "P2"}
	p3 := &mockStackPlayer{name: "P3"}
	pm.SetPlayers([]AbilityPlayer{p1, p2, p3})
	pm.SetActivePlayer(p1)
	pm.SetPhase("Main Phase")

	bolt := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
		Effects:  []Effect{{Type: DealDamage, Value: 3}},
	}

	var p1Cast, p2Activate, p3Cast atomic.Int32
	pm.DecisionFunc = func(player AbilityPlayer) *PriorityDecision {
		switch player.GetName() {
		case "P1":
			// P1 passes immediately after casting once
			if p1Cast.Load() == 0 {
				p1Cast.Add(1)
				return &PriorityDecision{
					Action: PriorityActionCastSpell,
					Spell:  bolt,
					Player: player,
				}
			}
		case "P2":
			// P2 activates once
			if p2Activate.Load() == 0 {
				p2Activate.Add(1)
				ability := &Ability{
					ID:                uuid.New(),
					Name:              "P2 ability",
					Type:              Activated,
					TimingRestriction: AnyTime,
				}
				return &PriorityDecision{
					Action:  PriorityActionActivateAbility,
					Ability: ability,
					Player:  player,
				}
			}
		case "P3":
			// P3 casts once
			if p3Cast.Load() == 0 {
				p3Cast.Add(1)
				return &PriorityDecision{
					Action: PriorityActionCastSpell,
					Spell:  bolt,
					Player: player,
				}
			}
		}
		return &PriorityDecision{Action: PriorityActionPass, Player: player}
	}

	err := pm.ProcessPriorityRound(100)
	if err != nil {
		t.Fatalf("ProcessPriorityRound: %v", err)
	}

	if p1Cast.Load() != 1 {
		t.Errorf("expected P1 to cast once, got %d", p1Cast.Load())
	}
	if p2Activate.Load() != 1 {
		t.Errorf("expected P2 to activate once, got %d", p2Activate.Load())
	}
	if p3Cast.Load() != 1 {
		t.Errorf("expected P3 to cast once, got %d", p3Cast.Load())
	}
	if !stack.IsEmpty() {
		t.Errorf("expected stack empty after full round, got %d items", stack.Size())
	}
}

// TestDecisionFunc_FallbackToPass verifies that when DecisionFunc is nil
// the default behaviour (always pass) terminates immediately.
func TestDecisionFunc_FallbackToPass(t *testing.T) {
	gs := &mockStackGameState{isMainPhase: true}
	ee := NewExecutionEngine(gs)
	stack := NewStack(gs, ee)
	pm := NewPriorityManager(stack, gs)

	p1 := &mockStackPlayer{name: "P1"}
	p2 := &mockStackPlayer{name: "P2"}
	pm.SetPlayers([]AbilityPlayer{p1, p2})
	pm.SetActivePlayer(p1)
	pm.SetPhase("Main Phase")

	// DecisionFunc is nil — should use default pass

	err := pm.ProcessPriorityRound(50)
	if err != nil {
		t.Fatalf("ProcessPriorityRound: %v", err)
	}

	if !stack.IsEmpty() {
		t.Errorf("expected stack empty, got %d items", stack.Size())
	}
}

// TestProcessPriorityRound_IterationLimit verifies that the iteration
// limit prevents runaway loops when DecisionFunc keeps returning
// non-pass actions.
func TestProcessPriorityRound_IterationLimit(t *testing.T) {
	gs := &mockStackGameState{isMainPhase: true}
	ee := NewExecutionEngine(gs)
	stack := NewStack(gs, ee)
	pm := NewPriorityManager(stack, gs)

	p1 := &mockStackPlayer{name: "P1"}
	pm.SetPlayers([]AbilityPlayer{p1})
	pm.SetActivePlayer(p1)
	pm.SetPhase("Main Phase")

	bolt := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
		Effects:  []Effect{{Type: DealDamage, Value: 3}},
	}

	// Always return CastSpell — should hit the limit
	pm.DecisionFunc = func(player AbilityPlayer) *PriorityDecision {
		return &PriorityDecision{
			Action: PriorityActionCastSpell,
			Spell:  bolt,
			Player: player,
		}
	}

	err := pm.ProcessPriorityRound(10) // small limit
	if err != nil {
		t.Fatalf("ProcessPriorityRound: %v", err)
	}

	// Stack should have some items but not an unbounded number
	if stack.Size() == 0 {
		t.Error("expected at least one item on stack after limited round")
	}
	if stack.Size() > 15 {
		t.Errorf("iteration limit of 10 should bound stack to ~10 items, got %d", stack.Size())
	}
}
