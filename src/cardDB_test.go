package main

import (
	"testing"
)

var testCardDB *CardDB

func init() {
	// Hardcoded card data for testing purposes
	cards := []Card{
		{Name: "Mountain", OracleText: "{T}: Add {R}.", TypeLine: "Basic Land — Mountain"},
		{Name: "Plains", OracleText: "{T}: Add {W}.", TypeLine: "Basic Land — Plains"},
		{Name: "Island", OracleText: "{T}: Add {U}.", TypeLine: "Basic Land — Island"},
		{Name: "Swamp", OracleText: "{T}: Add {B}.", TypeLine: "Basic Land — Swamp"},
		{Name: "Forest", OracleText: "{T}: Add {G}.", TypeLine: "Basic Land — Forest"},
		{Name: "Badlands", OracleText: "{T}: Add {B} or {R}.", TypeLine: "Land — Swamp Mountain"},
		{Name: "Bayou", OracleText: "{T}: Add {B} or {G}.", TypeLine: "Land — Swamp Forest"},
		{Name: "Plateau", OracleText: "{T}: Add {W} or {R}.", TypeLine: "Land — Mountain Plains"},
		{Name: "Scrubland", OracleText: "{T}: Add {W} or {B}.", TypeLine: "Land — Plains Swamp"},
		{Name: "Taiga", OracleText: "{T}: Add {R} or {G}.", TypeLine: "Land — Mountain Forest"},
		{Name: "Tropical Island", OracleText: "{T}: Add {U} or {G}.", TypeLine: "Land — Forest Island"},
		{Name: "Tundra", OracleText: "{T}: Add {W} or {U}.", TypeLine: "Land — Plains Island"},
		{Name: "Underground Sea", OracleText: "{T}: Add {U} or {B}.", TypeLine: "Land — Island Swamp"},
		{Name: "Volcanic Island", OracleText: "{T}: Add {U} or {R}.", TypeLine: "Land — Island Mountain"},
		{Name: "Savannah", OracleText: "{T}: Add {W} or {G}.", TypeLine: "Land — Forest Plains"},
		{Name: "Llanowar Elves", OracleText: "{T}: Add {G}.", TypeLine: "Creature — Elf Druid", Power: "1", Toughness: "1"},
		{Name: "Birds of Paradise", OracleText: "{T}: Add one mana of any color.", TypeLine: "Creature — Bird", Power: "0", Toughness: "1"},
		{Name: "Sol Ring", OracleText: "{T}: Add {C}{C}.", TypeLine: "Artifact"},
		{Name: "Ashnod's Altar", OracleText: "Sacrifice a creature: Add {C}{C}.", TypeLine: "Artifact"},
	}

	cardMap := make(map[string]Card)
	for _, card := range cards {
		cardMap[card.Name] = card
	}
	testCardDB = &CardDB{cards: cardMap}
}

func TestMountainManaProducer(t *testing.T) {
	card, exists := testCardDB.GetCardByName("Mountain")
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
	basicLands := map[string]ManaType{
		"Plains":   White,
		"Island":   Blue,
		"Swamp":    Black,
		"Mountain": Red,
		"Forest":   Green,
	}

	for name, manaType := range basicLands {
		card, exists := testCardDB.GetCardByName(name)
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
		card, exists := testCardDB.GetCardByName(name)
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
		manaTypeMap := make(map[ManaType]bool)
		for _, manaType := range manaTypes {
			manaTypeMap[manaType] = true
		}
		for _, producedManaType := range producedManaTypes {
			if !manaTypeMap[producedManaType] {
				t.Errorf("Expected '%s' to produce %v, got %v", name, manaTypes, producedManaTypes)
				break
			}
		}
	}
}

func TestManaCreaturesAndArtifacts(t *testing.T) {
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
		card, exists := testCardDB.GetCardByName(name)
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
		manaTypeMap := make(map[ManaType]bool)
		for _, manaType := range attributes.manaTypes {
			manaTypeMap[manaType] = true
		}
		for _, producedManaType := range producedManaTypes {
			if !manaTypeMap[producedManaType] {
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
