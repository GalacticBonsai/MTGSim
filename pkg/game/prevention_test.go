package game

import "testing"

func TestDamagePrevention_Player(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	g.AddDamagePrevention(p2, 2)
	g.ApplyDamageToPlayer(p2, 3)
	if p2.GetLifeTotal() != 19 {
		t.Fatalf("expected life 19 after preventing 2 of 3, got %d", p2.GetLifeTotal())
	}
}

func TestDamagePrevention_Permanent(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	c := SimpleCard{Name: "Wall", TypeLine: "Creature", Power: "0", Toughness: "2"}
	wall := NewPermanent(c, p1, p1)
	p1.Battlefield = append(p1.Battlefield, wall)

	g.AddDamagePrevention(wall, 1)
	g.ApplyDamageToPermanent(wall, 2)
	if wall.GetDamageCounters() != 1 {
		t.Fatalf("expected 1 damage after prevention, got %d", wall.GetDamageCounters())
	}
	g.ApplyStateBasedActions()
	if len(p1.Battlefield) != 1 {
		t.Fatalf("expected wall to survive, battlefield size %d", len(p1.Battlefield))
	}
}
