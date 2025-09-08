package game

import (
	"testing"
)

func makeCard(name, typeLine, power, toughness string) SimpleCard {
	return SimpleCard{Name: name, TypeLine: typeLine, Power: power, Toughness: toughness}
}

func TestZoneMoves_Basic(t *testing.T) {
	p1 := NewPlayer("Alice", 20)
	p2 := NewPlayer("Bob", 20)
	_ = p2

	// Setup library: top to bottom: Plains, Bears
	plains := makeCard("Plains", "Basic Land — Plains", "", "")
	bears := makeCard("Grizzly Bears", "Creature — Bear", "2", "2")
	p1.Library = []SimpleCard{plains, bears}

	g := NewGame(p1, p2)
	if g.NumPlayers() != 2 {
		t.Fatalf("expected 2 players")
	}

	// Draw one card
	drawn := p1.Draw(1)
	if drawn != 1 {
		t.Fatalf("expected to draw 1, got %d", drawn)
	}
	if len(p1.Library) != 1 || len(p1.Hand) != 1 {
		t.Fatalf("expected library=1 hand=1, got %d %d", len(p1.Library), len(p1.Hand))
	}
	if p1.Hand[0].Name != "Plains" {
		t.Fatalf("expected Plains in hand, got %s", p1.Hand[0].Name)
	}

	// Play land to battlefield
	perm, err := p1.PlayLand("Plains")
	if err != nil {
		t.Fatalf("play land error: %v", err)
	}
	if perm == nil || len(p1.Hand) != 0 || len(p1.Battlefield) != 1 {
		t.Fatalf("expected hand=0, battlefield=1")
	}
	if !perm.IsLand() || perm.GetName() != "Plains" {
		t.Fatalf("unexpected permanent on battlefield")
	}

	// Draw second card (bear) and summon as creature permanent
	p1.Draw(1)
	if len(p1.Hand) != 1 || p1.Hand[0].Name != "Grizzly Bears" {
		t.Fatalf("expected Grizzly Bears in hand")
	}
	bearPerm, err := p1.SummonCreature("Grizzly Bears")
	if err != nil {
		t.Fatalf("summon creature error: %v", err)
	}
	if bearPerm == nil || !bearPerm.IsCreature() || len(p1.Battlefield) != 2 {
		t.Fatalf("expected bear permanent on battlefield")
	}

	// Destroy the bear -> to graveyard
	ok := p1.DestroyPermanent(bearPerm)
	if !ok {
		t.Fatalf("destroy permanent failed")
	}
	if len(p1.Battlefield) != 1 || len(p1.Graveyard) != 1 || p1.Graveyard[0].Name != "Grizzly Bears" {
		t.Fatalf("expected bear in graveyard")
	}

	// Exile it from graveyard
	ok = p1.ExileFromGraveyard("Grizzly Bears")
	if !ok {
		t.Fatalf("exile from graveyard failed")
	}
	if len(p1.Graveyard) != 0 || len(p1.Exile) != 1 || p1.Exile[0].Name != "Grizzly Bears" {
		t.Fatalf("expected bear in exile")
	}
}
