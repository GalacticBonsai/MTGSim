// Package dashboard serves a web dashboard for browsing simulation results.
package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/pkg/simulation"
	"github.com/mtgsim/mtgsim/pkg/stats"
)

//go:embed static/*
var staticFS embed.FS

// ResultsProvider returns a snapshot of the current simulation results.
// The caller is responsible for any synchronization needed to produce the snapshot.
type ResultsProvider func() []simulation.Result

// EDHResultsProvider returns a snapshot of EDH-format aggregate stats
// (per-deck wins, losses, commander damage KOs, etc.). It is optional;
// when nil the dashboard falls back to the 1v1 legacy view only.
type EDHResultsProvider func() []simulation.EDHDeckStats

// EDHGamesProvider returns recent EDH pod records for replay/event-log views.
type EDHGamesProvider func() []simulation.EDHGameRecord

// EDHSummaryProvider returns global EDH telemetry for dashboard highlight cards.
type EDHSummaryProvider func() simulation.EDHSummary

// CardLibraryProvider returns the persistent global card stats library.
type CardLibraryProvider func() map[string]stats.GlobalCardStats

// ImplementationReportProvider returns the card implementation report.
type ImplementationReportProvider func() *card.ImplementationReport

// GameRunner interface for triggering more games
type GameRunner interface {
	RunGames(count int)
	IsRunning() bool
	SetSuggestedDeck(seat *simulation.EDHSeat)
}

// SuggestedDeckProvider returns the currently suggested deck, or nil if none is set.
type SuggestedDeckProvider func() *simulation.EDHSeat

// Server serves the dashboard.
type Server struct {
	provider           ResultsProvider
	edhProvider        EDHResultsProvider
	edhGames           EDHGamesProvider
	edhSummary         EDHSummaryProvider
	cardLibrary        CardLibraryProvider
	implReport         ImplementationReportProvider
	gameRunner         GameRunner
	suggestedDeck      *simulation.EDHSeat
	suggestedDeckMu    sync.Mutex
	cardDB             *card.CardDB
	snapshotManager    *SnapshotManager
	port               int
	mux                *http.ServeMux
}

// NewServer creates a new dashboard server backed by the given results provider.
func NewServer(provider ResultsProvider, port int) *Server {
	return &Server{
		provider: provider,
		port:     port,
		mux:      http.NewServeMux(),
	}
}

// SetEDHProvider attaches an EDH results provider to the server. The
// /api/edh-results endpoint and the EDH section of the HTML view are
// only enabled when a non-nil provider has been set.
func (s *Server) SetEDHProvider(p EDHResultsProvider) { s.edhProvider = p }

// SetEDHGamesProvider attaches a recent-pod provider for /api/edh-games.
func (s *Server) SetEDHGamesProvider(p EDHGamesProvider) { s.edhGames = p }

// SetEDHSummaryProvider attaches global EDH telemetry for /api/edh-results.
func (s *Server) SetEDHSummaryProvider(p EDHSummaryProvider) { s.edhSummary = p }

// SetCardLibraryProvider attaches a global card stats provider for /api/card-library.
func (s *Server) SetCardLibraryProvider(p CardLibraryProvider) { s.cardLibrary = p }

// SetImplementationReportProvider attaches an implementation report for /api/implementation.
func (s *Server) SetImplementationReportProvider(p ImplementationReportProvider) { s.implReport = p }

// SetGameRunner attaches a game runner for /api/run-games endpoint.
func (s *Server) SetGameRunner(gr GameRunner) { s.gameRunner = gr }

// SetCardDB attaches a card database for deck parsing.
func (s *Server) SetCardDB(db *card.CardDB) { s.cardDB = db }

// SetSnapshotManager attaches a snapshot manager for meta tracking.
func (s *Server) SetSnapshotManager(sm *SnapshotManager) { s.snapshotManager = sm }

// SetSuggestedDeck sets the deck to use for suggested deck games.
func (s *Server) SetSuggestedDeck(seat *simulation.EDHSeat) {
	s.suggestedDeckMu.Lock()
	defer s.suggestedDeckMu.Unlock()
	s.suggestedDeck = seat
	if s.gameRunner != nil {
		s.gameRunner.SetSuggestedDeck(seat)
	}
}

// GetSuggestedDeck returns the currently set suggested deck, or nil if none.
func (s *Server) GetSuggestedDeck() *simulation.EDHSeat {
	s.suggestedDeckMu.Lock()
	defer s.suggestedDeckMu.Unlock()
	return s.suggestedDeck
}

// Handler returns the underlying http.Handler (useful for tests).
func (s *Server) Handler() http.Handler {
	s.registerRoutes()
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/results", s.handleResults)
	s.mux.HandleFunc("/api/edh-results", s.handleEDHResults)
	s.mux.HandleFunc("/api/edh-games", s.handleEDHGames)
	s.mux.HandleFunc("/api/card-library", s.handleCardLibrary)
	s.mux.HandleFunc("/api/implementation", s.handleImplementation)
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/run-games", s.handleRunGames)
	s.mux.HandleFunc("/api/game-status", s.handleGameStatus)
	s.mux.HandleFunc("/api/suggested-deck", s.handleSuggestedDeck)
	s.mux.HandleFunc("/api/upload-deck", s.handleUploadDeck)
	s.mux.HandleFunc("/api/card-recommendations", s.handleCardRecommendations)
	s.mux.HandleFunc("/api/sideboard-suggestions", s.handleSideboardSuggestions)
	s.mux.HandleFunc("/api/matchup-matrix", s.handleMatchupMatrix)
	s.mux.HandleFunc("/api/card-search", s.handleCardSearch)
	s.mux.HandleFunc("/api/save-snapshot", s.handleSaveSnapshot)
	s.mux.HandleFunc("/api/snapshots", s.handleListSnapshots)
	s.mux.HandleFunc("/api/snapshot-comparison", s.handleSnapshotComparison)
	s.mux.HandleFunc("/api/meta-trends", s.handleMetaTrends)
	s.mux.HandleFunc("/style.css", serveStatic("style.css", "text/css"))
	s.mux.HandleFunc("/app.js", serveStatic("app.js", "application/javascript"))
	s.mux.HandleFunc("/", s.handleIndex)
}

// Start starts the server (blocking).
func (s *Server) Start() error {
	s.registerRoutes()
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting dashboard server on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, s.mux)
}

type deckRow struct {
	Name    string  `json:"name"`
	Wins    int     `json:"wins"`
	Losses  int     `json:"losses"`
	WinRate float64 `json:"win_rate"`
}

type resultsResponse struct {
	TotalGames  int       `json:"total_games"`
	UniqueDecks int       `json:"unique_decks"`
	Decks       []deckRow `json:"decks"`

	TotalDecks int  `json:"totalDecks,omitempty"`
	Truncated  bool `json:"truncated,omitempty"`
}

// handleResults returns deck win/loss aggregates derived from simulation.Results.
func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	snapshot := s.provider()

	rows := make([]deckRow, 0, len(snapshot))

	totalRecords := 0

	for _, res := range snapshot {
		rows = append(rows, deckRow{
			Name:    res.Name,
			Wins:    res.Wins,
			Losses:  res.Losses,
			WinRate: res.WinPercentage(),
		})

		totalRecords += res.Wins + res.Losses
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].WinRate > rows[j].WinRate
	})

	resp := resultsResponse{
		TotalGames:  totalRecords / 2,
		UniqueDecks: len(rows),
		TotalDecks:  len(rows),
		Decks:       rows,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

type edhResultsResponse struct {
	Enabled    bool                      `json:"enabled"`
	Decks      []simulation.EDHDeckStats `json:"decks"`
	Summary    simulation.EDHSummary     `json:"summary"`
	TotalDecks int                       `json:"totalDecks,omitempty"`
	Truncated  bool                      `json:"truncated,omitempty"`
}

func (s *Server) handleEDHResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.edhProvider == nil {
		_ = json.NewEncoder(w).Encode(edhResultsResponse{
			Enabled: false,
		})
		return
	}

	allDecks := s.edhProvider()

	resp := edhResultsResponse{
		Enabled:    true,
		TotalDecks: len(allDecks),
		Decks:      allDecks,
	}

	if s.edhSummary != nil {
		resp.Summary = s.edhSummary()
	}

	_ = json.NewEncoder(w).Encode(resp)
}

type edhGamesResponse struct {
	Enabled bool                       `json:"enabled"`
	Games   []simulation.EDHGameRecord `json:"games"`
}

func (s *Server) handleEDHGames(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.edhGames == nil {
		_ = json.NewEncoder(w).Encode(edhGamesResponse{Enabled: false})
		return
	}
	_ = json.NewEncoder(w).Encode(edhGamesResponse{Enabled: true, Games: s.edhGames()})
}

type cardLibraryResponse struct {
	Enabled bool                             `json:"enabled"`
	Cards   map[string]stats.GlobalCardStats `json:"cards"`
}

func (s *Server) handleCardLibrary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cardLibrary == nil {
		_ = json.NewEncoder(w).Encode(cardLibraryResponse{Enabled: false})
		return
	}
	_ = json.NewEncoder(w).Encode(cardLibraryResponse{Enabled: true, Cards: s.cardLibrary()})
}

type implementationResponse struct {
	Enabled bool                       `json:"enabled"`
	Report  *card.ImplementationReport `json:"report"`
}

func (s *Server) handleImplementation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.implReport == nil {
		_ = json.NewEncoder(w).Encode(implementationResponse{Enabled: false})
		return
	}
	_ = json.NewEncoder(w).Encode(implementationResponse{Enabled: true, Report: s.implReport()})
}

// handleHealth returns server health.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}

// handleRunGames triggers running more games
func (s *Server) handleRunGames(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if s.gameRunner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "game runner not configured",
		})
		return
	}
	
	// Get count from query parameter, default to 100
	countStr := r.URL.Query().Get("count")
	count := 100
	if countStr != "" {
		_, _ = fmt.Sscanf(countStr, "%d", &count)
	}
	if count < 1 || count > 10000 {
		count = 100
	}
	
	s.gameRunner.RunGames(count)
	
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "games started",
		"count":   count,
	})
}

// handleGameStatus returns the current status of game running
func (s *Server) handleGameStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	running := false
	if s.gameRunner != nil {
		running = s.gameRunner.IsRunning()
	}
	
	_ = json.NewEncoder(w).Encode(map[string]any{
		"running": running,
	})
}

// handleSuggestedDeck returns the current suggested deck for testing
func (s *Server) handleSuggestedDeck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	deck := s.GetSuggestedDeck()
	if deck == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": nil,
		})
		return
	}
	
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name": deck.DeckPath,
	})
}

// handleUploadDeck handles deck file uploads and sets as suggested deck
func (s *Server) handleUploadDeck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "only POST allowed",
		})
		return
	}
	
	if s.cardDB == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "card database not initialized",
		})
		return
	}
	
	// Parse multipart form with max 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "failed to parse form: " + err.Error(),
		})
		return
	}
	
	file, handler, err := r.FormFile("deck")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "no deck file provided",
		})
		return
	}
	defer file.Close()
	
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "deck-*.txt")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "failed to create temp file: " + err.Error(),
		})
		return
	}
	defer os.Remove(tmpFile.Name())
	
	// Copy uploaded file to temp file
	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "failed to save file: " + err.Error(),
		})
		return
	}
	tmpFile.Close()
	
	// Parse the deck
	commanders, main, side, err := deck.ImportCommanderDeckfileWithCommanders(tmpFile.Name(), s.cardDB)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "failed to parse deck: " + err.Error(),
		})
		return
	}
	
	// Convert card.Card to game.SimpleCard
	convertCards := func(cards []card.Card) []game.SimpleCard {
		result := make([]game.SimpleCard, len(cards))
		for i, c := range cards {
			result[i] = game.SimpleCard{
				Name:           c.Name,
				TypeLine:       c.TypeLine,
				Power:          c.Power,
				Toughness:      c.Toughness,
				OracleText:     c.OracleText,
				Colors:         c.Colors,
				ColorIdentity:  c.ColorIdentity,
				ManaCost:       c.ManaCost,
			}
		}
		return result
	}
	
	// Determine deck name: prefer explicit name from file, fallback to uploaded filename
	deckName := main.Name
	if deckName == "" || strings.Contains(deckName, "/tmp/deck-") {
		deckName = handler.Filename
	}
	// Strip extension for cleaner display
	if ext := filepath.Ext(deckName); ext != "" {
		deckName = strings.TrimSuffix(deckName, ext)
	}

	// Create an EDHSeat from the parsed deck
	seat := simulation.EDHSeat{
		DeckPath:   handler.Filename,
		DeckName:   deckName,
		Library:    convertCards(main.Cards),
		Sideboard:  convertCards(side.Cards),
		Commanders: convertCards(commanders),
	}
	
	// Set as suggested deck
	s.SetSuggestedDeck(&seat)
	
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "deck uploaded and set",
		"name":    handler.Filename,
	})
}

// handleCardRecommendations returns deck improvement suggestions.
func (s *Server) handleCardRecommendations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.edhProvider == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}

	deckName := r.URL.Query().Get("deck")
	if deckName == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "deck query parameter required"})
		return
	}

	edhStats := s.edhProvider()
	var deckStats *simulation.EDHDeckStats
	for i := range edhStats {
		if edhStats[i].DeckName == deckName {
			deckStats = &edhStats[i]
			break
		}
	}

	if deckStats == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "deck not found"})
		return
	}

	cardLib := map[string]stats.GlobalCardStats{}
	if s.cardLibrary != nil {
		cardLib = s.cardLibrary()
	}

	recs := GenerateDeckRecommendations(deckStats, cardLib, s.cardDB)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"recommendations": recs,
	})
}

// handleSideboardSuggestions returns sideboard swap suggestions.
func (s *Server) handleSideboardSuggestions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.edhProvider == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}

	deckName := r.URL.Query().Get("deck")
	if deckName == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "deck query parameter required"})
		return
	}

	edhStats := s.edhProvider()
	var deckStats *simulation.EDHDeckStats
	for i := range edhStats {
		if edhStats[i].DeckName == deckName {
			deckStats = &edhStats[i]
			break
		}
	}

	if deckStats == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "deck not found"})
		return
	}

	cardLib := map[string]stats.GlobalCardStats{}
	if s.cardLibrary != nil {
		cardLib = s.cardLibrary()
	}

	suggs := GenerateSideboardSuggestions(deckStats, edhStats, cardLib, s.cardDB)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"suggestions": suggs,
	})
}

// handleMatchupMatrix returns a matrix of deck matchups.
func (s *Server) handleMatchupMatrix(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.edhProvider == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}

	edhStats := s.edhProvider()

	// For now, return basic deck statistics in a matrix-friendly format
	// This would be enhanced with actual matchup tracking in a production system
	type DeckStats struct {
		Name      string  `json:"name"`
		Commander string  `json:"commander"`
		WinRate   float64 `json:"win_rate"`
		Games     int     `json:"games"`
	}

	decks := make([]DeckStats, len(edhStats))
	for i, d := range edhStats {
		decks[i] = DeckStats{
			Name:      d.DeckName,
			Commander: d.CommanderName,
			WinRate:   d.WinRate,
			Games:     d.Games,
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"decks": decks,
	})
}

// handleCardSearch searches for cards by name or attributes.
func (s *Server) handleCardSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query().Get("q")
	if query == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "query parameter required"})
		return
	}

	if s.cardLibrary == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false, "cards": []any{}})
		return
	}

	cardLib := s.cardLibrary()
	
	// Simple card search - can be enhanced with Scryfall API integration
	results := []map[string]any{}
	queryLower := strings.ToLower(query)
	
	count := 0
	for name, stats := range cardLib {
		if strings.Contains(strings.ToLower(name), queryLower) {
			wr := 0.0
			if stats.Casts > 0 {
				wr = (float64(stats.Wins) / float64(stats.Casts)) * 100
			}
			results = append(results, map[string]any{
				"name":     name,
				"wins":     stats.Wins,
				"casts":    stats.Casts,
				"win_rate": wr,
				"image":    stats.ImageURL,
			})
			count++
			if count >= 50 {
				break
			}
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"results": results,
	})
}

// handleSaveSnapshot saves the current meta state.
func (s *Server) handleSaveSnapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.snapshotManager == nil || s.edhProvider == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "snapshot manager not initialized"})
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		name = time.Now().Format("2006-01-02 15:04:05")
	}

	edhStats := s.edhProvider()
	cardLib := map[string]stats.GlobalCardStats{}
	if s.cardLibrary != nil {
		cl := s.cardLibrary()
		for k, v := range cl {
			cardLib[k] = v
		}
	}

	if err := s.snapshotManager.SaveSnapshot(name, edhStats, cardLib); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"name":    name,
		"timestamp": time.Now().Unix(),
	})
}

// handleListSnapshots returns all available snapshots.
func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.snapshotManager == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false, "snapshots": []any{}})
		return
	}

	snapshots, err := s.snapshotManager.LoadSnapshots()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	// Return summary of snapshots
	type SnapshotSummary struct {
		Name       string    `json:"name"`
		Timestamp  time.Time `json:"timestamp"`
		DeckCount  int       `json:"deck_count"`
		AverageWR  float64   `json:"average_wr"`
	}

	summaries := []SnapshotSummary{}
	for _, snap := range snapshots {
		avgWR := 0.0
		if len(snap.Decks) > 0 {
			totalWR := 0.0
			for _, d := range snap.Decks {
				totalWR += d.WinRate
			}
			avgWR = totalWR / float64(len(snap.Decks))
		}

		summaries = append(summaries, SnapshotSummary{
			Name:      snap.Name,
			Timestamp: snap.Timestamp,
			DeckCount: len(snap.Decks),
			AverageWR: avgWR,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled":   true,
		"snapshots": summaries,
	})
}

// handleSnapshotComparison compares two snapshots.
func (s *Server) handleSnapshotComparison(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.snapshotManager == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "snapshot manager not initialized"})
		return
	}

	// For now, compare latest two snapshots
	snapshots, err := s.snapshotManager.LoadSnapshots()
	if err != nil || len(snapshots) < 2 {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "need at least 2 snapshots for comparison"})
		return
	}

	comp := CompareSnapshots(snapshots[1], snapshots[0])
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled":      true,
		"comparison":   comp,
	})
}

// handleMetaTrends analyzes meta trends across snapshots.
func (s *Server) handleMetaTrends(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.snapshotManager == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false, "trends": []any{}})
		return
	}

	snapshots, err := s.snapshotManager.LoadSnapshots()
	if err != nil || len(snapshots) == 0 {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false, "trends": []any{}})
		return
	}

	trends := AnalyzeTrends(snapshots)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"trends":  trends,
	})
}

// handleIndex returns the HTML dashboard.

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	data, _ := staticFS.ReadFile("static/index.html")
	_, _ = w.Write(data)
}

func serveStatic(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		data, _ := staticFS.ReadFile("static/" + name)
		_, _ = w.Write(data)
	}
}
