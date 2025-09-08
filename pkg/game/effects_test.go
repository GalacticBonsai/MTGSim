package game

import "testing"

func TestTempPump_AffectsCombatAndResetsEOT(t *testing.T) {
    p1 := NewPlayer("P1", 20)
    p2 := NewPlayer("P2", 20)
    g := NewGame(p1, p2)

    // Create a 1/1 attacker
    c := SimpleCard{Name: "Soldier", TypeLine: "Creature", Power: "1", Toughness: "1"}
    att := NewPermanent(c, p1, p1)
    p1.Battlefield = append(p1.Battlefield, att)

    // Apply +2/+2 until EOT
    g.ApplyTempPump(att, 2, 2)
    if att.GetPower() != 3 || att.GetToughness() != 3 {
        t.Fatalf("expected 3/3 after pump, got %d/%d", att.GetPower(), att.GetToughness())
    }

    // Attack unblocked and deal 3
    g.BeginCombat()
    if err := g.DeclareAttacker(att, p2); err != nil { t.Fatalf("declare attacker: %v", err) }
    g.ResolveCombatDamage()
    if p2.GetLifeTotal() != 17 {
        t.Fatalf("expected defender at 17 life, got %d", p2.GetLifeTotal())
    }

    // Advance to end then to next turn to clear EOT effects
    for i := 0; i < 6; i++ { g.AdvancePhase() } // to End
    g.AdvancePhase() // wrap to next turn Untap and clear EOT

    if att.GetPower() != 1 || att.GetToughness() != 1 {
        t.Fatalf("expected pump to reset at EOT, got %d/%d", att.GetPower(), att.GetToughness())
    }
}

