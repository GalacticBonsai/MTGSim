package main

import (
	"testing"
)

func TestFlashAbility(t *testing.T) {
	card := Card{Name: "Ambush Viper", Keywords: []string{"Flash"}}
	player := &Player{Name: "P1"}
	player.Hand = append(player.Hand, card)
	// Simulate casting with Flash (should log, not restrict timing)
	player.CastSpell(&card, nil)
	// No assert: just ensure no panic and log output
}

func TestScryAbility(t *testing.T) {
	card := Card{Name: "Opt", Keywords: []string{"Scry"}}
	player := &Player{Name: "P1"}
	player.Deck.Cards = []Card{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	player.CastSpell(&card, nil)
	if len(player.Deck.Cards) != 3 {
		t.Errorf("Scry should not remove cards from deck")
	}
}

func TestMillAbility(t *testing.T) {
	card := Card{Name: "Glimpse the Unthinkable", Keywords: []string{"Mill"}}
	opp := &Player{Name: "P2"}
	opp.Deck.Cards = []Card{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	player := &Player{Name: "P1", Opponents: []*Player{opp}}
	player.CastSpell(&card, nil)
	if len(opp.Graveyard) != 3 {
		t.Errorf("Mill should put 3 cards in graveyard, got %d", len(opp.Graveyard))
	}
}

func TestFightAbility(t *testing.T) {
	creature := &Permanent{source: Card{Name: "Bear"}, power: 2, toughness: 2}
	target := &Permanent{source: Card{Name: "Wolf"}, power: 3, toughness: 3}
	creature.Fight(target)
	// Both should have damage counters
	if creature.damage_counters == 3 || target.damage_counters == 2 {
		// pass
	} else {
		t.Errorf("Fight should assign damage to both creatures: got %d and %d", creature.damage_counters, target.damage_counters)
	}
}

func TestGoadAbility(t *testing.T) {
	card := Card{Name: "Disrupt Decorum", Keywords: []string{"Goad"}}
	target := &Permanent{source: Card{Name: "Goblin"}, tokenType: Creature}
	player := &Player{Name: "P1"}
	player.CastSpell(&card, target)
	if !target.goaded {
		t.Errorf("Goad should set goaded to true on target creature")
	}
}

func TestEquipAbility(t *testing.T) {
	card := Card{Name: "Sword of Fire and Ice", TypeLine: "Artifact - Equipment", Keywords: []string{"Equip"}}
	target := &Permanent{source: Card{Name: "Knight"}, tokenType: Creature}
	player := &Player{Name: "P1"}
	player.CastSpell(&card, target)
	// No assert: just ensure no panic and log output
}

func TestEnchantAbility(t *testing.T) {
	card := Card{Name: "Pacifism", TypeLine: "Enchantment - Aura", Keywords: []string{"Enchant"}}
	target := &Permanent{source: Card{Name: "Orc"}, tokenType: Creature}
	player := &Player{Name: "P1"}
	player.CastSpell(&card, target)
	// No assert: just ensure no panic and log output
}

func TestWardAbility(t *testing.T) {
	card := Card{Name: "Murder"}
	target := &Permanent{source: Card{Name: "Adeline", Keywords: []string{"Ward"}}, tokenType: Creature}
	player := &Player{Name: "P1"}
	player.CastSpell(&card, target)
	if len(player.Graveyard) == 0 {
		t.Errorf("Ward should counter the spell and put it in graveyard")
	}
}
