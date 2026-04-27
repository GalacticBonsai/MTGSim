package game

import "testing"

// makeCommanderCard returns a SimpleCard suitable for use as a commander.
func makeCommanderCard(name string, power, toughness string) SimpleCard {
	return SimpleCard{
		Name:      name,
		TypeLine:  "Legendary Creature — Human",
		Power:     power,
		Toughness: toughness,
	}
}

func TestCommandZone_RegisterAndCast_1v1(t *testing.T) {
	p1 := NewPlayer("Alice", 40)
	p2 := NewPlayer("Bob", 40)
	_ = NewGame(p1, p2)

	cmdr := makeCommanderCard("Test Commander", "3", "3")
	p1.RegisterCommander(cmdr)

	if !p1.IsCommanderName("Test Commander") {
		t.Fatalf("expected commander to be registered")
	}
	if len(p1.CommandZone) != 1 || p1.CommandZone[0].Name != "Test Commander" {
		t.Fatalf("expected commander in command zone, got %v", p1.CommandZone)
	}

	if tax := p1.CommanderTax("Test Commander"); tax != 0 {
		t.Fatalf("expected initial tax 0, got %d", tax)
	}

	perm := p1.CastCommander("Test Commander")
	if perm == nil || !perm.IsCommander() {
		t.Fatalf("expected to cast commander as flagged permanent")
	}
	if len(p1.CommandZone) != 0 || len(p1.Battlefield) != 1 {
		t.Fatalf("expected commander to leave CZ and enter battlefield")
	}
	if tax := p1.CommanderTax("Test Commander"); tax != 2 {
		t.Fatalf("expected tax 2 after first cast, got %d", tax)
	}
}

func TestCommanderTax_ScalesPerCast(t *testing.T) {
	p := NewPlayer("Solo", 40)
	cmdr := makeCommanderCard("Tax Cmdr", "1", "1")
	p.RegisterCommander(cmdr)

	for i := 0; i < 3; i++ {
		if perm := p.CastCommander("Tax Cmdr"); perm == nil {
			t.Fatalf("cast %d failed", i)
		}
		// send back to CZ via the zone-change helper to allow another cast
		p.SendCommanderToCommandZone(p.Battlefield[0])
	}
	if tax := p.CommanderTax("Tax Cmdr"); tax != 6 {
		t.Fatalf("expected tax 6 after 3 casts, got %d", tax)
	}
}

func TestCommanderDies_RedirectsToCommandZone_1v1(t *testing.T) {
	p1 := NewPlayer("Alice", 40)
	p2 := NewPlayer("Bob", 40)
	g := NewGame(p1, p2)

	cmdr := makeCommanderCard("Doomed Cmdr", "2", "2")
	p1.RegisterCommander(cmdr)
	perm := p1.CastCommander("Doomed Cmdr")

	// deal lethal damage, then run SBA
	perm.AddDamage(5)
	g.ApplyStateBasedActions()

	if len(p1.Battlefield) != 0 {
		t.Fatalf("expected commander off battlefield, got %d", len(p1.Battlefield))
	}
	if len(p1.Graveyard) != 0 {
		t.Fatalf("expected commander not in graveyard (CR 903.9), got %d", len(p1.Graveyard))
	}
	if len(p1.CommandZone) != 1 || p1.CommandZone[0].Name != "Doomed Cmdr" {
		t.Fatalf("expected commander redirected to command zone, got %v", p1.CommandZone)
	}
}

func TestCommanderDamage_21Lethal_1v1(t *testing.T) {
	p1 := NewPlayer("Attacker", 40)
	p2 := NewPlayer("Defender", 40)
	g := NewGame(p1, p2)

	cmdr := makeCommanderCard("Voltron", "7", "7")
	p1.RegisterCommander(cmdr)
	perm := p1.CastCommander("Voltron")
	perm.SetEnteredTurn(0) // bypass summoning sickness for the test

	// three swings of 7 = 21 commander damage
	for i := 0; i < 3; i++ {
		g.BeginCombat()
		if err := g.DeclareAttacker(perm, p2); err != nil {
			t.Fatalf("attack %d: %v", i, err)
		}
		g.ResolveCombatDamage()
		perm.Untap()
	}

	if got := p2.CommanderDamageFrom(p1, "Voltron"); got != 21 {
		t.Fatalf("expected 21 commander damage, got %d", got)
	}
	g.ApplyStateBasedActions()
	if !p2.HasLost() {
		t.Fatalf("expected defender to have lost from 21 commander damage")
	}
}

func TestCommanderDamage_NotLethalAt20(t *testing.T) {
	p1 := NewPlayer("A", 40)
	p2 := NewPlayer("B", 40)
	g := NewGame(p1, p2)

	cmdr := makeCommanderCard("Almost", "5", "5")
	p1.RegisterCommander(cmdr)
	perm := p1.CastCommander("Almost")
	perm.SetEnteredTurn(0)

	// 4 swings of 5 = 20 damage
	for i := 0; i < 4; i++ {
		g.BeginCombat()
		if err := g.DeclareAttacker(perm, p2); err != nil {
			t.Fatalf("attack %d: %v", i, err)
		}
		g.ResolveCombatDamage()
		perm.Untap()
	}
	g.ApplyStateBasedActions()
	if p2.HasLost() {
		t.Fatalf("defender should not lose at 20 commander damage")
	}
	if p2.GetLifeTotal() != 20 {
		t.Fatalf("expected life total 20 after 20 damage, got %d", p2.GetLifeTotal())
	}
}

func TestMultiplayer_TurnRotation_4Players(t *testing.T) {
	p1, p2, p3, p4 := NewPlayer("A", 40), NewPlayer("B", 40), NewPlayer("C", 40), NewPlayer("D", 40)
	g := NewGame(p1, p2, p3, p4)
	if g.NumPlayers() != 4 {
		t.Fatalf("expected 4 players, got %d", g.NumPlayers())
	}
	expected := []*Player{p2, p3, p4, p1}
	for _, want := range expected {
		// 7 phase advances per turn to wrap from Untap back to Untap of next player
		for i := 0; i < 7; i++ {
			g.AdvancePhase()
		}
		if g.GetCurrentPlayerRaw() != want {
			t.Fatalf("expected %s active, got %s", want.GetName(), g.GetCurrentPlayerRaw().GetName())
		}
	}
	if g.GetTurnNumber() != 2 {
		t.Fatalf("expected turn 2 after wrap, got %d", g.GetTurnNumber())
	}
}
