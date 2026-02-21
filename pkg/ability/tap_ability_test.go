package ability

import (
	"testing"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/types"
)

// TestTapStateValidation tests that tapped permanents cannot activate tap abilities
func TestTapStateValidation(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[types.ManaType]int),
	}

	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}

	engine := NewExecutionEngine(gameState)

	// Create a permanent with a tap ability
	permanent := &mockPermanent{
		id:         uuid.New(),
		name:       "Test Creature",
		owner:      player,
		controller: player,
		tapped:     false,
		abilities:  []*Ability{},
	}

	tapAbility := &Ability{
		ID:   uuid.New(),
		Name: "Tap Ability",
		Type: Activated,
		Cost: Cost{TapCost: true},
		Effects: []Effect{
			{
				Type:        DealDamage,
				Value:       1,
				Duration:    Instant,
				Description: "Deal 1 damage",
				Targets: []Target{
					{Type: PlayerTarget, Required: true, Count: 1},
				},
			},
		},
		Source: permanent,
	}

	permanent.abilities = []*Ability{tapAbility}

	t.Run("Can activate when untapped", func(t *testing.T) {
		// Ensure permanent is untapped
		permanent.tapped = false

		canActivate := engine.canActivateAbility(tapAbility, player)
		if !canActivate {
			t.Error("Should be able to activate tap ability when permanent is untapped")
		}

		// Execute the ability
		err := engine.ExecuteAbility(tapAbility, player, []interface{}{player})
		if err != nil {
			t.Errorf("ExecuteAbility() error = %v", err)
		}

		// Verify permanent is now tapped
		if !permanent.tapped {
			t.Error("Permanent should be tapped after activating tap ability")
		}
	})

	t.Run("Cannot activate when already tapped", func(t *testing.T) {
		// Ensure permanent is tapped
		permanent.tapped = true

		canActivate := engine.canActivateAbility(tapAbility, player)
		if canActivate {
			t.Error("Should not be able to activate tap ability when permanent is already tapped")
		}

		// Try to execute the ability - should fail
		err := engine.ExecuteAbility(tapAbility, player, []interface{}{player})
		if err == nil {
			t.Error("ExecuteAbility() should fail when permanent is tapped")
		}
	})

	t.Run("Can activate again after untapping", func(t *testing.T) {
		// Untap the permanent
		permanent.tapped = false

		canActivate := engine.canActivateAbility(tapAbility, player)
		if !canActivate {
			t.Error("Should be able to activate tap ability after untapping")
		}
	})
}

// TestMultipleTapAbilities tests permanents with multiple tap abilities
func TestMultipleTapAbilities(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[types.ManaType]int),
	}

	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}

	engine := NewExecutionEngine(gameState)

	// Create a permanent with multiple tap abilities
	permanent := &mockPermanent{
		id:         uuid.New(),
		name:       "Multi-Ability Creature",
		owner:      player,
		controller: player,
		tapped:     false,
		abilities:  []*Ability{},
	}

	manaAbility := &Ability{
		ID:   uuid.New(),
		Name: "Mana Ability",
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
		Source: permanent,
	}

	damageAbility := &Ability{
		ID:   uuid.New(),
		Name: "Damage Ability",
		Type: Activated,
		Cost: Cost{TapCost: true},
		Effects: []Effect{
			{
				Type:        DealDamage,
				Value:       1,
				Duration:    Instant,
				Description: "Deal 1 damage",
				Targets: []Target{
					{Type: PlayerTarget, Required: true, Count: 1},
				},
			},
		},
		Source: permanent,
	}

	permanent.abilities = []*Ability{manaAbility, damageAbility}

	t.Run("Can activate one tap ability", func(t *testing.T) {
		// Ensure permanent is untapped
		permanent.tapped = false

		// Activate mana ability
		err := engine.ExecuteAbility(manaAbility, player, []interface{}{})
		if err != nil {
			t.Errorf("ExecuteAbility() error = %v", err)
		}

		// Verify permanent is now tapped
		if !permanent.tapped {
			t.Error("Permanent should be tapped after activating first tap ability")
		}
	})

	t.Run("Cannot activate second tap ability when tapped", func(t *testing.T) {
		// Permanent should still be tapped from previous test
		if !permanent.tapped {
			permanent.tapped = true // Ensure it's tapped
		}

		canActivate := engine.canActivateAbility(damageAbility, player)
		if canActivate {
			t.Error("Should not be able to activate second tap ability when permanent is tapped")
		}
	})

	t.Run("Can activate different ability after untapping", func(t *testing.T) {
		// Untap the permanent
		permanent.tapped = false

		// Now activate the damage ability instead
		err := engine.ExecuteAbility(damageAbility, player, []interface{}{player})
		if err != nil {
			t.Errorf("ExecuteAbility() error = %v", err)
		}

		// Verify permanent is tapped again
		if !permanent.tapped {
			t.Error("Permanent should be tapped after activating second tap ability")
		}
	})
}

// TestTapAbilitiesWithAdditionalCosts tests tap abilities that have both tap and mana costs
func TestTapAbilitiesWithAdditionalCosts(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		manaPool: map[types.ManaType]int{types.Any: 3}, // Give player some mana
	}

	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}

	engine := NewExecutionEngine(gameState)

	// Create a permanent with a tap ability that also costs mana
	permanent := &mockPermanent{
		id:         uuid.New(),
		name:       "Expensive Creature",
		owner:      player,
		controller: player,
		tapped:     false,
		abilities:  []*Ability{},
	}

	expensiveAbility := &Ability{
		ID:   uuid.New(),
		Name: "Expensive Tap Ability",
		Type: Activated,
		Cost: Cost{
			TapCost:  true,
			ManaCost: map[types.ManaType]int{types.Any: 2},
		},
		Effects: []Effect{
			{
				Type:        DealDamage,
				Value:       3,
				Duration:    Instant,
				Description: "Deal 3 damage",
				Targets: []Target{
					{Type: PlayerTarget, Required: true, Count: 1},
				},
			},
		},
		Source: permanent,
	}

	permanent.abilities = []*Ability{expensiveAbility}

	t.Run("Can activate with sufficient mana and untapped", func(t *testing.T) {
		// Ensure permanent is untapped and player has mana
		permanent.tapped = false
		player.manaPool[types.Any] = 3

		canActivate := engine.canActivateAbility(expensiveAbility, player)
		if !canActivate {
			t.Error("Should be able to activate expensive tap ability with sufficient mana")
		}

		initialMana := player.manaPool[types.Any]
		err := engine.ExecuteAbility(expensiveAbility, player, []interface{}{player})
		if err != nil {
			t.Errorf("ExecuteAbility() error = %v", err)
		}

		// Verify both tap and mana costs were paid
		if !permanent.tapped {
			t.Error("Permanent should be tapped after activating ability")
		}

		if player.manaPool[types.Any] >= initialMana {
			t.Error("Mana should have been spent")
		}
	})

	t.Run("Cannot activate without sufficient mana", func(t *testing.T) {
		// Reset permanent to untapped but remove mana
		permanent.tapped = false
		player.manaPool[types.Any] = 1 // Not enough mana

		canActivate := engine.canActivateAbility(expensiveAbility, player)
		if canActivate {
			t.Error("Should not be able to activate expensive tap ability without sufficient mana")
		}
	})

	t.Run("Cannot activate when tapped even with mana", func(t *testing.T) {
		// Give player mana but keep permanent tapped
		permanent.tapped = true
		player.manaPool[types.Any] = 5

		canActivate := engine.canActivateAbility(expensiveAbility, player)
		if canActivate {
			t.Error("Should not be able to activate tap ability when permanent is tapped, even with mana")
		}
	})
}

// TestTapAbilityTiming tests tap abilities in different game phases
func TestTapAbilityTiming(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[types.ManaType]int),
	}

	// Create a permanent with a sorcery-speed tap ability
	permanent := &mockPermanent{
		id:         uuid.New(),
		name:       "Sorcery Speed Creature",
		owner:      player,
		controller: player,
		tapped:     false,
		abilities:  []*Ability{},
	}

	sorcerySpeedAbility := &Ability{
		ID:   uuid.New(),
		Name: "Sorcery Speed Tap Ability",
		Type: Activated,
		Cost: Cost{TapCost: true},
		Effects: []Effect{
			{
				Type:        DealDamage,
				Value:       2,
				Duration:    Instant,
				Description: "Deal 2 damage",
				Targets: []Target{
					{Type: PlayerTarget, Required: true, Count: 1},
				},
			},
		},
		TimingRestriction: SorcerySpeed,
		Source:            permanent,
	}

	permanent.abilities = []*Ability{sorcerySpeedAbility}

	t.Run("Can activate in main phase", func(t *testing.T) {
		gameState := &mockGameState{
			players:       []AbilityPlayer{player},
			currentPlayer: player,
			isMainPhase:   true,
		}

		engine := NewExecutionEngine(gameState)
		permanent.tapped = false

		canActivate := engine.canActivateAbility(sorcerySpeedAbility, player)
		if !canActivate {
			t.Error("Should be able to activate sorcery speed tap ability in main phase")
		}
	})

	t.Run("Cannot activate outside main phase", func(t *testing.T) {
		gameState := &mockGameState{
			players:       []AbilityPlayer{player},
			currentPlayer: player,
			isMainPhase:   false, // Not main phase
		}

		engine := NewExecutionEngine(gameState)
		permanent.tapped = false

		canActivate := engine.canActivateAbility(sorcerySpeedAbility, player)
		if canActivate {
			t.Error("Should not be able to activate sorcery speed tap ability outside main phase")
		}
	})
}

// TestSummoningSickness tests that creatures cannot use tap abilities the turn they enter
func TestSummoningSickness(t *testing.T) {
	player := &mockPlayer{
		name:     "Test Player",
		life:     20,
		manaPool: make(map[types.ManaType]int),
	}

	gameState := &mockGameState{
		players:       []AbilityPlayer{player},
		currentPlayer: player,
		isMainPhase:   true,
	}

	engine := NewExecutionEngine(gameState)

	// Create a creature that just entered the battlefield
	creature := &mockPermanent{
		id:         uuid.New(),
		name:       "Fresh Creature",
		owner:      player,
		controller: player,
		tapped:     false,
		abilities:  []*Ability{},
		summoningSick: true, // Just entered this turn
	}

	tapAbility := &Ability{
		ID:   uuid.New(),
		Name: "Creature Tap Ability",
		Type: Activated,
		Cost: Cost{TapCost: true},
		Effects: []Effect{
			{
				Type:        DealDamage,
				Value:       1,
				Duration:    Instant,
				Description: "Deal 1 damage",
				Targets: []Target{
					{Type: PlayerTarget, Required: true, Count: 1},
				},
			},
		},
		Source: creature,
	}

	creature.abilities = []*Ability{tapAbility}

	t.Run("Cannot activate tap ability with summoning sickness", func(t *testing.T) {
		canActivate := engine.canActivateAbility(tapAbility, player)
		if canActivate {
			t.Error("Should not be able to activate tap ability on creature with summoning sickness")
		}
	})

	t.Run("Can activate after summoning sickness wears off", func(t *testing.T) {
		// Simulate creature surviving to next turn
		creature.summoningSick = false

		canActivate := engine.canActivateAbility(tapAbility, player)
		if !canActivate {
			t.Error("Should be able to activate tap ability after summoning sickness wears off")
		}
	})

	t.Run("Mana abilities work despite summoning sickness", func(t *testing.T) {
		// Reset summoning sickness
		creature.summoningSick = true

		manaAbility := &Ability{
			ID:   uuid.New(),
			Name: "Creature Mana Ability",
			Type: Mana, // Mana abilities ignore summoning sickness
			Cost: Cost{TapCost: true},
			Effects: []Effect{
				{
					Type:        AddMana,
					Value:       1,
					Duration:    Instant,
					Description: "Add {G}",
				},
			},
			Source: creature,
		}

		canActivate := engine.canActivateAbility(manaAbility, player)
		if !canActivate {
			t.Error("Should be able to activate mana abilities despite summoning sickness")
		}
	})
}
