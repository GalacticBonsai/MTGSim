// MTGSim-EDH — headless multiplayer Commander simulator.
//
// Loads a directory of (optionally) Commander-formatted deck files, runs
// N-player pods through the EDH rules engine, and exposes the aggregate
// results both on stdout and through the existing dashboard server.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/dashboard"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/pkg/simulation"
)

func main() {
	games := flag.Int("games", 50, "Number of pods to simulate")
	podSize := flag.Int("pod", 4, "Players per pod (2-6)")
	decksDir := flag.String("decks", "decks", "Directory containing Commander deck files")
	maxTurns := flag.Int("max-turns", 50, "Hard turn limit per pod")
	mulligans := flag.Int("mulligans", 0, "Mulligans every player takes before the game")
	port := flag.Int("port", 8080, "Dashboard port (0 to disable)")
	keepAlive := flag.Bool("keep-alive", true, "Keep the dashboard server running after games finish")
	logLevel := flag.String("log", "META", "Log level (META, GAME, PLAYER, CARD)")
	seed := flag.Int64("seed", 0, "RNG seed (0 = time-based)")
	replayDir := flag.String("replay", "", "Directory to write per-pod replay JSON (empty = disabled)")
	flag.Parse()

	if *podSize < 2 || *podSize > 6 {
		fmt.Fprintln(os.Stderr, "pod size must be in [2,6]")
		os.Exit(1)
	}

	logger.SetLogLevel(logger.ParseLogLevel(*logLevel))

	logger.LogMeta("Loading card database...")
	cardDB, err := card.LoadCardDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading card database: %v\n", err)
		os.Exit(1)
	}
	logger.LogMeta("Card database loaded with %d cards", cardDB.Size())

	deckFiles, err := simulation.GetDecks(*decksDir)
	if err != nil || len(deckFiles) < *podSize {
		fmt.Fprintf(os.Stderr, "Need at least %d deck files in %s\n", *podSize, *decksDir)
		os.Exit(1)
	}
	logger.LogMeta("Found %d deck files in %s", len(deckFiles), *decksDir)

	seats := loadSeats(deckFiles, cardDB)
	if len(seats) < *podSize {
		fmt.Fprintf(os.Stderr, "Only %d deck files were importable; need %d\n", len(seats), *podSize)
		os.Exit(1)
	}

	edhResults := simulation.NewEDHResults()
	legacyResults := simulation.NewResults()
	var mu sync.Mutex

	if *port > 0 {
		startDashboard(legacyResults, edhResults, &mu, *port)
	}

	rngSeed := *seed
	if rngSeed == 0 {
		rngSeed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(rngSeed))
	logger.LogMeta("Starting %d %d-player pods (seed=%d)...", *games, *podSize, rngSeed)
	start := time.Now()

	if *replayDir != "" {
		if err := os.MkdirAll(*replayDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "cannot create replay dir %q: %v\n", *replayDir, err)
			os.Exit(1)
		}
	}

	for i := 0; i < *games; i++ {
		pod := pickPod(seats, *podSize, rng, *mulligans)
		rec, err := simulation.SimulateEDHGame(simulation.EDHRunOptions{
			Seats: pod, MaxTurns: *maxTurns, RNG: rand.New(rand.NewSource(rng.Int63())),
			RecordEvents: *replayDir != "",
		})
		if err != nil {
			logger.LogMeta("Pod %d skipped: %v", i+1, err)
			continue
		}
		mu.Lock()
		edhResults.RecordGame(rec)
		recordLegacy(legacyResults, rec)
		mu.Unlock()
		if *replayDir != "" {
			writeReplay(*replayDir, i+1, rec)
		}
		if (i+1)%10 == 0 {
			logger.LogMeta("Completed %d/%d pods", i+1, *games)
		}
	}

	elapsed := time.Since(start)
	logger.LogMeta("Simulated %d pods in %.2fs (avg %.2f turns/pod)",
		*games, elapsed.Seconds(), edhResults.AverageTurns())

	printSummary(edhResults)

	if *port > 0 && *keepAlive {
		fmt.Printf("\nDashboard still running on http://localhost:%d (press Ctrl+C to exit)\n", *port)
		select {}
	}
}

func loadSeats(deckFiles []string, cardDB *card.CardDB) []simulation.EDHSeat {
	out := make([]simulation.EDHSeat, 0, len(deckFiles))
	for _, path := range deckFiles {
		seat, err := loadEDHSeat(path, cardDB)
		if err != nil {
			logger.LogMeta("Skipping %s: %v", path, err)
			continue
		}
		out = append(out, seat)
	}
	return out
}

// loadEDHSeat imports a deck file as a runner seat. If the file declares
// a commander it is registered; otherwise the player is seated at 40
// life with no commander but the rest of the EDH plumbing still applies.
func loadEDHSeat(path string, cardDB deck.CardDatabase) (simulation.EDHSeat, error) {
	if cmdr, main, err := deck.ImportCommanderDeckfile(path, cardDB); err == nil {
		c := toSimpleCard(cmdr)
		return simulation.EDHSeat{
			DeckPath: path, DeckName: main.Name,
			Library:   librarySimpleCards(main),
			Commander: &c,
		}, nil
	}
	main, _, err := deck.ImportDeckfile(path, cardDB)
	if err != nil {
		return simulation.EDHSeat{}, err
	}
	return simulation.EDHSeat{
		DeckPath: path, DeckName: main.Name,
		Library: librarySimpleCards(main),
	}, nil
}

func librarySimpleCards(d deck.Deck) []game.SimpleCard {
	out := make([]game.SimpleCard, len(d.Cards))
	for i, c := range d.Cards {
		out[i] = toSimpleCard(c)
	}
	return out
}

func toSimpleCard(c card.Card) game.SimpleCard {
	return game.SimpleCard{
		Name: c.Name, TypeLine: c.TypeLine, Power: c.Power,
		Toughness: c.Toughness, OracleText: c.OracleText, Colors: c.Colors,
	}
}

func pickPod(seats []simulation.EDHSeat, n int, rng *rand.Rand, mulligans int) []simulation.EDHSeat {
	idxs := rng.Perm(len(seats))[:n]
	pod := make([]simulation.EDHSeat, n)
	for i, j := range idxs {
		s := seats[j]
		s.Mulligans = mulligans
		pod[i] = s
	}
	return pod
}

func recordLegacy(r *simulation.Results, rec simulation.EDHGameRecord) {
	for _, p := range rec.Players {
		if p.DeckName == rec.Winner {
			r.AddWin(p.DeckName)
		} else {
			r.AddLoss(p.DeckName)
		}
	}
}

func printSummary(r *simulation.EDHResults) {
	logger.LogMeta("=== EDH Results ===")
	for _, s := range r.DeckStats() {
		logger.LogMeta("Deck %-30s G:%-3d W:%-3d L:%-3d WR:%5.1f%%  AvgLife:%5.1f  CmdrDmgKO:%d",
			s.DeckName, s.Games, s.Wins, s.Losses, s.WinRate, s.AvgFinalLife, s.CommanderDamageKOs)
	}
}

// writeReplay serializes a single pod's record (including the event log
// when present) to JSON. The file name encodes the pod index so a batch
// run produces stable, deterministic output names.
func writeReplay(dir string, podIndex int, rec simulation.EDHGameRecord) {
	path := filepath.Join(dir, fmt.Sprintf("pod-%04d.json", podIndex))
	bs, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		logger.LogMeta("replay marshal pod %d: %v", podIndex, err)
		return
	}
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		logger.LogMeta("replay write pod %d: %v", podIndex, err)
	}
}

func startDashboard(legacy *simulation.Results, edh *simulation.EDHResults, mu *sync.Mutex, port int) {
	server := dashboard.NewServer(func() []simulation.Result {
		mu.Lock()
		defer mu.Unlock()
		return legacy.GetResults()
	}, port)
	server.SetEDHProvider(func() []simulation.EDHDeckStats {
		mu.Lock()
		defer mu.Unlock()
		return edh.DeckStats()
	})
	go func() {
		if err := server.Start(); err != nil {
			logger.LogMeta("Dashboard error: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("\n🧙 MTGSim EDH Dashboard available at: http://localhost:%d\n\n", port)
}
