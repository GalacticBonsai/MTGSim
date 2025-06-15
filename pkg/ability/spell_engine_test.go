// Package ability provides comprehensive spell casting engine testing for MTG simulation.
package ability

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
)

// TestSpellEngineBasic tests basic spell casting functionality
func TestSpellEngineBasic(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Player1"}
	player2 := &mockStackPlayer{name: "Player2"}
	players := []AbilityPlayer{player1, player2}

	spellCastingEngine.SetPlayers(players)
	spellCastingEngine.SetActivePlayer(player1)
	spellCastingEngine.SetPhase("Main Phase")

	// Create Lightning Bolt card
	lightningBolt := card.Card{
		Name:       "Lightning Bolt",
		ManaCost:   "{R}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
	}

	// Cast Lightning Bolt (targeting player2)
	targets := []interface{}{player2}
	err := spellCastingEngine.CastSpell(lightningBolt, player1, targets)
	if err != nil {
		t.Errorf("Failed to cast Lightning Bolt: %v", err)
	}

	// Check stack state
	if spellCastingEngine.IsStackEmpty() {
		t.Error("Stack should not be empty after casting spell")
	}

	stackState := spellCastingEngine.GetStackState()
	if len(stackState) != 1 {
		t.Errorf("Expected 1 item on stack, got %d", len(stackState))
	}

	// Check priority player
	priorityPlayer := spellCastingEngine.GetPriorityPlayer()
	if priorityPlayer.GetName() != "Player1" {
		t.Errorf("Expected Player1 to have priority, got %s", priorityPlayer.GetName())
	}
}

// TestSpellEngineInstantVsSorcery tests timing restrictions for different spell types
func TestSpellEngineInstantVsSorcery(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Player1"}
	players := []AbilityPlayer{player1}

	spellCastingEngine.SetPlayers(players)
	spellCastingEngine.SetActivePlayer(player1)

	// Create instant and sorcery cards
	lightningBolt := card.Card{
		Name:       "Lightning Bolt",
		ManaCost:   "{R}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
	}

	divination := card.Card{
		Name:       "Divination",
		ManaCost:   "{2}{U}",
		CMC:        3,
		TypeLine:   "Sorcery",
		OracleText: "Draw two cards.",
	}

	// Test instant during combat phase
	spellCastingEngine.SetPhase("Combat Phase")
	gameState.isCombatPhase = true
	gameState.isMainPhase = false

	targets := []interface{}{player1} // Target self for testing
	err := spellCastingEngine.CastInstantSpell(lightningBolt, player1, targets)
	if err != nil {
		t.Errorf("Should be able to cast instant during combat: %v", err)
	}

	// Test sorcery during combat phase (should fail)
	err = spellCastingEngine.CastSorcerySpell(divination, player1, []interface{}{})
	if err == nil {
		t.Error("Should not be able to cast sorcery during combat phase")
	}

	// Clear stack
	spellCastingEngine.GetStack().Pop()

	// Test sorcery during main phase with empty stack
	spellCastingEngine.SetPhase("Main Phase")
	gameState.isMainPhase = true
	gameState.isCombatPhase = false

	err = spellCastingEngine.CastSorcerySpell(divination, player1, []interface{}{})
	if err != nil {
		t.Errorf("Should be able to cast sorcery during main phase: %v", err)
	}

	// Test sorcery with non-empty stack (should fail)
	err = spellCastingEngine.CastSorcerySpell(divination, player1, []interface{}{})
	if err == nil {
		t.Error("Should not be able to cast sorcery with non-empty stack")
	}
}

// TestSpellEngineCounterspell tests counterspell mechanics
func TestSpellEngineCounterspell(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Player1"}
	player2 := &mockStackPlayer{name: "Player2"}
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

	// Player1 casts Lightning Bolt (targeting player2)
	targets := []interface{}{player2}
	err := spellCastingEngine.CastSpell(lightningBolt, player1, targets)
	if err != nil {
		t.Errorf("Failed to cast Lightning Bolt: %v", err)
	}

	// Player1 passes priority to Player2
	err = spellCastingEngine.GetPriorityManager().PassPriority(player1)
	if err != nil {
		t.Errorf("Failed to pass priority: %v", err)
	}

	// Get the Lightning Bolt from stack
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

	// Player2 counters Lightning Bolt
	err = spellCastingEngine.CounterSpell(counterspell, player2, lightningBoltItem)
	if err != nil {
		t.Errorf("Failed to cast counterspell: %v", err)
	}

	// Stack should have 2 items
	if spellCastingEngine.GetStack().Size() != 2 {
		t.Errorf("Expected 2 items on stack, got %d", spellCastingEngine.GetStack().Size())
	}

	// Lightning Bolt should NOT be marked as countered yet (counterspell hasn't resolved)
	if lightningBoltItem.Countered {
		t.Error("Lightning Bolt should not be marked as countered yet (counterspell hasn't resolved)")
	}

	// Resolve the stack to see the counterspell effect
	err = spellCastingEngine.ResolveStack()
	if err != nil {
		t.Errorf("Failed to resolve stack: %v", err)
	}

	// Now Lightning Bolt should be marked as countered
	if !lightningBoltItem.Countered {
		t.Error("Lightning Bolt should be marked as countered after counterspell resolves")
	}
}

// TestSpellEngineResolution tests spell resolution
func TestSpellEngineResolution(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Player1"}
	players := []AbilityPlayer{player1}

	spellCastingEngine.SetPlayers(players)
	spellCastingEngine.SetActivePlayer(player1)
	spellCastingEngine.SetPhase("Main Phase")

	// Create Lightning Bolt card
	lightningBolt := card.Card{
		Name:       "Lightning Bolt",
		ManaCost:   "{R}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
	}

	// Cast Lightning Bolt (targeting player1)
	targets := []interface{}{player1}
	err := spellCastingEngine.CastSpell(lightningBolt, player1, targets)
	if err != nil {
		t.Errorf("Failed to cast Lightning Bolt: %v", err)
	}

	// Verify stack has the spell
	if spellCastingEngine.GetStack().Size() != 1 {
		t.Errorf("Expected 1 item on stack, got %d", spellCastingEngine.GetStack().Size())
	}

	// Resolve the stack
	err = spellCastingEngine.ResolveStack()
	if err != nil {
		t.Errorf("Failed to resolve stack: %v", err)
	}

	// Stack should be empty
	if !spellCastingEngine.IsStackEmpty() {
		t.Error("Stack should be empty after resolution")
	}
}

// TestSpellEngineMultipleSpells tests casting multiple spells
func TestSpellEngineMultipleSpells(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Player1"}
	player2 := &mockStackPlayer{name: "Player2"}
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

	// giantGrowth removed since it requires a creature target which we don't have in this test

	healingSalve := card.Card{
		Name:       "Healing Salve",
		ManaCost:   "{W}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Target player gains 3 life.",
	}

	// Cast multiple spells
	// Lightning Bolt targeting player2
	err := spellCastingEngine.CastSpell(lightningBolt, player1, []interface{}{player2})
	if err != nil {
		t.Errorf("Failed to cast Lightning Bolt: %v", err)
	}

	// Giant Growth needs a creature target - skip for now or create a mock creature
	// For now, let's skip this spell to avoid the targeting issue

	// Healing Salve targeting player1
	err = spellCastingEngine.CastSpell(healingSalve, player1, []interface{}{player1})
	if err != nil {
		t.Errorf("Failed to cast Healing Salve: %v", err)
	}

	// Stack should have 2 items (Lightning Bolt and Healing Salve)
	if spellCastingEngine.GetStack().Size() != 2 {
		t.Errorf("Expected 2 items on stack, got %d", spellCastingEngine.GetStack().Size())
	}

	// Check stack state
	stackState := spellCastingEngine.GetStackState()
	if len(stackState) != 2 {
		t.Errorf("Expected 2 items in stack state, got %d", len(stackState))
	}

	// Resolve all spells
	err = spellCastingEngine.ResolveStack()
	if err != nil {
		t.Errorf("Failed to resolve stack: %v", err)
	}

	// Stack should be empty
	if !spellCastingEngine.IsStackEmpty() {
		t.Error("Stack should be empty after resolving all spells")
	}
}

// TestSpellEngineAbilityActivation tests activating abilities through the spell casting engine
func TestSpellEngineAbilityActivation(t *testing.T) {
	gameState := &mockStackGameState{isMainPhase: true}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Player1"}
	players := []AbilityPlayer{player1}

	spellCastingEngine.SetPlayers(players)
	spellCastingEngine.SetActivePlayer(player1)
	spellCastingEngine.SetPhase("Main Phase")

	// Create an activated ability
	ability := &Ability{
		Name: "Activated Ability",
		Type: Activated,
		Effects: []Effect{
			{
				Type:  DealDamage,
				Value: 1,
			},
		},
		TimingRestriction: AnyTime,
	}

	// Activate the ability
	err := spellCastingEngine.ActivateAbility(ability, player1, []interface{}{})
	if err != nil {
		t.Errorf("Failed to activate ability: %v", err)
	}

	// Stack should have the ability
	if spellCastingEngine.GetStack().Size() != 1 {
		t.Errorf("Expected 1 item on stack, got %d", spellCastingEngine.GetStack().Size())
	}

	// Top item should be an ability
	topItem := spellCastingEngine.GetStack().Peek()
	if topItem.Type != StackItemAbility {
		t.Error("Top item should be an ability")
	}

	if topItem.Ability.Name != "Activated Ability" {
		t.Errorf("Expected 'Activated Ability', got %s", topItem.Ability.Name)
	}
}
