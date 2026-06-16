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
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/dashboard"
	"github.com/mtgsim/mtgsim/pkg/database"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/pkg/scryfall"
	"github.com/mtgsim/mtgsim/pkg/simulation"
	"github.com/mtgsim/mtgsim/pkg/stats"
)

// EDHGameRunner manages running EDH games and updating results
type EDHGameRunner struct {
	seats          []simulation.EDHSeat
	uploadedSeats  []simulation.EDHSeat
	uploadedMu     sync.Mutex
	cardDB         *card.CardDB
	db             *database.DB
	edhResults     *simulation.EDHResults
	legacyRes      *simulation.Results
	cardLib        *stats.CardLibrary
	mu             *sync.RWMutex
	rng            *rand.Rand
	running        bool
	podSize        int
	maxTurns       int
	mulligans      int
	replayDir      string
	suggestedDeck   *simulation.EDHSeat
	suggestedDeckMu sync.Mutex
	gameLogBuffer   []simulation.EDHGameRecord
	gameLogMu       sync.Mutex
}

// SetSuggestedDeck sets the suggested deck to use in every pod
func (gr *EDHGameRunner) SetSuggestedDeck(seat *simulation.EDHSeat) {
	gr.suggestedDeckMu.Lock()
	defer gr.suggestedDeckMu.Unlock()
	gr.suggestedDeck = seat
	if seat != nil && seat.DeckName != "" {
		gr.uploadedMu.Lock()
		found := false
		for _, s := range gr.uploadedSeats {
			if s.DeckName == seat.DeckName {
				found = true
				break
			}
		}
		if !found {
			gr.uploadedSeats = append(gr.uploadedSeats, *seat)
		}
		gr.uploadedMu.Unlock()
	}
}

// ResetCardLibrary clears the in-memory card stats library.
func (gr *EDHGameRunner) ResetCardLibrary() {
	gr.cardLib.Reset()
	logger.LogMeta("Card library reset by user")
}

// ResetGameLogs clears the in-memory game log ring buffer.
func (gr *EDHGameRunner) ResetGameLogs() {
	gr.gameLogMu.Lock()
	gr.gameLogBuffer = nil
	gr.gameLogMu.Unlock()
	gr.edhResults.Clear()
	logger.LogMeta("Game logs reset by user")
}

// RunGames runs the specified number of EDH pods
func (gr *EDHGameRunner) RunGames(count int) {
	go func() {
		gr.mu.Lock()
		if gr.running {
			gr.mu.Unlock()
			logger.LogMeta("Games already running")
			return
		}
		gr.running = true
		gr.mu.Unlock()

		gr.runBatch(count)

		gr.mu.Lock()
		gr.running = false
		gr.mu.Unlock()
	}()
}

func (gr *EDHGameRunner) runBatch(count int) {
	logger.LogMeta("Starting %d EDH pods...", count)

	// Load uploaded deck names from DB
	var uploadedNames []string
	if gr.db != nil {
		if names, err := gr.db.GetUploadedDeckNames(); err == nil {
			uploadedNames = names
		}
	}
	uploadedRRIdx := 0

	// Merge uploaded seats with filesystem seats for pod picking
	gr.uploadedMu.Lock()
	allSeats := make([]simulation.EDHSeat, 0, len(gr.seats)+len(gr.uploadedSeats))
	allSeats = append(allSeats, gr.seats...)
	allSeats = append(allSeats, gr.uploadedSeats...)
	gr.uploadedMu.Unlock()

	numWorkers := (runtime.NumCPU()) / 2
	if numWorkers < 1 {
		numWorkers = 1
	}
	gamesChan := make(chan int, numWorkers)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range gamesChan {
				gr.suggestedDeckMu.Lock()
				sd := gr.suggestedDeck
				gr.suggestedDeckMu.Unlock()
				var chosen string
				if sd != nil {
					chosen = sd.DeckName
				} else if len(uploadedNames) > 0 {
					chosen = uploadedNames[uploadedRRIdx%len(uploadedNames)]
					uploadedRRIdx++
				}
				pod := gr.pickPodWithUploaded(allSeats, gr.podSize, gr.rng, gr.mulligans, chosen)

			rec, err := simulation.SimulateEDHGame(simulation.EDHRunOptions{
				Seats: pod, MaxTurns: gr.maxTurns, RNG: rand.New(rand.NewSource(gr.rng.Int63())),
				RecordEvents: true,
			})
				if err != nil {
					logger.LogMeta("Pod skipped: %v", err)
					continue
				}
			gr.mu.Lock()
			recordLegacy(gr.legacyRes, rec)
			gr.mu.Unlock()
			gr.edhResults.RecordGame(rec)
			gr.gameLogMu.Lock()
			gr.gameLogBuffer = append(gr.gameLogBuffer, rec)
			if len(gr.gameLogBuffer) > 100 {
				gr.gameLogBuffer = gr.gameLogBuffer[len(gr.gameLogBuffer)-100:]
			}
			gr.gameLogMu.Unlock()
				for _, p := range rec.Players {
					for cName, perf := range p.CardStats {
						wins := 0
						if p.DeckName == rec.Winner {
							wins = perf.Casts
						}
						gr.cardLib.RecordCounts(cName, perf.Casts, wins)
					}
				}
				if gr.db != nil {
					persistEDHPod(gr.db, rec)
				}
				if gr.replayDir != "" {
					writeReplay(gr.replayDir, i+1, rec)
				}
				if (i+1)%10 == 0 {
					logger.LogMeta("Completed %d/%d pods", i+1, count)
				}
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	for i := 0; i < count; i++ {
		gamesChan <- i
	}
	close(gamesChan)
	wg.Wait()

	if gr.cardLib != nil {
		_ = gr.cardLib.Save()
	}

	logger.LogMeta("Batch of %d pods completed!", count)
}

func (gr *EDHGameRunner) pickPodWithUploaded(seats []simulation.EDHSeat, n int, rng *rand.Rand, mulligans int, chosen string) []simulation.EDHSeat {
	return pickPodFromPool(seats, n, rng, mulligans, chosen)
}

func pickPodFromPool(seats []simulation.EDHSeat, n int, rng *rand.Rand, mulligans int, chosen string) []simulation.EDHSeat {
	pod := make([]simulation.EDHSeat, 0, n)
	used := make(map[string]bool)

	// Place the chosen uploaded deck
	if chosen != "" {
		for _, s := range seats {
			if s.DeckName == chosen {
				s.Mulligans = mulligans
				pod = append(pod, s)
				used[chosen] = true
				break
			}
		}
	}

	// Fill remaining with random non-uploaded decks
	var pool []simulation.EDHSeat
	for _, s := range seats {
		if !used[s.DeckName] {
			pool = append(pool, s)
		}
	}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	needed := n - len(pod)
	if needed > len(pool) {
		needed = len(pool)
	}
	for _, s := range pool[:needed] {
		s.Mulligans = mulligans
		pod = append(pod, s)
	}

	rng.Shuffle(n, func(i, j int) { pod[i], pod[j] = pod[j], pod[i] })
	return pod
}

// IsRunning returns whether games are currently running
func (gr *EDHGameRunner) IsRunning() bool {
	gr.mu.Lock()
	defer gr.mu.Unlock()
	return gr.running
}

func main() {
	games := flag.Int("games", 0, "Number of pods to simulate (0 = continuous mode)")
	podSize := flag.Int("pod", 4, "Players per pod (2-6)")
	decksDir := flag.String("decks", "decks", "Directory containing Commander deck files")
	maxTurns := flag.Int("max-turns", 50, "Hard turn limit per pod")
	mulligans := flag.Int("mulligans", 0, "Mulligans every player takes before the game")
	port := flag.Int("port", 8080, "Dashboard port (0 to disable)")
	keepAlive := flag.Bool("keep-alive", true, "Keep the dashboard server running after games finish")
	logLevel := flag.String("log", "META", "Log level (META, GAME, PLAYER, CARD)")
	seed := flag.Int64("seed", 0, "RNG seed (0 = time-based)")
	replayDir := flag.String("replay", "", "Directory to write per-pod replay JSON (empty = disabled)")
	sideboardVariants := flag.Int("sideboard-variants", 0, "Generated sideboard variants per imported deck (0 = disabled)")
	sideboardSwaps := flag.Int("sideboard-swaps", 3, "Cards swapped per generated sideboard variant")
	cardStatsFlag := flag.String("card-stats", "card_library.json", "Path to a JSON file for persistent global card stats (loads existing, merges new, saves on exit)")
	dbPath := flag.String("db", "", "PostgreSQL DSN for persistent results (empty = disabled)")
	workerMode := flag.Bool("worker", false, "Run in worker mode: poll simulation_jobs table instead of serving the dashboard")
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

	rngSeed := *seed
	if rngSeed == 0 {
		rngSeed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(rngSeed))

	seats := loadSeats(deckFiles, cardDB)
	if *sideboardVariants > 0 {
		before := len(seats)
		seats = simulation.ExpandSideboardVariants(seats, simulation.SideboardVariantOptions{
			VariantsPerDeck: *sideboardVariants,
			SwapsPerVariant: *sideboardSwaps,
			RNG:             rand.New(rand.NewSource(rng.Int63())),
		})
		logger.LogMeta("Generated %d sideboard variants (%d total seats)", len(seats)-before, len(seats))
	}
	if len(seats) < *podSize {
		fmt.Fprintf(os.Stderr, "Only %d decklists/variants were importable; need %d\n", len(seats), *podSize)
		os.Exit(1)
	}

	// Warn about unimplemented cards in loaded EDH decks
	implTracker := abil.NewImplementationTracker()
	warnedDecks := map[string]bool{}
	for _, seat := range seats {
		if warnedDecks[seat.DeckPath] {
			continue
		}
		warnedDecks[seat.DeckPath] = true
		var allNames []string
		for _, c := range seat.Library {
			allNames = append(allNames, c.Name)
		}
		for _, c := range seat.Sideboard {
			allNames = append(allNames, c.Name)
		}
		for _, c := range seat.Commanders {
			allNames = append(allNames, c.Name)
		}
		seen := map[string]bool{}
		var deckCards []card.Card
		for _, name := range allNames {
			if seen[name] {
				continue
			}
			seen[name] = true
			if c, ok := cardDB.GetCardByName(name); ok {
				deckCards = append(deckCards, c)
			}
		}
		_ = deckCards
		// unimpl := implTracker.CheckDeck(deckCards, cardDB)
		// for _, name := range unimpl {
		// 	logger.LogMeta("Warning: unimplemented card in %s: %s", seat.DeckPath, name)
		// }
	}
	var implReport *card.ImplementationReport
	if report, err := card.ComputeImplementationStatus(cardDB, implTracker); err == nil {
		logger.LogMeta("Implementation status: %d/%d cards (%.1f%%)", report.ImplementedCount, report.TotalCards, report.Percentage)
		_ = implTracker.Save()
		implReport = report
	}

	edhResults := simulation.NewEDHResults()
	legacyResults := simulation.NewResults()
	var mu sync.RWMutex

	var db *database.DB
	if *dbPath != "" {
		var err error
		db, err = database.Open(*dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = db.Close() }()
		logger.LogMeta("Database opened: %s", *dbPath)
	}

	cardLib, err := stats.LoadCardLibrary(*cardStatsFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading card stats library: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if *cardStatsFlag != "" {
			if err := cardLib.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving card stats library: %v\n", err)
			}
		}
	}()

	if *workerMode {
		runWorker(seats, cardDB, db, cardLib, rng, *podSize, *maxTurns, *mulligans, *replayDir)
		return
	}

	// Initialize Scryfall client for image enrichment.
	scryfallClient := scryfall.NewClient()
	imageCache := map[string]string{}

	// Enrich existing card library images from the card database.
	for name := range cardLib.Cards {
		if c, ok := cardDB.GetCardByName(name); ok && c.ImageURIs != nil && c.ImageURIs.Normal != "" {
			cardLib.SetImageURL(name, c.ImageURIs.Normal)
			imageCache[name] = c.ImageURIs.Normal
			continue
		}
		cd, err := scryfallClient.GetCardByName(name)
		if err == nil && cd.ImageURIs != nil && cd.ImageURIs.Normal != "" {
			cardLib.SetImageURL(name, cd.ImageURIs.Normal)
			imageCache[name] = cd.ImageURIs.Normal
		}
	}

	var gameRunner *EDHGameRunner
	if *port > 0 {
		gameRunner = startDashboard(legacyResults, edhResults, cardLib, &mu, *port, implReport, seats, cardDB, *podSize, *maxTurns, *mulligans, *replayDir, rng, db, scryfallClient)
	}
	if gameRunner == nil {
		gameRunner = &EDHGameRunner{
			seats:      seats,
			cardDB:     cardDB,
			db:         db,
			edhResults: edhResults,
			legacyRes:  legacyResults,
			cardLib:    cardLib,
			mu:         &mu,
			rng:        rng,
			podSize:    *podSize,
			maxTurns:   *maxTurns,
			mulligans:  *mulligans,
			replayDir:  *replayDir,
		}
	}
	batchSize := *games
	if batchSize <= 0 {
		batchSize = 50
	}

	if db != nil {
		if sum, err := db.GetEDHSummary(); err == nil {
			logger.LogMeta("Database connected: %d existing pods persisted", sum.TotalGames)
		}
		if decks, err := db.GetEDHDeckStats(); err == nil {
			totalCards := 0
			for _, d := range decks {
				totalCards += len(d.CardStats)
			}
			logger.LogMeta("Database: %d decks, %d card-stat entries", len(decks), totalCards)
		}
	} else {
		logger.LogMeta("No database configured — results will NOT persist across restarts")
	}
	logger.LogMeta("Starting %d %d-player pods (seed=%d)...", batchSize, *podSize, rngSeed)
	start := time.Now()

	if *replayDir != "" {
		if err := os.MkdirAll(*replayDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "cannot create replay dir %q: %v\n", *replayDir, err)
			os.Exit(1)
		}
	}

	gameRunner.RunGames(batchSize)
	time.Sleep(100 * time.Millisecond)
	for gameRunner.IsRunning() {
		time.Sleep(200 * time.Millisecond)
	}

	// Backfill image URLs for any newly recorded cards.
	for name := range cardLib.Cards {
		if _, ok := imageCache[name]; ok {
			continue
		}
		var imageURL string
		if c, ok := cardDB.GetCardByName(name); ok && c.ImageURIs != nil && c.ImageURIs.Normal != "" {
			imageURL = c.ImageURIs.Normal
		} else {
			cd, err := scryfallClient.GetCardByName(name)
			if err == nil && cd.ImageURIs != nil && cd.ImageURIs.Normal != "" {
				imageURL = cd.ImageURIs.Normal
			}
		}

		// Download and cache the image if we have a URL
		if imageURL != "" {
			// Store URL first for web display
			cardLib.SetImageURL(name, imageURL)
			imageCache[name] = imageURL
			if db != nil {
				_ = db.UpdateCardImageURL(name, imageURL)
			}

			// Download and cache the image file for offline use
			// Do this asynchronously to not block the main process
			go func(url string) {
				if _, err := scryfallClient.DownloadAndCacheImage(url); err != nil {
					// Image caching failed, but we still have the URL for fallback
					logger.LogMeta("Failed to cache image for card: %v", err)
				}
			}(imageURL)
		}
	}

	elapsed := time.Since(start)
	logger.LogMeta("Simulated %d pods in %.2fs (avg %.2f turns/pod)",
		batchSize, elapsed.Seconds(), edhResults.AverageTurns())

	printSummary(edhResults, cardLib)

	if *games == 0 {
		logger.LogMeta("Continuous mode: looping batches of %d pods", batchSize)
		for {
			time.Sleep(2 * time.Second)
			logger.LogMeta("Starting continuous batch...")
			gameRunner.RunGames(batchSize)
			for gameRunner.IsRunning() {
				time.Sleep(1 * time.Second)
			}
			logger.LogMeta("Continuous batch completed")
		}
	}

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
	if cmdrs, main, side, err := deck.ImportCommanderDeckfileWithCommanders(path, cardDB); err == nil {
		commanders := simpleCards(cmdrs)
		primary := commanders[0]
		return simulation.EDHSeat{
			DeckPath: path, DeckName: main.Name,
			Library: librarySimpleCards(main), Sideboard: librarySimpleCards(side),
			Commanders: commanders, Commander: &primary,
		}, nil
	}
	main, side, err := deck.ImportDeckfile(path, cardDB)
	if err != nil {
		return simulation.EDHSeat{}, err
	}
	return simulation.EDHSeat{
		DeckPath: path, DeckName: main.Name,
		Library: librarySimpleCards(main), Sideboard: librarySimpleCards(side),
	}, nil
}

func librarySimpleCards(d deck.Deck) []game.SimpleCard {
	return simpleCards(d.Cards)
}

func simpleCards(cards []card.Card) []game.SimpleCard {
	out := make([]game.SimpleCard, len(cards))
	for i, c := range cards {
		out[i] = toSimpleCard(c)
	}
	return out
}

func toSimpleCard(c card.Card) game.SimpleCard {
	return game.SimpleCard{
		Name: c.Name, TypeLine: c.TypeLine, Power: c.Power,
		Toughness: c.Toughness, OracleText: c.OracleText, Colors: c.Colors,
		ColorIdentity: c.ColorIdentity,
		ManaCost:      c.ManaCost,
	}
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

func printSummary(r *simulation.EDHResults, cardLib *stats.CardLibrary) {
	logger.LogMeta("=== EDH Results ===")
	for _, s := range r.DeckStats() {
		logger.LogMeta("Deck %-30s G:%-3d W:%-3d L:%-3d WR:%5.1f%%  AvgLife:%5.1f  CmdrDmgKO:%d",
			s.DeckName, s.Games, s.Wins, s.Losses, s.WinRate, s.AvgFinalLife, s.CommanderDamageKOs)
		type cardEntry struct {
			name    string
			casts   int
			wins    int
			winRate float64
		}
		var entries []cardEntry
		for cName, cp := range s.CardStats {
			if cp.Casts >= 5 {
				entries = append(entries, cardEntry{
					name: cName, casts: cp.Casts, wins: cp.Wins,
					winRate: 100 * float64(cp.Wins) / float64(cp.Casts),
				})
			}
		}
		if len(entries) == 0 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].winRate != entries[j].winRate {
				return entries[i].winRate > entries[j].winRate
			}
			return entries[i].casts > entries[j].casts
		})
		// for _, e := range entries {
		// 	logger.LogMeta("    Card %-26s C:%-3d W:%-3d WR:%5.1f%%", e.name, e.casts, e.wins, e.winRate)
		// }
	}
	// if len(cardLib.Cards) > 0 {
	// 	logger.LogMeta("=== Global Card Library ===")
	// 	for _, e := range cardLib.TopCards(5, 20) {
	// 		logger.LogMeta("%-40s C:%-5d W:%-5d WR:%5.1f%%", e.Name, e.Casts, e.Wins, e.WinRate)
	// 	}
	// }
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

func persistEDHPod(db *database.DB, rec simulation.EDHGameRecord) {
	pod := database.EDHPodRecord{
		TotalTurns:        rec.Turns,
		Winner:            rec.Winner,
		WinnerCondition:   string(rec.WinnerCondition),
		MaxStormCount:     rec.MaxStormCount,
		TotalManaSpent:    rec.TotalManaSpent,
		TotalManaProduced: rec.TotalManaProduced,
		TotalCardsPlayed:  rec.TotalCardsPlayed,
		TotalCombatDamage: rec.TotalCombatDamage,
		TotalEliminations: rec.TotalEliminations,
	}
	players := make([]database.EDHPlayerRecord, len(rec.Players))
	cardStats := make(map[string]map[string]struct{ Casts, Wins int })
	for i, p := range rec.Players {
		players[i] = database.EDHPlayerRecord{
			DeckName:       p.DeckName,
			CommanderName:  p.CommanderName,
			Mulligans:      p.Mulligans,
			FinalLife:      p.FinalLife,
			CommanderCasts: p.CommanderCasts,
			CardsPlayed:    p.CardsPlayed,
			LandsPlayed:    p.LandsPlayed,
			SpellsCast:     p.SpellsCast,
			CreaturesCast:  p.CreaturesCast,
			ManaSpent:      p.ManaSpent,
			ManaProduced:   p.ManaProduced,
			CombatDamage:   p.CombatDamage,
			Eliminations:   p.Eliminations,
			MaxStormCount:  p.MaxStormCount,
			Eliminated:     p.Eliminated,
			KillSource:     string(p.KillSource),
		}
		cs := make(map[string]struct{ Casts, Wins int })
		for cName, perf := range p.CardStats {
			wins := 0
			if p.DeckName == rec.Winner {
				wins = perf.Casts
			}
			cs[cName] = struct{ Casts, Wins int }{Casts: perf.Casts, Wins: wins}
		}
		cardStats[p.DeckName] = cs
	}
	if err := db.RecordEDHPod(pod, players, cardStats); err != nil {
		logger.LogMeta("Database persist error: %v", err)
	}
}

// runWorker enters a loop polling simulation_jobs and running them.
func runWorker(seats []simulation.EDHSeat, cardDB *card.CardDB, db *database.DB, cardLib *stats.CardLibrary, rng *rand.Rand, podSize, maxTurns, mulligans int, replayDir string) {
	if db == nil {
		logger.LogMeta("Worker requires a database (-db flag)")
		os.Exit(1)
	}

	runner := &EDHGameRunner{
		seats:      seats,
		cardDB:     cardDB,
		db:         db,
		edhResults: simulation.NewEDHResults(),
		legacyRes:  simulation.NewResults(),
		cardLib:    cardLib,
		mu:         &sync.RWMutex{},
		rng:        rng,
		podSize:    podSize,
		maxTurns:   maxTurns,
		mulligans:  mulligans,
		replayDir:  replayDir,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	pollInterval := 5 * time.Second
	logger.LogMeta("Worker started, polling for jobs every %v", pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			logger.LogMeta("Worker shutting down...")
			_ = cardLib.Save()
			return
		case <-ticker.C:
			job, err := db.ClaimNextJob()
			if err != nil {
				logger.LogMeta("Worker claim error: %v", err)
				continue
			}
			if job == nil {
				continue
			}

			logger.LogMeta("Worker claimed job %d (config: %s)", job.ID, job.Config)

			var cfg struct {
				Count int `json:"count"`
			}
			if err := json.Unmarshal([]byte(job.Config), &cfg); err != nil || cfg.Count <= 0 {
				cfg.Count = 50
			}

			runner.RunGames(cfg.Count)
			for runner.IsRunning() {
				time.Sleep(time.Second)
			}

			summary := fmt.Sprintf("Completed %d pods", cfg.Count)
			if err := db.CompleteJob(job.ID, summary); err != nil {
				logger.LogMeta("Worker error completing job %d: %v", job.ID, err)
			} else {
				logger.LogMeta("Worker completed job %d: %s", job.ID, summary)
			}

			if err := cardLib.Save(); err != nil {
				logger.LogMeta("Worker error saving card library: %v", err)
			}
		}
	}
}

func startDashboard(legacy *simulation.Results, edh *simulation.EDHResults, cardLib *stats.CardLibrary, mu *sync.RWMutex, port int, implReport *card.ImplementationReport, seats []simulation.EDHSeat, cardDB *card.CardDB, podSize, maxTurns, mulligans int, replayDir string, rng *rand.Rand, db *database.DB, scryfallClient *scryfall.Client) *EDHGameRunner {
	server := dashboard.NewServer(func() []simulation.Result {
		mu.RLock()
		defer mu.RUnlock()
		return legacy.GetResults()
	}, port)
	server.SetEDHProvider(func() []simulation.EDHDeckStats {
		if db != nil {
			s, err := db.GetEDHDeckStats()
			if err != nil {
				logger.LogMeta("DB EDH query error: %v", err)
				return nil
			}
			out := make([]simulation.EDHDeckStats, len(s))
			for i, d := range s {
				out[i] = simulation.EDHDeckStats{
					DeckName:            d.DeckName,
					CommanderName:       d.CommanderName,
					Games:               d.Games,
					Wins:                d.Wins,
					Losses:              d.Losses,
					WinRate:             d.WinRate,
					AvgFinalLife:        d.AvgFinalLife,
					AvgMulligans:        d.AvgMulligans,
					CommanderDamageKOs:  d.CommanderDamageKOs,
					LifeLossKOs:         d.LifeLossKOs,
					MillKOs:             d.MillKOs,
					DeckoutKOs:          d.DeckoutKOs,
					EffectKOs:           d.EffectKOs,
					CombatWins:          d.CombatWins,
					EffectWins:          d.EffectWins,
					DeckoutWins:         d.DeckoutWins,
					AvgCommanderCasts:   d.AvgCommanderCasts,
					AvgManaSpent:        d.AvgManaSpent,
					AvgManaProduced:     d.AvgManaProduced,
					AvgCardsPlayed:      d.AvgCardsPlayed,
					AvgLandsPlayed:      d.AvgLandsPlayed,
					AvgSpellsCast:       d.AvgSpellsCast,
					AvgCreaturesCast:    d.AvgCreaturesCast,
					AvgCombatDamage:     d.AvgCombatDamage,
					MaxStormCount:       d.MaxStormCount,
					TotalManaSpent:      d.TotalManaSpent,
					TotalManaProduced:   d.TotalManaProduced,
					TotalCardsPlayed:    d.TotalCardsPlayed,
					TotalCombatDamage:   d.TotalCombatDamage,
					Eliminations:        d.Eliminations,
					CardStats:           map[string]simulation.CardPerformance{},
				}
				for k, v := range d.CardStats {
					out[i].CardStats[k] = simulation.CardPerformance{Casts: v.Casts, Wins: v.Wins}
				}
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].WinRate != out[j].WinRate {
					return out[i].WinRate > out[j].WinRate
				}
				if out[i].Games != out[j].Games {
					return out[i].Games > out[j].Games
				}
				return out[i].DeckName < out[j].DeckName
			})
			return out
		}
		mu.RLock()
		defer mu.RUnlock()
		return edh.DeckStats()
	})
	server.SetEDHSummaryProvider(func() simulation.EDHSummary {
		if db != nil {
			s, err := db.GetEDHSummary()
			if err != nil {
				logger.LogMeta("DB summary error: %v", err)
				return simulation.EDHSummary{}
			}
			return simulation.EDHSummary{
				TotalGames:           s.TotalGames,
				AverageTurns:         s.AverageTurns,
				TotalManaSpent:       s.TotalManaSpent,
				AverageManaSpent:     s.AverageManaSpent,
				TotalManaProduced:    s.TotalManaProduced,
				AverageManaProduced:  s.AverageManaProduced,
				TotalCardsPlayed:     s.TotalCardsPlayed,
				AverageCardsPlayed:   s.AverageCardsPlayed,
				HighestStormCount:    s.HighestStormCount,
				TotalCombatDamage:    s.TotalCombatDamage,
				TotalEliminations:    s.TotalEliminations,
				AverageEliminations:  s.AverageEliminations,
				AverageCombatDamage:  s.AverageCombatDamage,
			}
		}
		mu.RLock()
		defer mu.RUnlock()
		return edh.Summary()
	})
	server.SetEDHGamesProvider(func() []simulation.EDHGameRecord {
		if db != nil {
			pods, err := db.GetRecentEDHPods(10)
			if err != nil {
				logger.LogMeta("DB pods error: %v", err)
				return nil
			}
			out := make([]simulation.EDHGameRecord, len(pods))
			for i, p := range pods {
				out[i] = simulation.EDHGameRecord{
					Turns:             p.TotalTurns,
					Winner:            p.Winner,
					WinnerCondition:   simulation.WinCondition(p.WinnerCondition),
					MaxStormCount:     p.MaxStormCount,
					TotalManaSpent:    p.TotalManaSpent,
					TotalManaProduced: p.TotalManaProduced,
					TotalCardsPlayed:  p.TotalCardsPlayed,
					TotalCombatDamage: p.TotalCombatDamage,
					TotalEliminations: p.TotalEliminations,
					Players:           make([]simulation.EDHPlayerRecord, len(p.Players)),
				}
				for j, pl := range p.Players {
					out[i].Players[j] = simulation.EDHPlayerRecord{
						DeckName:      pl.DeckName,
						CommanderName: pl.CommanderName,
						FinalLife:     pl.FinalLife,
						Eliminated:    pl.Eliminated,
						KillSource:    simulation.KillSource(pl.KillSource),
						ManaSpent:     pl.ManaSpent,
						ManaProduced:  pl.ManaProduced,
						CardsPlayed:   pl.CardsPlayed,
						CombatDamage:  pl.CombatDamage,
						Eliminations:  pl.Eliminations,
					}
				}
			}
			return out
		}
		mu.RLock()
		defer mu.RUnlock()
		return edh.RecentGames(10)
	})
	server.SetCardLibraryProvider(func() map[string]stats.GlobalCardStats {
		if db != nil {
			cards, err := db.GetGlobalCardStats()
			if err != nil {
				logger.LogMeta("DB card stats error: %v", err)
				return nil
			}
			out := make(map[string]stats.GlobalCardStats, len(cards))
			for _, c := range cards {
				out[c.CardName] = stats.GlobalCardStats{
					Casts: c.Casts, Wins: c.Wins, WinRate: c.WinRate, ImageURL: c.ImageURL,
				}
			}
			return out
		}
		return cardLib.Snapshot()
	})
	server.SetImplementationReportProvider(func() *card.ImplementationReport {
		return implReport
	})
	server.SetCardDB(cardDB)
	server.SetScryfallClient(scryfallClient)

	// Wire job creator and status provider when using a database.
	if db != nil {
		server.SetJobCreator(func(count int) (int64, error) {
			cfg, _ := json.Marshal(map[string]int{"count": count})
			return db.CreateJob(string(cfg))
		})
		server.SetJobStatusProvider(func() (int, int) {
			pending, running, err := db.GetJobCounts()
			if err != nil {
				return 0, 0
			}
			return pending, running
		})
	}

	// Wire upload/delete recorders to persist deck names to database
	if db != nil {
		server.SetUploadRecorder(func(name string) {
			if err := db.RecordUploadedDeck(name); err != nil {
				logger.LogMeta("Failed to record uploaded deck: %v", err)
			}
		})
		server.SetDeleteRecorder(func(name string) {
			if err := db.DeleteUploadedDeck(name); err != nil {
				logger.LogMeta("Failed to delete uploaded deck: %v", err)
			}
		})
		server.SetUploadedDecksProvider(func() []string {
			names, _ := db.GetUploadedDeckNames()
			return names
		})
	}

	// Create and set game runner
	gameRunner := &EDHGameRunner{
		seats:      seats,
		cardDB:     cardDB,
		db:         db,
		edhResults: edh,
		legacyRes:  legacy,
		cardLib:    cardLib,
		mu:         mu,
		rng:        rng,
		podSize:    podSize,
		maxTurns:   maxTurns,
		mulligans:  mulligans,
		replayDir:  replayDir,
	}
	server.SetGameRunner(gameRunner)
	server.SetDataResetter(gameRunner)

	server.SetGameLogProvider(func() ([]dashboard.GameLogEntry, func(id int) *simulation.EDHGameRecord) {
		gameRunner.gameLogMu.Lock()
		defer gameRunner.gameLogMu.Unlock()
		summaries := make([]dashboard.GameLogEntry, 0, len(gameRunner.gameLogBuffer))
		for i, rec := range gameRunner.gameLogBuffer {
			players := make([]string, len(rec.Players))
			for j, p := range rec.Players {
				players[j] = p.DeckName
			}
			summaries = append(summaries, dashboard.GameLogEntry{
				ID:          i,
				Winner:      rec.Winner,
				Turns:       rec.Turns,
				Players:     players,
				EventCount:  len(rec.Events),
				TotalMana:   rec.TotalManaSpent,
				TotalCards:  rec.TotalCardsPlayed,
				TotalCombat: rec.TotalCombatDamage,
			})
		}
		getGame := func(id int) *simulation.EDHGameRecord {
			gameRunner.gameLogMu.Lock()
			defer gameRunner.gameLogMu.Unlock()
			if id < 0 || id >= len(gameRunner.gameLogBuffer) {
				return nil
			}
			r := gameRunner.gameLogBuffer[id]
			return &r
		}
		return summaries, getGame
	})

	go func() {
		if err := server.Start(); err != nil {
			logger.LogMeta("Dashboard error: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("\n🧙 MTGSim EDH Dashboard available at: http://localhost:%d\n\n", port)
	return gameRunner
}
