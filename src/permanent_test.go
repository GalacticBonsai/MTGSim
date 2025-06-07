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
