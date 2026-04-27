// MTGSim Dashboard - serves a web dashboard for browsing simulation results.
// Runs games using a lightweight pkg/game loop and publishes wins/losses
// through pkg/simulation.Results to the dashboard server.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
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
	games := flag.Int("games", 50, "Number of games to simulate")
	decksDir := flag.String("decks", "decks/1v1", "Directory containing deck files")
	port := flag.Int("port", 8080, "Dashboard port")
	logLevel := flag.String("log", "META", "Log level (META, GAME, PLAYER, CARD)")
	keepAlive := flag.Bool("keep-alive", true, "Keep server running after simulation")
	flag.Parse()

	logger.SetLogLevel(logger.ParseLogLevel(*logLevel))

	logger.LogMeta("Loading card database...")
	cardDB, err := card.LoadCardDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading card database: %v\n", err)
		os.Exit(1)
	}
	logger.LogMeta("Card database loaded with %d cards", cardDB.Size())

	deckFiles, err := simulation.GetDecks(*decksDir)
	if err != nil || len(deckFiles) < 2 {
		fmt.Fprintf(os.Stderr, "Need at least 2 deck files in %s\n", *decksDir)
		os.Exit(1)
	}
	logger.LogMeta("Found %d deck files", len(deckFiles))

	results := simulation.NewResults()
	var mu sync.Mutex
	provider := func() []simulation.Result {
		mu.Lock()
		defer mu.Unlock()
		return results.GetResults()
	}

	server := dashboard.NewServer(provider, *port)
	go func() {
		if err := server.Start(); err != nil {
			logger.LogMeta("Dashboard error: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("\n🧙 MTGSim Dashboard available at: http://localhost:%d\n\n", *port)
	logger.LogMeta("Starting %d games...", *games)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < *games; i++ {
		d1Path := deckFiles[rng.Intn(len(deckFiles))]
		d2Path := deckFiles[rng.Intn(len(deckFiles))]
		for d2Path == d1Path && len(deckFiles) > 1 {
			d2Path = deckFiles[rng.Intn(len(deckFiles))]
		}

		m1, _, err1 := deck.ImportDeckfile(d1Path, cardDB)
		m2, _, err2 := deck.ImportDeckfile(d2Path, cardDB)
		if err1 != nil || err2 != nil || m1.Size() == 0 || m2.Size() == 0 {
			logger.LogMeta("Skipping game %d due to deck import error", i+1)
			continue
		}

		winner, loser := simulateGame(m1, m2)
		if winner != nil && loser != nil {
			mu.Lock()
			results.AddWin(winner.GetName())
			results.AddLoss(loser.GetName())
			mu.Unlock()
		}

		if (i+1)%10 == 0 {
			logger.LogMeta("Completed %d/%d games", i+1, *games)
		}
	}

	logger.LogMeta("Simulation completed!")

	results.PrintTopResults()

	if *keepAlive {
		fmt.Printf("\nDashboard still running on http://localhost:%d (press Ctrl+C to exit)\n", *port)
		select {}
	}
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
