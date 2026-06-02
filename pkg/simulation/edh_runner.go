package simulation

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"

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
	Sideboard []game.SimpleCard
	// Commanders holds all designated command-zone cards. Commander is retained
	// for callers/tests that only need a single commander.
	Commanders []game.SimpleCard
	Commander  *game.SimpleCard // nil if the deck has no commander designated
	Mulligans  int              // mulligans the player will take before the game starts
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
	var log *EDHEventLog
	if opts.RecordEvents {
		log = NewEDHEventLog()
	}
	metrics := newEDHMetrics(len(opts.Seats))

	players, casts := setupEDHPlayers(opts.Seats, rng)
	g := game.NewGame(players...)
	if log != nil {
		for _, s := range opts.Seats {
			log.Append(EDHEvent{Turn: 1, Phase: "setup", Kind: EventGameStart, Actor: s.DeckName, Detail: s.DeckPath})
		}
	}

	priority := opts.Priority
	if priority == nil {
		priority = NewStackAwareHandler(g, log)
	}

	turnLimitHit := false

	for {
		anyAlive, stuck := stepOneEDHTurn(g, casts, priority, log, metrics)
		if !anyAlive {
			break
		}
		if stuck {
			turnLimitHit = true
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

	rec := finalizeRecord(g, opts.Seats, casts, turnLimitHit, metrics)
	if log != nil {
		log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: "end", Kind: EventGameEnd, Actor: rec.Winner})
		rec.Events = log.Events()
	}
	return rec, nil
}

// setupEDHPlayers materializes Player objects, registers commanders, and
// performs initial draws with intelligent mulligan decisions.
// Per the cEDH mulligan framework (Sperling 2023), the default action is to
// mulligan; a hand is only kept if it contains a compelling reason (Truly
// Broken Things or strong Development). As the hand shrinks, leniency increases.
func setupEDHPlayers(seats []EDHSeat, rng *rand.Rand) ([]*game.Player, []int) {
	players := make([]*game.Player, len(seats))
	casts := make([]int, len(seats))
	for i := range seats {
		s := &seats[i]
		p := game.NewEDHPlayer(s.DeckName)
		p.Library = append(p.Library, s.Library...)
		rng.Shuffle(len(p.Library), func(a, b int) { p.Library[a], p.Library[b] = p.Library[b], p.Library[a] })
		for _, commander := range seatCommanders(*s) {
			p.RegisterCommander(commander)
		}
		p.DrawOpeningHand()

		taken := s.Mulligans
		if taken <= 0 {
			taken = iterativeMulligan(p, rng, i, seatCommanders(*s))
		} else {
			// If a caller forces >0 mulligans, cast to int and execute directly.
			_, _ = p.LondonMulligan(rng, taken)
		}
		s.Mulligans = taken
		players[i] = p
	}
	return players, casts
}

// iterativeMulligan evaluates the player's hand after each draw, mulliganing
// until the hand is acceptable or the player reaches 4 cards. Returns the
// number of mulligans taken.
// Flow: 7 → evaluate → (free) 7 → evaluate → 6 → evaluate → 5 → evaluate → 4 → keep.
func iterativeMulligan(p *game.Player, rng *rand.Rand, seat int, commanders []game.SimpleCard) int {
	for m := 0; m < 4; m++ {
		keep, _ := evaluateOpeningHand(p.Hand, commanders, seat, m)
		if keep {
			return m
		}
	// m=0: free mulligan (still 7), m=1: bottom 1 (6), m=2: bottom 2 (5), m=3: bottom 3 (4)
		_, _ = p.LondonMulligan(rng, m+1)
	}
	return 4
}

// evaluateOpeningHand returns true if the hand is worth keeping, following
// the cEDH mulligan framework:
//
//	Level 1 — Truly Broken Things: T1 Remora, fast mana + engine, Ad Naus with
//	         mana, Necro with mana, fast Dockside. These are auto-keeps.
//	Level 2 — Development: 3 lands + curve, 2 lands + ramp, 2 lands + 1-drop,
//	         premium lands. Kept if strong enough.
//	Level 3 — Everything else is a mulligan unless we're at 4-5 cards (desperate).
//
// leniency increases as mulligansTaken increases (fewer cards → more willing).
func evaluateOpeningHand(hand []game.SimpleCard, commanders []game.SimpleCard, seat int, mulligansTaken int) (bool, string) {
	lands := 0
	premiumLands := 0
	fastMana0 := 0
	fastMana1 := 0
	twoDropRamp := 0
	hasRemora := false
	hasRhystic := false
	hasSentinel := false
	hasAdNaus := false
	hasNecro := false
	hasDockside := false
	hasFreeInteraction := 0
	hasTutor := 0

	maxCmc := 0
	oneDrops := 0

	for _, c := range hand {
		if c.IsLand() {
			lands++
			switch c.Name {
			case "Ancient Tomb", "City of Traitors":
				premiumLands++
			}
			continue
		}
		cmc := int(c.GetMinManaCost().Total())
		if cmc > maxCmc {
			maxCmc = cmc
		}
		if cmc == 1 && !c.IsLand() {
			oneDrops++
		}
		switch c.Name {
		case "Mana Crypt", "Jeweled Lotus", "Chrome Mox", "Mox Diamond", "Mox Opal", "Lotus Petal":
			fastMana0++
		case "Sol Ring", "Mana Vault":
			fastMana1++
		case "Arcane Signet", "Fellwar Stone":
			twoDropRamp++
		case "Mystic Remora":
			hasRemora = true
		case "Rhystic Study":
			hasRhystic = true
		case "Esper Sentinel":
			hasSentinel = true
		case "Ad Nauseam", "Ad Nauseum":
			hasAdNaus = true
		case "Necropotence":
			hasNecro = true
		case "Dockside Extortionist":
			hasDockside = true
		case "Force of Will", "Pact of Negation", "Force of Negation", "Deflecting Swat", "Fierce Guardianship":
			hasFreeInteraction++
		case "Demonic Tutor", "Vampiric Tutor", "Imperial Seal", "Enlightened Tutor",
			"Mystical Tutor", "Worldly Tutor", "Gamble":
			hasTutor++
		}
	}

	fastManaTotal := fastMana0 + fastMana1 + twoDropRamp

	// --- Level 1: Truly Broken Things (auto-keep) ---

	if hasRemora && lands >= 1 {
		return true, "T1 Mystic Remora"
	}
	if (hasRhystic || hasSentinel) && lands >= 2 && fastManaTotal >= 1 {
		return true, "t1-2 draw engine + ramp"
	}
	if fastMana0 >= 1 && lands >= 1 && (hasRemora || hasSentinel || hasRhystic || hasTutor >= 1) {
		return true, "0-CMC fast mana + broken followup"
	}
	if fastMana1 >= 1 && lands >= 2 {
		return true, "Sol Ring/Vault + 2 lands"
	}
	if hasAdNaus && lands+fastManaTotal+premiumLands >= 4 {
		return true, "Ad Nauseum with mana"
	}
	if hasNecro && lands+fastManaTotal+premiumLands >= 3 {
		return true, "Necropotence with mana"
	}
	if hasDockside && lands >= 2 && fastManaTotal >= 1 {
		return true, "fast Dockside"
	}

	// --- Level 2: Development ---

	if lands >= 3 && maxCmc <= lands+1 {
		return true, "3 lands + good curve"
	}
	if lands >= 2 && fastManaTotal >= 1 && maxCmc <= 4 {
		return true, "2 lands + ramp + curve"
	}
	if lands >= 2 && oneDrops >= 1 {
		return true, "2 lands + 1-drop"
	}
	if premiumLands >= 1 && lands >= 2 {
		return true, "premium land + 2 lands"
	}

	// --- Level 3: Leniency for smaller hands ---

	if mulligansTaken >= 2 && lands >= 2 && maxCmc <= lands+2 {
		return true, "playable 6-card hand"
	}
	if mulligansTaken >= 3 && lands >= 1 {
		return true, "desperate 4-5 card keep"
	}

	return false, "no compelling reason to keep"
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
func finalizeRecord(g *game.Game, seats []EDHSeat, casts []int, turnLimitHit bool, metrics *edhMetrics) EDHGameRecord {
	rec := EDHGameRecord{Turns: g.GetTurnNumber(), Players: make([]EDHPlayerRecord, len(seats))}
	var winner string
	var winnerPlayer *game.Player
	for i, p := range g.GetPlayersRaw() {
		pr := EDHPlayerRecord{
			DeckName:       seats[i].DeckName,
			Mulligans:      seats[i].Mulligans,
			FinalLife:      p.GetLifeTotal(),
			CommanderCasts: casts[i],
		}
		pr.CommanderName = strings.Join(seatCommanderNames(seats[i]), " / ")
		if p.HasLost() {
			pr.Eliminated = true
			pr.KillSource = classifyElimination(p, turnLimitHit)
		} else {
			winner = seats[i].DeckName
			winnerPlayer = p
		}
		if metrics != nil {
			metrics.applyToPlayerRecord(i, &pr)
		}
		rec.Players[i] = pr
	}
	rec.Winner = winner
	rec.WinnerCondition = classifyWinCondition(winnerPlayer, g)
	if turnLimitHit && winner == "" {
		rec.WinnerCondition = WinConditionTurnLimit
	}
	if metrics != nil {
		metrics.applyToGameRecord(&rec)
	}
	return rec
}

func seatCommanders(s EDHSeat) []game.SimpleCard {
	if len(s.Commanders) > 0 {
		return s.Commanders
	}
	if s.Commander != nil {
		return []game.SimpleCard{*s.Commander}
	}
	return nil
}

func seatCommanderNames(s EDHSeat) []string {
	commanders := seatCommanders(s)
	names := make([]string, 0, len(commanders))
	for _, c := range commanders {
		names = append(names, c.Name)
	}
	return names
}



// edhActionStateSnapshot returns a string representation of the current EDH game state
// excluding turn and phase numbers so it can be used to detect meaningful progress
// across individual phase actions.
func edhActionStateSnapshot(g *game.Game) string {
	var b strings.Builder
	for _, p := range g.GetPlayersRaw() {
		fmt.Fprintf(&b, "%s:L%d,lost:%v,Lib%d,Hand%d,BF%d,G%d,E%d;",
			p.GetName(), p.GetLifeTotal(), p.HasLost(),
			len(p.Library), len(p.Hand), len(p.Battlefield),
			len(p.Graveyard), len(p.Exile))
	}
	return b.String()
}
