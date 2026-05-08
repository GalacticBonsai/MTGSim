package simulation

import (
	"testing"
)

func TestEDHResults_RecordAndAggregate(t *testing.T) {
	r := NewEDHResults()

	r.RecordGame(EDHGameRecord{
		Turns: 10, Winner: "Alpha", MaxStormCount: 3, TotalManaSpent: 11, TotalCardsPlayed: 8,
		TotalCombatDamage: 21, TotalEliminations: 2,
		Players: []EDHPlayerRecord{
			{DeckName: "Alpha", CommanderName: "Atraxa", Mulligans: 0, FinalLife: 17, CommanderCasts: 1,
				CardsPlayed: 5, LandsPlayed: 2, SpellsCast: 3, ManaSpent: 7, MaxStormCount: 3, CombatDamage: 21, Eliminations: 2},
			{DeckName: "Beta", CommanderName: "Edgar", Mulligans: 1, FinalLife: 0, CommanderCasts: 2,
				Eliminated: true, KillSource: KillSourceLifeLoss},
			{DeckName: "Gamma", CommanderName: "Krenko", Mulligans: 2, FinalLife: 0, CommanderCasts: 1,
				Eliminated: true, KillSource: KillSourceCommanderDamage},
		},
	})
	r.RecordGame(EDHGameRecord{
		Turns: 14, Winner: "Beta", MaxStormCount: 2, TotalManaSpent: 9, TotalCardsPlayed: 6,
		TotalCombatDamage: 15, TotalEliminations: 1,
		Players: []EDHPlayerRecord{
			{DeckName: "Alpha", CommanderName: "Atraxa", Mulligans: 0, FinalLife: 0, CommanderCasts: 1,
				CardsPlayed: 3, LandsPlayed: 1, SpellsCast: 2, ManaSpent: 4, MaxStormCount: 2, CombatDamage: 0,
				Eliminated: true, KillSource: KillSourceCommanderDamage},
			{DeckName: "Beta", CommanderName: "Edgar", Mulligans: 0, FinalLife: 8, CommanderCasts: 1,
				CardsPlayed: 3, LandsPlayed: 1, SpellsCast: 2, ManaSpent: 5, MaxStormCount: 2, CombatDamage: 15, Eliminations: 1},
		},
	})

	if r.GameCount() != 2 {
		t.Fatalf("want 2 games, got %d", r.GameCount())
	}
	if got := r.AverageTurns(); got != 12.0 {
		t.Fatalf("want avg turns 12, got %v", got)
	}

	stats := r.DeckStats()
	byName := map[string]EDHDeckStats{}
	for _, s := range stats {
		byName[s.DeckName] = s
	}

	alpha := byName["Alpha"]
	if alpha.Games != 2 || alpha.Wins != 1 || alpha.Losses != 1 {
		t.Fatalf("Alpha aggregate wrong: %+v", alpha)
	}
	if alpha.WinRate < 49.9 || alpha.WinRate > 50.1 {
		t.Fatalf("Alpha win rate want ~50%%, got %v", alpha.WinRate)
	}
	if alpha.CommanderDamageKOs != 1 {
		t.Fatalf("Alpha commander dmg KOs want 1, got %d", alpha.CommanderDamageKOs)
	}
	if alpha.AvgFinalLife != 8.5 {
		t.Fatalf("Alpha avg final life want 8.5, got %v", alpha.AvgFinalLife)
	}
	if alpha.MaxStormCount != 3 || alpha.TotalManaSpent != 11 || alpha.TotalCardsPlayed != 8 || alpha.Eliminations != 2 {
		t.Fatalf("Alpha tuning metrics wrong: %+v", alpha)
	}
	if alpha.AvgManaSpent != 5.5 || alpha.AvgCardsPlayed != 4.0 || alpha.AvgCombatDamage != 10.5 {
		t.Fatalf("Alpha tuning averages wrong: %+v", alpha)
	}

	beta := byName["Beta"]
	if beta.Wins != 1 || beta.Losses != 1 || beta.LifeLossKOs != 1 {
		t.Fatalf("Beta aggregate wrong: %+v", beta)
	}

	gamma := byName["Gamma"]
	if gamma.Games != 1 || gamma.Wins != 0 || gamma.Losses != 1 || gamma.CommanderDamageKOs != 1 {
		t.Fatalf("Gamma aggregate wrong: %+v", gamma)
	}
	if gamma.AvgMulligans != 2.0 {
		t.Fatalf("Gamma avg mulligans want 2, got %v", gamma.AvgMulligans)
	}

	summary := r.Summary()
	if summary.TotalGames != 2 || summary.HighestStormCount != 3 || summary.TotalManaSpent != 20 || summary.TotalCardsPlayed != 14 {
		t.Fatalf("summary totals wrong: %+v", summary)
	}
	if summary.AverageManaSpent != 10 || summary.AverageCardsPlayed != 7 || summary.AverageCombatDamage != 18 {
		t.Fatalf("summary averages wrong: %+v", summary)
	}
}

func TestEDHResults_Empty(t *testing.T) {
	r := NewEDHResults()
	if r.GameCount() != 0 {
		t.Fatalf("expected 0 games on fresh aggregator")
	}
	if r.AverageTurns() != 0 {
		t.Fatalf("expected 0 average turns on empty aggregator")
	}
	if len(r.DeckStats()) != 0 {
		t.Fatalf("expected empty deck stats on fresh aggregator")
	}
}

func TestEDHResults_RecentGamesNewestFirstAndCopied(t *testing.T) {
	r := NewEDHResults()
	r.RecordGame(EDHGameRecord{Turns: 3, Winner: "A", Players: []EDHPlayerRecord{{DeckName: "A"}}})
	r.RecordGame(EDHGameRecord{Turns: 4, Winner: "B", Players: []EDHPlayerRecord{{DeckName: "B"}}, Events: []EDHEvent{{Kind: EventGameEnd}}})

	recent := r.RecentGames(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent games, got %d", len(recent))
	}
	if recent[0].Winner != "B" || recent[1].Winner != "A" {
		t.Fatalf("expected newest-first order, got %+v", recent)
	}
	recent[0].Players[0].DeckName = "mutated"
	recent[0].Events[0].Kind = EventTurnStart
	again := r.RecentGames(1)
	if again[0].Players[0].DeckName != "B" || again[0].Events[0].Kind != EventGameEnd {
		t.Fatalf("RecentGames should return copies, got %+v", again[0])
	}
}
