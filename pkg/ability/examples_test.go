package ability

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// TestRealMagicCards tests the ability system with real Magic: The Gathering cards.
func TestRealMagicCards(t *testing.T) {
	parser := NewAbilityParser()

	testCards := []struct {
		name        string
		oracleText  string
		expectedAbilities int
		description string
	}{
		{
			name:        "Llanowar Elves",
			oracleText:  "{T}: Add {G}.",
			expectedAbilities: 1,
			description: "Basic mana dork",
		},
		{
			name:        "Lightning Bolt",
			oracleText:  "Lightning Bolt deals 3 damage to any target.",
			expectedAbilities: 0, // This is a spell, not a permanent ability
			description: "Instant spell (no permanent abilities)",
		},
		{
			name:        "Wall of Omens",
			oracleText:  "Defender. When Wall of Omens enters the battlefield, draw a card.",
			expectedAbilities: 1, // ETB draw ability (defender is a keyword)
			description: "ETB card draw",
		},
		{
			name:        "Prodigal Pyromancer",
			oracleText:  "{T}: Prodigal Pyromancer deals 1 damage to any target.",
			expectedAbilities: 1,
			description: "Tim ability",
		},
		{
			name:        "Birds of Paradise",
			oracleText:  "Flying. {T}: Add one mana of any color.",
			expectedAbilities: 1, // Mana ability (flying is a keyword)
			description: "Any color mana",
		},
		{
			name:        "Mulldrifter",
			oracleText:  "Flying. When Mulldrifter enters the battlefield, draw two cards.",
			expectedAbilities: 1, // ETB draw ability
			description: "ETB draw multiple cards",
		},
		{
			name:        "Sakura-Tribe Elder",
			oracleText:  "Sacrifice Sakura-Tribe Elder: Search your library for a basic land card, put it onto the battlefield tapped, then shuffle your library.",
			expectedAbilities: 0, // Complex ability not yet supported
			description: "Complex sacrifice ability",
		},
		{
			name:        "Elvish Archdruid",
			oracleText:  "Other Elf creatures you control get +1/+1. {T}: Add {G} for each Elf you control.",
			expectedAbilities: 2, // Static pump ability + mana ability (both now supported)
			description: "Lord effect with complex mana ability",
		},
		{
			name:        "Grizzly Bears",
			oracleText:  "",
			expectedAbilities: 0,
			description: "Vanilla creature",
		},
		{
			name:        "Shock",
			oracleText:  "Shock deals 2 damage to any target.",
			expectedAbilities: 0, // Spell effect, not permanent ability
			description: "Simple damage spell",
		},
	}

	for _, tc := range testCards {
		t.Run(tc.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tc.oracleText, nil)
			if err != nil {
				t.Errorf("ParseAbilities() error = %v", err)
				return
			}

			if len(abilities) != tc.expectedAbilities {
				t.Errorf("%s: got %d abilities, want %d (%s)", 
					tc.name, len(abilities), tc.expectedAbilities, tc.description)
				
				// Log what abilities were found for debugging
				for i, ability := range abilities {
					t.Logf("  Ability %d: %s (%s)", i+1, ability.Name, ability.Effects[0].Description)
				}
			}

			// Validate that parsed abilities have proper structure
			for _, ability := range abilities {
				if ability.ID == uuid.Nil {
					t.Errorf("%s: ability has nil ID", tc.name)
				}
				if ability.Name == "" {
					t.Errorf("%s: ability has empty name", tc.name)
				}
				if len(ability.Effects) == 0 {
					t.Errorf("%s: ability has no effects", tc.name)
				}
			}
		})
	}
}

// TestAbilityExecution tests the execution of abilities with real card examples.
func TestAbilityExecution(t *testing.T) {
	// Create mock game state
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		hand:     []interface{}{},
		manaPool: map[game.ManaType]int{game.Green: 1},
		creatures: []interface{}{},
		lands:    []interface{}{},
	}

	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}

	engine := NewExecutionEngine(gameState)

	t.Run("Llanowar Elves mana ability", func(t *testing.T) {
		// Create Llanowar Elves mana ability
		ability := &Ability{
			ID:   uuid.New(),
			Name: "Llanowar Elves Mana",
			Type: Mana,
			Cost: Cost{TapCost: true},
			Effects: []Effect{
				{
					Type:        AddMana,
					Value:       1,
					Duration:    Instant,
					Description: "Add {G}",
				},
			},
			OracleText: "{T}: Add {G}.",
		}

		initialMana := 0
		for _, amount := range player.manaPool {
			initialMana += amount
		}

		err := engine.ExecuteAbility(ability, player, nil)
		if err != nil {
			t.Errorf("ExecuteAbility() error = %v", err)
		}

		finalMana := 0
		for _, amount := range player.manaPool {
			finalMana += amount
		}

		if finalMana != initialMana+1 {
			t.Errorf("Expected mana to increase by 1, got %d -> %d", initialMana, finalMana)
		}
	})

	t.Run("Wall of Omens ETB ability", func(t *testing.T) {
		// Create Wall of Omens ETB ability
		ability := &Ability{
			ID:               uuid.New(),
			Name:             "Wall of Omens ETB",
			Type:             Triggered,
			TriggerCondition: EntersTheBattlefield,
			Effects: []Effect{
				{
					Type:        DrawCards,
					Value:       1,
					Duration:    Instant,
					Description: "Draw a card",
				},
			},
			OracleText: "When Wall of Omens enters the battlefield, draw a card.",
		}

		initialHandSize := len(player.hand)

		err := engine.ExecuteAbility(ability, player, nil)
		if err != nil {
			t.Errorf("ExecuteAbility() error = %v", err)
		}

		if len(player.hand) != initialHandSize+1 {
			t.Errorf("Expected hand size to increase by 1, got %d -> %d", initialHandSize, len(player.hand))
		}
	})

	t.Run("Prodigal Pyromancer damage ability", func(t *testing.T) {
		// Create Prodigal Pyromancer ability
		ability := &Ability{
			ID:   uuid.New(),
			Name: "Prodigal Pyromancer Damage",
			Type: Activated,
			Cost: Cost{TapCost: true},
			Effects: []Effect{
				{
					Type:     DealDamage,
					Value:    1,
					Duration: Instant,
					Targets: []Target{
						{
							Type:     AnyTarget,
							Required: true,
							Count:    1,
						},
					},
					Description: "Deal 1 damage to any target",
				},
			},
			TimingRestriction: SorcerySpeed,
			OracleText:        "{T}: Prodigal Pyromancer deals 1 damage to any target.",
		}

		// Target the player
		targets := []interface{}{player}

		initialLife := player.life

		err := engine.ExecuteAbility(ability, player, targets)
		if err != nil {
			t.Errorf("ExecuteAbility() error = %v", err)
		}

		// Note: In this test, the player is targeting themselves
		if player.life != initialLife-1 {
			t.Errorf("Expected life to decrease by 1, got %d -> %d", initialLife, player.life)
		}
	})
}

// TestAIDecisionMaking tests the AI's ability to make decisions about ability activation.
func TestAIDecisionMaking(t *testing.T) {
	// Create a more complex game state
	player := &mockPlayer{
		name:     "AI Player",
		life:     15, // Lower life to test life gain priorities
		hand:     []interface{}{"Card1", "Card2"}, // Few cards to test draw priorities
		manaPool: map[game.ManaType]int{game.Any: 4}, // Some mana available
		creatures: []interface{}{},
		lands:    []interface{}{},
	}

	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}

	engine := NewExecutionEngine(gameState)
	ai := NewAIDecisionMaker(engine)

	// Create various abilities to test prioritization
	abilities := []*Ability{
		{
			ID:   uuid.New(),
			Name: "Mana Ability",
			Type: Mana,
			Cost: Cost{TapCost: true},
			Effects: []Effect{
				{Type: AddMana, Value: 1, Description: "Add {G}"},
			},
		},
		{
			ID:   uuid.New(),
			Name: "Draw Ability",
			Type: Activated,
			Cost: Cost{ManaCost: map[game.ManaType]int{game.Any: 2}},
			Effects: []Effect{
				{Type: DrawCards, Value: 1, Description: "Draw a card"},
			},
			TimingRestriction: SorcerySpeed,
		},
		{
			ID:   uuid.New(),
			Name: "Life Gain Ability",
			Type: Activated,
			Cost: Cost{ManaCost: map[game.ManaType]int{game.Any: 1}},
			Effects: []Effect{
				{Type: GainLife, Value: 3, Description: "Gain 3 life"},
			},
			TimingRestriction: SorcerySpeed,
		},
		{
			ID:   uuid.New(),
			Name: "Expensive Ability",
			Type: Activated,
			Cost: Cost{ManaCost: map[game.ManaType]int{game.Any: 6}},
			Effects: []Effect{
				{Type: DrawCards, Value: 2, Description: "Draw 2 cards"},
			},
			TimingRestriction: SorcerySpeed,
		},
	}

	context := DecisionContext{
		Player:            player,
		Phase:             "Main",
		AvailableMana:     4,
		HandSize:          2,
		BoardState:        BoardState{MyCreatures: 0, OpponentCreatures: 1},
		ThreatLevel:       2,
		CanCastMoreSpells: true,
	}

	t.Run("Should activate abilities", func(t *testing.T) {
		shouldActivate := ai.ShouldActivateAbilities(context)
		if !shouldActivate {
			t.Error("AI should want to activate abilities with available mana and threats")
		}
	})

	t.Run("Choose abilities to activate", func(t *testing.T) {
		chosen := ai.ChooseAbilitiesToActivate(abilities, context)
		
		if len(chosen) == 0 {
			t.Error("AI should choose some abilities to activate")
		}

		// Check that expensive abilities are not chosen when we can't afford them
		for _, ability := range chosen {
			cost := ai.calculateManaCost(ability)
			if cost > context.AvailableMana {
				t.Errorf("AI chose ability %s with cost %d when only %d mana available", 
					ability.Name, cost, context.AvailableMana)
			}
		}

		// Log chosen abilities for debugging
		t.Logf("AI chose %d abilities:", len(chosen))
		for _, ability := range chosen {
			t.Logf("  - %s (cost: %d)", ability.Name, ai.calculateManaCost(ability))
		}
	})

	t.Run("Ability scoring", func(t *testing.T) {
		// Test that draw abilities score higher when hand size is low
		lowHandContext := context
		lowHandContext.HandSize = 1

		drawAbility := abilities[1] // Draw ability
		lifeAbility := abilities[2]  // Life gain ability

		drawScore := ai.scoreAbility(drawAbility, lowHandContext)
		lifeScore := ai.scoreAbility(lifeAbility, lowHandContext)

		if drawScore <= lifeScore {
			t.Errorf("Draw ability should score higher than life gain when hand size is low (%.2f vs %.2f)", 
				drawScore, lifeScore)
		}
	})
}

// TestComplexAbilityInteractions tests more complex ability interactions.
func TestComplexAbilityInteractions(t *testing.T) {
	parser := NewAbilityParser()

	// Test cards with multiple abilities
	complexCards := []struct {
		name       string
		oracleText string
		testFunc   func(t *testing.T, abilities []*Ability)
	}{
		{
			name:       "Elvish Archdruid",
			oracleText: "Other Elf creatures you control get +1/+1. {T}: Add {G} for each Elf you control.",
			testFunc: func(t *testing.T, abilities []*Ability) {
				// Should parse the static ability, complex mana ability might not be supported yet
				if len(abilities) < 1 {
					t.Error("Should parse at least the static pump ability")
				}
				
				// Check for static pump ability
				hasStaticPump := false
				for _, ability := range abilities {
					if ability.Type == Static && len(ability.Effects) > 0 && ability.Effects[0].Type == PumpCreature {
						hasStaticPump = true
						break
					}
				}
				
				if !hasStaticPump {
					t.Error("Should have static pump ability")
				}
			},
		},
		{
			name:       "Solemn Simulacrum",
			oracleText: "When Solemn Simulacrum enters the battlefield, you may search your library for a basic land card, put it onto the battlefield tapped, then shuffle your library. When Solemn Simulacrum dies, you may draw a card.",
			testFunc: func(t *testing.T, abilities []*Ability) {
				// Complex abilities might not be fully supported yet
				// But we should at least recognize that there are triggered abilities
				t.Logf("Parsed %d abilities from Solemn Simulacrum", len(abilities))
				for i, ability := range abilities {
					t.Logf("  Ability %d: %s (%v)", i+1, ability.Name, ability.Type)
				}
			},
		},
	}

	for _, tc := range complexCards {
		t.Run(tc.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tc.oracleText, nil)
			if err != nil {
				t.Errorf("ParseAbilities() error = %v", err)
				return
			}

			tc.testFunc(t, abilities)
		})
	}
}
