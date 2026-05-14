package game

import "testing"

func TestAdvancePhaseAndTurnRotation(t *testing.T) {
	p1 := &Player{name: "P1"}
	p2 := &Player{name: "P2"}
	g := NewGame(p1, p2)

	if g.GetTurnNumber() != 1 {
		t.Fatalf("turn should start at 1, got %d", g.GetTurnNumber())
	}
	if g.GetCurrentPhase() != PhaseUntap {
		t.Fatalf("start phase should be Untap")
	}

	// Progress to End
	g.AdvancePhase() // Upkeep
	if g.GetCurrentPhase() != PhaseUpkeep {
		t.Fatalf("want Upkeep, got %v", g.GetCurrentPhase())
	}
	g.AdvancePhase() // Draw
	g.AdvancePhase() // Main1
	if !g.IsMainPhase() {
		t.Fatalf("expected main phase")
	}
	g.AdvancePhase() // Combat
	if !g.IsCombatPhase() {
		t.Fatalf("expected combat phase")
	}
	g.AdvancePhase() // Main2
	if !g.IsMainPhase() {
		t.Fatalf("expected main phase (second)")
	}
	g.AdvancePhase() // End
	if g.GetCurrentPhase() != PhaseEnd {
		t.Fatalf("want End, got %v", g.GetCurrentPhase())
	}
	g.AdvancePhase() // Cleanup
	if g.GetCurrentPhase() != PhaseCleanup {
		t.Fatalf("want Cleanup after End, got %v", g.GetCurrentPhase())
	}

	// Next call rotates to next player and Untap
	g.AdvancePhase() // -> next player's Untap
	if g.GetCurrentPhase() != PhaseUntap {
		t.Fatalf("want Untap after Cleanup")
	}
	if g.GetCurrentPlayerRaw() != p2 {
		t.Fatalf("expected turn to rotate to P2")
	}
	if g.GetTurnNumber() != 1 {
		t.Fatalf("turn number should remain 1 until it wraps to first player")
	}

	// Complete P2's turn to wrap back to P1 and increment turn
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	if g.GetCurrentPlayerRaw() != p1 {
		t.Fatalf("expected to rotate back to P1")
	}
	if g.GetCurrentPhase() != PhaseUntap {
		t.Fatalf("expected to be at Untap after rotation")
	}
	if g.GetTurnNumber() != 2 {
		t.Fatalf("expected turn number 2 after wrap, got %d", g.GetTurnNumber())
	}
}


func TestAdvancePhase_SkipsEliminatedPlayer(t *testing.T) {
	p1 := NewPlayer("P1", 40)
	p2 := NewPlayer("P2", 40)
	p3 := NewPlayer("P3", 40)
	g := NewGame(p1, p2, p3)

	// Eliminate P2 mid-game (set life to 0 and apply SBAs, or just call Lose)
	p2.Lose("test_elimination")

	// Advance from P1's Cleanup to next living player (should skip P2 and land on P3)
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	if g.GetCurrentPlayerRaw() != p3 {
		t.Fatalf("expected turn to skip P2 and rotate to P3, got %s", g.GetCurrentPlayerRaw().GetName())
	}
	if g.GetTurnNumber() != 1 {
		t.Fatalf("expected turn number to remain 1, got %d", g.GetTurnNumber())
	}

	// Advance from P3's Cleanup back to P1 (wrap-around)
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	if g.GetCurrentPlayerRaw() != p1 {
		t.Fatalf("expected turn to wrap back to P1, got %s", g.GetCurrentPlayerRaw().GetName())
	}
	if g.GetTurnNumber() != 2 {
		t.Fatalf("expected turn number 2 after wrap, got %d", g.GetTurnNumber())
	}
}

func TestAdvancePhase_SkipsFirstPlayerWhenDead(t *testing.T) {
	p1 := NewPlayer("P1", 40)
	p2 := NewPlayer("P2", 40)
	p3 := NewPlayer("P3", 40)
	g := NewGame(p1, p2, p3)

	// Eliminate P1 before any turns pass
	p1.Lose("test_elimination")

	// Advance from P1's initial Cleanup to next living player (should skip P1 and land on P2)
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	if g.GetCurrentPlayerRaw() != p2 {
		t.Fatalf("expected turn to skip dead P1 and rotate to P2, got %s", g.GetCurrentPlayerRaw().GetName())
	}
	// Turn number should remain 1 because P2 index (1) > P1 index (0)
	if g.GetTurnNumber() != 1 {
		t.Fatalf("expected turn number 1, got %d", g.GetTurnNumber())
	}

	// Advance from P2 -> P3
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	if g.GetCurrentPlayerRaw() != p3 {
		t.Fatalf("expected turn to rotate to P3, got %s", g.GetCurrentPlayerRaw().GetName())
	}

	// Advance from P3 -> wrap to P2 (since P1 is dead)
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	if g.GetCurrentPlayerRaw() != p2 {
		t.Fatalf("expected wrap to P2, got %s", g.GetCurrentPlayerRaw().GetName())
	}
	if g.GetTurnNumber() != 2 {
		t.Fatalf("expected turn number 2 after wrap, got %d", g.GetTurnNumber())
	}
}

func TestPlayerLose_ExilesAllZones(t *testing.T) {
	p := NewPlayer("P", 20)
	c1 := SimpleCard{Name: "Card1", TypeLine: "Creature"}
	c2 := SimpleCard{Name: "Card2", TypeLine: "Land"}
	c3 := SimpleCard{Name: "Card3", TypeLine: "Instant"}
	c4 := SimpleCard{Name: "Card4", TypeLine: "Sorcery"}
	c5 := SimpleCard{Name: "Card5", TypeLine: "Artifact"}
	c6 := SimpleCard{Name: "Commander", TypeLine: "Legendary Creature — Human"}

	p.Library = []SimpleCard{c1}
	p.Hand = []SimpleCard{c2}
	p.Graveyard = []SimpleCard{c3}
	p.Exile = []SimpleCard{c5}
	p.CommandZone = []SimpleCard{c6}

	perm := NewPermanent(c4, p, p)
	p.Battlefield = []*Permanent{perm}

	p.Lose("test")

	if !p.HasLost() {
		t.Fatal("expected player to be marked as lost")
	}
	if len(p.Library) != 0 {
		t.Fatalf("expected library to be empty, got %d", len(p.Library))
	}
	if len(p.Hand) != 0 {
		t.Fatalf("expected hand to be empty, got %d", len(p.Hand))
	}
	if len(p.Graveyard) != 0 {
		t.Fatalf("expected graveyard to be empty, got %d", len(p.Graveyard))
	}
	if len(p.Battlefield) != 0 {
		t.Fatalf("expected battlefield to be empty, got %d", len(p.Battlefield))
	}
	if len(p.CommandZone) != 0 {
		t.Fatalf("expected command zone to be empty, got %d", len(p.CommandZone))
	}
	if len(p.Exile) != 6 {
		t.Fatalf("expected 6 cards in exile, got %d (%v)", len(p.Exile), p.Exile)
	}

	// Verify double-Lose is a no-op
	p.Lose("test_again")
	if len(p.Exile) != 6 {
		t.Fatalf("expected exile to remain 6 after second Lose, got %d", len(p.Exile))
	}
}

func TestPlayerLose_ExilePreservesCommander(t *testing.T) {
	p := NewPlayer("P", 20)
	cmdr := SimpleCard{Name: "Atraxa", TypeLine: "Legendary Creature — Angel"}
	p.CommandZone = []SimpleCard{cmdr}
	p.Library = []SimpleCard{{Name: "Forest", TypeLine: "Basic Land — Forest"}}

	p.Lose("test")

	found := false
	for _, c := range p.Exile {
		if c.Name == "Atraxa" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected commander to be exiled")
	}
}
