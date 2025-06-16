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
		expected   string
	}{
		{
			name:       "Simple Flying",
			oracleText: "Flying",
			keywords:   []string{},
			expected:   "Should parse Flying keyword",
		},
		{
			name:       "Flying with reminder text",
			oracleText: "Flying (This creature can't be blocked except by creatures with flying or reach.)",
			keywords:   []string{},
			expected:   "Should parse Flying from reminder text",
		},
		{
			name:       "Simple Deathtouch",
			oracleText: "Deathtouch",
			keywords:   []string{},
			expected:   "Should parse Deathtouch keyword",
		},
		{
			name:       "Multi-keyword",
			oracleText: "Flying, haste",
			keywords:   []string{},
			expected:   "Should parse multiple keywords",
		},
		{
			name:       "Official Keywords",
			oracleText: "Some other text",
			keywords:   []string{"Flying", "Haste"},
			expected:   "Should parse from official keywords field",
		},
		{
			name:       "ETB Draw",
			oracleText: "When Cloudkin Seer enters, draw a card.",
			keywords:   []string{},
			expected:   "Should parse ETB draw ability",
		},
		{
			name:       "ETB Gain Life",
			oracleText: "When Healer of the Glade enters, you gain 3 life.",
			keywords:   []string{},
			expected:   "Should parse ETB gain life ability",
		},
	}

	fmt.Println("Testing Enhanced Ability Parser Improvements")
	fmt.Println("===========================================")

	successCount := 0
	totalCount := len(testCases)

	for _, tc := range testCases {
		fmt.Printf("\n🧪 Testing: %s\n", tc.name)
		fmt.Printf("   Oracle Text: %s\n", tc.oracleText)
		fmt.Printf("   Keywords: %v\n", tc.keywords)
		fmt.Printf("   Expected: %s\n", tc.expected)

		mockCard := MockCard{Name: tc.name, Keywords: tc.keywords}
		abilities, err := parser.ParseAbilities(tc.oracleText, mockCard)

		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
			continue
		}

		if len(abilities) > 0 {
			fmt.Printf("   ✅ Successfully parsed %d abilities:\n", len(abilities))
			for i, ability := range abilities {
				parseSource := "oracle text"
				if !ability.ParsedFromText {
					parseSource = "official keywords"
				}
				fmt.Printf("      %d. %s (Type: %v, Source: %s)\n", 
					i+1, ability.Name, ability.Type, parseSource)
				if len(ability.Effects) > 0 {
					fmt.Printf("         Effect: %s\n", ability.Effects[0].Description)
				}
			}
			successCount++
		} else {
			fmt.Printf("   ❌ No abilities parsed\n")
		}
	}

	fmt.Printf("\n\n📊 Summary\n")
	fmt.Printf("==========\n")
	fmt.Printf("Successfully parsed: %d/%d (%.1f%%)\n", 
		successCount, totalCount, float64(successCount)/float64(totalCount)*100)

	if successCount == totalCount {
		fmt.Println("🎉 All test cases passed! Enhanced parser is working correctly.")
	} else {
		fmt.Printf("⚠️  %d test cases still failing - need further improvements\n", totalCount-successCount)
	}

	// Test some specific failing cases from the log
	fmt.Printf("\n\n🔍 Testing Specific Failing Cases from Log\n")
	fmt.Printf("==========================================\n")

	failingCases := []string{
		"Flying",
		"Deathtouch",
		"Flying, haste",
		"Flying (This creature can't be blocked except by creatures with flying or reach.)",
		"Deathtouch (Any amount of damage this deals to a creature is enough to destroy it.)",
		"Haste (This creature can attack and {T} as soon as it comes under your control.)",
		"Trample (This creature can deal excess combat damage to the player or planeswalker it's attacking.)",
		"Reach (This creature can block creatures with flying.)",
		"Defender (This creature can't attack.)",
		"Vigilance (Attacking doesn't cause this creature to tap.)",
	}

	improvedCount := 0
	for i, oracleText := range failingCases {
		fmt.Printf("\n%d. Testing: %s\n", i+1, oracleText)
		
		mockCard := MockCard{Name: fmt.Sprintf("Test Card %d", i+1), Keywords: []string{}}
		abilities, err := parser.ParseAbilities(oracleText, mockCard)

		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
			continue
		}

		if len(abilities) > 0 {
			fmt.Printf("   ✅ Now parsing successfully! (%d abilities)\n", len(abilities))
			for _, ability := range abilities {
				fmt.Printf("      - %s: %s\n", ability.Name, ability.Effects[0].Description)
			}
			improvedCount++
		} else {
			fmt.Printf("   ❌ Still failing to parse\n")
		}
	}

	fmt.Printf("\n📈 Improvement Results\n")
	fmt.Printf("=====================\n")
	fmt.Printf("Previously failing cases now working: %d/%d (%.1f%%)\n", 
		improvedCount, len(failingCases), float64(improvedCount)/float64(len(failingCases))*100)
}
