package game

import "testing"

// TestTrigger_APNAPOrdering registers three triggers — one per player in
// a 3-player game — that all fire on the same ETB event. They each
// append a tag to a shared slice; CR 603.3b requires the active player's
// trigger to resolve first, then NAP1, then NAP2.
func TestTrigger_APNAPOrdering(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	p3 := NewPlayer("P3", 20)
	g := NewGame(p1, p2, p3)

	var order []string
	makeTrigger := func(owner *Player, tag string) *Trigger {
		return &Trigger{
			On:         EventEntersBattlefield,
			Controller: owner,
			Action:     func(g *Game, e Event) { order = append(order, tag) },
		}
	}

	// Register out of seat order to prove sorting (not insertion) wins.
	g.AddTrigger(makeTrigger(p3, "p3"))
	g.AddTrigger(makeTrigger(p1, "p1"))
	g.AddTrigger(makeTrigger(p2, "p2"))

	bear := SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
	p1.AddCardToHand(bear)
	if _, err := g.SummonCreature(p1, "Bear"); err != nil {
		t.Fatalf("summon: %v", err)
	}

	if got, want := order, []string{"p1", "p2", "p3"}; !equalStrings(got, want) {
		t.Fatalf("APNAP order with P1 active: got %v, want %v", got, want)
	}

	// Rotate active player to P2 by advancing through end of P1's turn.
	for i := 0; i < 7; i++ {
		g.AdvancePhase()
	}
	if g.GetActivePlayerRaw() != p2 {
		t.Fatalf("expected P2 active after rotating, got %s", g.GetActivePlayerRaw().GetName())
	}

	order = nil
	bear2 := SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
	p2.AddCardToHand(bear2)
	if _, err := g.SummonCreature(p2, "Bear"); err != nil {
		t.Fatalf("summon p2: %v", err)
	}
	if got, want := order, []string{"p2", "p3", "p1"}; !equalStrings(got, want) {
		t.Fatalf("APNAP order with P2 active: got %v, want %v", got, want)
	}
}

// TestTrigger_NilControllerLast ensures system-level triggers (no
// Controller) resolve after all controlled triggers, regardless of
// registration order.
func TestTrigger_NilControllerLast(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	var order []string
	g.AddTrigger(&Trigger{On: EventEntersBattlefield, Action: func(g *Game, e Event) { order = append(order, "nil") }})
	g.AddTrigger(&Trigger{On: EventEntersBattlefield, Controller: p2, Action: func(g *Game, e Event) { order = append(order, "p2") }})
	g.AddTrigger(&Trigger{On: EventEntersBattlefield, Controller: p1, Action: func(g *Game, e Event) { order = append(order, "p1") }})

	bear := SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
	p1.AddCardToHand(bear)
	if _, err := g.SummonCreature(p1, "Bear"); err != nil {
		t.Fatalf("summon: %v", err)
	}
	if got, want := order, []string{"p1", "p2", "nil"}; !equalStrings(got, want) {
		t.Fatalf("expected nil-controller trigger last: got %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
