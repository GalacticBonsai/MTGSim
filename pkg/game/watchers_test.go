package game

import "testing"

func TestWatcher_CreatureETBCountsAndResets(t *testing.T) {
    p1 := NewPlayer("P1", 20)
    p2 := NewPlayer("P2", 20)
    g := NewGame(p1, p2)

    // Two creatures in hand
    c1 := SimpleCard{Name: "C1", TypeLine: "Creature", Power: "1", Toughness: "1"}
    c2 := SimpleCard{Name: "C2", TypeLine: "Creature", Power: "1", Toughness: "1"}
    p1.Hand = append(p1.Hand, c1, c2)

    w := &CreatureETBWatcher{}
    g.AddWatcher(w)

    if _, err := g.SummonCreature(p1, c1.Name); err != nil { t.Fatalf("summon 1: %v", err) }
    if _, err := g.SummonCreature(p1, c2.Name); err != nil { t.Fatalf("summon 2: %v", err) }

    if w.Count != 2 { t.Fatalf("expected watcher count 2, got %d", w.Count) }

    // Advance to end then wrap
    for i := 0; i < 6; i++ { g.AdvancePhase() }
    g.AdvancePhase()

    if w.Count != 0 { t.Fatalf("expected watcher reset to 0, got %d", w.Count) }
}

