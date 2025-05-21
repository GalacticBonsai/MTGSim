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
		{"{X}{X}{W}{W}{W}", mana{pool: map[ManaType]int{White: 3, X: 2}}},                                                 // Entreat the Angels
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
		{"Sacrifice a creature: Add {C}{C}.", true, []ManaType{Colorless, Colorless}},
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

func TestCanCastCard(t *testing.T) {
	player := &Player{
		Name: "Test Player",
		Lands: []Permanant{
			{manaProducer: true, manaTypes: []ManaType{Red}},
			{manaProducer: true, manaTypes: []ManaType{Green}},
			{manaProducer: true, manaTypes: []ManaType{White}},
		},
		Creatures: []Permanant{
			{manaProducer: true, manaTypes: []ManaType{Blue}, summoningSickness: false},
		},
		Artifacts: []Permanant{
			{manaProducer: true, manaTypes: []ManaType{Colorless}},
		},
	}

	tests := []struct {
		cardManaCost string
		expected     bool
	}{
		{"{R}{G}{W}", true},  // Can cast with available mana
		{"{U}{U}", false},    // Not enough blue mana
		{"{C}", true},     // Can cast with artifact mana
		{"{R}{R}{G}", false}, // Not enough red mana
	}

	for _, test := range tests {
		cost := ParseManaCost(test.cardManaCost)
		err := player.tapForMana(cost)
		if (err == nil) != test.expected {
			t.Errorf("For mana cost %s, expected %v but got %v", test.cardManaCost, test.expected, err == nil)
		}
	}
}

func TestSimulateBoardState(t *testing.T) {
	player := &Player{
		Name: "Test Player",
		Lands: []Permanant{
			{manaProducer: true, manaTypes: []ManaType{Red}},
			{manaProducer: true, manaTypes: []ManaType{Green}},
			{manaProducer: true, manaTypes: []ManaType{White}},
		},
		Creatures: []Permanant{
			{manaProducer: true, manaTypes: []ManaType{Blue}, summoningSickness: false},
		},
		Artifacts: []Permanant{
			{manaProducer: true, manaTypes: []ManaType{Colorless}},
		},
	}

	card := Card{
		Name:     "Test Card",
		ManaCost: "{R}{G}{W}",
	}

	cost := ParseManaCost(card.ManaCost)
	err := player.tapForMana(cost)

	if err != nil {
		t.Errorf("Expected to cast card %s, but got error: %v", card.Name, err)
	}

	// Verify that lands and permanents were tapped correctly
	for _, land := range player.Lands {
		if land.manaProducer && !land.tapped {
			t.Errorf("Expected land to be tapped, but it was not")
		}
	}
	for _, creature := range player.Creatures {
		if creature.tapped {
			t.Errorf("Expected creature to Not be tapped, but it was")
		}
	}
}

func TestManaPool(t *testing.T) {
	manaPool := NewManaPool()
	manaPool.Add(Red, 2)
	manaPool.Add(Green, 1)

	cost := ParseManaCost("{R}{G}")
	if !manaPool.CanPay(cost) {
		t.Errorf("Expected to be able to pay {R}{G}, but cannot")
	}

	err := manaPool.Pay(cost)
	if err != nil {
		t.Errorf("Expected to pay {R}{G}, but got error: %v", err)
	}

	if manaPool.pool[Red] != 1 || manaPool.pool[Green] != 0 {
		t.Errorf("Mana pool not updated correctly after payment")
	}

	cost = ParseManaCost("{R}{R}")
	if manaPool.CanPay(cost) {
		t.Errorf("Expected not to be able to pay {R}{R}, but can")
	}
}
