package ability

import (
	"testing"
)

// TestNonCreatureSpells tests parsing of instant and sorcery spells
// These spells have effects that should be parsed even though they're not permanent abilities
func TestNonCreatureSpells(t *testing.T) {
	parser := NewAbilityParser()

	testSpells := []struct {
		name           string
		oracleText     string
		spellType      string
		expectedEffects int
		expectedTargets int
		description    string
	}{
		// Basic Damage Spells
		{
			name:           "Lightning Bolt",
			oracleText:     "Lightning Bolt deals 3 damage to any target.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "Classic 3 damage instant",
		},
		{
			name:           "Shock",
			oracleText:     "Shock deals 2 damage to any target.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "Simple 2 damage instant",
		},
		{
			name:           "Lava Spike",
			oracleText:     "Lava Spike deals 3 damage to target player or planeswalker.",
			spellType:      "Sorcery",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "Player/planeswalker targeting",
		},

		// Card Draw Spells
		{
			name:           "Divination",
			oracleText:     "Draw two cards.",
			spellType:      "Sorcery",
			expectedEffects: 1,
			expectedTargets: 0,
			description:    "Simple card draw",
		},
		{
			name:           "Ancestral Recall",
			oracleText:     "Target player draws three cards.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "Targeted card draw",
		},

		// Destruction Spells
		{
			name:           "Doom Blade",
			oracleText:     "Destroy target non-artifact creature.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "Conditional creature destruction",
		},
		{
			name:           "Terror",
			oracleText:     "Destroy target non-artifact, non-black creature.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "Multiple restriction destruction",
		},

		// Modal Spells
		{
			name:           "Cryptic Command",
			oracleText:     "Choose two — Counter target spell; or return target permanent to its owner's hand; or tap all creatures your opponents control; or draw a card.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 0, // Modal effects have variable targeting
			description:    "Four-mode instant",
		},
		{
			name:           "Charm of Choice",
			oracleText:     "Choose one — Target creature gets +2/+2 until end of turn; or destroy target artifact; or draw a card.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 0, // Modal effects have variable targeting
			description:    "Three-mode charm",
		},

		// Variable X-Cost Spells
		{
			name:           "Fireball",
			oracleText:     "Fireball deals X damage to any target.",
			spellType:      "Sorcery",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "X-cost damage spell",
		},
		{
			name:           "Mind Spring",
			oracleText:     "Draw X cards.",
			spellType:      "Sorcery",
			expectedEffects: 1,
			expectedTargets: 0,
			description:    "X-cost card draw",
		},

		// Multi-Target Spells
		{
			name:           "Forked Bolt",
			oracleText:     "Forked Bolt deals 2 damage divided as you choose among one or two targets.",
			spellType:      "Sorcery",
			expectedEffects: 1,
			expectedTargets: 1, // One target with Count:2 (up to two targets)
			description:    "Divided damage spell",
		},

		// Counterspells
		{
			name:           "Counterspell",
			oracleText:     "Counter target spell.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "Basic counterspell",
		},
		{
			name:           "Mana Leak",
			oracleText:     "Counter target spell unless its controller pays {3}.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 1,
			description:    "Conditional counterspell",
		},

		// Life Gain/Loss Spells
		{
			name:           "Healing Salve",
			oracleText:     "Choose one — Target player gains 3 life; or prevent the next 3 damage that would be dealt to any target this turn.",
			spellType:      "Instant",
			expectedEffects: 1,
			expectedTargets: 0, // Modal targeting
			description:    "Life gain or damage prevention",
		},
		{
			name:           "Drain Life",
			oracleText:     "Drain Life deals X damage to any target. You gain life equal to the damage dealt.",
			spellType:      "Sorcery",
			expectedEffects: 1, // Currently parsed as single X-cost damage effect
			expectedTargets: 1,
			description:    "Damage with life gain",
		},
	}

	for _, tc := range testSpells {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the spell's oracle text as if it were a spell effect
			abilities, err := parser.ParseAbilities(tc.oracleText, nil)
			if err != nil {
				t.Errorf("Failed to parse %s: %v", tc.name, err)
				return
			}

			// For spells, we expect the parser to extract the spell's effects
			// even though they're not permanent abilities
			if len(abilities) != tc.expectedEffects {
				t.Errorf("%s: expected %d effects, got %d. Abilities: %+v", 
					tc.name, tc.expectedEffects, len(abilities), abilities)
			}

			// Verify targeting information if we have abilities
			if len(abilities) > 0 {
				ability := abilities[0]
				if len(ability.Effects) > 0 {
					effect := ability.Effects[0]
					if len(effect.Targets) != tc.expectedTargets {
						t.Errorf("%s: expected %d targets, got %d. Targets: %+v",
							tc.name, tc.expectedTargets, len(effect.Targets), effect.Targets)
					}
				}
			}

			t.Logf("%s (%s): %s - Parsed %d effects", 
				tc.name, tc.spellType, tc.description, len(abilities))
		})
	}
}

// TestSpellEffectTypes tests that spells are parsed with correct effect types
func TestSpellEffectTypes(t *testing.T) {
	parser := NewAbilityParser()

	testCases := []struct {
		name         string
		oracleText   string
		expectedType EffectType
	}{
		{"Lightning Bolt", "Lightning Bolt deals 3 damage to any target.", DealDamage},
		{"Divination", "Draw two cards.", DrawCards},
		{"Doom Blade", "Destroy target non-artifact creature.", DestroyPermanent},
		{"Counterspell", "Counter target spell.", CounterSpell},
		{"Healing Salve", "Target player gains 3 life.", GainLife},
		{"Giant Growth", "Target creature gets +3/+3 until end of turn.", PumpCreature},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tc.oracleText, nil)
			if err != nil {
				t.Errorf("Failed to parse %s: %v", tc.name, err)
				return
			}
			
			if len(abilities) == 0 {
				t.Errorf("%s: expected to parse at least one ability", tc.name)
				return
			}

			ability := abilities[0]
			if len(ability.Effects) == 0 {
				t.Errorf("%s: expected at least one effect", tc.name)
				return
			}

			effect := ability.Effects[0]
			if effect.Type != tc.expectedType {
				t.Errorf("%s: expected effect type %v, got %v", 
					tc.name, tc.expectedType, effect.Type)
			}
		})
	}
}
