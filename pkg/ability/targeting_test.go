package ability

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/types"
)

// Mock implementations for targeting tests

type mockTargetPermanent struct {
	id           uuid.UUID
	name         string
	controller   AbilityPlayer
	tapped       bool
	isCreature   bool
	isArtifact   bool
	isEnchantment bool
	isLand       bool
	isPlaneswalker bool
	power        int
	toughness    int
	cmc          int
	abilities    []string
	hasHexproof  bool
	hasShroud    bool
}

func (m *mockTargetPermanent) GetID() uuid.UUID {
	return m.id
}

func (m *mockTargetPermanent) GetName() string {
	return m.name
}

func (m *mockTargetPermanent) GetOwner() AbilityPlayer {
	return m.controller
}

func (m *mockTargetPermanent) GetController() AbilityPlayer {
	return m.controller
}

func (m *mockTargetPermanent) IsTapped() bool {
	return m.tapped
}

func (m *mockTargetPermanent) Tap() {
	m.tapped = true
}

func (m *mockTargetPermanent) Untap() {
	m.tapped = false
}

func (m *mockTargetPermanent) GetAbilities() []*Ability {
	return nil
}

func (m *mockTargetPermanent) AddAbility(ability *Ability) {
	// Mock implementation
}

func (m *mockTargetPermanent) RemoveAbility(abilityID uuid.UUID) {
	// Mock implementation
}

// Implement CreatureStats interface
func (m *mockTargetPermanent) GetPower() int {
	return m.power
}

func (m *mockTargetPermanent) GetToughness() int {
	return m.toughness
}

func (m *mockTargetPermanent) GetCMC() int {
	return m.cmc
}

// Implement TypeChecker interface
func (m *mockTargetPermanent) IsCreature() bool {
	return m.isCreature
}

func (m *mockTargetPermanent) IsArtifact() bool {
	return m.isArtifact
}

func (m *mockTargetPermanent) IsEnchantment() bool {
	return m.isEnchantment
}

func (m *mockTargetPermanent) IsLand() bool {
	return m.isLand
}

func (m *mockTargetPermanent) IsPlaneswalker() bool {
	return m.isPlaneswalker
}

type mockTargetPlayer struct {
	name string
}

func (m *mockTargetPlayer) GetName() string {
	return m.name
}

func (m *mockTargetPlayer) GetLifeTotal() int {
	return 20
}

func (m *mockTargetPlayer) SetLifeTotal(life int) {
	// Mock implementation
}

func (m *mockTargetPlayer) GetHand() []interface{} {
	return nil
}

func (m *mockTargetPlayer) AddCardToHand(card interface{}) {
	// Mock implementation
}

func (m *mockTargetPlayer) GetCreatures() []interface{} {
	return nil
}

func (m *mockTargetPlayer) GetLands() []interface{} {
	return nil
}

func (m *mockTargetPlayer) CanPayCost(cost Cost) bool {
	return true
}

func (m *mockTargetPlayer) PayCost(cost Cost) error {
	return nil
}

func (m *mockTargetPlayer) GetManaPool() map[types.ManaType]int {
	return make(map[types.ManaType]int)
}

func TestTargetParser_ParseBasicTargets(t *testing.T) {
	parser := NewTargetParser()

	tests := []struct {
		name           string
		oracleText     string
		expectedCount  int
		expectedType   TargetType
		expectedRequired bool
	}{
		{
			name:           "Target creature",
			oracleText:     "Target creature gets +2/+2 until end of turn.",
			expectedCount:  1,
			expectedType:   CreatureTarget,
			expectedRequired: true,
		},
		{
			name:           "Target player",
			oracleText:     "Target player draws a card.",
			expectedCount:  1,
			expectedType:   PlayerTarget,
			expectedRequired: true,
		},
		{
			name:           "Target permanent",
			oracleText:     "Destroy target permanent.",
			expectedCount:  1,
			expectedType:   PermanentTarget,
			expectedRequired: true,
		},
		{
			name:           "Any target",
			oracleText:     "Deal 3 damage to any target.",
			expectedCount:  1,
			expectedType:   AnyTarget,
			expectedRequired: true,
		},
		{
			name:           "Each creature (non-targeting)",
			oracleText:     "Each creature gets +1/+1 until end of turn.",
			expectedCount:  1,
			expectedType:   CreatureTarget,
			expectedRequired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets, err := parser.ParseTargetRestrictions(tt.oracleText)
			if err != nil {
				t.Errorf("ParseTargetRestrictions() error = %v", err)
				return
			}

			if len(targets) != tt.expectedCount {
				t.Errorf("ParseTargetRestrictions() got %d targets, want %d", len(targets), tt.expectedCount)
				return
			}

			if len(targets) > 0 {
				target := targets[0]
				if target.Type != tt.expectedType {
					t.Errorf("ParseTargetRestrictions() target type = %v, want %v", target.Type, tt.expectedType)
				}

				if target.Required != tt.expectedRequired {
					t.Errorf("ParseTargetRestrictions() target required = %v, want %v", target.Required, tt.expectedRequired)
				}

				if target.IsEach != !tt.expectedRequired {
					t.Errorf("ParseTargetRestrictions() target isEach = %v, want %v", target.IsEach, !tt.expectedRequired)
				}
			}
		})
	}
}

func TestTargetParser_ParseComplexRestrictions(t *testing.T) {
	parser := NewTargetParser()

	tests := []struct {
		name                string
		oracleText          string
		expectedRestrictions int
		checkRestriction    func(restrictions []TargetRestriction) bool
	}{
		{
			name:                "Non-artifact creature",
			oracleText:          "Target non-artifact creature gets +3/+3.",
			expectedRestrictions: 2,
			checkRestriction: func(restrictions []TargetRestriction) bool {
				hasCreature := false
				hasNonArtifact := false
				for _, r := range restrictions {
					if r.Type == CreatureRestriction {
						hasCreature = true
					}
					if r.Type == ArtifactRestriction && r.Negated {
						hasNonArtifact = true
					}
				}
				return hasCreature && hasNonArtifact
			},
		},
		{
			name:                "Creature with flying",
			oracleText:          "Target creature with flying gains vigilance.",
			expectedRestrictions: 2,
			checkRestriction: func(restrictions []TargetRestriction) bool {
				hasCreature := false
				hasFlying := false
				for _, r := range restrictions {
					if r.Type == CreatureRestriction {
						hasCreature = true
					}
					if r.Type == FlyingRestriction {
						hasFlying = true
					}
				}
				return hasCreature && hasFlying
			},
		},
		{
			name:                "Creature with power 2 or less",
			oracleText:          "Destroy target creature with power 2 or less.",
			expectedRestrictions: 2,
			checkRestriction: func(restrictions []TargetRestriction) bool {
				hasCreature := false
				hasPowerRestriction := false
				for _, r := range restrictions {
					if r.Type == CreatureRestriction {
						hasCreature = true
					}
					if r.Type == PowerLessEqualRestriction {
						if value, ok := r.Value.(int); ok && value == 2 {
							hasPowerRestriction = true
						}
					}
				}
				return hasCreature && hasPowerRestriction
			},
		},
		{
			name:                "Creature you control",
			oracleText:          "Target creature you control gains trample.",
			expectedRestrictions: 2,
			checkRestriction: func(restrictions []TargetRestriction) bool {
				hasCreature := false
				hasControl := false
				for _, r := range restrictions {
					if r.Type == CreatureRestriction {
						hasCreature = true
					}
					if r.Type == YouControlRestriction {
						hasControl = true
					}
				}
				return hasCreature && hasControl
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets, err := parser.ParseTargetRestrictions(tt.oracleText)
			if err != nil {
				t.Errorf("ParseTargetRestrictions() error = %v", err)
				return
			}

			if len(targets) == 0 {
				t.Error("ParseTargetRestrictions() returned no targets")
				return
			}

			target := targets[0]
			if len(target.Restrictions) != tt.expectedRestrictions {
				t.Errorf("ParseTargetRestrictions() got %d restrictions, want %d", len(target.Restrictions), tt.expectedRestrictions)
				return
			}

			if !tt.checkRestriction(target.Restrictions) {
				t.Error("ParseTargetRestrictions() restrictions don't match expected pattern")
			}
		})
	}
}

func TestTargetValidator_ValidateBasicTargets(t *testing.T) {
	// Create mock game state
	player1 := &mockTargetPlayer{name: "Player 1"}
	player2 := &mockTargetPlayer{name: "Player 2"}

	gameState := &mockGameState{
		players:       []AbilityPlayer{player1, player2},
		currentPlayer: player1,
		isMainPhase:   true,
	}

	validator := NewTargetValidator(gameState)

	// Create test targets
	creature := &mockTargetPermanent{
		id:         uuid.New(),
		name:       "Test Creature",
		controller: player1,
		isCreature: true,
		power:      2,
		toughness:  3,
		cmc:        3,
	}

	artifact := &mockTargetPermanent{
		id:         uuid.New(),
		name:       "Test Artifact",
		controller: player1,
		isArtifact: true,
		cmc:        2,
	}

	tests := []struct {
		name           string
		target         interface{}
		enhancedTarget EnhancedTarget
		controller     AbilityPlayer
		expectedLegal  bool
		expectedReason string
	}{
		{
			name:   "Valid creature target",
			target: creature,
			enhancedTarget: EnhancedTarget{
				Type:     CreatureTarget,
				Required: true,
				Restrictions: []TargetRestriction{
					{Type: CreatureRestriction, Description: "must be a creature"},
				},
			},
			controller:     player1,
			expectedLegal:  true,
			expectedReason: "Valid target",
		},
		{
			name:   "Invalid creature target (artifact)",
			target: artifact,
			enhancedTarget: EnhancedTarget{
				Type:     CreatureTarget,
				Required: true,
				Restrictions: []TargetRestriction{
					{Type: CreatureRestriction, Description: "must be a creature"},
				},
			},
			controller:     player1,
			expectedLegal:  false,
			expectedReason: "Target type mismatch",
		},
		{
			name:   "Valid player target",
			target: player2,
			enhancedTarget: EnhancedTarget{
				Type:     PlayerTarget,
				Required: true,
				Restrictions: []TargetRestriction{
					{Type: PlayerRestriction, Description: "must be a player"},
				},
			},
			controller:     player1,
			expectedLegal:  true,
			expectedReason: "Valid target",
		},
		{
			name:   "Control restriction - valid",
			target: creature,
			enhancedTarget: EnhancedTarget{
				Type:     CreatureTarget,
				Required: true,
				Restrictions: []TargetRestriction{
					{Type: CreatureRestriction, Description: "must be a creature"},
					{Type: YouControlRestriction, Description: "you must control"},
				},
			},
			controller:     player1,
			expectedLegal:  true,
			expectedReason: "Valid target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			legality := validator.ValidateTarget(tt.target, tt.enhancedTarget, tt.controller)

			if legality.IsLegal != tt.expectedLegal {
				t.Errorf("ValidateTarget() legality = %v, want %v", legality.IsLegal, tt.expectedLegal)
			}

			if tt.expectedReason != "" && !strings.Contains(legality.Reason, tt.expectedReason) {
				t.Errorf("ValidateTarget() reason = %s, want to contain %s", legality.Reason, tt.expectedReason)
			}
		})
	}
}

func TestTargetValidator_ValidateComplexRestrictions(t *testing.T) {
	// Create mock game state
	player1 := &mockTargetPlayer{name: "Player 1"}
	player2 := &mockTargetPlayer{name: "Player 2"}

	gameState := &mockGameState{
		players:       []AbilityPlayer{player1, player2},
		currentPlayer: player1,
		isMainPhase:   true,
	}

	validator := NewTargetValidator(gameState)

	// Create test creatures with different properties
	smallCreature := &mockTargetPermanent{
		id:         uuid.New(),
		name:       "Small Creature",
		controller: player1,
		isCreature: true,
		power:      1,
		toughness:  1,
		cmc:        1,
	}

	bigCreature := &mockTargetPermanent{
		id:         uuid.New(),
		name:       "Big Creature",
		controller: player2,
		isCreature: true,
		power:      5,
		toughness:  5,
		cmc:        6,
	}

	artifactCreature := &mockTargetPermanent{
		id:         uuid.New(),
		name:       "Artifact Creature",
		controller: player1,
		isCreature: true,
		isArtifact: true,
		power:      2,
		toughness:  2,
		cmc:        3,
	}

	tests := []struct {
		name           string
		target         interface{}
		restrictions   []TargetRestriction
		controller     AbilityPlayer
		expectedLegal  bool
		description    string
	}{
		{
			name:   "Power restriction - valid",
			target: smallCreature,
			restrictions: []TargetRestriction{
				{Type: CreatureRestriction, Description: "must be a creature"},
				{Type: PowerLessEqualRestriction, Value: 2, Description: "power 2 or less"},
			},
			controller:    player1,
			expectedLegal: true,
			description:   "Small creature should pass power restriction",
		},
		{
			name:   "Power restriction - invalid",
			target: bigCreature,
			restrictions: []TargetRestriction{
				{Type: CreatureRestriction, Description: "must be a creature"},
				{Type: PowerLessEqualRestriction, Value: 2, Description: "power 2 or less"},
			},
			controller:    player1,
			expectedLegal: false,
			description:   "Big creature should fail power restriction",
		},
		{
			name:   "Non-artifact restriction - valid",
			target: smallCreature,
			restrictions: []TargetRestriction{
				{Type: CreatureRestriction, Description: "must be a creature"},
				{Type: ArtifactRestriction, Negated: true, Description: "must not be an artifact"},
			},
			controller:    player1,
			expectedLegal: true,
			description:   "Non-artifact creature should pass non-artifact restriction",
		},
		{
			name:   "Non-artifact restriction - invalid",
			target: artifactCreature,
			restrictions: []TargetRestriction{
				{Type: CreatureRestriction, Description: "must be a creature"},
				{Type: ArtifactRestriction, Negated: true, Description: "must not be an artifact"},
			},
			controller:    player1,
			expectedLegal: false,
			description:   "Artifact creature should fail non-artifact restriction",
		},
		{
			name:   "Control restriction - valid",
			target: smallCreature,
			restrictions: []TargetRestriction{
				{Type: CreatureRestriction, Description: "must be a creature"},
				{Type: YouControlRestriction, Description: "you must control"},
			},
			controller:    player1,
			expectedLegal: true,
			description:   "Own creature should pass control restriction",
		},
		{
			name:   "Control restriction - invalid",
			target: bigCreature,
			restrictions: []TargetRestriction{
				{Type: CreatureRestriction, Description: "must be a creature"},
				{Type: YouControlRestriction, Description: "you must control"},
			},
			controller:    player1,
			expectedLegal: false,
			description:   "Opponent's creature should fail control restriction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enhancedTarget := EnhancedTarget{
				Type:         CreatureTarget,
				Required:     true,
				Restrictions: tt.restrictions,
			}

			legality := validator.ValidateTarget(tt.target, enhancedTarget, tt.controller)

			if legality.IsLegal != tt.expectedLegal {
				t.Errorf("ValidateTarget() legality = %v, want %v: %s", legality.IsLegal, tt.expectedLegal, tt.description)
				t.Logf("Reason: %s", legality.Reason)
			}
		})
	}
}
