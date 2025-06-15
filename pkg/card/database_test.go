package card

import (
	"strings"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/types"
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

	testCardDB = NewCardDB(cards)
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
	if len(manaTypes) != 1 || manaTypes[0] != types.Red {
		t.Errorf("Expected 'Mountain' to produce {R}, got %v", manaTypes)
	}
}

func TestBasicLandsManaProducer(t *testing.T) {
	basicLands := map[string]types.ManaType{
		"Plains":   types.White,
		"Island":   types.Blue,
		"Swamp":    types.Black,
		"Mountain": types.Red,
		"Forest":   types.Green,
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
	dualLands := map[string][]types.ManaType{
		"Badlands":        {types.Black, types.Red},
		"Bayou":           {types.Black, types.Green},
		"Plateau":         {types.White, types.Red},
		"Scrubland":       {types.White, types.Black},
		"Taiga":           {types.Red, types.Green},
		"Tropical Island": {types.Blue, types.Green},
		"Tundra":          {types.White, types.Blue},
		"Underground Sea": {types.Blue, types.Black},
		"Volcanic Island": {types.Blue, types.Red},
		"Savannah":        {types.White, types.Green},
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
		manaTypeMap := make(map[types.ManaType]bool)
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
		manaTypes []types.ManaType
		power     string
		toughness string
	}{
		"Llanowar Elves":    {manaTypes: []types.ManaType{types.Green}, power: "1", toughness: "1"},
		"Birds of Paradise": {manaTypes: []types.ManaType{types.Any}, power: "0", toughness: "1"},
		"Sol Ring":          {manaTypes: []types.ManaType{types.Colorless, types.Colorless}},
		"Ashnod's Altar":    {manaTypes: []types.ManaType{types.Colorless, types.Colorless}},
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
		manaTypeMap := make(map[types.ManaType]bool)
		for _, manaType := range attributes.manaTypes {
			manaTypeMap[manaType] = true
		}
		for _, producedManaType := range producedManaTypes {
			if !manaTypeMap[producedManaType] {
				t.Errorf("Expected '%s' to produce %v, got %v", name, attributes.manaTypes, producedManaTypes)
				break
			}
		}

		if strings.Contains(card.TypeLine, "Creature") {
			if card.Power != attributes.power {
				t.Errorf("Expected '%s' to have power %s, got %s", name, attributes.power, card.Power)
			}
			if card.Toughness != attributes.toughness {
				t.Errorf("Expected '%s' to have toughness %s, got %s", name, attributes.toughness, card.Toughness)
			}
		}
	}
}

func TestCardDatabase(t *testing.T) {
	// Test database size
	if testCardDB.Size() != 19 {
		t.Errorf("Expected database to have 19 cards, got %d", testCardDB.Size())
	}

	// Test getting existing card
	card, exists := testCardDB.GetCardByName("Lightning Bolt")
	if exists {
		t.Errorf("Expected 'Lightning Bolt' not to exist in test database, but it was found: %+v", card)
	}

	// Test getting non-existing card
	_, exists = testCardDB.GetCardByName("NonExistentCard")
	if exists {
		t.Errorf("Expected 'NonExistentCard' not to exist in database")
	}
}
