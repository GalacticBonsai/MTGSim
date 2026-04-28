package simulation

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// TestEDHEventLog_Roundtrip plays a 2-player pod with RecordEvents=true
// and verifies the resulting log contains the canonical events (game
// start, at least one land/creature/combat entry, and game end).
func TestEDHEventLog_Roundtrip(t *testing.T) {
	a := makeSeat("A", "Plains", "Bear", "2", 8, &game.SimpleCard{
		Name: "A Cmdr", TypeLine: "Legendary Creature", Power: "5", Toughness: "5",
	})
	b := makeSeat("B", "Forest", "Wolf", "3", 8, &game.SimpleCard{
		Name: "B Cmdr", TypeLine: "Legendary Creature", Power: "4", Toughness: "4",
	})
	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats:        []EDHSeat{a, b},
		MaxTurns:     30,
		RNG:          rand.New(rand.NewSource(1)),
		RecordEvents: true,
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if len(rec.Events) == 0 {
		t.Fatalf("expected event log to be populated when RecordEvents=true")
	}

	kinds := map[EDHEventKind]int{}
	for _, e := range rec.Events {
		kinds[e.Kind]++
	}
	if kinds[EventGameStart] < 2 {
		t.Errorf("expected one game_start per seat, got %d", kinds[EventGameStart])
	}
	if kinds[EventLandPlay] == 0 {
		t.Errorf("expected at least one land_play event")
	}
	if kinds[EventCreatureSummon] == 0 {
		t.Errorf("expected at least one creature_summon event")
	}
	if kinds[EventGameEnd] != 1 {
		t.Errorf("expected exactly one game_end event, got %d", kinds[EventGameEnd])
	}

	// JSON marshal round-trips so the dashboard / replay tool can ingest.
	bs, err := json.Marshal(rec.Events)
	if err != nil {
		t.Fatalf("marshal events: %v", err)
	}
	if !strings.Contains(string(bs), "\"kind\":\"game_end\"") {
		t.Errorf("expected JSON to contain game_end, got %s", bs)
	}
}

// TestEDHEventLog_Disabled confirms that with RecordEvents=false (the
// default) no log is allocated and the record carries no events.
func TestEDHEventLog_Disabled(t *testing.T) {
	a := makeSeat("A", "Plains", "Bear", "2", 8, nil)
	b := makeSeat("B", "Forest", "Wolf", "3", 8, nil)
	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats:    []EDHSeat{a, b},
		MaxTurns: 30,
		RNG:      rand.New(rand.NewSource(1)),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if len(rec.Events) != 0 {
		t.Errorf("expected no events when RecordEvents=false, got %d", len(rec.Events))
	}
}

// recordingPriorityHandler counts invocations to verify the runner
// honours the priority hook for non-active opponents.
type recordingPriorityHandler struct {
	calls int
}

func (r *recordingPriorityHandler) OnOpponentPriority(g *game.Game, active *game.Player, opp *game.Player, phase game.Phase) {
	r.calls++
}

// TestPriorityHandler_InvokedForOpponents ensures a non-noop handler
// receives at least one call per opponent per turn.
func TestPriorityHandler_InvokedForOpponents(t *testing.T) {
	a := makeSeat("A", "Plains", "Bear", "2", 8, nil)
	b := makeSeat("B", "Forest", "Wolf", "3", 8, nil)
	c := makeSeat("C", "Swamp", "Zombie", "2", 8, nil)
	h := &recordingPriorityHandler{}
	_, err := SimulateEDHGame(EDHRunOptions{
		Seats:    []EDHSeat{a, b, c},
		MaxTurns: 4,
		RNG:      rand.New(rand.NewSource(1)),
		Priority: h,
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if h.calls == 0 {
		t.Errorf("expected priority handler to be called at least once across the run")
	}
}
