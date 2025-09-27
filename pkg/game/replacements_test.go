package game

import "testing"

func TestReplacement_DiesExileInstead(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	c := SimpleCard{Name: "Victim", TypeLine: "Creature", Power: "2", Toughness: "2"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Add replacement until EOT
	g.AddWouldDieExileUntilEOT(perm)

	// Deal lethal damage directly
	perm.AddDamage(2)
	g.ApplyStateBasedActions()

	// Should be gone from battlefield and in exile, not graveyard
	if len(p1.Battlefield) != 0 {
		t.Fatalf("expected battlefield empty, got %d", len(p1.Battlefield))
	}
	if len(p1.Exile) != 1 {
		t.Fatalf("expected exile size 1, got %d", len(p1.Exile))
	}
	if len(p1.Graveyard) != 0 {
		t.Fatalf("expected graveyard size 0, got %d", len(p1.Graveyard))
	}
}
