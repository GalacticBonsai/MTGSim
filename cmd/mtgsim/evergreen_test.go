package main

import (
	"testing"
	"github.com/mtgsim/mtgsim/pkg/card"
)

func TestCardHasEvergreenAbility(t *testing.T) {
	testCard := card.Card{
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

	// Create a permanent to test the hasEvergreenAbility method
	permanent := &Permanent{source: testCard}

	for _, test := range tests {
		hasAbility := permanent.hasEvergreenAbility(test.abilityName)
		if hasAbility != test.expected {
			t.Errorf("hasEvergreenAbility(%s) = %v; expected %v", test.abilityName, hasAbility, test.expected)
		}
	}
}

func TestCardWithMultipleAbilities(t *testing.T) {
	testCard := card.Card{
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

	permanent := &Permanent{source: testCard}

	for _, test := range tests {
		hasAbility := permanent.hasEvergreenAbility(test.abilityName)
		if hasAbility != test.expected {
			t.Errorf("hasEvergreenAbility(%s) = %v; expected %v", test.abilityName, hasAbility, test.expected)
		}
	}
}

func TestCardWithoutAbilities(t *testing.T) {
	testCard := card.Card{
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

	permanent := &Permanent{source: testCard}

	for _, test := range tests {
		hasAbility := permanent.hasEvergreenAbility(test.abilityName)
		if hasAbility != test.expected {
			t.Errorf("hasEvergreenAbility(%s) = %v; expected %v", test.abilityName, hasAbility, test.expected)
		}
	}
}

func TestOracleTextAbilities(t *testing.T) {
	testCard := card.Card{
		Name:       "Test Creature",
		Keywords:   []string{},
		OracleText: "This creature has flying and can't be blocked.",
	}

	permanent := &Permanent{source: testCard}

	// Should detect flying from oracle text
	if !permanent.hasEvergreenAbility("Flying") {
		t.Error("Should detect flying from oracle text")
	}

	// Should detect unblockable from oracle text
	if !permanent.hasEvergreenAbility("can't be blocked") {
		t.Error("Should detect unblockable from oracle text")
	}
}

func TestDefenderAbility(t *testing.T) {
	defenderCard := card.Card{
		Name:     "Wall of Stone",
		Keywords: []string{"Defender"},
	}

	normalCard := card.Card{
		Name:     "Grizzly Bears",
		Keywords: []string{},
	}

	defender := &Permanent{source: defenderCard}
	normal := &Permanent{source: normalCard}

	if !defender.hasEvergreenAbility("Defender") {
		t.Error("Defender creature should have defender ability")
	}

	if normal.hasEvergreenAbility("Defender") {
		t.Error("Normal creature should not have defender ability")
	}
}

func TestVigilanceAbility(t *testing.T) {
	vigilantCard := card.Card{
		Name:     "Serra Angel",
		Keywords: []string{"Flying", "Vigilance"},
	}

	normalCard := card.Card{
		Name:     "Grizzly Bears",
		Keywords: []string{},
	}

	vigilant := &Permanent{source: vigilantCard}
	normal := &Permanent{source: normalCard}

	if !vigilant.hasEvergreenAbility("Vigilance") {
		t.Error("Vigilant creature should have vigilance ability")
	}

	if normal.hasEvergreenAbility("Vigilance") {
		t.Error("Normal creature should not have vigilance ability")
	}
}

func TestFirstStrikeAndDoubleStrike(t *testing.T) {
	firstStrikeCard := card.Card{
		Name:     "First Strike Creature",
		Keywords: []string{"First Strike"},
	}

	doubleStrikeCard := card.Card{
		Name:     "Double Strike Creature",
		Keywords: []string{"Double Strike"},
	}

	normalCard := card.Card{
		Name:     "Normal Creature",
		Keywords: []string{},
	}

	firstStrike := &Permanent{source: firstStrikeCard}
	doubleStrike := &Permanent{source: doubleStrikeCard}
	normal := &Permanent{source: normalCard}

	if !firstStrike.hasEvergreenAbility("First Strike") {
		t.Error("First strike creature should have first strike")
	}

	if !doubleStrike.hasEvergreenAbility("Double Strike") {
		t.Error("Double strike creature should have double strike")
	}

	if normal.hasEvergreenAbility("First Strike") || normal.hasEvergreenAbility("Double Strike") {
		t.Error("Normal creature should not have first or double strike")
	}
}

func TestDeathtouchAbility(t *testing.T) {
	deathtouchCard := card.Card{
		Name:     "Deathtouch Creature",
		Keywords: []string{"Deathtouch"},
	}

	normalCard := card.Card{
		Name:     "Normal Creature",
		Keywords: []string{},
	}

	deathtouch := &Permanent{source: deathtouchCard}
	normal := &Permanent{source: normalCard}

	if !deathtouch.hasEvergreenAbility("Deathtouch") {
		t.Error("Deathtouch creature should have deathtouch")
	}

	if normal.hasEvergreenAbility("Deathtouch") {
		t.Error("Normal creature should not have deathtouch")
	}
}

func TestIndestructibleAbility(t *testing.T) {
	indestructibleCard := card.Card{
		Name:     "Indestructible Creature",
		Keywords: []string{"Indestructible"},
	}

	normalCard := card.Card{
		Name:     "Normal Creature",
		Keywords: []string{},
	}

	indestructible := &Permanent{source: indestructibleCard}
	normal := &Permanent{source: normalCard}

	if !indestructible.hasEvergreenAbility("Indestructible") {
		t.Error("Indestructible creature should have indestructible")
	}

	if normal.hasEvergreenAbility("Indestructible") {
		t.Error("Normal creature should not have indestructible")
	}
}

func TestTrampleAbility(t *testing.T) {
	trampleCard := card.Card{
		Name:     "Trample Creature",
		Keywords: []string{"Trample"},
	}

	normalCard := card.Card{
		Name:     "Normal Creature",
		Keywords: []string{},
	}

	trample := &Permanent{source: trampleCard}
	normal := &Permanent{source: normalCard}

	if !trample.hasEvergreenAbility("Trample") {
		t.Error("Trample creature should have trample")
	}

	if normal.hasEvergreenAbility("Trample") {
		t.Error("Normal creature should not have trample")
	}
}

func TestLifelinkAbility(t *testing.T) {
	lifelinkCard := card.Card{
		Name:     "Lifelink Creature",
		Keywords: []string{"Lifelink"},
	}

	normalCard := card.Card{
		Name:     "Normal Creature",
		Keywords: []string{},
	}

	lifelink := &Permanent{source: lifelinkCard}
	normal := &Permanent{source: normalCard}

	if !lifelink.hasEvergreenAbility("Lifelink") {
		t.Error("Lifelink creature should have lifelink")
	}

	if normal.hasEvergreenAbility("Lifelink") {
		t.Error("Normal creature should not have lifelink")
	}
}
