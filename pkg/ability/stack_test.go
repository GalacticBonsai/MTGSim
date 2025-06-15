// Package ability provides comprehensive stack system testing for MTG simulation.
package ability

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/types"
)

// MockGameState for stack testing
type mockStackGameState struct {
	currentPlayer AbilityPlayer
	activePlayer  AbilityPlayer
	allPlayers    []AbilityPlayer
	isMainPhase   bool
	isCombatPhase bool
}

func (m *mockStackGameState) GetPlayer(name string) AbilityPlayer {
	for _, player := range m.allPlayers {
		if player.GetName() == name {
			return player
		}
	}
	return nil
}

func (m *mockStackGameState) GetAllPlayers() []AbilityPlayer {
	return m.allPlayers
}

func (m *mockStackGameState) GetCurrentPlayer() AbilityPlayer {
	return m.currentPlayer
}

func (m *mockStackGameState) GetActivePlayer() AbilityPlayer {
	return m.activePlayer
}

func (m *mockStackGameState) IsMainPhase() bool {
	return m.isMainPhase
}

func (m *mockStackGameState) IsCombatPhase() bool {
	return m.isCombatPhase
}

func (m *mockStackGameState) CanActivateAbilities() bool {
	return true
}

func (m *mockStackGameState) AddManaToPool(player AbilityPlayer, manaType types.ManaType, amount int) {}
func (m *mockStackGameState) DealDamage(source any, target any, amount int)                {}
func (m *mockStackGameState) DrawCards(player AbilityPlayer, count int)                   {}
func (m *mockStackGameState) GainLife(player AbilityPlayer, amount int)                   {}
func (m *mockStackGameState) LoseLife(player AbilityPlayer, amount int)                   {}

// MockPlayer for testing
type mockStackPlayer struct {
	name string
}

func (m *mockStackPlayer) GetName() string {
	return m.name
}

func (m *mockStackPlayer) PayCost(cost Cost) error {
	return nil
}

func (m *mockStackPlayer) AddCardToHand(card any) {}
func (m *mockStackPlayer) RemoveCardFromHand(card any) bool { return true }
func (m *mockStackPlayer) GetHandSize() int { return 7 }
func (m *mockStackPlayer) GetLifeTotal() int { return 20 }
func (m *mockStackPlayer) SetLifeTotal(life int) {}
func (m *mockStackPlayer) GetManaPool() map[types.ManaType]int { return make(map[types.ManaType]int) }
func (m *mockStackPlayer) AddMana(manaType types.ManaType, amount int) {}
func (m *mockStackPlayer) SpendMana(manaType types.ManaType, amount int) bool { return true }
func (m *mockStackPlayer) CanPayCost(cost Cost) bool { return true }
func (m *mockStackPlayer) GetCreatures() []any { return []any{} }
func (m *mockStackPlayer) GetPermanents() []any { return []any{} }
func (m *mockStackPlayer) AddPermanent(permanent any) {}
func (m *mockStackPlayer) RemovePermanent(permanent any) bool { return true }
func (m *mockStackPlayer) GetHand() []any { return []any{} }
func (m *mockStackPlayer) GetLands() []any { return []any{} }

// TestStackBasicOperations tests basic stack operations
func TestStackBasicOperations(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)

	// Test empty stack
	if !stack.IsEmpty() {
		t.Error("New stack should be empty")
	}

	if stack.Size() != 0 {
		t.Errorf("Empty stack size should be 0, got %d", stack.Size())
	}

	if stack.Peek() != nil {
		t.Error("Peek on empty stack should return nil")
	}

	// Create test spell
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

	player := &mockStackPlayer{name: "Player1"}

	// Test adding spell to stack
	stack.AddSpell(spell, player, []interface{}{})

	if stack.IsEmpty() {
		t.Error("Stack should not be empty after adding spell")
	}

	if stack.Size() != 1 {
		t.Errorf("Stack size should be 1, got %d", stack.Size())
	}

	// Test peek
	topItem := stack.Peek()
	if topItem == nil {
		t.Error("Peek should return the top item")
		return
	}

	if topItem.Spell == nil {
		t.Error("Top item should have a spell")
		return
	}

	if topItem.Spell.Name != "Lightning Bolt" {
		t.Errorf("Expected Lightning Bolt, got %s", topItem.Spell.Name)
	}

	// Test pop
	poppedItem := stack.Pop()
	if poppedItem.Spell.Name != "Lightning Bolt" {
		t.Errorf("Expected Lightning Bolt, got %s", poppedItem.Spell.Name)
	}

	if !stack.IsEmpty() {
		t.Error("Stack should be empty after popping last item")
	}
}

// TestStackLIFOOrder tests that stack follows LIFO order
func TestStackLIFOOrder(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)

	player := &mockStackPlayer{name: "Player1"}

	// Add multiple spells
	spells := []*Spell{
		{ID: uuid.New(), Name: "Lightning Bolt", TypeLine: "Instant"},
		{ID: uuid.New(), Name: "Counterspell", TypeLine: "Instant"},
		{ID: uuid.New(), Name: "Giant Growth", TypeLine: "Instant"},
	}

	for _, spell := range spells {
		stack.AddSpell(spell, player, []interface{}{})
	}

	if stack.Size() != 3 {
		t.Errorf("Expected stack size 3, got %d", stack.Size())
	}

	// Pop in LIFO order
	expectedOrder := []string{"Giant Growth", "Counterspell", "Lightning Bolt"}
	for i, expectedName := range expectedOrder {
		item := stack.Pop()
		if item.Spell.Name != expectedName {
			t.Errorf("Expected %s at position %d, got %s", expectedName, i, item.Spell.Name)
		}
	}

	if !stack.IsEmpty() {
		t.Error("Stack should be empty after popping all items")
	}
}

// TestStackWithAbilities tests adding abilities to the stack
func TestStackWithAbilities(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)

	player := &mockStackPlayer{name: "Player1"}

	// Create test ability
	ability := &Ability{
		ID:   uuid.New(),
		Name: "Tap for Mana",
		Type: Activated,
		Effects: []Effect{
			{
				Type:  AddMana,
				Value: 1,
			},
		},
	}

	// Add ability to stack
	stack.AddAbility(ability, player, []interface{}{})

	if stack.Size() != 1 {
		t.Errorf("Expected stack size 1, got %d", stack.Size())
	}

	topItem := stack.Peek()
	if topItem.Type != StackItemAbility {
		t.Error("Top item should be an ability")
	}

	if topItem.Ability.Name != "Tap for Mana" {
		t.Errorf("Expected 'Tap for Mana', got %s", topItem.Ability.Name)
	}
}

// TestStackMixedItems tests stack with both spells and abilities
func TestStackMixedItems(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)

	player := &mockStackPlayer{name: "Player1"}

	// Add spell
	spell := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
	}
	stack.AddSpell(spell, player, []interface{}{})

	// Add ability
	ability := &Ability{
		ID:   uuid.New(),
		Name: "Activated Ability",
		Type: Activated,
	}
	stack.AddAbility(ability, player, []interface{}{})

	// Add another spell
	spell2 := &Spell{
		ID:       uuid.New(),
		Name:     "Counterspell",
		TypeLine: "Instant",
	}
	stack.AddSpell(spell2, player, []interface{}{})

	if stack.Size() != 3 {
		t.Errorf("Expected stack size 3, got %d", stack.Size())
	}

	// Check resolution order (LIFO)
	// Should resolve: Counterspell, Activated Ability, Lightning Bolt
	item1 := stack.Pop()
	if item1.Type != StackItemSpell || item1.Spell.Name != "Counterspell" {
		t.Error("First item should be Counterspell")
	}

	item2 := stack.Pop()
	if item2.Type != StackItemAbility || item2.Ability.Name != "Activated Ability" {
		t.Error("Second item should be Activated Ability")
	}

	item3 := stack.Pop()
	if item3.Type != StackItemSpell || item3.Spell.Name != "Lightning Bolt" {
		t.Error("Third item should be Lightning Bolt")
	}
}

// TestStackCountering tests counterspell mechanics
func TestStackCountering(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)

	player1 := &mockStackPlayer{name: "Player1"}
	player2 := &mockStackPlayer{name: "Player2"}

	// Player1 casts Lightning Bolt
	lightningBolt := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
	}
	stack.AddSpell(lightningBolt, player1, []interface{}{})

	// Player2 counters with Counterspell
	counterspell := &Spell{
		ID:       uuid.New(),
		Name:     "Counterspell",
		TypeLine: "Instant",
	}
	stack.AddSpell(counterspell, player2, []interface{}{})

	if stack.Size() != 2 {
		t.Errorf("Expected stack size 2, got %d", stack.Size())
	}

	// Get the Lightning Bolt from stack to counter it
	items := stack.GetItems()
	var lightningBoltItem *StackItem
	for _, item := range items {
		if item.Spell != nil && item.Spell.Name == "Lightning Bolt" {
			lightningBoltItem = item
			break
		}
	}

	if lightningBoltItem == nil {
		t.Fatal("Lightning Bolt not found on stack")
	}

	// Counter the Lightning Bolt
	err := stack.CounterSpell(lightningBoltItem, player2)
	if err != nil {
		t.Errorf("Failed to counter spell: %v", err)
	}

	if !lightningBoltItem.Countered {
		t.Error("Lightning Bolt should be marked as countered")
	}
}

// TestStackResolution tests basic stack resolution
func TestStackResolution(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	stack := NewStack(gameState, executionEngine)

	player := &mockStackPlayer{name: "Player1"}

	// Add a simple spell
	spell := &Spell{
		ID:       uuid.New(),
		Name:     "Lightning Bolt",
		TypeLine: "Instant",
		Effects: []Effect{
			{
				Type:        DealDamage,
				Value:       3,
				Duration:    Instant,
				Description: "Deal 3 damage",
			},
		},
	}
	stack.AddSpell(spell, player, []interface{}{})

	if stack.Size() != 1 {
		t.Errorf("Expected stack size 1, got %d", stack.Size())
	}

	// Resolve the spell
	err := stack.ResolveTop()
	if err != nil {
		t.Errorf("Failed to resolve spell: %v", err)
	}

	if !stack.IsEmpty() {
		t.Error("Stack should be empty after resolution")
	}
}

// TestStackGetState tests stack state reporting
func TestStackGetState(t *testing.T) {
	gameState := &mockStackGameState{}
	executionEngine := NewExecutionEngine(gameState)
	spellCastingEngine := NewSpellCastingEngine(gameState, executionEngine)

	player := &mockStackPlayer{name: "Player1"}

	// Add some items to stack
	spell1 := &Spell{ID: uuid.New(), Name: "Lightning Bolt", TypeLine: "Instant"}
	spell2 := &Spell{ID: uuid.New(), Name: "Counterspell", TypeLine: "Instant"}

	spellCastingEngine.GetStack().AddSpell(spell1, player, []interface{}{})
	spellCastingEngine.GetStack().AddSpell(spell2, player, []interface{}{})

	state := spellCastingEngine.GetStackState()
	if len(state) != 2 {
		t.Errorf("Expected 2 items in state, got %d", len(state))
	}

	// Check that state shows items in correct order (bottom to top)
	if !containsSubstring(state[0], "Lightning Bolt") {
		t.Error("First item should be Lightning Bolt")
	}

	if !containsSubstring(state[1], "Counterspell") {
		t.Error("Second item should be Counterspell")
	}
}
