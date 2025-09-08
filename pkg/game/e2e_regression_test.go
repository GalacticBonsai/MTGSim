package game

import "testing"

// Fast end-to-end style regressions that exercise multiple subsystems together.

func TestE2E_TriggerWatcher_ETBAndReset(t *testing.T) {
    p1 := NewPlayer("P1", 20)
    p2 := NewPlayer("P2", 20)
    g := NewGame(p1, p2)

    // Prepare library so ETB trigger can draw
    p1.Library = append(p1.Library, SimpleCard{Name: "Filler"})

    // Watcher for ETB counts
    w := &CreatureETBWatcher{}
    g.AddWatcher(w)

    // Trigger: when any creature ETBs under P1, draw a card
    g.AddTrigger(&Trigger{
        On: EventEntersBattlefield,
        Condition: func(e Event) bool {
            return e.ZoneChange != nil && e.ZoneChange.Permanent != nil && e.ZoneChange.Permanent.GetController() == p1 && e.ZoneChange.Permanent.IsCreature()
        },
        Action: func(g *Game, e Event) {
            p1.Draw(1)
        },
    })

    // Put a 2/2 into P1's hand and summon it (emits ETB)
    bear := SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
    p1.AddCardToHand(bear)
    if _, err := g.SummonCreature(p1, "Bear"); err != nil {
        t.Fatalf("summon failed: %v", err)
    }

    if w.Count != 1 {
        t.Fatalf("expected watcher count 1, got %d", w.Count)
    }
    if len(p1.Hand) != 1 { // drew 1 card from trigger; hand was emptied by summoning the Bear
        t.Fatalf("expected hand size 1 after ETB draw, got %d", len(p1.Hand))
    }

    // Advance to end to reset watchers, then to next untap
    for i := 0; i < 7; i++ { g.AdvancePhase() }
    if w.Count != 0 {
        t.Fatalf("expected watcher reset at EOT, got %d", w.Count)
    }

    _ = p2 // quiet linter
}

func TestE2E_Replacement_And_Prevention(t *testing.T) {
    p1 := NewPlayer("P1", 20)
    p2 := NewPlayer("P2", 20)
    g := NewGame(p1, p2)

    // P1 summons a 1/1 that is marked with would-die → exile this turn
    s11 := SimpleCard{Name: "OneOne", TypeLine: "Creature", Power: "1", Toughness: "1"}
    p1.AddCardToHand(s11)
    perm, err := g.SummonCreature(p1, "OneOne")
    if err != nil { t.Fatalf("summon failed: %v", err) }

    g.AddWouldDieExileUntilEOT(perm)

    // Deal lethal damage and apply SBAs → should move to exile, not graveyard
    perm.AddDamage(1)
    g.ApplyStateBasedActions()

    if len(p1.Exile) != 1 || p1.Exile[0].Name != "OneOne" {
        t.Fatalf("expected creature exiled on death replacement, exile=%v", p1.Exile)
    }
    if len(p1.Graveyard) != 0 {
        t.Fatalf("expected no card in graveyard, got %v", p1.Graveyard)
    }

    // Player damage prevention – add 3, then deal 5 → net 2
    start := p2.GetLifeTotal()
    g.AddDamagePrevention(p2, 3)
    g.ApplyDamageToPlayer(p2, 5)
    if p2.GetLifeTotal() != start-2 {
        t.Fatalf("expected life %d after prevention, got %d", start-2, p2.GetLifeTotal())
    }
}

func TestE2E_Continuous_SetPT_ResetsEOT(t *testing.T) {
    p1 := NewPlayer("P1", 20)
    p2 := NewPlayer("P2", 20)
    g := NewGame(p1, p2)

    // 2/2 creature
    c := SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
    p1.AddCardToHand(c)
    perm, err := g.SummonCreature(p1, "Bear")
    if err != nil { t.Fatalf("summon failed: %v", err) }

    // Set 5/5 until EOT
    g.ApplySetPTUntilEOT(perm, 5, 5)
    if perm.GetPower() != 5 || perm.GetToughness() != 5 {
        t.Fatalf("expected 5/5 now, got %d/%d", perm.GetPower(), perm.GetToughness())
    }

    // End of turn cleanup
    for i := 0; i < 7; i++ { g.AdvancePhase() }
    if perm.GetPower() != 2 || perm.GetToughness() != 2 {
        t.Fatalf("expected reset to 2/2 after EOT, got %d/%d", perm.GetPower(), perm.GetToughness())
    }

    _ = p2 // quiet linter
}

