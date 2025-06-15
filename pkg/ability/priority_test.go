// Package ability provides comprehensive priority system testing for MTG simulation.
package ability

import (
	"testing"

	"github.com/google/uuid"
)

// TestPriorityBasicPassing tests basic priority passing mechanics
func TestPriorityBasicPassing(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)
	priorityManager := NewPriorityManager(stack, gameState)

	player1 := &mockStackPlayer{name: "Player1"}
	player2 := &mockStackPlayer{name: "Player2"}
	players := []AbilityPlayer{player1, player2}

	priorityManager.SetPlayers(players)
	priorityManager.SetActivePlayer(player1)

	// Initially, active player should have priority
	currentPlayer := priorityManager.GetCurrentPlayer()
	if currentPlayer.GetName() != "Player1" {
		t.Errorf("Expected Player1 to have priority, got %s", currentPlayer.GetName())
	}

	// Player1 passes priority
	err := priorityManager.PassPriority(player1)
	if err != nil {
		t.Errorf("Failed to pass priority: %v", err)
	}

	// Priority should move to Player2
	currentPlayer = priorityManager.GetCurrentPlayer()
	if currentPlayer.GetName() != "Player2" {
		t.Errorf("Expected Player2 to have priority, got %s", currentPlayer.GetName())
	}

	// Player2 passes priority
	err = priorityManager.PassPriority(player2)
	if err != nil {
		t.Errorf("Failed to pass priority: %v", err)
	}

	// Since all players passed and stack is empty, priority should return to active player
	currentPlayer = priorityManager.GetCurrentPlayer()
	if currentPlayer.GetName() != "Player1" {
		t.Errorf("Expected priority to return to Player1, got %s", currentPlayer.GetName())
	}
}

// TestPriorityWithSpells tests priority passing with spells on the stack
func TestPriorityWithSpells(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)
	priorityManager := NewPriorityManager(stack, gameState)

	player1 := &mockStackPlayer{name: "Player1"}
	player2 := &mockStackPlayer{name: "Player2"}
	players := []AbilityPlayer{player1, player2}

	priorityManager.SetPlayers(players)
	priorityManager.SetActivePlayer(player1)
	priorityManager.SetPhase("Main Phase")

	// Create a spell
	spell := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
		Effects: []Effect{
			{
				Type:  DealDamage,
				Value: 3,
			},
		},
	}

	// Player1 casts spell
	err := priorityManager.CastSpell(player1, spell, []interface{}{})
	if err != nil {
		t.Errorf("Failed to cast spell: %v", err)
	}

	// Stack should have one item
	if stack.Size() != 1 {
		t.Errorf("Expected stack size 1, got %d", stack.Size())
	}

	// Priority should be with active player (Player1)
	currentPlayer := priorityManager.GetCurrentPlayer()
	if currentPlayer.GetName() != "Player1" {
		t.Errorf("Expected Player1 to have priority after casting, got %s", currentPlayer.GetName())
	}

	// Player1 passes
	err = priorityManager.PassPriority(player1)
	if err != nil {
		t.Errorf("Failed to pass priority: %v", err)
	}

	// Player2 should have priority
	currentPlayer = priorityManager.GetCurrentPlayer()
	if currentPlayer.GetName() != "Player2" {
		t.Errorf("Expected Player2 to have priority, got %s", currentPlayer.GetName())
	}

	// Player2 passes
	err = priorityManager.PassPriority(player2)
	if err != nil {
		t.Errorf("Failed to pass priority: %v", err)
	}

	// Stack should be empty after resolution
	if !stack.IsEmpty() {
		t.Error("Stack should be empty after all players pass")
	}
}

// TestPrioritySpellTiming tests spell timing restrictions
func TestPrioritySpellTiming(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)
	priorityManager := NewPriorityManager(stack, gameState)

	player1 := &mockStackPlayer{name: "Player1"}
	players := []AbilityPlayer{player1}

	priorityManager.SetPlayers(players)
	priorityManager.SetActivePlayer(player1)

	// Test instant spell (should be castable anytime)
	instantSpell := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
	}

	// Should be able to cast instant anytime
	canCast := priorityManager.CanCastSpell(instantSpell, player1)
	if !canCast {
		t.Error("Should be able to cast instant spell")
	}

	// Test sorcery spell timing
	sorcerySpell := &Spell{
		ID:       uuid.New(),
		Name:     "Divination",
		TypeLine: "Sorcery",
	}

	// Should not be able to cast sorcery outside main phase
	priorityManager.SetPhase("Combat Phase")
	canCast = priorityManager.CanCastSpell(sorcerySpell, player1)
	if canCast {
		t.Error("Should not be able to cast sorcery during combat")
	}

	// Should be able to cast sorcery during main phase with empty stack
	priorityManager.SetPhase("Main Phase")
	gameState.isMainPhase = true
	canCast = priorityManager.CanCastSpell(sorcerySpell, player1)
	if !canCast {
		t.Error("Should be able to cast sorcery during main phase")
	}

	// Add something to stack
	stack.AddSpell(instantSpell, player1, []interface{}{})

	// Should not be able to cast sorcery with non-empty stack
	canCast = priorityManager.CanCastSpell(sorcerySpell, player1)
	if canCast {
		t.Error("Should not be able to cast sorcery with non-empty stack")
	}
}

// TestPriorityAbilityActivation tests ability activation timing
func TestPriorityAbilityActivation(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)
	priorityManager := NewPriorityManager(stack, gameState)

	player1 := &mockStackPlayer{name: "Player1"}
	players := []AbilityPlayer{player1}

	priorityManager.SetPlayers(players)
	priorityManager.SetActivePlayer(player1)

	// Test instant-speed ability
	instantAbility := &Ability{
		ID:                uuid.New(),
		Name:              "Instant Ability",
		Type:              Activated,
		TimingRestriction: AnyTime,
	}

	canActivate := priorityManager.CanActivateAbility(instantAbility, player1)
	if !canActivate {
		t.Error("Should be able to activate instant-speed ability")
	}

	// Test sorcery-speed ability
	sorceryAbility := &Ability{
		ID:                uuid.New(),
		Name:              "Sorcery Ability",
		Type:              Activated,
		TimingRestriction: SorcerySpeed,
	}

	// Should not be able to activate during combat
	priorityManager.SetPhase("Combat Phase")
	canActivate = priorityManager.CanActivateAbility(sorceryAbility, player1)
	if canActivate {
		t.Error("Should not be able to activate sorcery-speed ability during combat")
	}

	// Should be able to activate during main phase with empty stack
	priorityManager.SetPhase("Main Phase")
	canActivate = priorityManager.CanActivateAbility(sorceryAbility, player1)
	if !canActivate {
		t.Error("Should be able to activate sorcery-speed ability during main phase")
	}
}

// TestPriorityManaAbilities tests that mana abilities don't use the stack
func TestPriorityManaAbilities(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)
	priorityManager := NewPriorityManager(stack, gameState)

	player1 := &mockStackPlayer{name: "Player1"}
	players := []AbilityPlayer{player1}

	priorityManager.SetPlayers(players)
	priorityManager.SetActivePlayer(player1)

	// Create mana ability
	manaAbility := &Ability{
		ID:   uuid.New(),
		Name: "Tap for Mana",
		Type: Mana,
		Effects: []Effect{
			{
				Type:  AddMana,
				Value: 1,
			},
		},
	}

	initialStackSize := stack.Size()

	// Activate mana ability
	err := priorityManager.ActivateAbility(player1, manaAbility, []interface{}{})
	if err != nil {
		t.Errorf("Failed to activate mana ability: %v", err)
	}

	// Stack size should not change (mana abilities don't use stack)
	if stack.Size() != initialStackSize {
		t.Errorf("Mana ability should not use stack, expected size %d, got %d", 
			initialStackSize, stack.Size())
	}
}

// TestPriorityWrongPlayer tests that only the player with priority can act
func TestPriorityWrongPlayer(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)
	priorityManager := NewPriorityManager(stack, gameState)

	player1 := &mockStackPlayer{name: "Player1"}
	player2 := &mockStackPlayer{name: "Player2"}
	players := []AbilityPlayer{player1, player2}

	priorityManager.SetPlayers(players)
	priorityManager.SetActivePlayer(player1)

	// Player1 has priority initially
	currentPlayer := priorityManager.GetCurrentPlayer()
	if currentPlayer.GetName() != "Player1" {
		t.Errorf("Expected Player1 to have priority, got %s", currentPlayer.GetName())
	}

	// Player2 tries to pass priority (should fail)
	err := priorityManager.PassPriority(player2)
	if err == nil {
		t.Error("Player2 should not be able to pass priority when they don't have it")
	}

	// Player2 tries to cast spell (should fail)
	spell := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
	}

	err = priorityManager.CastSpell(player2, spell, []interface{}{})
	if err == nil {
		t.Error("Player2 should not be able to cast spell when they don't have priority")
	}

	// Player1 should still have priority
	currentPlayer = priorityManager.GetCurrentPlayer()
	if currentPlayer.GetName() != "Player1" {
		t.Errorf("Player1 should still have priority, got %s", currentPlayer.GetName())
	}
}

// TestPriorityMultipleSpells tests priority with multiple spells
func TestPriorityMultipleSpells(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)
	priorityManager := NewPriorityManager(stack, gameState)

	player1 := &mockStackPlayer{name: "Player1"}
	player2 := &mockStackPlayer{name: "Player2"}
	players := []AbilityPlayer{player1, player2}

	priorityManager.SetPlayers(players)
	priorityManager.SetActivePlayer(player1)
	priorityManager.SetPhase("Main Phase")

	// Player1 casts Lightning Bolt
	lightningBolt := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
	}

	err := priorityManager.CastSpell(player1, lightningBolt, []interface{}{})
	if err != nil {
		t.Errorf("Failed to cast Lightning Bolt: %v", err)
	}

	// Player1 passes
	err = priorityManager.PassPriority(player1)
	if err != nil {
		t.Errorf("Failed to pass priority: %v", err)
	}

	// Player2 casts Counterspell
	counterspell := &Spell{
		ID:       uuid.New(),
		Name:     "Counterspell",
		TypeLine: "Instant",
	}

	err = priorityManager.CastSpell(player2, counterspell, []interface{}{})
	if err != nil {
		t.Errorf("Failed to cast Counterspell: %v", err)
	}

	// Stack should have 2 items
	if stack.Size() != 2 {
		t.Errorf("Expected stack size 2, got %d", stack.Size())
	}

	// Top item should be Counterspell
	topItem := stack.Peek()
	if topItem.Spell.Name != "Counterspell" {
		t.Errorf("Expected Counterspell on top, got %s", topItem.Spell.Name)
	}
}
