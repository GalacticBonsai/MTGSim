package simulation

import (
	"errors"
	"math/rand"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// EDHSeat is one configured seat in a pod. The runner is intentionally
// decoupled from pkg/deck and pkg/card: callers (e.g. cmd/mtgsim-edh)
// are responsible for importing decklists and converting cards into the
// engine's SimpleCard form before seating a player.
type EDHSeat struct {
	DeckPath  string
	DeckName  string
	Library   []game.SimpleCard
	Commander *game.SimpleCard // nil if the deck has no commander designated
	Mulligans int              // mulligans the player will take before the game starts
}

// EDHRunOptions configures one pod simulation.
type EDHRunOptions struct {
	Seats    []EDHSeat
	MaxTurns int
	RNG      *rand.Rand
	// Priority is the optional opponent-priority handler. nil falls
	// back to NoopPriorityHandler so the runner stays sorcery-speed.
	Priority PriorityHandler
	// RecordEvents enables per-pod event logging. The resulting log is
	// attached to EDHGameRecord.Events. Off by default to keep batch
	// runs cheap.
	RecordEvents bool
}

// SimulateEDHGame runs a single pod and returns the recorded game.
// The implementation is deliberately simple — a thin "play a land,
// summon every creature, attack the most threatening opponent" AI —
// so it exercises the multiplayer / EDH plumbing (command zone,
// 21-damage SBA, life totals) without depending on a complete cost /
// mana solver. Phase 4 added APNAP-ordered triggers, threat assessment,
// an opponent-priority hook, and an optional per-pod event log.
func SimulateEDHGame(opts EDHRunOptions) (EDHGameRecord, error) {
	if len(opts.Seats) < 2 {
		return EDHGameRecord{}, errors.New("EDH pod requires at least 2 seats")
	}
	rng := opts.RNG
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	maxTurns := opts.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}
	priority := opts.Priority
	if priority == nil {
		priority = NoopPriorityHandler{}
	}
	var log *EDHEventLog
	if opts.RecordEvents {
		log = NewEDHEventLog()
	}

	players, casts := setupEDHPlayers(opts.Seats, rng)
	g := game.NewGame(players...)
	if log != nil {
		for _, s := range opts.Seats {
			log.Append(EDHEvent{Turn: 1, Phase: "setup", Kind: EventGameStart, Actor: s.DeckName, Detail: s.DeckPath})
		}
	}

	turnLimitHit := false
	for {
		if anyAlive := stepOneEDHTurn(g, opts.Seats, casts, priority, log); !anyAlive {
			break
		}
		if survivors(g) <= 1 {
			break
		}
		if g.GetTurnNumber() > maxTurns {
			turnLimitHit = true
			break
		}
	}

	rec := finalizeRecord(g, opts.Seats, casts, turnLimitHit)
	if log != nil {
		log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: "end", Kind: EventGameEnd, Actor: rec.Winner})
		rec.Events = log.Events()
	}
	return rec, nil
}

// setupEDHPlayers materializes Player objects, registers commanders, and
// performs initial draws (including any requested mulligans).
func setupEDHPlayers(seats []EDHSeat, rng *rand.Rand) ([]*game.Player, []int) {
	players := make([]*game.Player, len(seats))
	casts := make([]int, len(seats))
	for i, s := range seats {
		p := game.NewEDHPlayer(s.DeckName)
		p.Library = append(p.Library, s.Library...)
		rng.Shuffle(len(p.Library), func(a, b int) { p.Library[a], p.Library[b] = p.Library[b], p.Library[a] })
		if s.Commander != nil {
			p.RegisterCommander(*s.Commander)
		}
		if s.Mulligans > 0 {
			_, _ = p.LondonMulligan(rng, s.Mulligans)
		} else {
			p.DrawOpeningHand()
		}
		players[i] = p
	}
	return players, casts
}

// survivors counts players that have not been eliminated.
func survivors(g *game.Game) int {
	n := 0
	for _, p := range g.GetPlayersRaw() {
		if !p.HasLost() {
			n++
		}
	}
	return n
}

// finalizeRecord builds the EDHGameRecord after the simulation loop ends.
func finalizeRecord(g *game.Game, seats []EDHSeat, casts []int, turnLimitHit bool) EDHGameRecord {
	rec := EDHGameRecord{Turns: g.GetTurnNumber(), Players: make([]EDHPlayerRecord, len(seats))}
	var winner string
	for i, p := range g.GetPlayersRaw() {
		pr := EDHPlayerRecord{
			DeckName:       seats[i].DeckName,
			Mulligans:      seats[i].Mulligans,
			FinalLife:      p.GetLifeTotal(),
			CommanderCasts: casts[i],
		}
		if seats[i].Commander != nil {
			pr.CommanderName = seats[i].Commander.Name
		}
		if p.HasLost() {
			pr.Eliminated = true
			pr.KillSource = classifyElimination(p, turnLimitHit)
		} else {
			winner = seats[i].DeckName
		}
		rec.Players[i] = pr
	}
	rec.Winner = winner
	return rec
}


