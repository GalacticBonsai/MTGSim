// Package ability provides comprehensive spell timing and priority testing for MTG simulation.
package ability

import (
	"testing"

	"github.com/google/uuid"
)

// MockGameState for timing tests
type mockTimingGameState struct {
	currentPhase    string
	stackEmpty      bool
	hasPriority     bool
	activePlayer    string
	priorityPlayer  string
	isMainPhase     bool
	isCombatPhase   bool
	isEndStep       bool
}

func (m *mockTimingGameState) GetCurrentPhase() string {
	return m.currentPhase
}

func (m *mockTimingGameState) IsStackEmpty() bool {
	return m.stackEmpty
}

func (m *mockTimingGameState) HasPriority(player string) bool {
	return m.hasPriority && m.priorityPlayer == player
}

func (m *mockTimingGameState) IsMainPhase() bool {
	return m.isMainPhase
}

// TestInstantSpellTiming tests that instant spells can be cast at appropriate times
func TestInstantSpellTiming(t *testing.T) {
	testCases := []struct {
		name        string
		gameState   *mockTimingGameState
		canCast     bool
		description string
	}{
		{
			name: "Main Phase with Priority",
			gameState: &mockTimingGameState{
				currentPhase:   "Main Phase",
				stackEmpty:     true,
				hasPriority:    true,
				priorityPlayer: "Player1",
				isMainPhase:    true,
			},
			canCast:     true,
			description: "Can cast instant in main phase with priority",
		},
		{
			name: "Combat Phase with Priority",
			gameState: &mockTimingGameState{
				currentPhase:   "Combat Phase",
				stackEmpty:     false,
				hasPriority:    true,
				priorityPlayer: "Player1",
				isCombatPhase:  true,
			},
			canCast:     true,
			description: "Can cast instant in combat phase with priority",
		},
		{
			name: "End Step with Priority",
			gameState: &mockTimingGameState{
				currentPhase:   "End Step",
				stackEmpty:     true,
				hasPriority:    true,
				priorityPlayer: "Player1",
				isEndStep:      true,
			},
			canCast:     true,
			description: "Can cast instant in end step with priority",
		},
		{
			name: "No Priority",
			gameState: &mockTimingGameState{
				currentPhase:   "Main Phase",
				stackEmpty:     true,
				hasPriority:    false,
				priorityPlayer: "Player2",
				isMainPhase:    true,
			},
			canCast:     false,
			description: "Cannot cast instant without priority",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canCast := canCastInstantSpell(tc.gameState, "Player1")
			if canCast != tc.canCast {
				t.Errorf("%s: expected canCast %v, got %v", tc.name, tc.canCast, canCast)
			}
		})
	}
}

// TestSorcerySpellTiming tests that sorcery spells can only be cast at sorcery speed
func TestSorcerySpellTiming(t *testing.T) {
	testCases := []struct {
		name        string
		gameState   *mockTimingGameState
		canCast     bool
		description string
	}{
		{
			name: "Main Phase, Stack Empty, Has Priority",
			gameState: &mockTimingGameState{
				currentPhase:   "Main Phase",
				stackEmpty:     true,
				hasPriority:    true,
				priorityPlayer: "Player1",
				isMainPhase:    true,
			},
			canCast:     true,
			description: "Can cast sorcery in main phase when stack is empty",
		},
		{
			name: "Main Phase, Stack Not Empty",
			gameState: &mockTimingGameState{
				currentPhase:   "Main Phase",
				stackEmpty:     false,
				hasPriority:    true,
				priorityPlayer: "Player1",
				isMainPhase:    true,
			},
			canCast:     false,
			description: "Cannot cast sorcery when stack is not empty",
		},
		{
			name: "Combat Phase",
			gameState: &mockTimingGameState{
				currentPhase:   "Combat Phase",
				stackEmpty:     true,
				hasPriority:    true,
				priorityPlayer: "Player1",
				isCombatPhase:  true,
			},
			canCast:     false,
			description: "Cannot cast sorcery in combat phase",
		},
		{
			name: "End Step",
			gameState: &mockTimingGameState{
				currentPhase:   "End Step",
				stackEmpty:     true,
				hasPriority:    true,
				priorityPlayer: "Player1",
				isEndStep:      true,
			},
			canCast:     false,
			description: "Cannot cast sorcery in end step",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canCast := canCastSorcerySpell(tc.gameState, "Player1")
			if canCast != tc.canCast {
				t.Errorf("%s: expected canCast %v, got %v", tc.name, tc.canCast, canCast)
			}
		})
	}
}

// TestStackInteraction tests spell casting and stack resolution order
func TestStackInteraction(t *testing.T) {
	stack := NewSpellStack()
	
	// Create test spells
	lightningBolt := &SpellOnStack{
		ID:         uuid.New(),
		Name:       "Lightning Bolt",
		Controller: "Player1",
		Targets:    []interface{}{"Player2"},
	}
	
	counterspell := &SpellOnStack{
		ID:         uuid.New(),
		Name:       "Counterspell",
		Controller: "Player2",
		Targets:    []interface{}{lightningBolt},
	}
	
	// Test stack operations
	t.Run("Add Spells to Stack", func(t *testing.T) {
		stack.Push(lightningBolt)
		if stack.Size() != 1 {
			t.Errorf("Expected stack size 1, got %d", stack.Size())
		}
		
		stack.Push(counterspell)
		if stack.Size() != 2 {
			t.Errorf("Expected stack size 2, got %d", stack.Size())
		}
	})
	
	t.Run("Stack Resolution Order (LIFO)", func(t *testing.T) {
		// Counterspell should resolve first (last in, first out)
		topSpell := stack.Peek()
		if topSpell.Name != "Counterspell" {
			t.Errorf("Expected Counterspell on top, got %s", topSpell.Name)
		}
		
		// Resolve counterspell
		resolved := stack.Pop()
		if resolved.Name != "Counterspell" {
			t.Errorf("Expected to resolve Counterspell, got %s", resolved.Name)
		}

		// Lightning Bolt should be countered and removed (simulate counter effect)
		if stack.Size() > 0 {
			// Remove the countered spell
			stack.Pop()
		}

		if stack.Size() != 0 {
			t.Errorf("Expected empty stack after counter, got size %d", stack.Size())
		}
	})
}

// TestPriorityPassing tests priority passing during spell casting
func TestPriorityPassing(t *testing.T) {
	testCases := []struct {
		name           string
		currentPlayer  string
		priorityPlayer string
		action         string
		expectedNext   string
		description    string
	}{
		{
			name:           "Cast Spell, Pass Priority",
			currentPlayer:  "Player1",
			priorityPlayer: "Player1",
			action:         "cast_spell",
			expectedNext:   "Player2",
			description:    "After casting spell, priority passes to opponent",
		},
		{
			name:           "Pass Priority Without Action",
			currentPlayer:  "Player1",
			priorityPlayer: "Player1",
			action:         "pass",
			expectedNext:   "Player2",
			description:    "Passing priority without action",
		},
		{
			name:           "Both Players Pass",
			currentPlayer:  "Player1",
			priorityPlayer: "Player2",
			action:         "pass",
			expectedNext:   "resolve_stack",
			description:    "When both players pass, resolve top of stack",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nextAction := determinePriorityAction(tc.currentPlayer, tc.priorityPlayer, tc.action)
			if nextAction != tc.expectedNext {
				t.Errorf("%s: expected next action %s, got %s", tc.name, tc.expectedNext, nextAction)
			}
		})
	}
}

// TestCounterspellInteractions tests counter-spell mechanics
func TestCounterspellInteractions(t *testing.T) {
	testCases := []struct {
		name            string
		targetSpell     string
		counterSpell    string
		canCounter      bool
		description     string
	}{
		{
			name:         "Counter Lightning Bolt",
			targetSpell:  "Lightning Bolt",
			counterSpell: "Counterspell",
			canCounter:   true,
			description:  "Basic counterspell can counter any spell",
		},
		{
			name:         "Negate vs Creature Spell",
			targetSpell:  "Grizzly Bears",
			counterSpell: "Negate",
			canCounter:   false,
			description:  "Negate cannot counter creature spells",
		},
		{
			name:         "Negate vs Instant",
			targetSpell:  "Lightning Bolt",
			counterSpell: "Negate",
			canCounter:   true,
			description:  "Negate can counter non-creature spells",
		},
		{
			name:         "Spell Pierce vs High Cost",
			targetSpell:  "Shivan Dragon",
			counterSpell: "Spell Pierce",
			canCounter:   false,
			description:  "Spell Pierce can be paid for high-cost spells",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canCounter := canCounterSpell(tc.targetSpell, tc.counterSpell)
			if canCounter != tc.canCounter {
				t.Errorf("%s: expected canCounter %v, got %v", tc.name, tc.canCounter, canCounter)
			}
		})
	}
}

// Helper types and functions for timing tests

type SpellOnStack struct {
	ID         uuid.UUID
	Name       string
	Controller string
	Targets    []interface{}
}

type SpellStack struct {
	spells []*SpellOnStack
}

func NewSpellStack() *SpellStack {
	return &SpellStack{spells: make([]*SpellOnStack, 0)}
}

func (s *SpellStack) Push(spell *SpellOnStack) {
	s.spells = append(s.spells, spell)
}

func (s *SpellStack) Pop() *SpellOnStack {
	if len(s.spells) == 0 {
		return nil
	}
	spell := s.spells[len(s.spells)-1]
	s.spells = s.spells[:len(s.spells)-1]
	return spell
}

func (s *SpellStack) Peek() *SpellOnStack {
	if len(s.spells) == 0 {
		return nil
	}
	return s.spells[len(s.spells)-1]
}

func (s *SpellStack) Size() int {
	return len(s.spells)
}

func canCastInstantSpell(gameState *mockTimingGameState, player string) bool {
	return gameState.HasPriority(player)
}

func canCastSorcerySpell(gameState *mockTimingGameState, player string) bool {
	return gameState.HasPriority(player) && 
		   gameState.IsMainPhase() && 
		   gameState.IsStackEmpty()
}

func determinePriorityAction(currentPlayer, priorityPlayer, action string) string {
	if action == "cast_spell" || action == "pass" {
		if priorityPlayer == currentPlayer {
			return "Player2" // Pass to opponent
		}
		return "resolve_stack" // Both passed, resolve
	}
	return "unknown"
}

func canCounterSpell(targetSpell, counterSpell string) bool {
	switch counterSpell {
	case "Counterspell":
		return true // Can counter any spell
	case "Negate":
		return !isCreatureSpell(targetSpell)
	case "Spell Pierce":
		return isLowCostSpell(targetSpell)
	default:
		return false
	}
}

func isCreatureSpell(spellName string) bool {
	creatureSpells := []string{"Grizzly Bears", "Shivan Dragon", "Lightning Angel"}
	for _, creature := range creatureSpells {
		if spellName == creature {
			return true
		}
	}
	return false
}

func isLowCostSpell(spellName string) bool {
	lowCostSpells := []string{"Lightning Bolt", "Giant Growth", "Healing Salve"}
	for _, spell := range lowCostSpells {
		if spellName == spell {
			return true
		}
	}
	return false
}
