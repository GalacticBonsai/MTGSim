package ability

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestAbilityParser_ParseManaAbilities(t *testing.T) {
	parser := NewAbilityParser()

	tests := []struct {
		name        string
		oracleText  string
		expectedLen int
		expectedType AbilityType
		expectedEffect EffectType
	}{
		{
			name:        "Basic mana ability",
			oracleText:  "{T}: Add {G}.",
			expectedLen: 1,
			expectedType: Mana,
			expectedEffect: AddMana,
		},
		{
			name:        "Any color mana",
			oracleText:  "{T}: Add one mana of any color.",
			expectedLen: 1,
			expectedType: Mana,
			expectedEffect: AddMana,
		},
		{
			name:        "Colorless mana",
			oracleText:  "{T}: Add {C}{C}.",
			expectedLen: 1,
			expectedType: Mana,
			expectedEffect: AddMana,
		},
		{
			name:        "Multiple abilities",
			oracleText:  "{T}: Add {R}. {T}: Add {G}.",
			expectedLen: 2,
			expectedType: Mana,
			expectedEffect: AddMana,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tt.oracleText, nil)
			if err != nil {
				t.Errorf("ParseAbilities() error = %v", err)
				return
			}

			if len(abilities) != tt.expectedLen {
				t.Errorf("ParseAbilities() got %d abilities, want %d", len(abilities), tt.expectedLen)
				return
			}

			if len(abilities) > 0 {
				ability := abilities[0]
				if ability.Type != tt.expectedType {
					t.Errorf("ParseAbilities() ability type = %v, want %v", ability.Type, tt.expectedType)
				}

				if len(ability.Effects) > 0 && ability.Effects[0].Type != tt.expectedEffect {
					t.Errorf("ParseAbilities() effect type = %v, want %v", ability.Effects[0].Type, tt.expectedEffect)
				}

				if ability.Cost.TapCost != true {
					t.Errorf("ParseAbilities() expected tap cost to be true")
				}
			}
		})
	}
}

func TestAbilityParser_ParseTriggeredAbilities(t *testing.T) {
	parser := NewAbilityParser()

	tests := []struct {
		name        string
		oracleText  string
		expectedLen int
		expectedTrigger TriggerCondition
		expectedEffect EffectType
		expectedValue int
	}{
		{
			name:        "ETB draw card",
			oracleText:  "When this creature enters the battlefield, draw a card.",
			expectedLen: 1,
			expectedTrigger: EntersTheBattlefield,
			expectedEffect: DrawCards,
			expectedValue: 1,
		},
		{
			name:        "ETB draw multiple cards",
			oracleText:  "When this creature enters the battlefield, draw 2 cards.",
			expectedLen: 1,
			expectedTrigger: EntersTheBattlefield,
			expectedEffect: DrawCards,
			expectedValue: 2,
		},
		{
			name:        "ETB deal damage",
			oracleText:  "When this creature enters the battlefield, it deals 3 damage to any target.",
			expectedLen: 1,
			expectedTrigger: EntersTheBattlefield,
			expectedEffect: DealDamage,
			expectedValue: 3,
		},
		{
			name:        "ETB gain life",
			oracleText:  "When this creature enters the battlefield, you gain 4 life.",
			expectedLen: 1,
			expectedTrigger: EntersTheBattlefield,
			expectedEffect: GainLife,
			expectedValue: 4,
		},
		{
			name:        "Death trigger",
			oracleText:  "When this creature dies, draw a card.",
			expectedLen: 1,
			expectedTrigger: Dies,
			expectedEffect: DrawCards,
			expectedValue: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tt.oracleText, nil)
			if err != nil {
				t.Errorf("ParseAbilities() error = %v", err)
				return
			}

			if len(abilities) != tt.expectedLen {
				t.Errorf("ParseAbilities() got %d abilities, want %d", len(abilities), tt.expectedLen)
				return
			}

			if len(abilities) > 0 {
				ability := abilities[0]
				if ability.Type != Triggered {
					t.Errorf("ParseAbilities() ability type = %v, want %v", ability.Type, Triggered)
				}

				if ability.TriggerCondition != tt.expectedTrigger {
					t.Errorf("ParseAbilities() trigger condition = %v, want %v", ability.TriggerCondition, tt.expectedTrigger)
				}

				if len(ability.Effects) > 0 {
					effect := ability.Effects[0]
					if effect.Type != tt.expectedEffect {
						t.Errorf("ParseAbilities() effect type = %v, want %v", effect.Type, tt.expectedEffect)
					}

					if effect.Value != tt.expectedValue {
						t.Errorf("ParseAbilities() effect value = %d, want %d", effect.Value, tt.expectedValue)
					}
				}
			}
		})
	}
}

func TestAbilityParser_ParseActivatedAbilities(t *testing.T) {
	parser := NewAbilityParser()

	tests := []struct {
		name        string
		oracleText  string
		expectedLen int
		expectedEffect EffectType
		expectedManaCost int
		expectedTapCost bool
	}{
		{
			name:        "Activated draw ability",
			oracleText:  "{2}, {T}: Draw 1 card.",
			expectedLen: 1,
			expectedEffect: DrawCards,
			expectedManaCost: 2,
			expectedTapCost: true,
		},
		{
			name:        "Activated damage ability",
			oracleText:  "{1}, {T}: This creature deals 2 damage to any target.",
			expectedLen: 1,
			expectedEffect: DealDamage,
			expectedManaCost: 1,
			expectedTapCost: true,
		},
		{
			name:        "Pump ability",
			oracleText:  "{T}: Target creature gets +1/+1 until end of turn.",
			expectedLen: 1,
			expectedEffect: PumpCreature,
			expectedManaCost: 0,
			expectedTapCost: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tt.oracleText, nil)
			if err != nil {
				t.Errorf("ParseAbilities() error = %v", err)
				return
			}

			if len(abilities) != tt.expectedLen {
				t.Errorf("ParseAbilities() got %d abilities, want %d", len(abilities), tt.expectedLen)
				return
			}

			if len(abilities) > 0 {
				ability := abilities[0]
				if ability.Type != Activated {
					t.Errorf("ParseAbilities() ability type = %v, want %v", ability.Type, Activated)
				}

				if ability.Cost.TapCost != tt.expectedTapCost {
					t.Errorf("ParseAbilities() tap cost = %v, want %v", ability.Cost.TapCost, tt.expectedTapCost)
				}

				if tt.expectedManaCost > 0 {
					if ability.Cost.ManaCost[game.Any] != tt.expectedManaCost {
						t.Errorf("ParseAbilities() mana cost = %d, want %d", ability.Cost.ManaCost[game.Any], tt.expectedManaCost)
					}
				}

				if len(ability.Effects) > 0 && ability.Effects[0].Type != tt.expectedEffect {
					t.Errorf("ParseAbilities() effect type = %v, want %v", ability.Effects[0].Type, tt.expectedEffect)
				}
			}
		})
	}
}

func TestAbilityParser_ParseStaticAbilities(t *testing.T) {
	parser := NewAbilityParser()

	tests := []struct {
		name        string
		oracleText  string
		expectedLen int
		expectedEffect EffectType
		expectedPower int
		expectedToughness int
	}{
		{
			name:        "Static pump all",
			oracleText:  "Creatures you control get +1/+1.",
			expectedLen: 1,
			expectedEffect: PumpCreature,
			expectedPower: 1,
			expectedToughness: 1,
		},
		{
			name:        "Static pump others",
			oracleText:  "Other creatures you control get +2/+2.",
			expectedLen: 1,
			expectedEffect: PumpCreature,
			expectedPower: 2,
			expectedToughness: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tt.oracleText, nil)
			if err != nil {
				t.Errorf("ParseAbilities() error = %v", err)
				return
			}

			if len(abilities) != tt.expectedLen {
				t.Errorf("ParseAbilities() got %d abilities, want %d", len(abilities), tt.expectedLen)
				return
			}

			if len(abilities) > 0 {
				ability := abilities[0]
				if ability.Type != Static {
					t.Errorf("ParseAbilities() ability type = %v, want %v", ability.Type, Static)
				}

				if len(ability.Effects) > 0 {
					effect := ability.Effects[0]
					if effect.Type != tt.expectedEffect {
						t.Errorf("ParseAbilities() effect type = %v, want %v", effect.Type, tt.expectedEffect)
					}

					// Decode power/toughness from value
					power := effect.Value / 100
					toughness := effect.Value % 100

					if power != tt.expectedPower {
						t.Errorf("ParseAbilities() power = %d, want %d", power, tt.expectedPower)
					}

					if toughness != tt.expectedToughness {
						t.Errorf("ParseAbilities() toughness = %d, want %d", toughness, tt.expectedToughness)
					}
				}
			}
		})
	}
}

func TestAbilityParser_ParseComplexAbilities(t *testing.T) {
	parser := NewAbilityParser()

	tests := []struct {
		name        string
		oracleText  string
		expectedLen int
		description string
	}{
		{
			name:        "Multiple abilities",
			oracleText:  "{T}: Add {G}. When this creature enters the battlefield, draw a card.",
			expectedLen: 2,
			description: "Should parse both mana ability and ETB trigger",
		},
		{
			name:        "Complex creature",
			oracleText:  "Flying. When this creature enters the battlefield, you gain 3 life. {2}, {T}: Draw a card.",
			expectedLen: 2, // Flying is a keyword, not parsed as ability
			description: "Should parse ETB trigger and activated ability",
		},
		{
			name:        "No abilities",
			oracleText:  "This is just flavor text with no abilities.",
			expectedLen: 0,
			description: "Should not parse any abilities from flavor text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tt.oracleText, nil)
			if err != nil {
				t.Errorf("ParseAbilities() error = %v", err)
				return
			}

			if len(abilities) != tt.expectedLen {
				t.Errorf("ParseAbilities() got %d abilities, want %d: %s", len(abilities), tt.expectedLen, tt.description)
			}
		})
	}
}

func TestAbilityParser_ParseTargetTypes(t *testing.T) {
	parser := NewAbilityParser()

	tests := []struct {
		targetString string
		expectedType TargetType
	}{
		{"creature", CreatureTarget},
		{"target creature", CreatureTarget},
		{"player", PlayerTarget},
		{"target player", PlayerTarget},
		{"permanent", PermanentTarget},
		{"any target", AnyTarget},
		{"unknown target", AnyTarget}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.targetString, func(t *testing.T) {
			result := parser.parseTargetType(tt.targetString)
			if result != tt.expectedType {
				t.Errorf("parseTargetType(%s) = %v, want %v", tt.targetString, result, tt.expectedType)
			}
		})
	}
}

func TestAbilityParser_SplitOracleText(t *testing.T) {
	parser := NewAbilityParser()

	tests := []struct {
		name        string
		oracleText  string
		expectedLen int
	}{
		{
			name:        "Single sentence",
			oracleText:  "When this creature enters the battlefield, draw a card",
			expectedLen: 1,
		},
		{
			name:        "Multiple sentences",
			oracleText:  "Flying. When this creature enters the battlefield, draw a card. {T}: Add {G}.",
			expectedLen: 3,
		},
		{
			name:        "Empty text",
			oracleText:  "",
			expectedLen: 0,
		},
		{
			name:        "Text with mana symbols",
			oracleText:  "{T}: Add {G}. {2}, {T}: Draw a card.",
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.splitOracleText(tt.oracleText)
			if len(result) != tt.expectedLen {
				t.Errorf("splitOracleText() got %d sentences, want %d", len(result), tt.expectedLen)
			}
		})
	}
}
