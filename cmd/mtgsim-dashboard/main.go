// MTGSim Dashboard - serves a web dashboard for browsing simulation results.
// Runs games using a lightweight pkg/game loop and publishes wins/losses
// through pkg/simulation.Results to the dashboard server.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/dashboard"
	"github.com/mtgsim/mtgsim/pkg/database"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/pkg/simulation"
	"github.com/mtgsim/mtgsim/pkg/stats"
)

// GameRunner manages running games and updating results
type GameRunner struct {
	deckFiles       []string
	cardDB          *card.CardDB
	results         *simulation.Results
	mu              *sync.Mutex
	suggestedDeck   *simulation.EDHSeat
	suggestedDeckMu *sync.Mutex
	rng             *rand.Rand
	running         bool
	db              *database.DB
}

// RunGames runs the specified number of games
func (gr *GameRunner) RunGames(count int) {
	go func() {
		gr.mu.Lock()
		if gr.running {
			gr.mu.Unlock()
			logger.LogMeta("Games already running")
			return
		}
		gr.running = true
		gr.mu.Unlock()

		logger.LogMeta("Starting %d games...", count)

		for i := 0; i < count; i++ {
			d1Path := gr.deckFiles[gr.rng.Intn(len(gr.deckFiles))]
			d2Path := gr.deckFiles[gr.rng.Intn(len(gr.deckFiles))]
			for d2Path == d1Path && len(gr.deckFiles) > 1 {
				d2Path = gr.deckFiles[gr.rng.Intn(len(gr.deckFiles))]
			}

			m1, _, err1 := deck.ImportDeckfile(d1Path, gr.cardDB)
			m2, _, err2 := deck.ImportDeckfile(d2Path, gr.cardDB)
			if err1 != nil || err2 != nil || m1.Size() == 0 || m2.Size() == 0 {
				logger.LogMeta("Skipping game due to deck import error")
				continue
			}

			winner, loser := simulateGame(m1, m2)
			if winner != nil && loser != nil {
				gr.mu.Lock()
				gr.results.AddWin(winner.GetName())
				gr.results.AddLoss(loser.GetName())
				if gr.db != nil {
					_ = gr.db.Record1v1Game(winner.GetName(), loser.GetName(), winner.GetName(), 0)
				}
				gr.mu.Unlock()
			}

			if (i+1)%10 == 0 {
				logger.LogMeta("Completed %d/%d games", i+1, count)
			}
		}

		logger.LogMeta("Batch of %d games completed!", count)

		gr.mu.Lock()
		gr.running = false
		gr.mu.Unlock()
	}()
}

// IsRunning returns whether games are currently running
func (gr *GameRunner) IsRunning() bool {
	gr.mu.Lock()
	defer gr.mu.Unlock()
	return gr.running
}

// SetSuggestedDeck sets the suggested deck to be used for games
func (gr *GameRunner) SetSuggestedDeck(seat *simulation.EDHSeat) {
	gr.suggestedDeckMu.Lock()
	defer gr.suggestedDeckMu.Unlock()
	gr.suggestedDeck = seat
}

func main() {
	games := flag.Int("games", 50, "Number of games to simulate")
	decksDir := flag.String("decks", "decks/1v1", "Directory containing deck files")
	port := flag.Int("port", 8080, "Dashboard port")
	logLevel := flag.String("log", "META", "Log level (META, GAME, PLAYER, CARD)")
	keepAlive := flag.Bool("keep-alive", true, "Keep server running after simulation")
	dbPath := flag.String("db", "", "Path to SQLite database for persistent results (empty = disabled)")
	flag.Parse()

	logger.SetLogLevel(logger.ParseLogLevel(*logLevel))

	logger.LogMeta("Loading card database...")
	cardDB, err := card.LoadCardDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading card database: %v\n", err)
		os.Exit(1)
	}
	logger.LogMeta("Card database loaded with %d cards", cardDB.Size())

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

	deckFiles, err := simulation.GetDecks(*decksDir)
	if err != nil || len(deckFiles) < 2 {
		fmt.Fprintf(os.Stderr, "Need at least 2 deck files in %s\n", *decksDir)
		os.Exit(1)
	}
	logger.LogMeta("Found %d deck files", len(deckFiles))

	results := simulation.NewResults()
	var mu sync.Mutex

	server := dashboard.NewServer(func() []simulation.Result {
		if db != nil {
			r, err := db.Get1v1DeckStats()
			if err != nil {
				logger.LogMeta("DB query error: %v", err)
				return nil
			}
			return db1v1ToResults(r)
		}
		mu.Lock()
		defer mu.Unlock()
		return results.GetResults()
	}, *port)

	if db != nil {
		server.SetEDHProvider(func() []simulation.EDHDeckStats {
			r, err := db.GetEDHDeckStats()
			if err != nil {
				logger.LogMeta("DB query error: %v", err)
				return nil
			}
			return dbEDHToSimulation(r)
		})
		server.SetEDHSummaryProvider(func() simulation.EDHSummary {
			s, err := db.GetEDHSummary()
			if err != nil {
				logger.LogMeta("DB query error: %v", err)
				return simulation.EDHSummary{}
			}
			return dbSummaryToSimulation(s)
		})
		server.SetEDHGamesProvider(func() []simulation.EDHGameRecord {
			p, err := db.GetRecentEDHPods(10)
			if err != nil {
				logger.LogMeta("DB query error: %v", err)
				return nil
			}
			return dbPodsToSimulation(p)
		})
		server.SetCardLibraryProvider(func() map[string]stats.GlobalCardStats {
			c, err := db.GetGlobalCardStats()
			if err != nil {
				logger.LogMeta("DB query error: %v", err)
				return nil
			}
			return dbCardsToStats(c)
		})
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create game runner that can be triggered via API
	gameRunner := &GameRunner{
		deckFiles:       deckFiles,
		cardDB:          cardDB,
		results:         results,
		mu:              &mu,
		suggestedDeckMu: &sync.Mutex{},
		rng:             rng,
		db:              db,
	}

	server.SetGameRunner(gameRunner)
	server.SetCardDB(cardDB)
	go func() {
		if err := server.Start(); err != nil {
			logger.LogMeta("Dashboard error: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("\n🧙 MTGSim Dashboard available at: http://localhost:%d\n\n", *port)

	if db == nil {
		logger.LogMeta("Starting %d games...", *games)
		gameRunner.RunGames(*games)
		logger.LogMeta("Simulation completed!")
		results.PrintTopResults()
	} else {
		logger.LogMeta("Running in database-backed mode. Waiting for external runner data...")
	}

	if *keepAlive {
		fmt.Printf("\nDashboard still running on http://localhost:%d (press Ctrl+C to exit)\n", *port)
		select {}
	}
}

func db1v1ToResults(r []database.Deck1v1Stats) []simulation.Result {
	out := make([]simulation.Result, len(r))
	for i, d := range r {
		out[i] = simulation.Result{Name: d.Name, Wins: d.Wins, Losses: d.Losses}
	}
	return out
}

func dbEDHToSimulation(s []database.EDHDeckStats) []simulation.EDHDeckStats {
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
			AvgCardsPlayed:      d.AvgCardsPlayed,
			AvgLandsPlayed:      d.AvgLandsPlayed,
			AvgSpellsCast:       d.AvgSpellsCast,
			AvgCreaturesCast:    d.AvgCreaturesCast,
			AvgCombatDamage:     d.AvgCombatDamage,
			MaxStormCount:       d.MaxStormCount,
			TotalManaSpent:      d.TotalManaSpent,
			TotalCardsPlayed:    d.TotalCardsPlayed,
			TotalCombatDamage:   d.TotalCombatDamage,
			Eliminations:        d.Eliminations,
			CardStats:           map[string]simulation.CardPerformance{},
		}
		for k, v := range d.CardStats {
			out[i].CardStats[k] = simulation.CardPerformance{Casts: v.Casts, Wins: v.Wins}
		}
	}
	// Sort by win rate descending to match in-memory behavior
	sort.Slice(out, func(i, j int) bool {
		if out[i].WinRate != out[j].WinRate {
			return out[i].WinRate > out[j].WinRate
		}
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		if out[i].CommanderName != out[j].CommanderName {
			return out[i].CommanderName < out[j].CommanderName
		}
		return out[i].DeckName < out[j].DeckName
	})
	return out
}

func dbSummaryToSimulation(s database.EDHSummary) simulation.EDHSummary {
	return simulation.EDHSummary{
		TotalGames:          s.TotalGames,
		AverageTurns:        s.AverageTurns,
		TotalManaSpent:      s.TotalManaSpent,
		AverageManaSpent:    s.AverageManaSpent,
		TotalCardsPlayed:    s.TotalCardsPlayed,
		AverageCardsPlayed:  s.AverageCardsPlayed,
		HighestStormCount:   s.HighestStormCount,
		TotalCombatDamage:   s.TotalCombatDamage,
		TotalEliminations:   s.TotalEliminations,
		AverageEliminations: s.AverageEliminations,
		AverageCombatDamage: s.AverageCombatDamage,
	}
}

func dbPodsToSimulation(pods []database.EDHRecentPod) []simulation.EDHGameRecord {
	out := make([]simulation.EDHGameRecord, len(pods))
	for i, p := range pods {
		out[i] = simulation.EDHGameRecord{
			Turns:             p.TotalTurns,
			Winner:            p.Winner,
			WinnerCondition:   simulation.WinCondition(p.WinnerCondition),
			MaxStormCount:     p.MaxStormCount,
			TotalManaSpent:    p.TotalManaSpent,
			TotalCardsPlayed:  p.TotalCardsPlayed,
			TotalCombatDamage: p.TotalCombatDamage,
			TotalEliminations: p.TotalEliminations,
			Players:           make([]simulation.EDHPlayerRecord, len(p.Players)),
		}
		for j, pl := range p.Players {
			out[i].Players[j] = simulation.EDHPlayerRecord{
				DeckName:       pl.DeckName,
				CommanderName:  pl.CommanderName,
				FinalLife:      pl.FinalLife,
				Eliminated:     pl.Eliminated,
				KillSource:     simulation.KillSource(pl.KillSource),
				ManaSpent:      pl.ManaSpent,
				CardsPlayed:    pl.CardsPlayed,
				CombatDamage:   pl.CombatDamage,
				Eliminations:   pl.Eliminations,
			}
		}
	}
	return out
}

func dbCardsToStats(cards []database.GlobalCardStats) map[string]stats.GlobalCardStats {
	out := make(map[string]stats.GlobalCardStats, len(cards))
	for _, c := range cards {
		out[c.CardName] = stats.GlobalCardStats{
			Casts:    c.Casts,
			Wins:     c.Wins,
			WinRate:  c.WinRate,
			ImageURL: c.ImageURL,
		}
	}
	return out
}

func simulateGame(d1, d2 deck.Deck) (winner, loser *game.Player) {
	p1 := game.NewPlayer(d1.Name, 20)
	p2 := game.NewPlayer(d2.Name, 20)

	for _, c := range d1.Cards {
		p1.Library = append(p1.Library, game.SimpleCard{
			Name: c.Name, TypeLine: c.TypeLine, Power: c.Power,
			Toughness: c.Toughness, OracleText: c.OracleText, Colors: c.Colors,
		})
	}
	for _, c := range d2.Cards {
		p2.Library = append(p2.Library, game.SimpleCard{
			Name: c.Name, TypeLine: c.TypeLine, Power: c.Power,
			Toughness: c.Toughness, OracleText: c.OracleText, Colors: c.Colors,
		})
	}

	rand.Shuffle(len(p1.Library), func(i, j int) { p1.Library[i], p1.Library[j] = p1.Library[j], p1.Library[i] })
	rand.Shuffle(len(p2.Library), func(i, j int) { p2.Library[i], p2.Library[j] = p2.Library[j], p2.Library[i] })

	p1.Draw(7)
	p2.Draw(7)

	g := game.NewGame(p1, p2)
	maxTurns := 25
	for g.GetTurnNumber() <= maxTurns {
		ap := g.GetActivePlayerRaw()
		switch g.GetCurrentPhase() {
		case game.PhaseUntap:
			for _, perm := range ap.Battlefield {
				perm.Untap()
			}
		case game.PhaseDraw:
			ap.Draw(1)
		case game.PhaseMain1:
			for i, c := range ap.Hand {
				if c.IsLand() {
					ap.Hand = append(ap.Hand[:i], ap.Hand[i+1:]...)
					_, _ = g.PlayLand(ap, c.Name)
					break
				}
			}
		case game.PhaseCombat:
			for _, perm := range ap.GetCreatures() {
				if !perm.IsTapped() {
					_ = g.DeclareAttacker(perm, opponentOf(g, ap))
				}
			}
			g.ResolveCombatDamage()
		}
		g.AdvancePhase()
		if p1.HasLost() || p2.HasLost() || p1.GetLifeTotal() <= 0 || p2.GetLifeTotal() <= 0 {
			break
		}
	}

	if p1.GetLifeTotal() > p2.GetLifeTotal() {
		return p1, p2
	}
	if p2.GetLifeTotal() > p1.GetLifeTotal() {
		return p2, p1
	}
	return p1, p2
}

func opponentOf(g *game.Game, p *game.Player) *game.Player {
	for _, o := range g.GetPlayersRaw() {
		if o != p {
			return o
		}
	}
	return nil
}
