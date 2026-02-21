package game

import "testing"

func TestSetPT_LastWinsAndResetsEOT(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	// 2/2
	c := SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
	be := NewPermanent(c, p1, p1)
	p1.Battlefield = append(p1.Battlefield, be)

	// set to 4/4 then 5/5; last-wins
	g.ApplySetPTUntilEOT(be, 4, 4)
	g.ApplySetPTUntilEOT(be, 5, 5)
	if be.GetPower() != 5 || be.GetToughness() != 5 {
		t.Fatalf("expected 5/5 after last set, got %d/%d", be.GetPower(), be.GetToughness())
	}

	// Attack unblocked deals 5
	g.BeginCombat()
	if err := g.DeclareAttacker(be, p2); err != nil {
		t.Fatalf("declare attacker: %v", err)
	}
	g.ResolveCombatDamage()
	if p2.GetLifeTotal() != 15 {
		t.Fatalf("expected defender at 15, got %d", p2.GetLifeTotal())
	}

	// Advance to end and wrap to next turn to clear EOT
	for i := 0; i < 6; i++ {
		g.AdvancePhase()
	}
	g.AdvancePhase()

	if be.GetPower() != 2 || be.GetToughness() != 2 {
		t.Fatalf("expected reset to base 2/2, got %d/%d", be.GetPower(), be.GetToughness())
	}
}
