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
	"runtime"
	"sort"
	"sync"
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
}

// SetSuggestedDeck sets the suggested deck to use in every pod
func (gr *EDHGameRunner) SetSuggestedDeck(seat *simulation.EDHSeat) {
	gr.suggestedDeckMu.Lock()
	defer gr.suggestedDeckMu.Unlock()
	gr.suggestedDeck = seat
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

		logger.LogMeta("Starting %d EDH pods...", count)

		// Load uploaded deck names from DB
		var uploadedNames []string
		if gr.db != nil {
			if names, err := gr.db.GetUploadedDeckNames(); err == nil {
				uploadedNames = names
			}
		}

		numWorkers := (runtime.NumCPU() * 4) / 5
		if numWorkers < 1 {
			numWorkers = 1
		}
		if numWorkers > 4 {
			numWorkers = 4
		}
		gamesChan := make(chan int, numWorkers)
		var wg sync.WaitGroup

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range gamesChan {
					gr.mu.Lock()
					pod := pickPodWithUploaded(gr.seats, gr.podSize, gr.rng, gr.mulligans, uploadedNames)
					gr.mu.Unlock()
					
					rec, err := simulation.SimulateEDHGame(simulation.EDHRunOptions{
						Seats: pod, MaxTurns: gr.maxTurns, RNG: rand.New(rand.NewSource(gr.rng.Int63())),
						RecordEvents: gr.replayDir != "",
					})
					if err != nil {
						logger.LogMeta("Pod skipped: %v", err)
						continue
					}
					// RecordGame and RecordCounts have their own internal locks;
					// only hold gr.mu for the non-thread-safe legacyResults.
					gr.mu.Lock()
					recordLegacy(gr.legacyRes, rec)
					gr.mu.Unlock()
					gr.edhResults.RecordGame(rec)
					for _, p := range rec.Players {
						for cName, perf := range p.CardStats {
							wins := 0
							if p.DeckName == rec.Winner {
								wins = perf.Casts
							}
							gr.cardLib.RecordCounts(cName, perf.Casts, wins)
						}
					}
					if gr.replayDir != "" {
						writeReplay(gr.replayDir, i+1, rec)
					}
					if (i+1)%10 == 0 {
						logger.LogMeta("Completed %d/%d pods", i+1, count)
					}
					// Yield so the dashboard server goroutines don't starve
					time.Sleep(1 * time.Millisecond)
				}
			}()
		}

		for i := 0; i < count; i++ {
			gamesChan <- i
		}
		close(gamesChan)

		wg.Wait()

		logger.LogMeta("Batch of %d pods completed!", count)

		gr.mu.Lock()
		gr.running = false
		gr.mu.Unlock()
	}()
}

// IsRunning returns whether games are currently running
func (gr *EDHGameRunner) IsRunning() bool {
	gr.mu.Lock()
	defer gr.mu.Unlock()
	return gr.running
}

// pickPodWithUploaded picks n random seats, ensuring at least one uploaded
// deck is included when the uploaded list is non-empty, then shuffles seats.
func pickPodWithUploaded(seats []simulation.EDHSeat, n int, rng *rand.Rand, mulligans int, uploaded []string) []simulation.EDHSeat {
	pod := pickPod(seats, n, rng, mulligans)
	if len(uploaded) > 0 && !containsAny(pod, uploaded) {
		uName := uploaded[rng.Intn(len(uploaded))]
		for _, s := range seats {
			if s.DeckName == uName {
				idx := rng.Intn(n)
				pod[idx] = s
				pod[idx].Mulligans = mulligans
				break
			}
		}
	}
	rng.Shuffle(n, func(i, j int) { pod[i], pod[j] = pod[j], pod[i] })
	return pod
}

func containsAny(pod []simulation.EDHSeat, names []string) bool {
	for _, s := range pod {
		for _, n := range names {
			if s.DeckName == n {
				return true
			}
		}
	}
	return false
}

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
	sideboardVariants := flag.Int("sideboard-variants", 0, "Generated sideboard variants per imported deck (0 = disabled)")
	sideboardSwaps := flag.Int("sideboard-swaps", 3, "Cards swapped per generated sideboard variant")
	cardStatsFlag := flag.String("card-stats", "card_library.json", "Path to a JSON file for persistent global card stats (loads existing, merges new, saves on exit)")
	dbPath := flag.String("db", "", "PostgreSQL DSN for persistent results (empty = disabled)")
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
		defer db.Close()
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

	if *port > 0 {
		startDashboard(legacyResults, edhResults, cardLib, &mu, *port, implReport, seats, cardDB, *podSize, *maxTurns, *mulligans, *replayDir, rng, db)
	}
	logger.LogMeta("Starting %d %d-player pods (seed=%d)...", *games, *podSize, rngSeed)
	start := time.Now()

	if *replayDir != "" {
		if err := os.MkdirAll(*replayDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "cannot create replay dir %q: %v\n", *replayDir, err)
			os.Exit(1)
		}
	}

	// Cap CPU at ~20% of available cores, max 4 workers
	numWorkers := (runtime.NumCPU()) / 5
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > 4 {
		numWorkers = 4
	}
	gamesChan := make(chan int, numWorkers)
	var wg sync.WaitGroup

	// Load uploaded deck names for the initial batch
	var uploadedNames []string
	if db != nil {
		if names, err := db.GetUploadedDeckNames(); err == nil {
			uploadedNames = names
		}
	}

	// Start worker goroutines
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range gamesChan {
				mu.Lock()
				pod := pickPodWithUploaded(seats, *podSize, rng, *mulligans, uploadedNames)
				mu.Unlock()
				rec, err := simulation.SimulateEDHGame(simulation.EDHRunOptions{
					Seats: pod, MaxTurns: *maxTurns, RNG: rand.New(rand.NewSource(rng.Int63())),
					RecordEvents: *replayDir != "",
				})
				if err != nil {
					logger.LogMeta("Pod %d skipped: %v", i+1, err)
					continue
				}
				// RecordGame and RecordCounts have their own internal locks;
				// only hold the external mu for the non-thread-safe legacyResults.
				mu.Lock()
				recordLegacy(legacyResults, rec)
				mu.Unlock()
				edhResults.RecordGame(rec)
				for _, p := range rec.Players {
					for cName, perf := range p.CardStats {
						wins := 0
						if p.DeckName == rec.Winner {
							wins = perf.Casts
						}
						cardLib.RecordCounts(cName, perf.Casts, wins)
					}
				}
				if db != nil {
					persistEDHPod(db, rec)
				}
				if *replayDir != "" {
					writeReplay(*replayDir, i+1, rec)
				}
				if (i+1)%10 == 0 {
					logger.LogMeta("Completed %d/%d pods", i+1, *games)
				}
				// Brief yield so the dashboard (and other goroutines) get scheduling time
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	// Send game indices to workers
	for i := 0; i < *games; i++ {
		gamesChan <- i
	}
	close(gamesChan)

	// Wait for all workers to complete
	wg.Wait()

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
		*games, elapsed.Seconds(), edhResults.AverageTurns())

	printSummary(edhResults, cardLib)

	// Cooldown before exit so Docker's restart doesn't immediately respawn us
	// and starve the dashboard of CPU time.
	logger.LogMeta("Cooling down for 10s before next batch...")
	time.Sleep(10 * time.Second)

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

func startDashboard(legacy *simulation.Results, edh *simulation.EDHResults, cardLib *stats.CardLibrary, mu *sync.RWMutex, port int, implReport *card.ImplementationReport, seats []simulation.EDHSeat, cardDB *card.CardDB, podSize, maxTurns, mulligans int, replayDir string, rng *rand.Rand, db *database.DB) {
	server := dashboard.NewServer(func() []simulation.Result {
		mu.RLock()
		defer mu.RUnlock()
		return legacy.GetResults()
	}, port)
	server.SetEDHProvider(func() []simulation.EDHDeckStats {
		mu.RLock()
		defer mu.RUnlock()
		return edh.DeckStats()
	})
	server.SetEDHSummaryProvider(func() simulation.EDHSummary {
		mu.RLock()
		defer mu.RUnlock()
		return edh.Summary()
	})
	server.SetEDHGamesProvider(func() []simulation.EDHGameRecord {
		mu.RLock()
		defer mu.RUnlock()
		return edh.RecentGames(10)
	})
	server.SetCardLibraryProvider(func() map[string]stats.GlobalCardStats {
		return cardLib.Snapshot()
	})
	server.SetImplementationReportProvider(func() *card.ImplementationReport {
		return implReport
	})
	server.SetCardDB(cardDB)

	// Wire upload recorder to persist deck names to database
	if db != nil {
		server.SetUploadRecorder(func(name string) {
			if err := db.RecordUploadedDeck(name); err != nil {
				logger.LogMeta("Failed to record uploaded deck: %v", err)
			}
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

	go func() {
		if err := server.Start(); err != nil {
			logger.LogMeta("Dashboard error: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("\n🧙 MTGSim EDH Dashboard available at: http://localhost:%d\n\n", port)
}
