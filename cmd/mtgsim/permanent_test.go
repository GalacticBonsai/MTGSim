package main

import (
	"testing"
	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/card"
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
	owner := &Player{
		Name:      "Test Player",
		LifeTotal: 20,
	}

	p1 := &Permanent{
		id:              uuid.New(),
		power:           3,
		toughness:       5,
		damage_counters: 0,
		source:          card.Card{Name: "Attacker"},
		owner:           owner,
	}

	p2 := &Permanent{
		id:              uuid.New(),
		power:           2,
		toughness:       4,
		damage_counters: 0,
		source:          card.Card{Name: "Defender"},
		owner:           owner,
	}

	// Create a game instance to use dealDamageBetweenCreatures
	game := NewGame(nil)
	game.dealDamageBetweenCreatures(p1, p2)

	if p2.damage_counters != 3 {
		t.Errorf("Expected defender to have 3 damage counters, got %d", p2.damage_counters)
	}

	if p1.damage_counters != 2 {
		t.Errorf("Expected attacker to have 2 damage counters, got %d", p1.damage_counters)
	}
}

func TestPermanentCheckLife(t *testing.T) {
	owner := &Player{
		Name:      "Player 1",
		Creatures: []*Permanent{},
		Graveyard: []card.Card{},
	}

	p := &Permanent{
		id:              uuid.New(),
		toughness:       4,
		damage_counters: 4,
		source:          card.Card{Name: "Test Creature"},
		owner:           owner,
	}

	owner.Creatures = append(owner.Creatures, p)

	// Create a game and add the player
	game := NewGame(nil)
	game.Players = []*Player{owner}

	// Check state-based actions
	game.checkStateBasedActions()

	if len(owner.Creatures) != 0 {
		t.Errorf("Expected creature to be destroyed, but %d creatures remain", len(owner.Creatures))
	}
	if len(owner.Graveyard) != 1 {
		t.Errorf("Expected creature to be in graveyard, but graveyard has %d cards", len(owner.Graveyard))
	}
}

func TestPermanentIndestructible(t *testing.T) {
	owner := &Player{
		Name:      "Player 1",
		Creatures: []*Permanent{},
		Graveyard: []card.Card{},
	}

	p := &Permanent{
		id:              uuid.New(),
		toughness:       4,
		damage_counters: 10, // Lethal damage
		source: card.Card{
			Name:     "Indestructible Creature",
			Keywords: []string{"Indestructible"},
		},
		owner: owner,
	}

	owner.Creatures = append(owner.Creatures, p)

	// Create a game and add the player
	game := NewGame(nil)
	game.Players = []*Player{owner}

	// Check state-based actions
	game.checkStateBasedActions()

	if len(owner.Creatures) != 1 {
		t.Errorf("Expected indestructible creature to survive, but %d creatures remain", len(owner.Creatures))
	}
	if len(owner.Graveyard) != 0 {
		t.Errorf("Expected graveyard to be empty, but graveyard has %d cards", len(owner.Graveyard))
	}
}

func TestPermanentLifelink(t *testing.T) {
	owner := &Player{
		Name:      "Test Player",
		LifeTotal: 15,
	}

	opponent := &Player{
		Name:      "Opponent",
		LifeTotal: 20,
	}

	lifelinkCreature := &Permanent{
		id:        uuid.New(),
		power:     3,
		toughness: 3,
		source: card.Card{
			Name:     "Lifelink Creature",
			Keywords: []string{"Lifelink"},
		},
		owner:     owner,
		attacking: opponent,
	}

	// Create a game instance to test lifelink
	game := NewGame(nil)
	game.Players = []*Player{owner, opponent}

	// Simulate unblocked combat damage
	game.dealCombatDamageForCreature(lifelinkCreature)

	// Check that owner gained life
	expectedLife := 15 + 3 // Original life + lifelink damage
	if owner.LifeTotal != expectedLife {
		t.Errorf("Expected owner to have %d life from lifelink, got %d", expectedLife, owner.LifeTotal)
	}

	// Check that opponent lost life
	expectedOpponentLife := 20 - 3 // Original life - damage
	if opponent.LifeTotal != expectedOpponentLife {
		t.Errorf("Expected opponent to have %d life, got %d", expectedOpponentLife, opponent.LifeTotal)
	}
}

func TestPermanentHaste(t *testing.T) {
	hasteCard := card.Card{
		Name:     "Haste Creature",
		Keywords: []string{"Haste"},
	}

	normalCard := card.Card{
		Name:     "Normal Creature",
		Keywords: []string{},
	}

	haste := &Permanent{
		source:            hasteCard,
		summoningSickness: true,
	}

	normal := &Permanent{
		source:            normalCard,
		summoningSickness: true,
	}

	if !haste.hasEvergreenAbility("Haste") {
		t.Error("Haste creature should have haste ability")
	}

	if normal.hasEvergreenAbility("Haste") {
		t.Error("Normal creature should not have haste ability")
	}
}

func TestPermanentReach(t *testing.T) {
	reachCard := card.Card{
		Name:     "Giant Spider",
		Keywords: []string{"Reach"},
	}

	normalCard := card.Card{
		Name:     "Grizzly Bears",
		Keywords: []string{},
	}

	reach := &Permanent{source: reachCard}
	normal := &Permanent{source: normalCard}

	if !reach.hasEvergreenAbility("Reach") {
		t.Error("Reach creature should have reach ability")
	}

	if normal.hasEvergreenAbility("Reach") {
		t.Error("Normal creature should not have reach ability")
	}
}

func TestPermanentTap(t *testing.T) {
	p := &Permanent{
		id:     uuid.New(),
		tapped: false,
	}

	// Test tapping
	p.tap()
	if !p.tapped {
		t.Error("Permanent should be tapped after calling tap()")
	}

	// Test that tapping an already tapped permanent doesn't change state
	p.tap()
	if !p.tapped {
		t.Error("Permanent should remain tapped")
	}
}

func TestPermanentUntap(t *testing.T) {
	p := &Permanent{
		id:     uuid.New(),
		tapped: true,
	}

	// Test untapping
	p.untap()
	if p.tapped {
		t.Error("Permanent should be untapped after calling untap()")
	}

	// Test that untapping an already untapped permanent doesn't change state
	p.untap()
	if p.tapped {
		t.Error("Permanent should remain untapped")
	}
}
