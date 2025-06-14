package main

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
)

func TestGameCreation(t *testing.T) {
	// Create a mock card database for testing
	cards := []card.Card{
		{Name: "Lightning Bolt", CMC: 1, ManaCost: "{R}", TypeLine: "Instant"},
		{Name: "Mountain", CMC: 0, TypeLine: "Basic Land — Mountain"},
		{Name: "Forest", CMC: 0, TypeLine: "Basic Land — Forest"},
		{Name: "Llanowar Elves", CMC: 1, ManaCost: "{G}", TypeLine: "Creature — Elf Druid", Power: "1", Toughness: "1"},
	}
	
	cardDB := card.NewCardDB(cards)
	if cardDB == nil {
		t.Fatal("Failed to create card database")
	}

	// Test game creation
	game := NewGame(cardDB)
	if game == nil {
		t.Fatal("Failed to create game")
	}

	if len(game.Players) != 0 {
		t.Errorf("Expected 0 players initially, got %d", len(game.Players))
	}

	if game.turnNumber != 1 {
		t.Errorf("Expected turn number 1, got %d", game.turnNumber)
	}
}

func TestPlayerCreation(t *testing.T) {
	player := &Player{
		Name:      "Test Player",
		LifeTotal: 20,
		Hand:      make([]card.Card, 0),
		Graveyard: make([]card.Card, 0),
		Exile:     make([]card.Card, 0),
	}

	if player.Name != "Test Player" {
		t.Errorf("Expected player name 'Test Player', got '%s'", player.Name)
	}

	if player.LifeTotal != 20 {
		t.Errorf("Expected life total 20, got %d", player.LifeTotal)
	}

	if len(player.Hand) != 0 {
		t.Errorf("Expected empty hand, got %d cards", len(player.Hand))
	}
}

func TestPlayerDrawCard(t *testing.T) {
	// Create test cards
	testCards := []card.Card{
		{Name: "Card 1"},
		{Name: "Card 2"},
		{Name: "Card 3"},
	}

	player := &Player{
		Name:          "Test Player",
		LifeTotal:     20,
		Hand:          make([]card.Card, 0),
		Graveyard:     make([]card.Card, 0),
		Exile:         make([]card.Card, 0),
		Creatures:     make([]*Permanent, 0),
		Enchantments:  make([]*Permanent, 0),
		Artifacts:     make([]*Permanent, 0),
		Planeswalkers: make([]*Permanent, 0),
		Lands:         make([]*Permanent, 0),
	}

	// Set up deck
	player.Deck.Cards = testCards

	// Test drawing from non-empty deck using deck's DrawCard method
	initialDeckSize := len(player.Deck.Cards)
	initialHandSize := len(player.Hand)

	drawnCard := player.Deck.DrawCard()
	player.Hand = append(player.Hand, drawnCard)

	if len(player.Deck.Cards) != initialDeckSize-1 {
		t.Errorf("Expected deck size to decrease by 1, got %d", len(player.Deck.Cards))
	}

	if len(player.Hand) != initialHandSize+1 {
		t.Errorf("Expected hand size to increase by 1, got %d", len(player.Hand))
	}

	// Test drawing from empty deck
	player.Deck.Cards = []card.Card{} // Empty the deck

	// Drawing from empty deck should return empty card
	emptyCard := player.Deck.DrawCard()
	if emptyCard.Name != "" {
		t.Errorf("Expected empty card from empty deck, got %s", emptyCard.Name)
	}
}

func TestGameBasicFlow(t *testing.T) {
	// This is a basic integration test to ensure the game can start without crashing
	cards := []card.Card{
		{Name: "Lightning Bolt", CMC: 1, ManaCost: "{R}", TypeLine: "Instant"},
		{Name: "Mountain", CMC: 0, TypeLine: "Basic Land — Mountain"},
	}
	
	cardDB := card.NewCardDB(cards)
	game := NewGame(cardDB)

	// Create two players with simple decks
	player1 := &Player{
		Name:          "Player 1",
		LifeTotal:     20,
		Hand:          make([]card.Card, 0),
		Graveyard:     make([]card.Card, 0),
		Exile:         make([]card.Card, 0),
		Creatures:     make([]*Permanent, 0),
		Enchantments:  make([]*Permanent, 0),
		Artifacts:     make([]*Permanent, 0),
		Planeswalkers: make([]*Permanent, 0),
		Lands:         make([]*Permanent, 0),
	}
	player1.Deck.Cards = []card.Card{
		{Name: "Mountain"}, {Name: "Mountain"}, {Name: "Lightning Bolt"},
	}
	player1.Deck.Name = "Player 1 Deck"

	player2 := &Player{
		Name:          "Player 2",
		LifeTotal:     20,
		Hand:          make([]card.Card, 0),
		Graveyard:     make([]card.Card, 0),
		Exile:         make([]card.Card, 0),
		Creatures:     make([]*Permanent, 0),
		Enchantments:  make([]*Permanent, 0),
		Artifacts:     make([]*Permanent, 0),
		Planeswalkers: make([]*Permanent, 0),
		Lands:         make([]*Permanent, 0),
	}
	player2.Deck.Cards = []card.Card{
		{Name: "Mountain"}, {Name: "Mountain"}, {Name: "Lightning Bolt"},
	}
	player2.Deck.Name = "Player 2 Deck"

	game.Players = []*Player{player1, player2}

	// Test that the game can start and finish without crashing
	winner, loser := game.Start()
	
	if winner == nil || loser == nil {
		t.Errorf("Expected both winner and loser to be non-nil")
	}
	
	if winner == loser {
		t.Errorf("Winner and loser should be different players")
	}
	
	// One of the players should have 0 or negative life
	if winner.LifeTotal <= 0 {
		t.Errorf("Winner should have positive life, got %d", winner.LifeTotal)
	}
	
	if loser.LifeTotal > 0 {
		t.Errorf("Loser should have 0 or negative life, got %d", loser.LifeTotal)
	}
}

func TestEvasionAbilities(t *testing.T) {
	// Test that evasion abilities work correctly
	cards := []card.Card{
		{Name: "Serra Angel", CMC: 5, ManaCost: "{3}{W}{W}", TypeLine: "Creature — Angel", Power: "4", Toughness: "4", Keywords: []string{"Flying", "Vigilance"}},
		{Name: "Grizzly Bears", CMC: 2, ManaCost: "{1}{G}", TypeLine: "Creature — Bear", Power: "2", Toughness: "2"},
		{Name: "Giant Spider", CMC: 4, ManaCost: "{3}{G}", TypeLine: "Creature — Spider", Power: "2", Toughness: "4", Keywords: []string{"Reach"}},
		{Name: "Plains", CMC: 0, TypeLine: "Basic Land — Plains"},
		{Name: "Forest", CMC: 0, TypeLine: "Basic Land — Forest"},
	}

	cardDB := card.NewCardDB(cards)
	_ = NewGame(cardDB) // We don't need the game instance for this test

	// Create flying creature
	flyingCreature := &Permanent{
		source:    cards[0], // Serra Angel
		power:     4,
		toughness: 4,
	}

	// Create ground creature
	groundCreature := &Permanent{
		source:    cards[1], // Grizzly Bears
		power:     2,
		toughness: 2,
	}

	// Create reach creature
	reachCreature := &Permanent{
		source:    cards[2], // Giant Spider
		power:     2,
		toughness: 4,
	}

	// Test: Ground creature cannot block flying creature
	if groundCreature.canBlock(flyingCreature) {
		t.Errorf("Ground creature should not be able to block flying creature")
	}

	// Test: Reach creature can block flying creature
	if !reachCreature.canBlock(flyingCreature) {
		t.Errorf("Reach creature should be able to block flying creature")
	}

	// Test: Flying creature can block flying creature
	if !flyingCreature.canBlock(flyingCreature) {
		t.Errorf("Flying creature should be able to block flying creature")
	}

	// Test: Any creature can block ground creature
	if !groundCreature.canBlock(groundCreature) {
		t.Errorf("Ground creature should be able to block ground creature")
	}
	if !reachCreature.canBlock(groundCreature) {
		t.Errorf("Reach creature should be able to block ground creature")
	}
	if !flyingCreature.canBlock(groundCreature) {
		t.Errorf("Flying creature should be able to block ground creature")
	}
}
