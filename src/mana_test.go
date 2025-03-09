package main

import (
	"testing"
)

func TestParseManaCost(t *testing.T) {
	tests := []struct {
		cost     string
		expected mana
	}{
		{"{W}{W}{U}{U}{B}{B}{R}{R}{G}{G}", mana{pool: map[ManaType]int{White: 2, Blue: 2, Black: 2, Red: 2, Green: 2}}},   // Progenitus
		{"{G}{G}{G}{G}{G}{G}{G}{G}", mana{pool: map[ManaType]int{Green: 8}}},                                              // Khalni Hydra
		{"{5}{C}{C}", mana{pool: map[ManaType]int{Any: 5, Colorless: 2}}},                                                 // Devourer of Destiny
		{"{W}{U}{B}{R}{G}{C}", mana{pool: map[ManaType]int{White: 1, Blue: 1, Black: 1, Red: 1, Green: 1, Colorless: 1}}}, // Slivdrazi Monstrosity
		{"{C}", mana{pool: map[ManaType]int{Colorless: 1}}},                                                               // Eldritch Immunity
		{"{S}", mana{pool: map[ManaType]int{Snow: 1}}},                                                                    // Icehide Golem
		{"{R/G}{R/G}", mana{pool: map[ManaType]int{}}},                                                                    // Gruul guildmage
		{"{2/W}{2/U}{2/B}{2/R}{2/G}", mana{pool: map[ManaType]int{}}},                                                     // Reaper King
		{"{U/P}", mana{pool: map[ManaType]int{}}},                                                                         // Gitaxian probe
		{"{2}{G}{G/U/P}{U}", mana{pool: map[ManaType]int{Any: 2, Green: 1, Blue: 1}}},                                     // Tamiyo, Compleated Sage
		{"{X}{X}{W}{W}{W}", mana{pool: map[ManaType]int{White: 3, X: 2}}},                                               // Entreat the Angels
	}

	for _, test := range tests {
		result := ParseManaCost(test.cost)
		if len(result.pool) != len(test.expected.pool) {
			t.Errorf("ParseManaCost(%s) = %v; expected %v", test.cost, result, test.expected)
			continue
		}
		for mt, count := range test.expected.pool {
			if result.pool[mt] != count {
				t.Errorf("ParseManaCost(%s) = %v; expected %v", test.cost, result, test.expected)
				break
			}
		}
	}
}

func TestCheckManaProducer(t *testing.T) {
	tests := []struct {
		oracleText string
		expected   bool
		manaTypes  []ManaType
	}{
		{"{T}, Sacrifice this artifact: Add one mana of any color", true, []ManaType{Any}},
		{"Sacrifice a creature: Add {C}{C}.", true, []ManaType{Colorless,Colorless}},
		{"({T}: Add {R}.)", true, []ManaType{Red}},
		{"({T}: Add {U} or {R}.)", true, []ManaType{Blue, Red}},
		{"{T}: Add one mana of any color in your commander’s color identity.", true, []ManaType{Any}},
	}

	for _, test := range tests {
		isProducer, manaTypes := CheckManaProducer(test.oracleText)
		if isProducer != test.expected || len(manaTypes) != len(test.manaTypes) {
			t.Errorf("CheckManaProducer(%s) = (%v, %v); expected (%v, %v)", test.oracleText, isProducer, manaTypes, test.expected, test.manaTypes)
			continue
		}
		for i, manaType := range test.manaTypes {
			if manaTypes[i] != manaType {
				t.Errorf("CheckManaProducer(%s) = (%v, %v); expected (%v, %v)", test.oracleText, isProducer, manaTypes, test.expected, test.manaTypes)
				break
			}
		}
	}
}
