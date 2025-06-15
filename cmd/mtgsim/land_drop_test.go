// Package main provides tests for land drop mechanics
package main

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/deck"
)

// TestOneLandPerTurnRule tests that the one land per turn rule is enforced
func TestOneLandPerTurnRule(t *testing.T) {
	// Create a test deck with multiple lands
	testDeck := deck.Deck{
		Name: "Land Test Deck",
		Cards: []card.Card{
			{Name: "Forest", TypeLine: "Basic Land — Forest"},
			{Name: "Mountain", TypeLine: "Basic Land — Mountain"},
			{Name: "Plains", TypeLine: "Basic Land — Plains"},
			{Name: "Island", TypeLine: "Basic Land — Island"},
			{Name: "Swamp", TypeLine: "Basic Land — Swamp"},
		},
	}

	// Create a game with test players
	cardDB := &card.CardDB{} // Mock card database
	game := NewGame(cardDB)

	// Create a player manually for testing
	player := &Player{
		Name:          testDeck.Name,
		LifeTotal:     20,
		Deck:          testDeck,
		Hand:          make([]card.Card, 0),
		Graveyard:     make([]card.Card, 0),
		Exile:         make([]card.Card, 0),
		Creatures:     make([]*Permanent, 0),
		Enchantments:  make([]*Permanent, 0),
		Artifacts:     make([]*Permanent, 0),
		Planeswalkers: make([]*Permanent, 0),
		Lands:         make([]*Permanent, 0),

		// Initialize land drop tracking
		LandDropsThisTurn:    0,
		LandDropsPerTurn:     1,
		HasPlayedLandThisTurn: false,
	}

	// Give player multiple lands in hand
	player.Hand = []card.Card{
		{Name: "Forest", TypeLine: "Basic Land — Forest"},
		{Name: "Mountain", TypeLine: "Basic Land — Mountain"},
		{Name: "Plains", TypeLine: "Basic Land — Plains"},
	}

	// Reset land drops for new turn
	game.resetLandDrops(player)

	// Verify initial state
	if player.LandDropsThisTurn != 0 {
		t.Errorf("Expected 0 land drops at start of turn, got %d", player.LandDropsThisTurn)
	}

	if player.LandDropsPerTurn != 1 {
		t.Errorf("Expected 1 land drop per turn, got %d", player.LandDropsPerTurn)
	}

	if player.HasPlayedLandThisTurn {
		t.Error("Expected HasPlayedLandThisTurn to be false at start of turn")
	}

	// Test that player can play first land
	if !game.canPlayLand(player) {
		t.Error("Player should be able to play first land")
	}

	initialHandSize := len(player.Hand)
	initialLandCount := len(player.Lands)

	// Play first land
	game.playLand(player)

	// Verify first land was played
	if len(player.Hand) != initialHandSize-1 {
		t.Errorf("Expected hand size to decrease by 1, got %d (was %d)", len(player.Hand), initialHandSize)
	}

	if len(player.Lands) != initialLandCount+1 {
		t.Errorf("Expected land count to increase by 1, got %d (was %d)", len(player.Lands), initialLandCount)
	}

	if player.LandDropsThisTurn != 1 {
		t.Errorf("Expected 1 land drop used, got %d", player.LandDropsThisTurn)
	}

	if !player.HasPlayedLandThisTurn {
		t.Error("Expected HasPlayedLandThisTurn to be true after playing land")
	}

	// Test that player cannot play second land
	if game.canPlayLand(player) {
		t.Error("Player should not be able to play second land in same turn")
	}

	currentHandSize := len(player.Hand)
	currentLandCount := len(player.Lands)

	// Attempt to play second land (should fail)
	game.playLand(player)

	// Verify second land was not played
	if len(player.Hand) != currentHandSize {
		t.Errorf("Hand size should not change when trying to play second land, got %d (was %d)", len(player.Hand), currentHandSize)
	}

	if len(player.Lands) != currentLandCount {
		t.Errorf("Land count should not change when trying to play second land, got %d (was %d)", len(player.Lands), currentLandCount)
	}

	if player.LandDropsThisTurn != 1 {
		t.Errorf("Land drops should still be 1, got %d", player.LandDropsThisTurn)
	}
}

// TestAdditionalLandDrops tests the additional land drop functionality
func TestAdditionalLandDrops(t *testing.T) {
	// Create a test deck with multiple lands
	testDeck := deck.Deck{
		Name: "Land Test Deck",
		Cards: []card.Card{
			{Name: "Forest", TypeLine: "Basic Land — Forest"},
			{Name: "Mountain", TypeLine: "Basic Land — Mountain"},
		},
	}

	// Create a game with test players
	cardDB := &card.CardDB{} // Mock card database
	game := NewGame(cardDB)

	// Create a player manually for testing
	player := &Player{
		Name:          testDeck.Name,
		LifeTotal:     20,
		Deck:          testDeck,
		Hand:          make([]card.Card, 0),
		Graveyard:     make([]card.Card, 0),
		Exile:         make([]card.Card, 0),
		Creatures:     make([]*Permanent, 0),
		Enchantments:  make([]*Permanent, 0),
		Artifacts:     make([]*Permanent, 0),
		Planeswalkers: make([]*Permanent, 0),
		Lands:         make([]*Permanent, 0),

		// Initialize land drop tracking
		LandDropsThisTurn:    0,
		LandDropsPerTurn:     1,
		HasPlayedLandThisTurn: false,
	}

	// Give player multiple lands in hand
	player.Hand = []card.Card{
		{Name: "Forest", TypeLine: "Basic Land — Forest"},
		{Name: "Mountain", TypeLine: "Basic Land — Mountain"},
	}

	// Reset land drops for new turn
	game.resetLandDrops(player)

	// Grant additional land drop
	game.grantAdditionalLandDrop(player)

	// Verify player now has 2 land drops available
	if player.LandDropsPerTurn != 2 {
		t.Errorf("Expected 2 land drops per turn after granting additional, got %d", player.LandDropsPerTurn)
	}

	// Play first land
	game.playLand(player)

	// Verify player can still play another land
	if !game.canPlayLand(player) {
		t.Error("Player should be able to play second land after being granted additional land drop")
	}

	// Play second land
	game.playLand(player)

	// Verify both lands were played
	if player.LandDropsThisTurn != 2 {
		t.Errorf("Expected 2 land drops used, got %d", player.LandDropsThisTurn)
	}

	if len(player.Lands) != 2 {
		t.Errorf("Expected 2 lands in play, got %d", len(player.Lands))
	}

	// Verify player cannot play third land
	if game.canPlayLand(player) {
		t.Error("Player should not be able to play third land even with additional land drop")
	}
}

// TestLandDropReset tests that land drops reset properly at start of turn
func TestLandDropReset(t *testing.T) {
	// Create a test deck
	testDeck := deck.Deck{
		Name: "Land Test Deck",
		Cards: []card.Card{
			{Name: "Forest", TypeLine: "Basic Land — Forest"},
		},
	}

	// Create a game with test players
	cardDB := &card.CardDB{} // Mock card database
	game := NewGame(cardDB)

	// Create a player manually for testing
	player := &Player{
		Name:          testDeck.Name,
		LifeTotal:     20,
		Deck:          testDeck,
		Hand:          make([]card.Card, 0),
		Graveyard:     make([]card.Card, 0),
		Exile:         make([]card.Card, 0),
		Creatures:     make([]*Permanent, 0),
		Enchantments:  make([]*Permanent, 0),
		Artifacts:     make([]*Permanent, 0),
		Planeswalkers: make([]*Permanent, 0),
		Lands:         make([]*Permanent, 0),

		// Initialize land drop tracking
		LandDropsThisTurn:    0,
		LandDropsPerTurn:     1,
		HasPlayedLandThisTurn: false,
	}

	// Simulate end of previous turn with modified state
	player.LandDropsThisTurn = 1
	player.LandDropsPerTurn = 2 // Simulate effect that granted extra land drop
	player.HasPlayedLandThisTurn = true

	// Reset for new turn
	game.resetLandDrops(player)

	// Verify land drops reset
	if player.LandDropsThisTurn != 0 {
		t.Errorf("Expected land drops to reset to 0, got %d", player.LandDropsThisTurn)
	}

	if player.HasPlayedLandThisTurn {
		t.Error("Expected HasPlayedLandThisTurn to reset to false")
	}

	// Note: LandDropsPerTurn should be reset in endStep, not resetLandDrops
	// This allows effects to persist through the turn but reset at end of turn

	// Simulate end step
	game.endStep(player)

	// Verify land drops per turn reset to default
	if player.LandDropsPerTurn != 1 {
		t.Errorf("Expected land drops per turn to reset to 1 at end of turn, got %d", player.LandDropsPerTurn)
	}
}

// TestHasLandDropAvailable tests the helper function
func TestHasLandDropAvailable(t *testing.T) {
	// Create a test deck
	testDeck := deck.Deck{
		Name: "Land Test Deck",
		Cards: []card.Card{
			{Name: "Forest", TypeLine: "Basic Land — Forest"},
		},
	}

	// Create a game with test players
	cardDB := &card.CardDB{} // Mock card database
	game := NewGame(cardDB)

	// Create a player manually for testing
	player := &Player{
		Name:          testDeck.Name,
		LifeTotal:     20,
		Deck:          testDeck,
		Hand:          make([]card.Card, 0),
		Graveyard:     make([]card.Card, 0),
		Exile:         make([]card.Card, 0),
		Creatures:     make([]*Permanent, 0),
		Enchantments:  make([]*Permanent, 0),
		Artifacts:     make([]*Permanent, 0),
		Planeswalkers: make([]*Permanent, 0),
		Lands:         make([]*Permanent, 0),

		// Initialize land drop tracking
		LandDropsThisTurn:    0,
		LandDropsPerTurn:     1,
		HasPlayedLandThisTurn: false,
	}

	// Reset for new turn
	game.resetLandDrops(player)

	// Should have land drop available initially
	if !game.hasLandDropAvailable(player) {
		t.Error("Player should have land drop available at start of turn")
	}

	// Use the land drop
	player.LandDropsThisTurn = 1

	// Should not have land drop available after using it
	if game.hasLandDropAvailable(player) {
		t.Error("Player should not have land drop available after using it")
	}

	// Grant additional land drop
	game.grantAdditionalLandDrop(player)

	// Should have land drop available again
	if !game.hasLandDropAvailable(player) {
		t.Error("Player should have land drop available after being granted additional")
	}
}
