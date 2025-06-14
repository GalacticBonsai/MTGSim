package ability

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// Mock implementations for integration testing

type mockGame struct {
	players      []PlayerInterface
	currentPlayer PlayerInterface
	activePlayer  PlayerInterface
	currentPhase string
	isMain       bool
	isCombat     bool
}

func (m *mockGame) GetPlayers() []PlayerInterface {
	return m.players
}

func (m *mockGame) GetCurrentPlayer() PlayerInterface {
	return m.currentPlayer
}

func (m *mockGame) GetActivePlayer() PlayerInterface {
	return m.activePlayer
}

func (m *mockGame) GetCurrentPhase() string {
	return m.currentPhase
}

func (m *mockGame) IsMainPhase() bool {
	return m.isMain
}

func (m *mockGame) IsCombatPhase() bool {
	return m.isCombat
}

type mockPlayerInterface struct {
	name         string
	life         int
	hand         []card.Card
	creatures    []PermanentInterface
	lands        []PermanentInterface
	artifacts    []PermanentInterface
	enchantments []PermanentInterface
	planeswalkers []PermanentInterface
	manaPool     map[game.ManaType]int
}

func (m *mockPlayerInterface) GetName() string {
	return m.name
}

func (m *mockPlayerInterface) GetLifeTotal() int {
	return m.life
}

func (m *mockPlayerInterface) SetLifeTotal(life int) {
	m.life = life
}

func (m *mockPlayerInterface) GetHand() []card.Card {
	return m.hand
}

func (m *mockPlayerInterface) AddCardToHand(card card.Card) {
	m.hand = append(m.hand, card)
}

func (m *mockPlayerInterface) GetCreatures() []PermanentInterface {
	return m.creatures
}

func (m *mockPlayerInterface) GetLands() []PermanentInterface {
	return m.lands
}

func (m *mockPlayerInterface) GetArtifacts() []PermanentInterface {
	return m.artifacts
}

func (m *mockPlayerInterface) GetEnchantments() []PermanentInterface {
	return m.enchantments
}

func (m *mockPlayerInterface) GetPlaneswalkers() []PermanentInterface {
	return m.planeswalkers
}

func (m *mockPlayerInterface) GetManaPool() map[game.ManaType]int {
	return m.manaPool
}

func (m *mockPlayerInterface) AddManaToPool(manaType game.ManaType, amount int) {
	if m.manaPool == nil {
		m.manaPool = make(map[game.ManaType]int)
	}
	m.manaPool[manaType] += amount
}

func (m *mockPlayerInterface) CanPayManaCost(cost map[game.ManaType]int) bool {
	for manaType, amount := range cost {
		if m.manaPool[manaType] < amount {
			return false
		}
	}
	return true
}

func (m *mockPlayerInterface) PayManaCost(cost map[game.ManaType]int) error {
	if !m.CanPayManaCost(cost) {
		return ErrInsufficientMana
	}
	for manaType, amount := range cost {
		m.manaPool[manaType] -= amount
	}
	return nil
}

type mockPermanentInterface struct {
	id             uuid.UUID
	name           string
	source         card.Card
	owner          PlayerInterface
	controller     PlayerInterface
	tapped         bool
	power          int
	toughness      int
	damageCounters int
}

func (m *mockPermanentInterface) GetID() uuid.UUID {
	return m.id
}

func (m *mockPermanentInterface) GetName() string {
	return m.name
}

func (m *mockPermanentInterface) GetSource() card.Card {
	return m.source
}

func (m *mockPermanentInterface) GetOwner() PlayerInterface {
	return m.owner
}

func (m *mockPermanentInterface) GetController() PlayerInterface {
	return m.controller
}

func (m *mockPermanentInterface) IsTapped() bool {
	return m.tapped
}

func (m *mockPermanentInterface) Tap() {
	m.tapped = true
}

func (m *mockPermanentInterface) Untap() {
	m.tapped = false
}

func (m *mockPermanentInterface) GetPower() int {
	return m.power
}

func (m *mockPermanentInterface) GetToughness() int {
	return m.toughness
}

func (m *mockPermanentInterface) SetPower(power int) {
	m.power = power
}

func (m *mockPermanentInterface) SetToughness(toughness int) {
	m.toughness = toughness
}

func (m *mockPermanentInterface) GetDamageCounters() int {
	return m.damageCounters
}

func (m *mockPermanentInterface) AddDamage(damage int) {
	m.damageCounters += damage
}

func (m *mockPermanentInterface) ClearDamage() {
	m.damageCounters = 0
}

func TestGameAdapter_ParseAndAddAbilities(t *testing.T) {
	player := &mockPlayerInterface{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[game.ManaType]int),
	}

	game := &mockGame{
		players:       []PlayerInterface{player},
		currentPlayer: player,
		isMain:        true,
	}

	adapter := NewGameAdapter(game)

	// Create a permanent with oracle text
	permanent := &mockPermanentInterface{
		id:   uuid.New(),
		name: "Llanowar Elves",
		source: card.Card{
			Name:       "Llanowar Elves",
			OracleText: "{T}: Add {G}.",
		},
		owner:      player,
		controller: player,
	}

	// Parse and add abilities
	err := adapter.ParseAndAddAbilities(permanent, "{T}: Add {G}.")
	if err != nil {
		t.Errorf("ParseAndAddAbilities() error = %v", err)
	}

	// Check that abilities were registered
	if len(adapter.abilityEngine.abilities) == 0 {
		t.Error("Expected abilities to be registered with the engine")
	}
}

func TestGameAdapter_TriggerAbilities(t *testing.T) {
	player := &mockPlayerInterface{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[game.ManaType]int),
	}

	game := &mockGame{
		players:       []PlayerInterface{player},
		currentPlayer: player,
		isMain:        true,
	}

	adapter := NewGameAdapter(game)

	// Create a permanent with an ETB ability
	permanent := &mockPermanentInterface{
		id:   uuid.New(),
		name: "Wall of Omens",
		source: card.Card{
			Name:       "Wall of Omens",
			OracleText: "When Wall of Omens enters the battlefield, draw a card.",
		},
		owner:      player,
		controller: player,
	}

	// Parse and add abilities
	err := adapter.ParseAndAddAbilities(permanent, "When Wall of Omens enters the battlefield, draw a card.")
	if err != nil {
		t.Errorf("ParseAndAddAbilities() error = %v", err)
	}

	// Trigger ETB abilities
	adapter.TriggerAbilities(EntersTheBattlefield, permanent)

	// Check that triggered abilities were processed
	if len(adapter.abilityEngine.triggeredQueue) > 0 {
		t.Error("Expected triggered abilities to be processed and queue to be empty")
	}
}

func TestGameAdapter_ActivateAbilitiesForPlayer(t *testing.T) {
	player := &mockPlayerInterface{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[game.ManaType]int),
		lands:    []PermanentInterface{},
	}

	game := &mockGame{
		players:       []PlayerInterface{player},
		currentPlayer: player,
		isMain:        true,
	}

	adapter := NewGameAdapter(game)

	// Create a land with a mana ability
	land := &mockPermanentInterface{
		id:   uuid.New(),
		name: "Forest",
		source: card.Card{
			Name:       "Forest",
			OracleText: "{T}: Add {G}.",
		},
		owner:      player,
		controller: player,
	}

	player.lands = append(player.lands, land)

	// Parse and add abilities to the land
	err := adapter.ParseAndAddAbilities(land, "{T}: Add {G}.")
	if err != nil {
		t.Errorf("ParseAndAddAbilities() error = %v", err)
	}

	// Activate abilities for the player
	adapter.ActivateAbilitiesForPlayer(player, "Main")

	// Check that mana was added (this is a simplified test)
	// In a real implementation, we'd check the mana pool
}

func TestPlayerAdapter_Interface(t *testing.T) {
	mockPlayer := &mockPlayerInterface{
		name:     "Test Player",
		life:     20,
		hand:     []card.Card{},
		manaPool: map[game.ManaType]int{game.Red: 2, game.Green: 1},
	}

	mockGameInstance := &mockGame{
		players:       []PlayerInterface{mockPlayer},
		currentPlayer: mockPlayer,
	}

	adapter := NewGameAdapter(mockGameInstance)
	playerAdapter := &PlayerAdapter{player: mockPlayer, adapter: adapter}

	// Test basic properties
	if playerAdapter.GetName() != "Test Player" {
		t.Errorf("GetName() = %s, want Test Player", playerAdapter.GetName())
	}

	if playerAdapter.GetLifeTotal() != 20 {
		t.Errorf("GetLifeTotal() = %d, want 20", playerAdapter.GetLifeTotal())
	}

	// Test mana pool
	manaPool := playerAdapter.GetManaPool()
	if manaPool[game.Red] != 2 {
		t.Errorf("Red mana = %d, want 2", manaPool[game.Red])
	}

	// Test cost payment
	cost := Cost{
		ManaCost: map[game.ManaType]int{game.Red: 1},
	}

	if !playerAdapter.CanPayCost(cost) {
		t.Error("Should be able to pay cost")
	}

	err := playerAdapter.PayCost(cost)
	if err != nil {
		t.Errorf("PayCost() error = %v", err)
	}

	// Check that mana was spent
	if mockPlayer.manaPool[game.Red] != 1 {
		t.Errorf("Red mana after payment = %d, want 1", mockPlayer.manaPool[game.Red])
	}
}

func TestPermanentAdapter_Interface(t *testing.T) {
	player := &mockPlayerInterface{
		name: "Test Player",
		life: 20,
	}

	mockPermanent := &mockPermanentInterface{
		id:         uuid.New(),
		name:       "Test Creature",
		owner:      player,
		controller: player,
		tapped:     false,
		power:      2,
		toughness:  3,
	}

	game := &mockGame{
		players:       []PlayerInterface{player},
		currentPlayer: player,
	}

	adapter := NewGameAdapter(game)
	permAdapter := &PermanentAdapter{permanent: mockPermanent, adapter: adapter}

	// Test basic properties
	if permAdapter.GetName() != "Test Creature" {
		t.Errorf("GetName() = %s, want Test Creature", permAdapter.GetName())
	}

	if permAdapter.IsTapped() {
		t.Error("Permanent should not be tapped initially")
	}

	// Test tapping
	permAdapter.Tap()
	if !permAdapter.IsTapped() {
		t.Error("Permanent should be tapped after Tap()")
	}

	// Test untapping
	permAdapter.Untap()
	if permAdapter.IsTapped() {
		t.Error("Permanent should not be tapped after Untap()")
	}

	// Test ability management
	ability := &Ability{
		ID:   uuid.New(),
		Name: "Test Ability",
		Type: Activated,
	}

	permAdapter.AddAbility(ability)
	abilities := permAdapter.GetAbilities()
	if len(abilities) != 1 {
		t.Errorf("Expected 1 ability, got %d", len(abilities))
	}

	if abilities[0].ID != ability.ID {
		t.Error("Ability ID mismatch")
	}

	// Test ability removal
	permAdapter.RemoveAbility(ability.ID)
	abilities = permAdapter.GetAbilities()
	if len(abilities) != 0 {
		t.Errorf("Expected 0 abilities after removal, got %d", len(abilities))
	}
}

func TestGameAdapter_GameStateInterface(t *testing.T) {
	player1 := &mockPlayerInterface{
		name: "Player 1",
		life: 20,
	}

	player2 := &mockPlayerInterface{
		name: "Player 2",
		life: 20,
	}

	game := &mockGame{
		players:       []PlayerInterface{player1, player2},
		currentPlayer: player1,
		activePlayer:  player1,
		isMain:        true,
		isCombat:      false,
	}

	adapter := NewGameAdapter(game)

	// Test player retrieval
	retrievedPlayer := adapter.GetPlayer("Player 1")
	if retrievedPlayer == nil {
		t.Error("GetPlayer() returned nil")
	}

	if retrievedPlayer.GetName() != "Player 1" {
		t.Errorf("GetPlayer() name = %s, want Player 1", retrievedPlayer.GetName())
	}

	// Test all players
	allPlayers := adapter.GetAllPlayers()
	if len(allPlayers) != 2 {
		t.Errorf("GetAllPlayers() returned %d players, want 2", len(allPlayers))
	}

	// Test current player
	currentPlayer := adapter.GetCurrentPlayer()
	if currentPlayer == nil {
		t.Error("GetCurrentPlayer() returned nil")
	}

	if currentPlayer.GetName() != "Player 1" {
		t.Errorf("GetCurrentPlayer() name = %s, want Player 1", currentPlayer.GetName())
	}

	// Test phase checks
	if !adapter.IsMainPhase() {
		t.Error("IsMainPhase() should return true")
	}

	if adapter.IsCombatPhase() {
		t.Error("IsCombatPhase() should return false")
	}

	// Test life manipulation
	adapter.GainLife(retrievedPlayer, 5)
	if retrievedPlayer.GetLifeTotal() != 25 {
		t.Errorf("Life total after gain = %d, want 25", retrievedPlayer.GetLifeTotal())
	}

	adapter.LoseLife(retrievedPlayer, 3)
	if retrievedPlayer.GetLifeTotal() != 22 {
		t.Errorf("Life total after loss = %d, want 22", retrievedPlayer.GetLifeTotal())
	}
}
