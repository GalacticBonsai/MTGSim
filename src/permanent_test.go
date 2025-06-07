package main

import (
	"testing"

	"github.com/google/uuid"
)

func TestPermanentTapUntap(t *testing.T) {
	p := &Permanent{
		id:                uuid.New(),
		summoningSickness: false,
		tapped:            false,
	}

	p.tap()
	if !p.tapped {
		t.Errorf("Expected Permanent to be tapped")
	}

	p.untap()
	if p.tapped {
		t.Errorf("Expected Permanent to be untapped")
	}
}

func TestPermanentDamages(t *testing.T) {
	p1 := &Permanent{
		id:              uuid.New(),
		power:           3,
		toughness:       5,
		damage_counters: 0,
		source:          Card{Name: "Attacker"},
	}

	p2 := &Permanent{
		id:              uuid.New(),
		power:           2,
		toughness:       4,
		damage_counters: 0,
		source:          Card{Name: "Defender"},
	}

	p1.damages(p2)
	if p2.damage_counters != 3 {
		t.Errorf("Expected defender to have 3 damage counters, got %d", p2.damage_counters)
	}
}

func TestPermanentCheckLife(t *testing.T) {
	p := Permanent{
		id:              uuid.New(),
		toughness:       4,
		damage_counters: 4,
		source:          Card{Name: "Test Creature"},
		owner: &Player{
			Name:      "Player 1",
			Creatures: []*Permanent{},
			Graveyard: []Card{},
		},
	}

	p.owner.Creatures = append(p.owner.Creatures, &p)
	p.checkLife()
	if len(p.owner.Creatures) != 0 {
		t.Errorf("Expected creature to be destroyed")
	}
	if len(p.owner.Graveyard) != 1 {
		t.Errorf("Expected creature to be in graveyard")
	}
}

func TestParseTapEffect(t *testing.T) {
	cases := []struct {
		oracle   string
		hasTap   bool
		cost     string
		effect   string
	}{
		{"{T}: Add {G}.", true, "", "Add {G}."},
		{"{T}: Add {R} or {G}.", true, "", "Add {R} or {G}."},
		{"Flying\n{T}: Draw a card.", true, "", "Draw a card."},
		{"Whenever you gain life, draw a card.", false, "", ""},
		{"{T}, Sacrifice this: Add {B}.", true, "Sacrifice this", "Add {B}."},
		{"{T}: Add one mana of any color.", true, "", "Add one mana of any color."},
		{"No tap ability here.", false, "", ""},
	}
	for _, c := range cases {
		hasTap, cost, effect := ParseTapEffect(c.oracle)
		if hasTap != c.hasTap || cost != c.cost || effect != c.effect {
			t.Errorf("ParseTapEffect(%q) = (%v, %q, %q), want (%v, %q, %q)", c.oracle, hasTap, cost, effect, c.hasTap, c.cost, c.effect)
		}
	}
}

func TestPermanent_HasTapAbility(t *testing.T) {
	p := &Permanent{source: Card{OracleText: "{T}: Add {G}."}}
	if !p.HasTapAbility() {
		t.Error("Expected HasTapAbility to be true for tap effect")
	}
	p2 := &Permanent{source: Card{OracleText: "Flying"}}
	if p2.HasTapAbility() {
		t.Error("Expected HasTapAbility to be false for no tap effect")
	}
}

func TestPermanent_GetTapEffect(t *testing.T) {
	p := &Permanent{source: Card{OracleText: "{T}: Add {G}."}}
	effect := p.GetTapEffect()
	if effect != "Add {G}." {
		t.Errorf("Expected GetTapEffect to return 'Add {G}.', got %q", effect)
	}
}
