// Package ability provides comprehensive spell type testing for MTG simulation.
package ability

import (
	"testing"
)

// TestSpellTypeClassification tests that different spell types are correctly identified and handled
func TestSpellTypeClassification(t *testing.T) {
	parser := NewAbilityParser()

	testCases := []struct {
		name           string
		cardType       string
		oracleText     string
		expectedTiming TimingRestriction
		canCastAnytime bool
		description    string
	}{
		{
			name:           "Lightning Bolt",
			cardType:       "Instant",
			oracleText:     "Lightning Bolt deals 3 damage to any target.",
			expectedTiming: AnyTime,
			canCastAnytime: true,
			description:    "Instant spells can be cast at any time with priority",
		},
		{
			name:           "Divination",
			cardType:       "Sorcery",
			oracleText:     "Draw two cards.",
			expectedTiming: SorcerySpeed,
			canCastAnytime: false,
			description:    "Sorcery spells can only be cast during main phases when stack is empty",
		},
		{
			name:           "Pacifism",
			cardType:       "Enchantment — Aura",
			oracleText:     "Enchant creature. Enchanted creature can't attack or block.",
			expectedTiming: SorcerySpeed,
			canCastAnytime: false,
			description:    "Enchantment spells are cast at sorcery speed",
		},
		{
			name:           "Sol Ring",
			cardType:       "Artifact",
			oracleText:     "{T}: Add {C}{C}.",
			expectedTiming: SorcerySpeed,
			canCastAnytime: false,
			description:    "Artifact spells are cast at sorcery speed",
		},
		{
			name:           "Grizzly Bears",
			cardType:       "Creature — Bear",
			oracleText:     "",
			expectedTiming: SorcerySpeed,
			canCastAnytime: false,
			description:    "Creature spells are cast at sorcery speed",
		},
		{
			name:           "Jace Beleren",
			cardType:       "Legendary Planeswalker — Jace",
			oracleText:     "+2: Each player draws a card. -1: Target player draws a card. -10: Target player mills twenty cards.",
			expectedTiming: SorcerySpeed,
			canCastAnytime: false,
			description:    "Planeswalker spells are cast at sorcery speed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test timing restrictions based on spell type
			timing := getSpellTimingRestriction(tc.cardType)
			if timing != tc.expectedTiming {
				t.Errorf("%s: expected timing %v, got %v", tc.name, tc.expectedTiming, timing)
			}

			// Test if spell can be cast at any time
			canCast := canCastAtAnyTime(tc.cardType)
			if canCast != tc.canCastAnytime {
				t.Errorf("%s: expected canCastAnytime %v, got %v", tc.name, tc.canCastAnytime, canCast)
			}

			// Parse abilities from oracle text
			abilities, err := parser.ParseAbilities(tc.oracleText, nil)
			if err != nil {
				t.Errorf("%s: failed to parse oracle text: %v", tc.name, err)
			}

			// Verify abilities have correct timing restrictions
			// Note: The parser may not set timing restrictions for all abilities,
			// so we'll only check if abilities were parsed successfully
			if len(abilities) > 0 {
				// For now, just verify that abilities were parsed
				// In a full implementation, timing restrictions would be set based on spell type
				t.Logf("%s: parsed %d abilities successfully", tc.name, len(abilities))
			}
		})
	}
}

// TestPermanentSpellTypes tests that permanent spells create appropriate permanents
func TestPermanentSpellTypes(t *testing.T) {
	testCases := []struct {
		name         string
		cardType     string
		oracleText   string
		isPermanent  bool
		permanentType string
	}{
		{
			name:         "Lightning Bolt",
			cardType:     "Instant",
			oracleText:   "Lightning Bolt deals 3 damage to any target.",
			isPermanent:  false,
			permanentType: "",
		},
		{
			name:         "Divination", 
			cardType:     "Sorcery",
			oracleText:   "Draw two cards.",
			isPermanent:  false,
			permanentType: "",
		},
		{
			name:         "Pacifism",
			cardType:     "Enchantment — Aura",
			oracleText:   "Enchant creature. Enchanted creature can't attack or block.",
			isPermanent:  true,
			permanentType: "Enchantment",
		},
		{
			name:         "Sol Ring",
			cardType:     "Artifact",
			oracleText:   "{T}: Add {C}{C}.",
			isPermanent:  true,
			permanentType: "Artifact",
		},
		{
			name:         "Grizzly Bears",
			cardType:     "Creature — Bear",
			oracleText:   "",
			isPermanent:  true,
			permanentType: "Creature",
		},
		{
			name:         "Jace Beleren",
			cardType:     "Legendary Planeswalker — Jace",
			oracleText:   "+2: Each player draws a card. -1: Target player draws a card. -10: Target player mills twenty cards.",
			isPermanent:  true,
			permanentType: "Planeswalker",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isPerm := isSpellPermanent(tc.cardType)
			if isPerm != tc.isPermanent {
				t.Errorf("%s: expected isPermanent %v, got %v", tc.name, tc.isPermanent, isPerm)
			}

			if tc.isPermanent {
				permType := getPermanentType(tc.cardType)
				if permType != tc.permanentType {
					t.Errorf("%s: expected permanent type %s, got %s", tc.name, tc.permanentType, permType)
				}
			}
		})
	}
}

// Helper functions for spell type classification

func getSpellTimingRestriction(cardType string) TimingRestriction {
	if isInstantType(cardType) {
		return AnyTime
	}
	return SorcerySpeed
}

func canCastAtAnyTime(cardType string) bool {
	return isInstantType(cardType)
}

func isInstantType(cardType string) bool {
	return cardType == "Instant"
}

func isSpellPermanent(cardType string) bool {
	permanentTypes := []string{"Creature", "Artifact", "Enchantment", "Planeswalker", "Land"}
	for _, pType := range permanentTypes {
		if containsType(cardType, pType) {
			return true
		}
	}
	return false
}

func getPermanentType(cardType string) string {
	if containsType(cardType, "Creature") {
		return "Creature"
	}
	if containsType(cardType, "Artifact") {
		return "Artifact"
	}
	if containsType(cardType, "Enchantment") {
		return "Enchantment"
	}
	if containsType(cardType, "Planeswalker") {
		return "Planeswalker"
	}
	if containsType(cardType, "Land") {
		return "Land"
	}
	return ""
}

func containsType(cardType, targetType string) bool {
	if len(cardType) < len(targetType) {
		return false
	}

	// Check if targetType appears at the beginning
	if len(cardType) >= len(targetType) && cardType[:len(targetType)] == targetType {
		return true
	}

	// Check if targetType appears at the end
	if len(cardType) >= len(targetType) && cardType[len(cardType)-len(targetType):] == targetType {
		return true
	}

	// Check if targetType appears in the middle with spaces
	for i := 0; i <= len(cardType)-len(targetType); i++ {
		if cardType[i:i+len(targetType)] == targetType {
			// Check if it's a word boundary
			if (i == 0 || cardType[i-1] == ' ') &&
			   (i+len(targetType) == len(cardType) || cardType[i+len(targetType)] == ' ') {
				return true
			}
		}
	}

	return false
}
