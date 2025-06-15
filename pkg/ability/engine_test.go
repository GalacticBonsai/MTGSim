package ability

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/types"
)

// Mock implementations for testing

type mockGameState struct {
	players       []AbilityPlayer
	currentPlayer AbilityPlayer
	isMainPhase   bool
	isCombat      bool
}

func (m *mockGameState) GetPlayer(name string) AbilityPlayer {
	for _, p := range m.players {
		if p.GetName() == name {
			return p
		}
	}
	return nil
}

func (m *mockGameState) GetAllPlayers() []AbilityPlayer {
	return m.players
}

func (m *mockGameState) GetCurrentPlayer() AbilityPlayer {
	return m.currentPlayer
}

func (m *mockGameState) GetActivePlayer() AbilityPlayer {
	return m.currentPlayer
}

func (m *mockGameState) IsMainPhase() bool {
	return m.isMainPhase
}

func (m *mockGameState) IsCombatPhase() bool {
	return m.isCombat
}

func (m *mockGameState) CanActivateAbilities() bool {
	return true
}

func (m *mockGameState) AddManaToPool(player AbilityPlayer, manaType types.ManaType, amount int) {
	if mp, ok := player.(*mockPlayer); ok {
		mp.manaPool[manaType] += amount
	}
}

func (m *mockGameState) DealDamage(source interface{}, target interface{}, amount int) {
	// Deal damage to players
	if player, ok := target.(*mockPlayer); ok {
		player.life -= amount
		if player.life < 0 {
			player.life = 0
		}
	}
	// TODO: Handle damage to creatures when creature system is implemented
}

func (m *mockGameState) DrawCards(player AbilityPlayer, count int) {
	if mp, ok := player.(*mockPlayer); ok {
		for i := 0; i < count; i++ {
			mp.hand = append(mp.hand, "Card")
		}
	}
}

func (m *mockGameState) GainLife(player AbilityPlayer, amount int) {
	if mp, ok := player.(*mockPlayer); ok {
		mp.life += amount
	}
}

func (m *mockGameState) LoseLife(player AbilityPlayer, amount int) {
	if mp, ok := player.(*mockPlayer); ok {
		mp.life -= amount
	}
}

type mockPlayer struct {
	name      string
	life      int
	hand      []interface{}
	manaPool  map[types.ManaType]int
	creatures []interface{}
	lands     []interface{}
}

func (m *mockPlayer) GetName() string {
	return m.name
}

func (m *mockPlayer) GetLifeTotal() int {
	return m.life
}

func (m *mockPlayer) SetLifeTotal(life int) {
	m.life = life
}

func (m *mockPlayer) GetHand() []interface{} {
	return m.hand
}

func (m *mockPlayer) AddCardToHand(card interface{}) {
	m.hand = append(m.hand, card)
}

func (m *mockPlayer) GetCreatures() []interface{} {
	return m.creatures
}

func (m *mockPlayer) GetLands() []interface{} {
	return m.lands
}

func (m *mockPlayer) CanPayCost(cost Cost) bool {
	// Simplified: just check if we have enough total mana
	totalMana := 0
	for _, amount := range m.manaPool {
		totalMana += amount
	}
	
	requiredMana := 0
	for _, amount := range cost.ManaCost {
		requiredMana += amount
	}
	
	return totalMana >= requiredMana
}

func (m *mockPlayer) PayCost(cost Cost) error {
	// Simplified: just remove mana from pool
	for manaType, amount := range cost.ManaCost {
		if m.manaPool[manaType] >= amount {
			m.manaPool[manaType] -= amount
		} else {
			// Try to pay with any mana
			totalMana := 0
			for _, amt := range m.manaPool {
				totalMana += amt
			}
			if totalMana >= amount {
				// Remove mana arbitrarily
				remaining := amount
				for mt, amt := range m.manaPool {
					if remaining <= 0 {
						break
					}
					if amt > 0 {
						take := amt
						if take > remaining {
							take = remaining
						}
						m.manaPool[mt] -= take
						remaining -= take
					}
				}
			} else {
				return ErrInsufficientMana
			}
		}
	}
	return nil
}

func (m *mockPlayer) GetManaPool() map[types.ManaType]int {
	return m.manaPool
}

type mockPermanent struct {
	id            uuid.UUID
	name          string
	owner         AbilityPlayer
	controller    AbilityPlayer
	tapped        bool
	abilities     []*Ability
	summoningSick bool
}

func (m *mockPermanent) GetID() uuid.UUID {
	return m.id
}

func (m *mockPermanent) GetName() string {
	return m.name
}

func (m *mockPermanent) GetOwner() AbilityPlayer {
	return m.owner
}

func (m *mockPermanent) GetController() AbilityPlayer {
	return m.controller
}

func (m *mockPermanent) IsTapped() bool {
	return m.tapped
}

func (m *mockPermanent) Tap() {
	m.tapped = true
}

func (m *mockPermanent) Untap() {
	m.tapped = false
}

func (m *mockPermanent) GetAbilities() []*Ability {
	return m.abilities
}

func (m *mockPermanent) AddAbility(ability *Ability) {
	m.abilities = append(m.abilities, ability)
}

func (m *mockPermanent) RemoveAbility(abilityID uuid.UUID) {
	for i, ability := range m.abilities {
		if ability.ID == abilityID {
			m.abilities = append(m.abilities[:i], m.abilities[i+1:]...)
			break
		}
	}
}

// Implement SummoningSickness interface
func (m *mockPermanent) HasSummoningSickness() bool {
	return m.summoningSick
}

func TestExecutionEngine_ExecuteManaAbility(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[types.ManaType]int),
	}
	
	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}
	
	engine := NewExecutionEngine(gameState)
	
	// Create a mana ability
	ability := &Ability{
		ID:   uuid.New(),
		Name: "Forest Mana",
		Type: Mana,
		Cost: Cost{TapCost: true},
		Effects: []Effect{
			{
				Type:        AddMana,
				Value:       1,
				Duration:    Instant,
				Description: "Add {G}",
			},
		},
		OracleText: "{T}: Add {G}.",
	}
	
	// Execute the ability
	err := engine.ExecuteAbility(ability, player, nil)
	if err != nil {
		t.Errorf("ExecuteAbility() error = %v", err)
	}
	
	// Check that mana was added (we can't easily test the exact type without more complex mocking)
	totalMana := 0
	for _, amount := range player.manaPool {
		totalMana += amount
	}
	
	if totalMana != 1 {
		t.Errorf("Expected 1 mana in pool, got %d", totalMana)
	}
}

func TestExecutionEngine_ExecuteDrawAbility(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		hand:     []interface{}{},
		manaPool: map[types.ManaType]int{types.Any: 3},
	}
	
	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}
	
	engine := NewExecutionEngine(gameState)
	
	// Create a draw ability
	ability := &Ability{
		ID:   uuid.New(),
		Name: "Draw Cards",
		Type: Activated,
		Cost: Cost{
			ManaCost: map[types.ManaType]int{types.Any: 2},
			TapCost:  true,
		},
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       2,
				Duration:    Instant,
				Description: "Draw 2 cards",
			},
		},
		TimingRestriction: SorcerySpeed,
	}
	
	initialHandSize := len(player.hand)
	
	// Execute the ability
	err := engine.ExecuteAbility(ability, player, nil)
	if err != nil {
		t.Errorf("ExecuteAbility() error = %v", err)
	}
	
	// Check that cards were drawn
	if len(player.hand) != initialHandSize+2 {
		t.Errorf("Expected hand size %d, got %d", initialHandSize+2, len(player.hand))
	}
	
	// Check that mana was spent
	if player.manaPool[types.Any] != 1 {
		t.Errorf("Expected 1 mana remaining, got %d", player.manaPool[types.Any])
	}
}

func TestExecutionEngine_CanActivateAbility(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		manaPool: map[types.ManaType]int{types.Any: 2},
	}
	
	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}
	
	engine := NewExecutionEngine(gameState)
	
	tests := []struct {
		name        string
		ability     *Ability
		gameState   *mockGameState
		expected    bool
		description string
	}{
		{
			name: "Can activate in main phase",
			ability: &Ability{
				Type: Activated,
				Cost: Cost{ManaCost: map[types.ManaType]int{types.Any: 1}},
				TimingRestriction: SorcerySpeed,
			},
			gameState: &mockGameState{
				players:       []AbilityPlayer{player},
				currentPlayer: player,
				isMainPhase:   true,
			},
			expected:    true,
			description: "Should be able to activate sorcery speed ability in main phase",
		},
		{
			name: "Cannot activate in combat",
			ability: &Ability{
				Type: Activated,
				Cost: Cost{ManaCost: map[types.ManaType]int{types.Any: 1}},
				TimingRestriction: SorcerySpeed,
			},
			gameState: &mockGameState{
				players:       []AbilityPlayer{player},
				currentPlayer: player,
				isMainPhase:   false,
				isCombat:      true,
			},
			expected:    false,
			description: "Should not be able to activate sorcery speed ability in combat",
		},
		{
			name: "Cannot activate without mana",
			ability: &Ability{
				Type: Activated,
				Cost: Cost{ManaCost: map[types.ManaType]int{types.Any: 5}},
				TimingRestriction: AnyTime,
			},
			gameState: &mockGameState{
				players:       []AbilityPlayer{player},
				currentPlayer: player,
				isMainPhase:   true,
			},
			expected:    false,
			description: "Should not be able to activate ability without enough mana",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine.gameState = tt.gameState
			result := engine.canActivateAbility(tt.ability, player)
			if result != tt.expected {
				t.Errorf("canActivateAbility() = %v, want %v: %s", result, tt.expected, tt.description)
			}
		})
	}
}

func TestExecutionEngine_ParseAndRegisterAbilities(t *testing.T) {
	gameState := &mockGameState{
		isMainPhase: true,
	}
	
	engine := NewExecutionEngine(gameState)
	
	tests := []struct {
		name        string
		oracleText  string
		expectedLen int
	}{
		{
			name:        "Parse mana ability",
			oracleText:  "{T}: Add {G}.",
			expectedLen: 1,
		},
		{
			name:        "Parse ETB ability",
			oracleText:  "When this creature enters the battlefield, draw a card.",
			expectedLen: 1,
		},
		{
			name:        "Parse multiple abilities",
			oracleText:  "{T}: Add {R}. When this creature enters the battlefield, you gain 2 life.",
			expectedLen: 2,
		},
		{
			name:        "Parse no abilities",
			oracleText:  "This creature has flying.",
			expectedLen: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abilities, err := engine.ParseAndRegisterAbilities(tt.oracleText, nil)
			if err != nil {
				t.Errorf("ParseAndRegisterAbilities() error = %v", err)
				return
			}
			
			if len(abilities) != tt.expectedLen {
				t.Errorf("ParseAndRegisterAbilities() got %d abilities, want %d", len(abilities), tt.expectedLen)
			}
			
			// Check that all abilities have valid IDs and are properly set up
			for _, ability := range abilities {
				if ability.ID == uuid.Nil {
					t.Errorf("Ability has nil ID")
				}
				if !ability.ParsedFromText {
					t.Errorf("Ability should be marked as parsed from text")
				}
			}
		})
	}
}

func TestExecutionEngine_ActivateManaAbilities(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[types.ManaType]int),
		lands:    []interface{}{},
	}
	
	land := &mockPermanent{
		id:         uuid.New(),
		name:       "Forest",
		owner:      player,
		controller: player,
		tapped:     false,
		abilities:  []*Ability{},
	}

	// Create a land with a mana ability
	manaAbility := &Ability{
		ID:   uuid.New(),
		Name: "Forest Mana",
		Type: Mana,
		Cost: Cost{TapCost: true},
		Effects: []Effect{
			{
				Type:        AddMana,
				Value:       1,
				Duration:    Instant,
				Description: "Add {G}",
			},
		},
		OracleText: "{T}: Add {G}.",
		Source:     land, // Set the source to the land
	}

	land.abilities = []*Ability{manaAbility}
	
	player.lands = append(player.lands, land)
	
	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}
	
	engine := NewExecutionEngine(gameState)
	
	// Activate mana abilities
	manaAdded := engine.ActivateManaAbilities(player)
	
	if manaAdded != 1 {
		t.Errorf("Expected 1 mana added, got %d", manaAdded)
	}
	
	// Check that the land is now tapped
	if !land.tapped {
		t.Errorf("Expected land to be tapped after activating mana ability")
	}
	
	// Check that mana was added to pool
	totalMana := 0
	for _, amount := range player.manaPool {
		totalMana += amount
	}
	
	if totalMana != 1 {
		t.Errorf("Expected 1 mana in pool, got %d", totalMana)
	}
}
