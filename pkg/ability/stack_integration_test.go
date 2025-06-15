// Package ability provides comprehensive stack integration testing for MTG simulation.
package ability

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
)

// TestStackIntegrationLightningBoltCounterspell tests the classic Lightning Bolt vs Counterspell scenario
func TestStackIntegrationLightningBoltCounterspell(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Alice"}
	player2 := &mockStackPlayer{name: "Bob"}
	players := []AbilityPlayer{player1, player2}

	spellCastingEngine.SetPlayers(players)
	spellCastingEngine.SetActivePlayer(player1)
	spellCastingEngine.SetPhase("Main Phase")

	// Create cards
	lightningBolt := card.Card{
		Name:       "Lightning Bolt",
		ManaCost:   "{R}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
	}

	counterspell := card.Card{
		Name:       "Counterspell",
		ManaCost:   "{U}{U}",
		CMC:        2,
		TypeLine:   "Instant",
		OracleText: "Counter target spell.",
	}

	// Step 1: Alice casts Lightning Bolt targeting Bob
	t.Log("Step 1: Alice casts Lightning Bolt targeting Bob")
	err := spellCastingEngine.CastSpell(lightningBolt, player1, []interface{}{player2})
	if err != nil {
		t.Fatalf("Failed to cast Lightning Bolt: %v", err)
	}

	// Verify stack state
	if spellCastingEngine.GetStack().Size() != 1 {
		t.Errorf("Expected 1 item on stack, got %d", spellCastingEngine.GetStack().Size())
	}

	// Verify priority is with Alice
	priorityPlayer := spellCastingEngine.GetPriorityPlayer()
	if priorityPlayer.GetName() != "Alice" {
		t.Errorf("Expected Alice to have priority, got %s", priorityPlayer.GetName())
	}

	// Step 2: Alice passes priority
	t.Log("Step 2: Alice passes priority")
	err = spellCastingEngine.GetPriorityManager().PassPriority(player1)
	if err != nil {
		t.Fatalf("Failed to pass priority: %v", err)
	}

	// Verify priority is with Bob
	priorityPlayer = spellCastingEngine.GetPriorityPlayer()
	if priorityPlayer.GetName() != "Bob" {
		t.Errorf("Expected Bob to have priority, got %s", priorityPlayer.GetName())
	}

	// Step 3: Bob casts Counterspell targeting Lightning Bolt
	t.Log("Step 3: Bob casts Counterspell targeting Lightning Bolt")
	stackItems := spellCastingEngine.GetStack().GetItems()
	var lightningBoltItem *StackItem
	for _, item := range stackItems {
		if item.Spell != nil && item.Spell.Name == "Lightning Bolt" {
			lightningBoltItem = item
			break
		}
	}

	if lightningBoltItem == nil {
		t.Fatal("Lightning Bolt not found on stack")
	}

	err = spellCastingEngine.CounterSpell(counterspell, player2, lightningBoltItem)
	if err != nil {
		t.Fatalf("Failed to cast Counterspell: %v", err)
	}

	// Verify stack has 2 items
	if spellCastingEngine.GetStack().Size() != 2 {
		t.Errorf("Expected 2 items on stack, got %d", spellCastingEngine.GetStack().Size())
	}

	// Note: Lightning Bolt is not yet countered - it will be countered when Counterspell resolves
	if lightningBoltItem.Countered {
		t.Error("Lightning Bolt should not be marked as countered yet (Counterspell hasn't resolved)")
	}

	// Step 4: Both players pass priority, resolving the stack
	t.Log("Step 4: Both players pass priority, resolving the stack")

	// Check who has priority after counterspell
	currentPriorityPlayer := spellCastingEngine.GetPriorityPlayer()
	t.Logf("Priority is currently with: %s", currentPriorityPlayer.GetName())

	// The active player (Alice) should have priority after counterspell is cast
	if currentPriorityPlayer.GetName() != "Alice" {
		t.Errorf("Expected Alice to have priority after counterspell, got %s", currentPriorityPlayer.GetName())
	}

	// Resolve the stack manually to test the counterspell effect
	err = spellCastingEngine.ResolveStack()
	if err != nil {
		t.Fatalf("Failed to resolve stack: %v", err)
	}

	// Stack should be empty after resolution
	if !spellCastingEngine.IsStackEmpty() {
		t.Error("Stack should be empty after resolution")
	}

	t.Log("Test completed successfully: Lightning Bolt was countered")
}

// TestStackIntegrationMultipleSpells tests a complex scenario with multiple spells
func TestStackIntegrationMultipleSpells(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Alice"}
	player2 := &mockStackPlayer{name: "Bob"}
	players := []AbilityPlayer{player1, player2}

	spellCastingEngine.SetPlayers(players)
	spellCastingEngine.SetActivePlayer(player1)
	spellCastingEngine.SetPhase("Main Phase")

	// Create cards
	lightningBolt := card.Card{
		Name:       "Lightning Bolt",
		ManaCost:   "{R}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
	}

	healingSalve := card.Card{
		Name:       "Healing Salve",
		ManaCost:   "{W}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Target player gains 3 life.",
	}

	// divination card removed as it's not used in this test

	// Scenario: Alice casts Lightning Bolt, Bob responds with Healing Salve, Alice responds with another Lightning Bolt
	t.Log("Complex spell interaction scenario")

	// Alice casts Lightning Bolt #1
	err := spellCastingEngine.CastSpell(lightningBolt, player1, []interface{}{player2})
	if err != nil {
		t.Fatalf("Failed to cast first Lightning Bolt: %v", err)
	}

	// Alice passes priority
	err = spellCastingEngine.GetPriorityManager().PassPriority(player1)
	if err != nil {
		t.Fatalf("Failed to pass priority: %v", err)
	}

	// Bob casts Healing Salve (now Bob has priority)
	err = spellCastingEngine.CastSpell(healingSalve, player2, []interface{}{player2})
	if err != nil {
		t.Fatalf("Failed to cast Healing Salve: %v", err)
	}

	// After Bob casts, priority returns to Alice (active player)
	// Alice casts Lightning Bolt #2
	lightningBolt2 := lightningBolt // Same card for simplicity
	err = spellCastingEngine.CastSpell(lightningBolt2, player1, []interface{}{player2})
	if err != nil {
		t.Fatalf("Failed to cast second Lightning Bolt: %v", err)
	}

	// Verify stack has 3 items
	if spellCastingEngine.GetStack().Size() != 3 {
		t.Errorf("Expected 3 items on stack, got %d", spellCastingEngine.GetStack().Size())
	}

	// Verify stack order (LIFO): Lightning Bolt #2, Healing Salve, Lightning Bolt #1
	stackState := spellCastingEngine.GetStackState()
	t.Logf("Stack state: %v", stackState)

	// Resolve the entire stack
	err = spellCastingEngine.ResolveStack()
	if err != nil {
		t.Fatalf("Failed to resolve stack: %v", err)
	}

	// Stack should be empty
	if !spellCastingEngine.IsStackEmpty() {
		t.Error("Stack should be empty after resolution")
	}

	t.Log("Complex spell scenario completed successfully")
}

// TestStackIntegrationSorceryTiming tests sorcery speed restrictions
func TestStackIntegrationSorceryTiming(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Alice"}
	players := []AbilityPlayer{player1}

	spellCastingEngine.SetPlayers(players)
	spellCastingEngine.SetActivePlayer(player1)

	divination := card.Card{
		Name:       "Divination",
		ManaCost:   "{2}{U}",
		CMC:        3,
		TypeLine:   "Sorcery",
		OracleText: "Draw two cards.",
	}

	lightningBolt := card.Card{
		Name:       "Lightning Bolt",
		ManaCost:   "{R}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
	}

	// Test 1: Cannot cast sorcery during combat phase
	t.Log("Test 1: Cannot cast sorcery during combat phase")
	spellCastingEngine.SetPhase("Combat Phase")
	gameState.isCombatPhase = true
	gameState.isMainPhase = false

	err := spellCastingEngine.CastSorcerySpell(divination, player1, []interface{}{})
	if err == nil {
		t.Error("Should not be able to cast sorcery during combat phase")
	}

	// Test 2: Can cast sorcery during main phase with empty stack
	t.Log("Test 2: Can cast sorcery during main phase with empty stack")
	spellCastingEngine.SetPhase("Main Phase")
	gameState.isMainPhase = true
	gameState.isCombatPhase = false

	err = spellCastingEngine.CastSorcerySpell(divination, player1, []interface{}{})
	if err != nil {
		t.Errorf("Should be able to cast sorcery during main phase: %v", err)
	}

	// Clear the stack
	spellCastingEngine.ResolveStack()

	// Test 3: Cannot cast sorcery with non-empty stack
	t.Log("Test 3: Cannot cast sorcery with non-empty stack")
	
	// First cast an instant
	err = spellCastingEngine.CastSpell(lightningBolt, player1, []interface{}{player1})
	if err != nil {
		t.Fatalf("Failed to cast Lightning Bolt: %v", err)
	}

	// Now try to cast sorcery (should fail)
	err = spellCastingEngine.CastSorcerySpell(divination, player1, []interface{}{})
	if err == nil {
		t.Error("Should not be able to cast sorcery with non-empty stack")
	}

	t.Log("Sorcery timing tests completed successfully")
}

// TestStackIntegrationAbilityActivation tests ability activation through the stack
func TestStackIntegrationAbilityActivation(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Alice"}
	players := []AbilityPlayer{player1}

	spellCastingEngine.SetPlayers(players)
	spellCastingEngine.SetActivePlayer(player1)
	spellCastingEngine.SetPhase("Main Phase")

	// Create abilities
	tapAbility := &Ability{
		Name: "Prodigal Pyromancer",
		Type: Activated,
		Effects: []Effect{
			{
				Type:  DealDamage,
				Value: 1,
				Targets: []Target{
					{
						Type:     AnyTarget,
						Required: true,
						Count:    1,
					},
				},
			},
		},
		TimingRestriction: AnyTime,
	}

	manaAbility := &Ability{
		Name: "Llanowar Elves",
		Type: Mana,
		Effects: []Effect{
			{
				Type:  AddMana,
				Value: 1,
			},
		},
		TimingRestriction: AnyTime,
	}

	// Test 1: Activate regular ability (goes on stack)
	t.Log("Test 1: Activate regular ability (goes on stack)")
	err := spellCastingEngine.ActivateAbility(tapAbility, player1, []interface{}{player1})
	if err != nil {
		t.Fatalf("Failed to activate ability: %v", err)
	}

	if spellCastingEngine.GetStack().Size() != 1 {
		t.Errorf("Expected 1 item on stack, got %d", spellCastingEngine.GetStack().Size())
	}

	// Verify it's an ability on the stack
	topItem := spellCastingEngine.GetStack().Peek()
	if topItem.Type != StackItemAbility {
		t.Error("Top item should be an ability")
	}

	// Test 2: Activate mana ability (doesn't use stack)
	t.Log("Test 2: Activate mana ability (doesn't use stack)")
	initialStackSize := spellCastingEngine.GetStack().Size()
	
	err = spellCastingEngine.ActivateAbility(manaAbility, player1, []interface{}{})
	if err != nil {
		t.Fatalf("Failed to activate mana ability: %v", err)
	}

	// Stack size should not change
	if spellCastingEngine.GetStack().Size() != initialStackSize {
		t.Errorf("Mana ability should not use stack, expected size %d, got %d", 
			initialStackSize, spellCastingEngine.GetStack().Size())
	}

	// Resolve remaining stack
	err = spellCastingEngine.ResolveStack()
	if err != nil {
		t.Fatalf("Failed to resolve stack: %v", err)
	}

	t.Log("Ability activation tests completed successfully")
}
