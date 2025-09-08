package game

import "testing"

func TestAdvancePhaseAndTurnRotation(t *testing.T) {
    p1 := &Player{name: "P1"}
    p2 := &Player{name: "P2"}
    g := NewGame(p1, p2)

    if g.GetTurnNumber() != 1 { t.Fatalf("turn should start at 1, got %d", g.GetTurnNumber()) }
    if g.GetCurrentPhase() != PhaseUntap { t.Fatalf("start phase should be Untap") }

    // Progress to End
    g.AdvancePhase() // Upkeep
    if g.GetCurrentPhase() != PhaseUpkeep { t.Fatalf("want Upkeep, got %v", g.GetCurrentPhase()) }
    g.AdvancePhase() // Draw
    g.AdvancePhase() // Main1
    if !g.IsMainPhase() { t.Fatalf("expected main phase") }
    g.AdvancePhase() // Combat
    if !g.IsCombatPhase() { t.Fatalf("expected combat phase") }
    g.AdvancePhase() // Main2
    if !g.IsMainPhase() { t.Fatalf("expected main phase (second)") }
    g.AdvancePhase() // End
    if g.GetCurrentPhase() != PhaseEnd { t.Fatalf("want End, got %v", g.GetCurrentPhase()) }

    // Next call rotates to next player and Untap
    g.AdvancePhase() // -> next player's Untap
    if g.GetCurrentPhase() != PhaseUntap { t.Fatalf("want Untap after End") }
    if g.GetCurrentPlayerRaw() != p2 { t.Fatalf("expected turn to rotate to P2") }
    if g.GetTurnNumber() != 1 { t.Fatalf("turn number should remain 1 until it wraps to first player") }

    // Complete P2's turn to wrap back to P1 and increment turn
    for i := 0; i < 7; i++ { g.AdvancePhase() }
    if g.GetCurrentPlayerRaw() != p1 { t.Fatalf("expected to rotate back to P1") }
    if g.GetCurrentPhase() != PhaseUntap { t.Fatalf("expected to be at Untap after rotation") }
    if g.GetTurnNumber() != 2 { t.Fatalf("expected turn number 2 after wrap, got %d", g.GetTurnNumber()) }
}

