package main

import (
	"testing"
)

func TestParseManaCost(t *testing.T) {
	tests := []struct {
		cost     string
		expected mana
	}{
		{"{W}{W}{U}{U}{B}{B}{R}{R}{G}{G}", mana{w: 2, u: 2, b: 2, r: 2, g: 2}}, // Progenitus
		{"{G}{G}{G}{G}{G}{G}{G}{G}", mana{g: 8}},                               // Khalni Hydra
		{"{5}{C}{C}", mana{a: 5, c: 2}},                                        // Devourer of Destiny
		{"{W}{U}{B}{R}{G}{C}", mana{w: 1, u: 1, b: 1, r: 1, g: 1, c: 1}},       // Slivdrazi Monstrosity
		{"{C}", mana{c: 1}},                          // Eldritch Immunity
		{"{S}", mana{s: 1}},                          // Icehide Golem
		{"{R/G}{R/G}", mana{}},                       // Gruul guildmage
		{"{2/W}{2/U}{2/B}{2/R}{2/G}", mana{}},        // Reaper King
		{"{U/P}", mana{}},                            // Gitaxian probe
		{"{2}{G}{G/U/P}{U}", mana{a: 2, g: 1, u: 1}}, // Tamiyo, Compleated Sage
		{"{X}{X}{W}{W}{W}", mana{w: 3, a: 2}},        // Entreat the Angels
	}

	for _, test := range tests {
		result := ParseManaCost(test.cost)
		if result != test.expected {
			t.Errorf("ParseManaCost(%s) = %v; expected %v", test.cost, result, test.expected)
		}
	}
}
