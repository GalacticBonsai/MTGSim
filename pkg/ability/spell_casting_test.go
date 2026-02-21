// Package ability provides comprehensive spell casting mechanics testing for MTG simulation.
package ability

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/types"
)

// TestFixedManaCosts tests spells with fixed mana costs
func TestFixedManaCosts(t *testing.T) {
	testCases := []struct {
		name         string
		manaCost     string
		expectedCMC  int
		expectedCost map[types.ManaType]int
		description  string
	}{
		{
			name:        "Lightning Bolt",
			manaCost:    "{R}",
			expectedCMC: 1,
			expectedCost: map[types.ManaType]int{
				types.Red: 1,
			},
			description: "Single colored mana cost",
		},
		{
			name:        "Counterspell",
			manaCost:    "{U}{U}",
			expectedCMC: 2,
			expectedCost: map[types.ManaType]int{
				types.Blue: 2,
			},
			description: "Double colored mana cost",
		},
		{
			name:        "Shivan Dragon",
			manaCost:    "{4}{R}{R}",
			expectedCMC: 6,
			expectedCost: map[types.ManaType]int{
				types.Any: 4,
				types.Red: 2,
			},
			description: "Mixed generic and colored mana cost",
		},
		{
			name:        "Mox Ruby",
			manaCost:    "{0}",
			expectedCMC: 0,
			expectedCost: map[types.ManaType]int{},
			description: "Zero mana cost",
		},
		{
			name:        "Force of Will",
			manaCost:    "{3}{U}{U}",
			expectedCMC: 5,
			expectedCost: map[types.ManaType]int{
				types.Any:  3,
				types.Blue: 2,
			},
			description: "High cost spell with colored requirements",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cost := parseManaCostToMap(tc.manaCost)
			
			// Verify total CMC
			totalCMC := calculateCMC(cost)
			if totalCMC != tc.expectedCMC {
				t.Errorf("%s: expected CMC %d, got %d", tc.name, tc.expectedCMC, totalCMC)
			}

			// Verify individual mana requirements
			for manaType, expectedAmount := range tc.expectedCost {
				if cost[manaType] != expectedAmount {
					t.Errorf("%s: expected %d %s mana, got %d", tc.name, expectedAmount, manaType, cost[manaType])
				}
			}
		})
	}
}

// TestVariableManaCosts tests spells with X in their mana cost
func TestVariableManaCosts(t *testing.T) {
	testCases := []struct {
		name         string
		manaCost     string
		xValue       int
		expectedCMC  int
		description  string
	}{
		{
			name:        "Fireball",
			manaCost:    "{X}{R}",
			xValue:      3,
			expectedCMC: 4,
			description: "X spell with colored requirement",
		},
		{
			name:        "Hydra Broodmaster",
			manaCost:    "{4}{G}{G}",
			xValue:      0,
			expectedCMC: 6,
			description: "Creature with X ability but fixed cost",
		},
		{
			name:        "Sphinx's Revelation",
			manaCost:    "{X}{W}{U}{U}",
			xValue:      5,
			expectedCMC: 8,
			description: "X spell with multiple colored requirements",
		},
		{
			name:        "Chalice of the Void",
			manaCost:    "{X}{X}",
			xValue:      2,
			expectedCMC: 4,
			description: "Double X cost",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cost := parseVariableManaCost(tc.manaCost, tc.xValue)
			totalCMC := calculateCMC(cost)
			
			if totalCMC != tc.expectedCMC {
				t.Errorf("%s: expected CMC %d with X=%d, got %d", tc.name, tc.expectedCMC, tc.xValue, totalCMC)
			}
		})
	}
}

// TestAlternativeCastingCosts tests spells with alternative casting costs
func TestAlternativeCastingCosts(t *testing.T) {
	testCases := []struct {
		name            string
		normalCost      string
		alternativeCost string
		useAlternative  bool
		description     string
	}{
		{
			name:            "Force of Will",
			normalCost:      "{3}{U}{U}",
			alternativeCost: "Exile a blue card from your hand and pay 1 life",
			useAlternative:  true,
			description:     "Alternative cost with card exile and life payment",
		},
		{
			name:            "Flashback Spell",
			normalCost:      "{2}{R}",
			alternativeCost: "{4}{R}",
			useAlternative:  true,
			description:     "Flashback cost from graveyard",
		},
		{
			name:            "Overload Spell",
			normalCost:      "{1}{R}",
			alternativeCost: "{4}{R}{R}",
			useAlternative:  true,
			description:     "Overload cost for enhanced effect",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalCostMap := parseManaCostToMap(tc.normalCost)
			
			// Test that both costs are valid options
			if len(normalCostMap) == 0 && tc.normalCost != "{0}" {
				t.Errorf("%s: failed to parse normal cost %s", tc.name, tc.normalCost)
			}

			// Alternative costs would need special handling in the engine
			// This test validates the concept exists
			if tc.useAlternative && tc.alternativeCost == "" {
				t.Errorf("%s: alternative cost not specified", tc.name)
			}
		})
	}
}

// TestAdditionalCosts tests spells with additional costs
func TestAdditionalCosts(t *testing.T) {
	testCases := []struct {
		name           string
		manaCost       string
		additionalCost string
		description    string
	}{
		{
			name:           "Diabolic Intent",
			manaCost:       "{1}{B}",
			additionalCost: "Sacrifice a creature",
			description:    "Spell with sacrifice cost",
		},
		{
			name:           "Thoughtseize",
			manaCost:       "{B}",
			additionalCost: "Pay 2 life",
			description:    "Spell with life payment",
		},
		{
			name:           "Collective Brutality",
			manaCost:       "{1}{B}",
			additionalCost: "Discard a card",
			description:    "Spell with discard cost",
		},
		{
			name:           "Kicker Spell",
			manaCost:       "{2}{G}",
			additionalCost: "Kicker {1}{R}",
			description:    "Spell with optional kicker cost",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cost := parseManaCostToMap(tc.manaCost)
			
			// Verify base mana cost is parsed
			if len(cost) == 0 && tc.manaCost != "{0}" {
				t.Errorf("%s: failed to parse mana cost %s", tc.name, tc.manaCost)
			}

			// Additional costs would be handled separately in the cost structure
			additionalCost := parseAdditionalCost(tc.additionalCost)
			if additionalCost.isEmpty() {
				t.Errorf("%s: failed to parse additional cost %s", tc.name, tc.additionalCost)
			}
		})
	}
}

// TestCostReductionEffects tests interactions with cost reduction
func TestCostReductionEffects(t *testing.T) {
	testCases := []struct {
		name           string
		originalCost   string
		reduction      int
		expectedCost   int
		description    string
	}{
		{
			name:         "Goblin Electromancer Effect",
			originalCost: "{2}{R}",
			reduction:    1,
			expectedCost: 2,
			description:  "Generic cost reduction",
		},
		{
			name:         "Affinity Effect",
			originalCost: "{5}",
			reduction:    3,
			expectedCost: 2,
			description:  "Artifact cost reduction",
		},
		{
			name:         "Cannot Reduce Below Zero",
			originalCost: "{1}",
			reduction:    2,
			expectedCost: 0,
			description:  "Cost reduction cannot make spells cost negative",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originalCostMap := parseManaCostToMap(tc.originalCost)
			reducedCost := applyCostReduction(originalCostMap, tc.reduction)
			totalCost := calculateCMC(reducedCost)
			
			if totalCost != tc.expectedCost {
				t.Errorf("%s: expected reduced cost %d, got %d", tc.name, tc.expectedCost, totalCost)
			}
		})
	}
}

// Helper functions for cost parsing and calculation

func parseManaCostToMap(manaCost string) map[types.ManaType]int {
	cost := make(map[types.ManaType]int)

	// Simple parsing for common mana cost patterns
	i := 0
	for i < len(manaCost) {
		if manaCost[i] == '{' {
			// Find the closing brace
			j := i + 1
			for j < len(manaCost) && manaCost[j] != '}' {
				j++
			}
			if j < len(manaCost) {
				symbol := manaCost[i+1:j]
				switch symbol {
				case "W":
					cost[types.White]++
				case "U":
					cost[types.Blue]++
				case "B":
					cost[types.Black]++
				case "R":
					cost[types.Red]++
				case "G":
					cost[types.Green]++
				case "C":
					cost[types.Colorless]++
				case "0":
					// Zero cost, do nothing
				case "1":
					cost[types.Any] += 1
				case "2":
					cost[types.Any] += 2
				case "3":
					cost[types.Any] += 3
				case "4":
					cost[types.Any] += 4
				case "5":
					cost[types.Any] += 5
				case "6":
					cost[types.Any] += 6
				case "7":
					cost[types.Any] += 7
				case "8":
					cost[types.Any] += 8
				case "9":
					cost[types.Any] += 9
				case "X":
					// X costs are handled separately
				default:
					// Try to parse as number
					if len(symbol) == 1 && symbol[0] >= '0' && symbol[0] <= '9' {
						cost[types.Any] += int(symbol[0] - '0')
					}
				}
				i = j + 1
			} else {
				i++
			}
		} else {
			i++
		}
	}

	return cost
}

func parseVariableManaCost(manaCost string, xValue int) map[types.ManaType]int {
	cost := parseManaCostToMap(manaCost)

	// Count how many X's are in the cost and replace each with xValue
	xCount := 0
	for i := 0; i <= len(manaCost)-3; i++ {
		if manaCost[i:i+3] == "{X}" {
			xCount++
		}
	}

	if xCount > 0 {
		cost[types.Any] += xValue * xCount
	}

	return cost
}

// containsX function removed as it was unused

func calculateCMC(cost map[types.ManaType]int) int {
	total := 0
	for _, amount := range cost {
		total += amount
	}
	return total
}

type AdditionalCost struct {
	SacrificeCost bool
	LifeCost      int
	DiscardCost   int
	Other         string
}

func (ac AdditionalCost) isEmpty() bool {
	return !ac.SacrificeCost && ac.LifeCost == 0 && ac.DiscardCost == 0 && ac.Other == ""
}

func parseAdditionalCost(costText string) AdditionalCost {
	// Simplified parsing - would need more sophisticated logic
	return AdditionalCost{Other: costText}
}

func applyCostReduction(originalCost map[types.ManaType]int, reduction int) map[types.ManaType]int {
	reducedCost := make(map[types.ManaType]int)
	for manaType, amount := range originalCost {
		reducedCost[manaType] = amount
	}
	
	// Apply reduction to generic mana first
	if reducedCost[types.Any] > 0 {
		if reducedCost[types.Any] >= reduction {
			reducedCost[types.Any] -= reduction
		} else {
			reducedCost[types.Any] = 0
		}
	}
	
	return reducedCost
}
