package card

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/types"
)

func TestParseManaCost(t *testing.T) {
	tests := []struct {
		cost     string
		expected map[types.ManaType]int
	}{
		{"{W}{W}{U}{U}{B}{B}{R}{R}{G}{G}", map[types.ManaType]int{types.White: 2, types.Blue: 2, types.Black: 2, types.Red: 2, types.Green: 2}}, // Progenitus
		{"{G}{G}{G}{G}{G}{G}{G}{G}", map[types.ManaType]int{types.Green: 8}},                                                                  // Khalni Hydra
		{"{5}{C}{C}", map[types.ManaType]int{types.Any: 5, types.Colorless: 2}},                                                               // Devourer of Destiny
		{"{W}{U}{B}{R}{G}{C}", map[types.ManaType]int{types.White: 1, types.Blue: 1, types.Black: 1, types.Red: 1, types.Green: 1, types.Colorless: 1}}, // Slivdrazi Monstrosity
		{"{C}", map[types.ManaType]int{types.Colorless: 1}},                                                                                   // Eldritch Immunity
		{"{X}{X}{W}{W}{W}", map[types.ManaType]int{types.White: 3, types.X: 2}},                                                               // Entreat the Angels
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
		manaTypes  []types.ManaType
	}{
		{"{T}, Sacrifice this artifact: Add one mana of any color", true, []types.ManaType{types.Any}},
		{"Sacrifice a creature: Add {C}{C}.", true, []types.ManaType{types.Colorless, types.Colorless}},
		{"({T}: Add {R}.)", true, []types.ManaType{types.Red}},
		{"({T}: Add {U} or {R}.)", true, []types.ManaType{types.Blue, types.Red}},
		{"{T}: Add one mana of any color in your commander's color identity.", true, []types.ManaType{types.Any}},
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
	manaPool.Add(types.Red, 2)
	manaPool.Add(types.Green, 1)

	cost := ParseManaCost("{R}{G}")
	if !manaPool.CanPay(cost) {
		t.Errorf("Expected to be able to pay {R}{G}, but cannot")
	}

	err := manaPool.Pay(cost)
	if err != nil {
		t.Errorf("Expected to pay {R}{G}, but got error: %v", err)
	}

	if manaPool.pool[types.Red] != 1 || manaPool.pool[types.Green] != 0 {
		t.Errorf("Mana pool not updated correctly after payment. Red: %d, Green: %d", 
			manaPool.pool[types.Red], manaPool.pool[types.Green])
	}

	cost = ParseManaCost("{R}{R}")
	if manaPool.CanPay(cost) {
		t.Errorf("Expected not to be able to pay {R}{R}, but can")
	}
}

func TestManaOperations(t *testing.T) {
	mana := NewMana()
	
	// Test adding mana
	mana.Add(types.Red, 3)
	mana.Add(types.Blue, 2)
	
	if mana.Get(types.Red) != 3 {
		t.Errorf("Expected 3 red mana, got %d", mana.Get(types.Red))
	}
	
	if mana.Get(types.Blue) != 2 {
		t.Errorf("Expected 2 blue mana, got %d", mana.Get(types.Blue))
	}
	
	// Test total
	if mana.Total() != 5 {
		t.Errorf("Expected total of 5 mana, got %d", mana.Total())
	}
	
	// Test getting non-existent mana type
	if mana.Get(types.Green) != 0 {
		t.Errorf("Expected 0 green mana, got %d", mana.Get(types.Green))
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
