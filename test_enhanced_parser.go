package main

import (
	"fmt"
	"log"

	"github.com/mtgsim/mtgsim/pkg/ability"
)

// MockCard represents a mock card for testing
type MockCard struct {
	Name     string
	Keywords []string
}

func (m MockCard) GetName() string {
	return m.Name
}

func (m MockCard) GetKeywords() []string {
	return m.Keywords
}

func main() {
	parser := ability.NewAbilityParser()

	// Test cases from the parsing failures log
	testCases := []struct {
		name       string
		oracleText string
		keywords   []string
	}{
		{
			name:       "Bartizan Bats",
			oracleText: "Flying (This creature can't be blocked except by creatures with flying or reach.)",
			keywords:   []string{"Flying"},
		},
		{
			name:       "Feral Abomination",
			oracleText: "Deathtouch (Any amount of damage this deals to a creature is enough to destroy it.)",
			keywords:   []string{"Deathtouch"},
		},
		{
			name:       "Air Elemental",
			oracleText: "Flying",
			keywords:   []string{"Flying"},
		},
		{
			name:       "Cloudkin Seer",
			oracleText: "Flying\nWhen Cloudkin Seer enters, draw a card.",
			keywords:   []string{"Flying"},
		},
		{
			name:       "Volcanic Dragon",
			oracleText: "Flying, haste",
			keywords:   []string{"Flying", "Haste"},
		},
		{
			name:       "Gravedigger",
			oracleText: "When Gravedigger enters, you may return target creature card from your graveyard to your hand.",
			keywords:   []string{},
		},
		{
			name:       "Disfigure",
			oracleText: "Target creature gets -2/-2 until end of turn.",
			keywords:   []string{},
		},
		{
			name:       "Unsummon",
			oracleText: "Return target creature to its owner's hand.",
			keywords:   []string{},
		},
	}

	fmt.Println("Testing Enhanced Ability Parser")
	fmt.Println("================================")

	successCount := 0
	totalCount := len(testCases)

	for _, tc := range testCases {
		fmt.Printf("\nTesting: %s\n", tc.name)
		fmt.Printf("Oracle Text: %s\n", tc.oracleText)
		fmt.Printf("Keywords: %v\n", tc.keywords)

		mockCard := MockCard{Name: tc.name, Keywords: tc.keywords}
		abilities, err := parser.ParseAbilities(tc.oracleText, mockCard)

		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		if len(abilities) > 0 {
			fmt.Printf("✅ Successfully parsed %d abilities:\n", len(abilities))
			for i, ability := range abilities {
				fmt.Printf("  %d. %s (Type: %v, ParsedFromText: %v)\n", 
					i+1, ability.Name, ability.Type, ability.ParsedFromText)
				if len(ability.Effects) > 0 {
					fmt.Printf("     Effect: %s\n", ability.Effects[0].Description)
				}
			}
			successCount++
		} else {
			fmt.Printf("❌ No abilities parsed\n")
		}
	}

	fmt.Printf("\n\nSummary\n")
	fmt.Printf("=======\n")
	fmt.Printf("Successfully parsed: %d/%d (%.1f%%)\n", 
		successCount, totalCount, float64(successCount)/float64(totalCount)*100)

	if successCount == totalCount {
		fmt.Println("🎉 All test cases passed!")
	} else {
		fmt.Printf("⚠️  %d test cases still failing\n", totalCount-successCount)
	}
}
