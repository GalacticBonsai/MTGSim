// Package dashboard serves a web dashboard for browsing simulation results.
package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/mtgsim/mtgsim/pkg/card"
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
}

// Server serves the dashboard.
type Server struct {
	provider    ResultsProvider
	edhProvider EDHResultsProvider
	edhGames    EDHGamesProvider
	edhSummary  EDHSummaryProvider
	cardLibrary CardLibraryProvider
	implReport  ImplementationReportProvider
	gameRunner  GameRunner
	port        int
	mux         *http.ServeMux
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

	const maxDecks = 10

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
	}

	if len(rows) > maxDecks {
		resp.Truncated = true
		resp.Decks = rows[:maxDecks]
	} else {
		resp.Decks = rows
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

	const maxDecks = 10

	allDecks := s.edhProvider()

	resp := edhResultsResponse{
		Enabled:    true,
		TotalDecks: len(allDecks),
	}

	if len(allDecks) > maxDecks {
		resp.Truncated = true
		resp.Decks = allDecks[:maxDecks]
	} else {
		resp.Decks = allDecks
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
		fmt.Sscanf(countStr, "%d", &count)
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
