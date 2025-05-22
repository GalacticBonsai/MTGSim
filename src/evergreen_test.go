package main

import (
	"testing"
)

func TestGetEvergreenAbilityByName(t *testing.T) {
	tests := []struct {
		abilityName string
		expected    bool
	}{
		{"Flying", true},
		{"Deathtouch", true},
		{"First Strike", true},
		{"Trample", true},
		{"NonexistentAbility", false},
	}

	for _, test := range tests {
		_, exists := GetEvergreenAbilityByName(test.abilityName)
		if exists != test.expected {
			t.Errorf("GetEvergreenAbilityByName(%s) = %v; expected %v", test.abilityName, exists, test.expected)
		}
	}
}

func TestCardHasEvergreenAbility(t *testing.T) {
	card := Card{
		Name:     "Serra Angel",
		Keywords: []string{"Flying", "Vigilance"},
	}

	tests := []struct {
		abilityName string
		expected    bool
	}{
		{"Flying", true},
		{"Vigilance", true},
		{"Deathtouch", false},
		{"First Strike", false},
	}

	for _, test := range tests {
		hasAbility := CardHasEvergreenAbility(card, test.abilityName)
		if hasAbility != test.expected {
			t.Errorf("CardHasEvergreenAbility(%s) = %v; expected %v", test.abilityName, hasAbility, test.expected)
		}
	}
}

func TestCardWithMultipleAbilities(t *testing.T) {
	card := Card{
		Name:     "Akroma, Angel of Wrath",
		Keywords: []string{"Flying", "First Strike", "Trample", "Haste", "Vigilance"},
	}

	tests := []struct {
		abilityName string
		expected    bool
	}{
		{"Flying", true},
		{"First Strike", true},
		{"Trample", true},
		{"Haste", true},
		{"Vigilance", true},
		{"Deathtouch", false},
	}

	for _, test := range tests {
		hasAbility := CardHasEvergreenAbility(card, test.abilityName)
		if hasAbility != test.expected {
			t.Errorf("CardHasEvergreenAbility(%s) = %v; expected %v", test.abilityName, hasAbility, test.expected)
		}
	}
}

func TestCardWithoutAbilities(t *testing.T) {
	card := Card{
		Name:     "Grizzly Bears",
		Keywords: []string{},
	}

	tests := []struct {
		abilityName string
		expected    bool
	}{
		{"Flying", false},
		{"Vigilance", false},
		{"Deathtouch", false},
	}

	for _, test := range tests {
		hasAbility := CardHasEvergreenAbility(card, test.abilityName)
		if hasAbility != test.expected {
			t.Errorf("CardHasEvergreenAbility(%s) = %v; expected %v", test.abilityName, hasAbility, test.expected)
		}
	}
}

func TestGetEvergreenAbilityDescriptions(t *testing.T) {
	tests := []struct {
		abilityName string
		expected    string
	}{
		{"Flying", "This creature can't be blocked except by creatures with flying or reach."},
		{"Deathtouch", "Any amount of damage this deals to a creature is enough to destroy it."},
		{"NonexistentAbility", ""},
	}

	for _, test := range tests {
		ability, exists := GetEvergreenAbilityByName(test.abilityName)
		if exists {
			if ability.Description != test.expected {
				t.Errorf("GetEvergreenAbilityByName(%s) description = %v; expected %v", test.abilityName, ability.Description, test.expected)
			}
		} else if test.expected != "" {
			t.Errorf("GetEvergreenAbilityByName(%s) = false; expected true", test.abilityName)
		}
	}
}
