package game

import (
	"testing"
)

func TestManaTypes(t *testing.T) {
	// Test that mana type constants are defined correctly
	manaTypes := []ManaType{
		White,
		Blue,
		Black,
		Red,
		Green,
		Colorless,
		Any,
		Phyrexian,
		Snow,
		X,
	}

	expectedValues := []string{
		"W", "U", "B", "R", "G", "C", "A", "P", "S", "X",
	}

	if len(manaTypes) != len(expectedValues) {
		t.Errorf("Mismatch in number of mana types")
	}

	for i, manaType := range manaTypes {
		if string(manaType) != expectedValues[i] {
			t.Errorf("Expected mana type %s, got %s", expectedValues[i], string(manaType))
		}
	}
}

func TestPermanentTypes(t *testing.T) {
	// Test that permanent type constants are defined correctly
	permanentTypes := []PermanentType{
		Creature,
		Artifact,
		Enchantment,
		Land,
		Planeswalker,
	}

	// Just verify they're all different values
	typeSet := make(map[PermanentType]bool)
	for _, pType := range permanentTypes {
		if typeSet[pType] {
			t.Errorf("Duplicate permanent type value: %d", pType)
		}
		typeSet[pType] = true
	}

	if len(typeSet) != 5 {
		t.Errorf("Expected 5 unique permanent types, got %d", len(typeSet))
	}
}

func TestLogLevels(t *testing.T) {
	// Test that log level constants are defined correctly
	logLevels := []LogLevel{
		META,
		GAME,
		PLAYER,
		CARD,
	}

	// Verify they're in ascending order (META should be lowest)
	for i := 1; i < len(logLevels); i++ {
		if logLevels[i] <= logLevels[i-1] {
			t.Errorf("Log levels should be in ascending order")
		}
	}

	// Verify specific values
	if META != 0 {
		t.Errorf("Expected META to be 0, got %d", META)
	}
	if GAME != 1 {
		t.Errorf("Expected GAME to be 1, got %d", GAME)
	}
	if PLAYER != 2 {
		t.Errorf("Expected PLAYER to be 2, got %d", PLAYER)
	}
	if CARD != 3 {
		t.Errorf("Expected CARD to be 3, got %d", CARD)
	}
}
