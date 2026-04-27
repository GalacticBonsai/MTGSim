package game

import "testing"

// CR 613: layered continuous-effect engine tests. Each test starts in 1v1
// to validate rules correctness before scaling to multiplayer.

func setupSinglePerm(t *testing.T, pow, tough int) (*Game, *Permanent, *Player, *Player) {
	t.Helper()
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Bear", TypeLine: "Creature", Power: itoa(pow), Toughness: itoa(tough)}
	be := NewPermanent(c, p1, p1)
	p1.Battlefield = append(p1.Battlefield, be)
	return g, be, p1, p2
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	if neg {
		out = "-" + out
	}
	return out
}

func TestLayer7B_SetOverridesPrinted(t *testing.T) {
	g, be, _, _ := setupSinglePerm(t, 2, 2)
	g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7B,
		Affects: func(q *Permanent) bool { return q == be },
		Apply:   func(_ *Permanent, v *PermanentView) { v.Power, v.Toughness = 1, 1 },
	})
	if be.GetPower() != 1 || be.GetToughness() != 1 {
		t.Fatalf("expected 1/1 from 7B set, got %d/%d", be.GetPower(), be.GetToughness())
	}
}

func TestLayer7C_StacksOnTopOf7B(t *testing.T) {
	g, be, _, _ := setupSinglePerm(t, 2, 2)
	// 7B becomes 1/1
	g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7B,
		Affects: func(q *Permanent) bool { return q == be },
		Apply:   func(_ *Permanent, v *PermanentView) { v.Power, v.Toughness = 1, 1 },
	})
	// 7C anthem +2/+2 (e.g. Glorious Anthem-style)
	g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7C,
		Affects: func(q *Permanent) bool { return q == be },
		Apply:   func(_ *Permanent, v *PermanentView) { v.Power += 2; v.Toughness += 2 },
	})
	if be.GetPower() != 3 || be.GetToughness() != 3 {
		t.Fatalf("expected 3/3 (7B 1/1 then 7C +2/+2), got %d/%d", be.GetPower(), be.GetToughness())
	}
}

func TestLayer_TimestampOrderWithinSublayer(t *testing.T) {
	g, be, _, _ := setupSinglePerm(t, 2, 2)
	// Earlier 7B sets to 1/1
	g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7B,
		Affects: func(q *Permanent) bool { return q == be },
		Apply:   func(_ *Permanent, v *PermanentView) { v.Power, v.Toughness = 1, 1 },
	})
	// Later 7B sets to 5/5; later timestamp wins within the sublayer
	g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7B,
		Affects: func(q *Permanent) bool { return q == be },
		Apply:   func(_ *Permanent, v *PermanentView) { v.Power, v.Toughness = 5, 5 },
	})
	if be.GetPower() != 5 || be.GetToughness() != 5 {
		t.Fatalf("expected 5/5 (last 7B wins), got %d/%d", be.GetPower(), be.GetToughness())
	}
}

func TestLayer7E_SwitchAfter7C(t *testing.T) {
	g, be, _, _ := setupSinglePerm(t, 4, 2)
	// 7C: +1/+0
	g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7C,
		Affects: func(q *Permanent) bool { return q == be },
		Apply:   func(_ *Permanent, v *PermanentView) { v.Power += 1 },
	})
	// 7E: switch P/T (e.g. Backslide-style)
	g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7E,
		Affects: func(q *Permanent) bool { return q == be },
		Apply:   func(_ *Permanent, v *PermanentView) { v.SwapPT = !v.SwapPT },
	})
	// Printed 4/2; +1 from 7C => 5/2; swap => 2/5
	if be.GetPower() != 2 || be.GetToughness() != 5 {
		t.Fatalf("expected 2/5 after switch, got %d/%d", be.GetPower(), be.GetToughness())
	}
}

func TestLayer_RemoveEffectRecomputes(t *testing.T) {
	g, be, _, _ := setupSinglePerm(t, 2, 2)
	id := g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7C,
		Affects: func(q *Permanent) bool { return q == be },
		Apply:   func(_ *Permanent, v *PermanentView) { v.Power += 3; v.Toughness += 3 },
	})
	if be.GetPower() != 5 {
		t.Fatalf("expected 5 after pump, got %d", be.GetPower())
	}
	g.RemoveLayeredEffect(id)
	if be.GetPower() != 2 || be.GetToughness() != 2 {
		t.Fatalf("expected 2/2 after removal, got %d/%d", be.GetPower(), be.GetToughness())
	}
}

func TestLayer_EOTClearsExpiringEffects(t *testing.T) {
	g, be, _, _ := setupSinglePerm(t, 2, 2)
	g.ApplySetPTUntilEOT(be, 7, 7) // registered as Layer 7B with ExpiresEOT=true
	if be.GetPower() != 7 || be.GetToughness() != 7 {
		t.Fatalf("expected 7/7 after set, got %d/%d", be.GetPower(), be.GetToughness())
	}
	for i := 0; i < 6; i++ {
		g.AdvancePhase()
	}
	g.AdvancePhase() // wrap to next turn -> EOT cleanup runs
	if be.GetPower() != 2 || be.GetToughness() != 2 {
		t.Fatalf("expected printed 2/2 after EOT, got %d/%d", be.GetPower(), be.GetToughness())
	}
}

func TestLayer_AnthemAcrossMultipleCreatures(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	a := NewPermanent(SimpleCard{Name: "A", TypeLine: "Creature", Power: "1", Toughness: "1"}, p1, p1)
	b := NewPermanent(SimpleCard{Name: "B", TypeLine: "Creature", Power: "2", Toughness: "2"}, p1, p1)
	p1.Battlefield = append(p1.Battlefield, a, b)
	g.AddLayeredEffect(&LayeredEffect{
		Layer: Layer7PT, Sublayer: Sublayer7C,
		Affects: func(q *Permanent) bool { return q.GetController() == p1 && q.IsCreature() },
		Apply:   func(_ *Permanent, v *PermanentView) { v.Power += 1; v.Toughness += 1 },
	})
	if a.GetPower() != 2 || a.GetToughness() != 2 || b.GetPower() != 3 || b.GetToughness() != 3 {
		t.Fatalf("anthem failed: A=%d/%d B=%d/%d", a.GetPower(), a.GetToughness(), b.GetPower(), b.GetToughness())
	}
}
