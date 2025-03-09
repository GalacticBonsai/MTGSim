package main

import (
	"testing"
)

func TestMountainManaProducer(t *testing.T) {
	SetLogLevel(WARN)

	card, exists := cardDB.GetCardByName("Mountain")
	if !exists {
		t.Fatalf("Card 'Mountain' not found in the database")
	}

	isProducer, manaTypes := CheckManaProducer(card.OracleText)
	if !isProducer {
		t.Errorf("Expected 'Mountain' to be a mana producer")
	}
	if len(manaTypes) != 1 || manaTypes[0] != Red {
		t.Errorf("Expected 'Mountain' to produce {R}, got %v", manaTypes)
	}
}

func TestBasicLandsManaProducer(t *testing.T) {
	SetLogLevel(WARN)

	basicLands := map[string]ManaType{
		"Plains":   White,
		"Island":   Blue,
		"Swamp":    Black,
		"Mountain": Red,
		"Forest":   Green,
	}

	for name, manaType := range basicLands {
		card, exists := cardDB.GetCardByName(name)
		if !exists {
			t.Fatalf("Card '%s' not found in the database", name)
		}

		isProducer, manaTypes := CheckManaProducer(card.OracleText)
		if !isProducer {
			t.Errorf("Expected '%s' to be a mana producer", name)
		}
		if len(manaTypes) != 1 || manaTypes[0] != manaType {
			t.Errorf("Expected '%s' to produce {%s}, got %v", name, manaType, manaTypes)
		}
	}
}

func TestBasicDualLandsManaProducer(t *testing.T) {
	SetLogLevel(WARN)

	dualLands := map[string][]ManaType{
		"Badlands":        {Black, Red},
		"Bayou":           {Black, Green},
		"Plateau":         {White, Red},
		"Scrubland":       {White, Black},
		"Taiga":           {Red, Green},
		"Tropical Island": {Blue, Green},
		"Tundra":          {White, Blue},
		"Underground Sea": {Blue, Black},
		"Volcanic Island": {Blue, Red},
		"Savannah":        {White, Green},
	}

	for name, manaTypes := range dualLands {
		card, exists := cardDB.GetCardByName(name)
		if !exists {
			t.Fatalf("Card '%s' not found in the database", name)
		}

		isProducer, producedManaTypes := CheckManaProducer(card.OracleText)
		if !isProducer {
			t.Errorf("Expected '%s' to be a mana producer", name)
		}
		if len(producedManaTypes) != len(manaTypes) {
			t.Errorf("Expected '%s' to produce %v, got %v", name, manaTypes, producedManaTypes)
			continue
		}
		for i, manaType := range manaTypes {
			if producedManaTypes[i] != manaType {
				t.Errorf("Expected '%s' to produce %v, got %v", name, manaTypes, producedManaTypes)
				break
			}
		}
	}
}

func TestManaCreaturesAndArtifacts(t *testing.T) {
	SetLogLevel(WARN)

	manaProducers := map[string]struct {
		manaTypes []ManaType
		power     string
		toughness string
	}{
		"Llanowar Elves":    {manaTypes: []ManaType{Green}, power: "1", toughness: "1"},
		"Birds of Paradise": {manaTypes: []ManaType{Any}, power: "0", toughness: "1"},
		"Sol Ring":          {manaTypes: []ManaType{Colorless, Colorless}},
		"Ashnod's Altar":    {manaTypes: []ManaType{Colorless, Colorless}},
	}

	for name, attributes := range manaProducers {
		card, exists := cardDB.GetCardByName(name)
		if !exists {
			t.Fatalf("Card '%s' not found in the database", name)
		}

		isProducer, producedManaTypes := CheckManaProducer(card.OracleText)
		if !isProducer {
			t.Errorf("Expected '%s' to be a mana producer", name)
		}
		if len(producedManaTypes) != len(attributes.manaTypes) {
			t.Errorf("Expected '%s' to produce %v, got %v", name, attributes.manaTypes, producedManaTypes)
			continue
		}
		for i, manaType := range attributes.manaTypes {
			if producedManaTypes[i] != manaType {
				t.Errorf("Expected '%s' to produce %v, got %v", name, attributes.manaTypes, producedManaTypes)
				break
			}
		}

		if card.TypeLine == "Creature" {
			if card.Power != attributes.power {
				t.Errorf("Expected '%s' to have power %s, got %s", name, attributes.power, card.Power)
			}
			if card.Toughness != attributes.toughness {
				t.Errorf("Expected '%s' to have toughness %s, got %s", name, attributes.toughness, card.Toughness)
			}
		}
	}
}
