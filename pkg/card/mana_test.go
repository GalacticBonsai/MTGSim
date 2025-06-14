package card

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestParseManaCost(t *testing.T) {
	tests := []struct {
		cost     string
		expected map[game.ManaType]int
	}{
		{"{W}{W}{U}{U}{B}{B}{R}{R}{G}{G}", map[game.ManaType]int{game.White: 2, game.Blue: 2, game.Black: 2, game.Red: 2, game.Green: 2}}, // Progenitus
		{"{G}{G}{G}{G}{G}{G}{G}{G}", map[game.ManaType]int{game.Green: 8}},                                                                  // Khalni Hydra
		{"{5}{C}{C}", map[game.ManaType]int{game.Any: 5, game.Colorless: 2}},                                                               // Devourer of Destiny
		{"{W}{U}{B}{R}{G}{C}", map[game.ManaType]int{game.White: 1, game.Blue: 1, game.Black: 1, game.Red: 1, game.Green: 1, game.Colorless: 1}}, // Slivdrazi Monstrosity
		{"{C}", map[game.ManaType]int{game.Colorless: 1}},                                                                                   // Eldritch Immunity
		{"{X}{X}{W}{W}{W}", map[game.ManaType]int{game.White: 3, game.X: 2}},                                                               // Entreat the Angels
	}

	for _, test := range tests {
		result := ParseManaCost(test.cost)
		if len(result.pool) != len(test.expected) {
			t.Errorf("ParseManaCost(%s) = %v; expected %v", test.cost, result.pool, test.expected)
			continue
		}
		for mt, count := range test.expected {
			if result.pool[mt] != count {
				t.Errorf("ParseManaCost(%s) = %v; expected %v", test.cost, result.pool, test.expected)
				break
			}
		}
	}
}

func TestCheckManaProducer(t *testing.T) {
	tests := []struct {
		oracleText string
		expected   bool
		manaTypes  []game.ManaType
	}{
		{"{T}, Sacrifice this artifact: Add one mana of any color", true, []game.ManaType{game.Any}},
		{"Sacrifice a creature: Add {C}{C}.", true, []game.ManaType{game.Colorless, game.Colorless}},
		{"({T}: Add {R}.)", true, []game.ManaType{game.Red}},
		{"({T}: Add {U} or {R}.)", true, []game.ManaType{game.Blue, game.Red}},
		{"{T}: Add one mana of any color in your commander's color identity.", true, []game.ManaType{game.Any}},
	}

	for _, test := range tests {
		isProducer, manaTypes := CheckManaProducer(test.oracleText)
		if isProducer != test.expected || len(manaTypes) != len(test.manaTypes) {
			t.Errorf("CheckManaProducer(%s) = (%v, %v); expected (%v, %v)", test.oracleText, isProducer, manaTypes, test.expected, test.manaTypes)
			continue
		}
		for i, manaType := range test.manaTypes {
			if i < len(manaTypes) && manaTypes[i] != manaType {
				t.Errorf("CheckManaProducer(%s) = (%v, %v); expected (%v, %v)", test.oracleText, isProducer, manaTypes, test.expected, test.manaTypes)
				break
			}
		}
	}
}

func TestManaPool(t *testing.T) {
	manaPool := NewManaPool()
	manaPool.Add(game.Red, 2)
	manaPool.Add(game.Green, 1)

	cost := ParseManaCost("{R}{G}")
	if !manaPool.CanPay(cost) {
		t.Errorf("Expected to be able to pay {R}{G}, but cannot")
	}

	err := manaPool.Pay(cost)
	if err != nil {
		t.Errorf("Expected to pay {R}{G}, but got error: %v", err)
	}

	if manaPool.pool[game.Red] != 1 || manaPool.pool[game.Green] != 0 {
		t.Errorf("Mana pool not updated correctly after payment. Red: %d, Green: %d", 
			manaPool.pool[game.Red], manaPool.pool[game.Green])
	}

	cost = ParseManaCost("{R}{R}")
	if manaPool.CanPay(cost) {
		t.Errorf("Expected not to be able to pay {R}{R}, but can")
	}
}

func TestManaOperations(t *testing.T) {
	mana := NewMana()
	
	// Test adding mana
	mana.Add(game.Red, 3)
	mana.Add(game.Blue, 2)
	
	if mana.Get(game.Red) != 3 {
		t.Errorf("Expected 3 red mana, got %d", mana.Get(game.Red))
	}
	
	if mana.Get(game.Blue) != 2 {
		t.Errorf("Expected 2 blue mana, got %d", mana.Get(game.Blue))
	}
	
	// Test total
	if mana.Total() != 5 {
		t.Errorf("Expected total of 5 mana, got %d", mana.Total())
	}
	
	// Test getting non-existent mana type
	if mana.Get(game.Green) != 0 {
		t.Errorf("Expected 0 green mana, got %d", mana.Get(game.Green))
	}
}

func TestComplexManaCosts(t *testing.T) {
	tests := []struct {
		name     string
		cost     string
		expected int // total mana value
	}{
		{"Lightning Bolt", "{R}", 1},
		{"Counterspell", "{U}{U}", 2},
		{"Wrath of God", "{2}{W}{W}", 4},
		{"Elspeth, Knight-Errant", "{2}{W}{W}", 4},
		{"Jace, the Mind Sculptor", "{2}{U}{U}", 4},
		{"Tarmogoyf", "{1}{G}", 2},
		{"Dark Confidant", "{1}{B}", 2},
	}

	for _, test := range tests {
		mana := ParseManaCost(test.cost)
		if mana.Total() != test.expected {
			t.Errorf("Card %s with cost %s: expected total %d, got %d", 
				test.name, test.cost, test.expected, mana.Total())
		}
	}
}
