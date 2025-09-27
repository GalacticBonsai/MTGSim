package bridge

import (
	"testing"

	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestResolution_DealsLethalDamage_KillsCreature_AndAuraDetaches(t *testing.T) {
	// Setup game: P1 has a 2/2 creature with an aura attached
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	creatureCard := game.SimpleCard{Name: "Grizzly Bears", TypeLine: "Creature", Power: "2", Toughness: "2"}
	auraCard := game.SimpleCard{Name: "Pacifism", TypeLine: "Enchantment — Aura"}

	creature := game.NewPermanent(creatureCard, p1, p1)
	aura := game.NewPermanent(auraCard, p1, p1)
	aura.AttachTo(creature)

	p1.Battlefield = append(p1.Battlefield, creature, aura)

	gs := NewAbilityGameState(g)
	sb := NewStackBridge(gs)

	// Create a spell that deals 2 damage to target creature
	eff := abil.Effect{Type: abil.DealDamage, Value: 2}
	sp := &abil.Spell{Name: "Bolt", Effects: []abil.Effect{eff}}

	controller := gs.GetAllPlayers()[0] // P1 casts it (controller doesn't matter for damage)
	sb.Stack().AddSpell(sp, controller, []interface{}{creature})

	if err := sb.Stack().ResolveTop(); err != nil {
		t.Fatalf("resolve error: %v", err)
	}

	// SBAs should have moved the creature to graveyard
	if len(p1.Battlefield) != 0 {
		t.Fatalf("expected battlefield empty, got %d", len(p1.Battlefield))
	}
	if len(p1.Graveyard) != 2 {
		t.Fatalf("expected both creature and aura in graveyard, got %d", len(p1.Graveyard))
	}
}

func TestResolution_PlayerLifeReachesZero_PlayerLoses(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 2)
	g := game.NewGame(p1, p2)

	gs := NewAbilityGameState(g)
	sb := NewStackBridge(gs)

	// Deal 3 damage to P2
	eff := abil.Effect{Type: abil.DealDamage, Value: 3}
	sp := &abil.Spell{Name: "BoltPlayer", Effects: []abil.Effect{eff}}

	controller := gs.GetAllPlayers()[0] // P1
	target := gs.GetAllPlayers()[1]     // P2 adapter is a valid player target
	sb.Stack().AddSpell(sp, controller, []interface{}{target})

	if err := sb.Stack().ResolveTop(); err != nil {
		t.Fatalf("resolve error: %v", err)
	}

	if !p2.HasLost() {
		t.Fatalf("expected P2 to have lost after life <= 0")
	}
}
