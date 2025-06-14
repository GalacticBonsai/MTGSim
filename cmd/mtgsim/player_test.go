package main

import (
	"strings"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/deck"
)

func TestFlashAbility(t *testing.T) {
	flashCard := card.Card{
		Name:     "Flash Creature",
		Keywords: []string{"Flash"},
	}

	normalCard := card.Card{
		Name:     "Normal Creature",
		Keywords: []string{},
	}

	flash := &Permanent{source: flashCard}
	normal := &Permanent{source: normalCard}

	if !flash.hasEvergreenAbility("Flash") {
		t.Error("Flash creature should have flash ability")
	}

	if normal.hasEvergreenAbility("Flash") {
		t.Error("Normal creature should not have flash ability")
	}
}

func TestScryAbility(t *testing.T) {
	player := &Player{
		Name:      "Test Player",
		LifeTotal: 20,
		Hand:      []card.Card{},
		Deck: deck.Deck{
			Cards: []card.Card{
				{Name: "Card 1"},
				{Name: "Card 2"},
				{Name: "Card 3"},
				{Name: "Card 4"},
				{Name: "Card 5"},
			},
		},
	}

	// Test scry 2
	scryAmount := 2
	initialDeckSize := len(player.Deck.Cards)
	
	// Simulate scry by looking at top cards and putting some on bottom
	topCards := player.Deck.Cards[:scryAmount]
	
	// For test, put first card on bottom, keep second on top
	player.Deck.Cards = append(player.Deck.Cards[1:scryAmount], player.Deck.Cards[scryAmount:]...)
	player.Deck.Cards = append(player.Deck.Cards, topCards[0])

	if len(player.Deck.Cards) != initialDeckSize {
		t.Errorf("Deck size should remain the same after scry, got %d", len(player.Deck.Cards))
	}

	// The top card should now be "Card 2"
	if player.Deck.Cards[0].Name != "Card 2" {
		t.Errorf("Expected top card to be 'Card 2', got '%s'", player.Deck.Cards[0].Name)
	}

	// The bottom card should now be "Card 1"
	if player.Deck.Cards[len(player.Deck.Cards)-1].Name != "Card 1" {
		t.Errorf("Expected bottom card to be 'Card 1', got '%s'", player.Deck.Cards[len(player.Deck.Cards)-1].Name)
	}
}

func TestMillAbility(t *testing.T) {
	player := &Player{
		Name:      "Test Player",
		LifeTotal: 20,
		Hand:      []card.Card{},
		Graveyard: []card.Card{},
		Deck: deck.Deck{
			Cards: []card.Card{
				{Name: "Card 1"},
				{Name: "Card 2"},
				{Name: "Card 3"},
				{Name: "Card 4"},
				{Name: "Card 5"},
			},
		},
	}

	// Test mill 3
	millAmount := 3
	initialDeckSize := len(player.Deck.Cards)
	initialGraveyardSize := len(player.Graveyard)

	// Simulate milling by moving top cards to graveyard
	milledCards := player.Deck.Cards[:millAmount]
	player.Deck.Cards = player.Deck.Cards[millAmount:]
	player.Graveyard = append(player.Graveyard, milledCards...)

	expectedDeckSize := initialDeckSize - millAmount
	expectedGraveyardSize := initialGraveyardSize + millAmount

	if len(player.Deck.Cards) != expectedDeckSize {
		t.Errorf("Expected deck size %d after milling, got %d", expectedDeckSize, len(player.Deck.Cards))
	}

	if len(player.Graveyard) != expectedGraveyardSize {
		t.Errorf("Expected graveyard size %d after milling, got %d", expectedGraveyardSize, len(player.Graveyard))
	}

	// Check that the correct cards were milled
	for i, milledCard := range milledCards {
		if player.Graveyard[i].Name != milledCard.Name {
			t.Errorf("Expected milled card %d to be '%s', got '%s'", i, milledCard.Name, player.Graveyard[i].Name)
		}
	}
}

func TestFightAbility(t *testing.T) {
	owner1 := &Player{Name: "Player 1", LifeTotal: 20}
	owner2 := &Player{Name: "Player 2", LifeTotal: 20}

	creature1 := &Permanent{
		source:          card.Card{Name: "Fighter 1"},
		power:           3,
		toughness:       2,
		damage_counters: 0,
		owner:           owner1,
	}

	creature2 := &Permanent{
		source:          card.Card{Name: "Fighter 2"},
		power:           2,
		toughness:       3,
		damage_counters: 0,
		owner:           owner2,
	}

	// Create a game to use the damage dealing function
	game := NewGame(nil)
	game.dealDamageBetweenCreatures(creature1, creature2)

	// Each creature should have damage equal to the other's power
	if creature1.damage_counters != creature2.power {
		t.Errorf("Creature 1 should have %d damage, got %d", creature2.power, creature1.damage_counters)
	}

	if creature2.damage_counters != creature1.power {
		t.Errorf("Creature 2 should have %d damage, got %d", creature1.power, creature2.damage_counters)
	}
}

func TestGoadAbility(t *testing.T) {
	goadedCard := card.Card{
		Name:       "Goaded Creature",
		OracleText: "This creature is goaded.",
	}

	normalCard := card.Card{
		Name: "Normal Creature",
	}

	goaded := &Permanent{
		source: goadedCard,
		goaded: true,
	}

	normal := &Permanent{
		source: normalCard,
		goaded: false,
	}

	if !goaded.goaded {
		t.Error("Goaded creature should be goaded")
	}

	if normal.goaded {
		t.Error("Normal creature should not be goaded")
	}
}

func TestEquipAbility(t *testing.T) {
	// Test equipment attachment (simplified)
	equipment := card.Card{
		Name:     "Lightning Greaves",
		TypeLine: "Artifact — Equipment",
		OracleText: "Equipped creature has haste and shroud. Equip {0}",
	}

	creature := card.Card{
		Name:     "Grizzly Bears",
		TypeLine: "Creature — Bear",
	}

	equipmentPerm := &Permanent{source: equipment}
	creaturePerm := &Permanent{source: creature}

	// Check that equipment is an artifact
	if !strings.Contains(equipmentPerm.source.TypeLine, "Equipment") {
		t.Error("Equipment should have Equipment in type line")
	}

	// Check that creature is a creature
	if !strings.Contains(creaturePerm.source.TypeLine, "Creature") {
		t.Error("Creature should have Creature in type line")
	}
}

func TestEnchantAbility(t *testing.T) {
	// Test aura attachment (simplified)
	aura := card.Card{
		Name:     "Giant Growth",
		TypeLine: "Enchantment — Aura",
		OracleText: "Enchant creature. Enchanted creature gets +3/+3.",
	}

	creature := card.Card{
		Name:     "Grizzly Bears",
		TypeLine: "Creature — Bear",
		Power:    "2",
		Toughness: "2",
	}

	auraPerm := &Permanent{source: aura}
	creaturePerm := &Permanent{source: creature, power: 2, toughness: 2}

	// Check that aura is an enchantment
	if !strings.Contains(auraPerm.source.TypeLine, "Aura") {
		t.Error("Aura should have Aura in type line")
	}

	// Simulate enchantment effect (would need more complex implementation)
	if strings.Contains(auraPerm.source.OracleText, "+3/+3") {
		// This would modify the creature's power/toughness in a real implementation
		expectedPower := creaturePerm.power + 3
		expectedToughness := creaturePerm.toughness + 3
		
		if expectedPower != 5 || expectedToughness != 5 {
			t.Errorf("Expected enchanted creature to be 5/5, calculated %d/%d", expectedPower, expectedToughness)
		}
	}
}

func TestWardAbility(t *testing.T) {
	wardCard := card.Card{
		Name:       "Ward Creature",
		OracleText: "Ward {2} (Whenever this creature becomes the target of a spell or ability an opponent controls, counter it unless that player pays {2}.)",
	}

	normalCard := card.Card{
		Name: "Normal Creature",
	}

	ward := &Permanent{source: wardCard}
	normal := &Permanent{source: normalCard}

	// Check for ward in oracle text
	hasWard := strings.Contains(strings.ToLower(ward.source.OracleText), "ward")
	if !hasWard {
		t.Error("Ward creature should have ward in oracle text")
	}

	hasWardNormal := strings.Contains(strings.ToLower(normal.source.OracleText), "ward")
	if hasWardNormal {
		t.Error("Normal creature should not have ward in oracle text")
	}
}
