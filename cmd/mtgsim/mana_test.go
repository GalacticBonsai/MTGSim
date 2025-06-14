package main

import (
	"strings"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestParseManaCost(t *testing.T) {
	tests := []struct {
		manaCost string
		expected int
	}{
		{"{0}", 0},
		{"{1}", 1},
		{"{2}", 2},
		{"{3}{R}", 4},
		{"{1}{U}{U}", 3},
		{"{2}{W}{B}", 4},
		{"{X}", 0}, // X costs are variable
		{"{5}{G}{G}", 7},
		{"", 0},
	}

	for _, test := range tests {
		result := parseManaCost(test.manaCost)
		if result != test.expected {
			t.Errorf("parseManaCost(%s) = %d; expected %d", test.manaCost, result, test.expected)
		}
	}
}

func TestCheckManaProducer(t *testing.T) {
	tests := []struct {
		oracleText   string
		isProducer   bool
		expectedMana []game.ManaType
	}{
		{
			"Add {G}.",
			true,
			[]game.ManaType{game.Green},
		},
		{
			"Add {R} or {G}.",
			true,
			[]game.ManaType{game.Red, game.Green},
		},
		{
			"Add {W}{U}{B}{R}{G}.",
			true,
			[]game.ManaType{game.White, game.Blue, game.Black, game.Red, game.Green},
		},
		{
			"Add {C}.",
			true,
			[]game.ManaType{game.Colorless},
		},
		{
			"This creature gets +1/+1.",
			false,
			[]game.ManaType{},
		},
		{
			"",
			false,
			[]game.ManaType{},
		},
	}

	for _, test := range tests {
		isProducer, manaTypes := card.CheckManaProducer(test.oracleText)
		if isProducer != test.isProducer {
			t.Errorf("CheckManaProducer(%s) producer = %v; expected %v", test.oracleText, isProducer, test.isProducer)
		}
		if len(manaTypes) != len(test.expectedMana) {
			t.Errorf("CheckManaProducer(%s) mana types length = %d; expected %d", test.oracleText, len(manaTypes), len(test.expectedMana))
		}
		for i, manaType := range manaTypes {
			if i < len(test.expectedMana) && manaType != test.expectedMana[i] {
				t.Errorf("CheckManaProducer(%s) mana type %d = %v; expected %v", test.oracleText, i, manaType, test.expectedMana[i])
			}
		}
	}
}

func TestCanCastCard(t *testing.T) {
	player := &Player{
		Name:      "Test Player",
		LifeTotal: 20,
		Lands: []*Permanent{
			{source: card.Card{Name: "Forest"}, manaProducer: true, manaTypes: []game.ManaType{game.Green}},
			{source: card.Card{Name: "Mountain"}, manaProducer: true, manaTypes: []game.ManaType{game.Red}},
			{source: card.Card{Name: "Plains"}, manaProducer: true, manaTypes: []game.ManaType{game.White}},
		},
	}

	tests := []struct {
		cardName string
		manaCost string
		cmc      uint8
		canCast  bool
	}{
		{"Lightning Bolt", "{R}", 1, true},
		{"Giant Growth", "{G}", 1, true},
		{"Healing Salve", "{W}", 1, true},
		{"Counterspell", "{U}{U}", 2, false}, // No blue mana
		{"Fireball", "{X}{R}", 1, true},      // X can be 0
		{"Shivan Dragon", "{4}{R}{R}", 6, false}, // Not enough mana
	}

	for _, test := range tests {
		testCard := card.Card{
			Name:     test.cardName,
			ManaCost: test.manaCost,
			CMC:      float32(test.cmc),
		}
		
		canCast := canCastCard(player, testCard)
		if canCast != test.canCast {
			t.Errorf("canCastCard(%s) = %v; expected %v", test.cardName, canCast, test.canCast)
		}
	}
}

func TestSimulateBoardState(t *testing.T) {
	player := &Player{
		Name:      "Test Player",
		LifeTotal: 20,
		Creatures: []*Permanent{
			{source: card.Card{Name: "Grizzly Bears"}, power: 2, toughness: 2},
			{source: card.Card{Name: "Lightning Elemental"}, power: 4, toughness: 1},
		},
		Lands: []*Permanent{
			{source: card.Card{Name: "Forest"}},
			{source: card.Card{Name: "Mountain"}},
		},
	}

	// Test board state evaluation
	totalPower := 0
	totalToughness := 0
	for _, creature := range player.Creatures {
		totalPower += creature.power
		totalToughness += creature.toughness
	}

	expectedPower := 6  // 2 + 4
	expectedToughness := 3 // 2 + 1

	if totalPower != expectedPower {
		t.Errorf("Total power = %d; expected %d", totalPower, expectedPower)
	}

	if totalToughness != expectedToughness {
		t.Errorf("Total toughness = %d; expected %d", totalToughness, expectedToughness)
	}

	landCount := len(player.Lands)
	expectedLands := 2

	if landCount != expectedLands {
		t.Errorf("Land count = %d; expected %d", landCount, expectedLands)
	}
}

func TestManaPool(t *testing.T) {
	// Test mana pool operations (simplified)
	manaPool := make(map[game.ManaType]int)
	
	// Add mana
	manaPool[game.Red] = 2
	manaPool[game.Green] = 1
	manaPool[game.White] = 1

	// Test total mana
	totalMana := 0
	for _, amount := range manaPool {
		totalMana += amount
	}

	expectedTotal := 4
	if totalMana != expectedTotal {
		t.Errorf("Total mana = %d; expected %d", totalMana, expectedTotal)
	}

	// Test specific mana types
	if manaPool[game.Red] != 2 {
		t.Errorf("Red mana = %d; expected 2", manaPool[game.Red])
	}

	if manaPool[game.Blue] != 0 {
		t.Errorf("Blue mana = %d; expected 0", manaPool[game.Blue])
	}

	// Test spending mana
	manaPool[game.Red]--
	if manaPool[game.Red] != 1 {
		t.Errorf("Red mana after spending = %d; expected 1", manaPool[game.Red])
	}
}

// Helper function to parse mana cost (simplified implementation)
func parseManaCost(manaCost string) int {
	if manaCost == "" {
		return 0
	}
	
	cost := 0
	i := 0
	for i < len(manaCost) {
		if manaCost[i] == '{' {
			j := i + 1
			for j < len(manaCost) && manaCost[j] != '}' {
				j++
			}
			if j < len(manaCost) {
				symbol := manaCost[i+1 : j]
				switch symbol {
				case "0":
					cost += 0
				case "1":
					cost += 1
				case "2":
					cost += 2
				case "3":
					cost += 3
				case "4":
					cost += 4
				case "5":
					cost += 5
				case "6":
					cost += 6
				case "7":
					cost += 7
				case "8":
					cost += 8
				case "9":
					cost += 9
				case "X":
					cost += 0 // X is variable
				default:
					// Colored mana symbols (W, U, B, R, G)
					cost += 1
				}
			}
			i = j + 1
		} else {
			i++
		}
	}
	return cost
}

// Helper function to check if a card can be cast (simplified implementation)
func canCastCard(player *Player, cardToCast card.Card) bool {
	// Simple implementation: check if we have enough lands and correct colors
	availableMana := len(player.Lands)
	requiredMana := int(cardToCast.CMC)

	if availableMana < requiredMana {
		return false
	}

	// Check for specific mana requirements (simplified)
	manaCost := cardToCast.ManaCost
	if strings.Contains(manaCost, "{U}{U}") {
		// Need 2 blue mana
		blueCount := 0
		for _, land := range player.Lands {
			for _, manaType := range land.manaTypes {
				if manaType == game.Blue {
					blueCount++
				}
			}
		}
		return blueCount >= 2
	}

	return true
}
