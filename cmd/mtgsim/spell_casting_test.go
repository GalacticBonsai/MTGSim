package main

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/types"
)

func TestManaGeneration(t *testing.T) {
	// Create a test game
	cardDB := &card.CardDB{}
	game := NewGame(cardDB)

	// Create a test player
	player := &Player{
		Name:     "Test Player",
		ManaPool: card.NewManaPool(),
		Lands: []*Permanent{
			{
				source:       card.Card{Name: "Forest"},
				manaProducer: true,
				manaTypes:    []types.ManaType{types.Green},
				tapped:       false,
			},
			{
				source:       card.Card{Name: "Mountain"},
				manaProducer: true,
				manaTypes:    []types.ManaType{types.Red},
				tapped:       false,
			},
		},
	}

	// Generate mana
	game.generateMana(player)

	// Check that mana was generated
	if player.ManaPool.Get(types.Green) != 1 {
		t.Errorf("Expected 1 green mana, got %d", player.ManaPool.Get(types.Green))
	}
	if player.ManaPool.Get(types.Red) != 1 {
		t.Errorf("Expected 1 red mana, got %d", player.ManaPool.Get(types.Red))
	}
	if player.ManaPool.Total() != 2 {
		t.Errorf("Expected 2 total mana, got %d", player.ManaPool.Total())
	}

	// Check that lands are tapped
	for _, land := range player.Lands {
		if !land.tapped {
			t.Errorf("Land %s should be tapped after generating mana", land.source.Name)
		}
	}
}

func TestManaDeduction(t *testing.T) {
	// Create a mana pool with some mana
	manaPool := card.NewManaPool()
	manaPool.Add(types.Red, 2)
	manaPool.Add(types.Green, 1)
	manaPool.Add(types.Colorless, 3) // Use colorless instead of Any

	// Test paying for a spell that costs {1}{R}
	cost := card.ParseManaCost("{1}{R}")

	// Should be able to pay
	if !manaPool.CanPay(cost) {
		t.Error("Should be able to pay {1}{R} with available mana")
	}

	// Pay the cost
	err := manaPool.Pay(cost)
	if err != nil {
		t.Errorf("Failed to pay mana cost: %v", err)
	}

	// Check remaining mana
	if manaPool.Get(types.Red) != 1 {
		t.Errorf("Expected 1 red mana remaining, got %d", manaPool.Get(types.Red))
	}
	if manaPool.Total() != 4 { // Should have 1R + 1G + 2 colorless remaining
		t.Errorf("Expected 4 total mana remaining, got %d", manaPool.Total())
	}
}

func TestSpellCasting(t *testing.T) {
	// Create a test game
	cardDB := &card.CardDB{}
	game := NewGame(cardDB)

	// Create a test player with mana and spells
	player := &Player{
		Name:     "Test Player",
		ManaPool: card.NewManaPool(),
		Hand: []card.Card{
			{
				Name:     "Lightning Bolt",
				ManaCost: "{R}",
				CMC:      1,
				TypeLine: "Instant",
			},
			{
				Name:     "Grizzly Bears",
				ManaCost: "{1}{G}",
				CMC:      2,
				TypeLine: "Creature — Bear",
				Power:    "2",
				Toughness: "2",
			},
		},
		Graveyard: []card.Card{},
		Creatures: []*Permanent{},
		Opponents: []*Player{
			{
				Name:      "Opponent",
				LifeTotal: 20,
			},
		},
	}

	// Add mana to cast spells
	player.ManaPool.Add(types.Red, 1)
	player.ManaPool.Add(types.Green, 1)
	player.ManaPool.Add(types.Colorless, 1)

	// Cast spells
	game.currentPhase = "Main"
	game.castSpells(player)

	// Check that Lightning Bolt was cast (should be in graveyard)
	if len(player.Graveyard) != 1 {
		t.Errorf("Expected 1 card in graveyard, got %d", len(player.Graveyard))
	}
	if len(player.Graveyard) > 0 && player.Graveyard[0].Name != "Lightning Bolt" {
		t.Errorf("Expected Lightning Bolt in graveyard, got %s", player.Graveyard[0].Name)
	}

	// Check that Grizzly Bears was cast (should be a creature)
	if len(player.Creatures) != 1 {
		t.Errorf("Expected 1 creature, got %d", len(player.Creatures))
	}
	if len(player.Creatures) > 0 && player.Creatures[0].source.Name != "Grizzly Bears" {
		t.Errorf("Expected Grizzly Bears creature, got %s", player.Creatures[0].source.Name)
	}

	// Check that hand is empty
	if len(player.Hand) != 0 {
		t.Errorf("Expected empty hand, got %d cards", len(player.Hand))
	}

	// Check that opponent took damage from Lightning Bolt
	if player.Opponents[0].LifeTotal != 17 { // 20 - 3 = 17
		t.Errorf("Expected opponent to have 17 life, got %d", player.Opponents[0].LifeTotal)
	}
}

func TestCannotCastWithoutMana(t *testing.T) {
	// Create a test game
	cardDB := &card.CardDB{}
	game := NewGame(cardDB)

	// Create a test player with no mana but with spells
	player := &Player{
		Name:     "Test Player",
		ManaPool: card.NewManaPool(), // Empty mana pool
		Hand: []card.Card{
			{
				Name:     "Lightning Bolt",
				ManaCost: "{R}",
				CMC:      1,
				TypeLine: "Instant",
			},
		},
		Graveyard: []card.Card{},
	}

	// Try to cast spells (should fail)
	game.castSpells(player)

	// Check that nothing was cast
	if len(player.Graveyard) != 0 {
		t.Errorf("Expected empty graveyard, got %d cards", len(player.Graveyard))
	}
	if len(player.Hand) != 1 {
		t.Errorf("Expected 1 card in hand, got %d", len(player.Hand))
	}
}

func TestColoredManaRequirements(t *testing.T) {
	// Create a mana pool with wrong colors
	manaPool := card.NewManaPool()
	manaPool.Add(types.Blue, 2)
	manaPool.Add(types.White, 1)

	// Test paying for a spell that costs {R}{R} (double red)
	cost := card.ParseManaCost("{R}{R}")
	
	// Should NOT be able to pay (no red mana)
	if manaPool.CanPay(cost) {
		t.Error("Should NOT be able to pay {R}{R} with only blue and white mana")
	}

	// Add red mana
	manaPool.Add(types.Red, 2)

	// Now should be able to pay
	if !manaPool.CanPay(cost) {
		t.Error("Should be able to pay {R}{R} with 2 red mana")
	}
}

func TestSummoningSickness(t *testing.T) {
	// Create a test game
	cardDB := &card.CardDB{}
	game := NewGame(cardDB)

	// Create a test player with a creature
	player := &Player{
		Name:     "Test Player",
		ManaPool: card.NewManaPool(),
		Hand: []card.Card{
			{
				Name:     "Grizzly Bears",
				ManaCost: "{1}{G}",
				CMC:      2,
				TypeLine: "Creature — Bear",
				Power:    "2",
				Toughness: "2",
			},
		},
		Creatures: []*Permanent{},
		Opponents: []*Player{
			{
				Name:      "Opponent",
				LifeTotal: 20,
				Creatures: []*Permanent{},
			},
		},
	}

	// Add mana to cast the creature
	player.ManaPool.Add(types.Green, 1)
	player.ManaPool.Add(types.Colorless, 1)

	// Cast the creature
	game.currentPhase = "Main"
	game.castSpells(player)

	// Check that creature was cast
	if len(player.Creatures) != 1 {
		t.Errorf("Expected 1 creature, got %d", len(player.Creatures))
	}

	creature := player.Creatures[0]

	// Check that creature has summoning sickness
	if !creature.summoningSickness {
		t.Error("Creature should have summoning sickness when first cast")
	}

	// Try to attack - should not be able to due to summoning sickness
	attackers := game.declareAttackers(player)
	if len(attackers) != 0 {
		t.Errorf("Creature with summoning sickness should not be able to attack, but %d attackers were declared", len(attackers))
	}

	// Simulate untap step (beginning of next turn)
	game.untapStep(player)

	// Check that summoning sickness is removed
	if creature.summoningSickness {
		t.Error("Creature should not have summoning sickness after untap step")
	}

	// Now should be able to attack
	attackers = game.declareAttackers(player)
	if len(attackers) != 1 {
		t.Errorf("Creature without summoning sickness should be able to attack, but %d attackers were declared", len(attackers))
	}
}

func TestHasteBypassesSummoningSickness(t *testing.T) {
	// Create a test game
	cardDB := &card.CardDB{}
	game := NewGame(cardDB)

	// Create a test player with a haste creature
	player := &Player{
		Name:     "Test Player",
		ManaPool: card.NewManaPool(),
		Hand: []card.Card{
			{
				Name:     "Lightning Elemental",
				ManaCost: "{3}{R}",
				CMC:      4,
				TypeLine: "Creature — Elemental",
				Power:    "4",
				Toughness: "1",
				Keywords: []string{"Haste"},
			},
		},
		Creatures: []*Permanent{},
		Opponents: []*Player{
			{
				Name:      "Opponent",
				LifeTotal: 20,
				Creatures: []*Permanent{},
			},
		},
	}

	// Add mana to cast the creature
	player.ManaPool.Add(types.Red, 1)
	player.ManaPool.Add(types.Colorless, 3)

	// Cast the creature
	game.currentPhase = "Main"
	game.castSpells(player)

	// Check that creature was cast
	if len(player.Creatures) != 1 {
		t.Errorf("Expected 1 creature, got %d", len(player.Creatures))
	}

	creature := player.Creatures[0]

	// Check that creature has summoning sickness (it should, even with haste)
	if !creature.summoningSickness {
		t.Error("Creature should have summoning sickness when first cast, even with haste")
	}

	// Try to attack - should be able to due to haste
	attackers := game.declareAttackers(player)
	if len(attackers) != 1 {
		t.Errorf("Creature with haste should be able to attack despite summoning sickness, but %d attackers were declared", len(attackers))
	}
}
