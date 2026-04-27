package game

import (
	"math/rand"
	"testing"
)

func TestEDH_StartingLife40(t *testing.T) {
	p := NewEDHPlayer("Cmdr")
	if p.GetLifeTotal() != 40 {
		t.Fatalf("expected 40 life, got %d", p.GetLifeTotal())
	}
}

func TestEDH_NewEDHGameMultiplayer(t *testing.T) {
	g := NewEDHGame("A", "B", "C", "D")
	if g.NumPlayers() != 4 {
		t.Fatalf("expected 4 players, got %d", g.NumPlayers())
	}
	for i := 0; i < 4; i++ {
		if g.GetPlayerByIndex(i).GetLifeTotal() != 40 {
			t.Fatalf("player %d not at 40 life", i)
		}
	}
}

// makeLibrary creates a deterministic library of n distinct cards.
func makeLibrary(n int) []SimpleCard {
	out := make([]SimpleCard, n)
	for i := 0; i < n; i++ {
		out[i] = SimpleCard{Name: itoa(i), TypeLine: "Land"}
	}
	return out
}

func TestLondonMulligan_FirstMulliganDraws7Bottoms1(t *testing.T) {
	p := NewEDHPlayer("P")
	p.Library = makeLibrary(60)
	p.Draw(OpeningHandSize) // initial 7
	rng := rand.New(rand.NewSource(42))
	bottomed, err := p.LondonMulligan(rng, 1)
	if err != nil {
		t.Fatalf("mulligan err: %v", err)
	}
	if bottomed != 1 {
		t.Fatalf("expected 1 bottomed, got %d", bottomed)
	}
	if p.HandSize() != 6 {
		t.Fatalf("expected 6 in hand after first mulligan, got %d", p.HandSize())
	}
	if p.LibrarySize() != 60-6 {
		t.Fatalf("expected library 54, got %d", p.LibrarySize())
	}
}

func TestLondonMulligan_TripleMulliganBottomsThree(t *testing.T) {
	p := NewEDHPlayer("P")
	p.Library = makeLibrary(60)
	p.Draw(OpeningHandSize)
	rng := rand.New(rand.NewSource(1))
	for i := 1; i <= 3; i++ {
		_, err := p.LondonMulligan(rng, i)
		if err != nil {
			t.Fatalf("mull #%d err: %v", i, err)
		}
	}
	// After taking 3 mulligans the player has drawn 7 then bottomed 3.
	if p.HandSize() != 4 {
		t.Fatalf("expected 4 in hand after 3 mulligans, got %d", p.HandSize())
	}
	if p.LibrarySize() != 60-4 {
		t.Fatalf("expected library 56, got %d", p.LibrarySize())
	}
}

func TestLondonMulligan_FreeMulliganBottomsZero(t *testing.T) {
	p := NewEDHPlayer("P")
	p.Library = makeLibrary(60)
	p.Draw(OpeningHandSize)
	rng := rand.New(rand.NewSource(7))
	bottomed, err := p.LondonMulligan(rng, 0)
	if err != nil {
		t.Fatalf("free mulligan err: %v", err)
	}
	if bottomed != 0 {
		t.Fatalf("free mulligan must bottom 0, got %d", bottomed)
	}
	if p.HandSize() != 7 {
		t.Fatalf("expected 7 in hand on a free mulligan, got %d", p.HandSize())
	}
}

func TestLondonMulligan_NegativeRejected(t *testing.T) {
	p := NewEDHPlayer("P")
	p.Library = makeLibrary(60)
	rng := rand.New(rand.NewSource(7))
	if _, err := p.LondonMulligan(rng, -1); err == nil {
		t.Fatalf("expected error on negative mulligan count")
	}
}

func TestLondonMulligan_ShuffleIsDeterministicWithSameSeed(t *testing.T) {
	p1 := NewEDHPlayer("P")
	p1.Library = makeLibrary(20)
	p1.Draw(OpeningHandSize)
	p2 := NewEDHPlayer("P")
	p2.Library = makeLibrary(20)
	p2.Draw(OpeningHandSize)
	rng1 := rand.New(rand.NewSource(123))
	rng2 := rand.New(rand.NewSource(123))
	_, _ = p1.LondonMulligan(rng1, 2)
	_, _ = p2.LondonMulligan(rng2, 2)
	if p1.HandSize() != p2.HandSize() {
		t.Fatalf("deterministic mulligan diverged")
	}
	for i := range p1.Hand {
		if p1.Hand[i].Name != p2.Hand[i].Name {
			t.Fatalf("hand[%d] differs: %q vs %q", i, p1.Hand[i].Name, p2.Hand[i].Name)
		}
	}
}
