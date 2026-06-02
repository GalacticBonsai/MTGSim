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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/game"
	"github.com/mtgsim/mtgsim/pkg/scryfall"
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

// GameLogEntry is a lightweight summary used to populate the game selector.
type GameLogEntry struct {
	ID          int      `json:"id"`
	Winner      string   `json:"winner"`
	Turns       int      `json:"turns"`
	Players     []string `json:"players"`
	EventCount  int      `json:"event_count"`
	TotalMana   int      `json:"total_mana"`
	TotalCards  int      `json:"total_cards"`
	TotalCombat int      `json:"total_combat"`
}

// GameLogProvider returns summaries for the game selector and full event
// logs for a specific game so the front-end can render a replay view.
type GameLogProvider func() (summaries []GameLogEntry, getGame func(id int) *simulation.EDHGameRecord)

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

// DataResetter allows resetting persistent game data from the dashboard UI.
type DataResetter interface {
	ResetCardLibrary()
	ResetGameLogs()
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
	recordUpload       func(name string)
	deleteUpload       func(name string)
	uploadedDecks      func() []string
	cardDB             *card.CardDB
	scryfallClient     *scryfall.Client
	gameLogProvider    GameLogProvider

	// In-memory byte caches for uploaded-filtered EDH results
	uploadedEdhCache      []byte
	uploadedEdhCached     time.Time
	snapshotManager    *SnapshotManager
	port               int
	mux                *http.ServeMux
	
	// Cache for provider results to reduce lock contention
	cacheMu              sync.RWMutex
	cacheMaxAge          time.Duration

	dataResetter DataResetter

	// In-memory byte caches for serialised API responses so we avoid
	// re-computing and re-serialising on every poll cycle.
	resultsCache         []byte
	resultsCached        time.Time
	edhResultsCacheBytes []byte
	edhResultsBytesCached time.Time
}

// NewServer creates a new dashboard server backed by the given results provider.
func NewServer(provider ResultsProvider, port int) *Server {
	return &Server{
		provider:    provider,
		port:        port,
		mux:         http.NewServeMux(),
		cacheMaxAge: 30 * time.Second,
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

// SetUploadRecorder sets a callback invoked when a deck is uploaded.
func (s *Server) SetUploadRecorder(fn func(name string)) { s.recordUpload = fn }

// SetDeleteRecorder sets a callback invoked when an uploaded deck is removed.
func (s *Server) SetDeleteRecorder(fn func(name string)) { s.deleteUpload = fn }

// SetUploadedDecksProvider sets a function that returns uploaded deck names.
func (s *Server) SetUploadedDecksProvider(fn func() []string) { s.uploadedDecks = fn }

// SetCardDB attaches a card database for deck parsing.
func (s *Server) SetCardDB(db *card.CardDB) { s.cardDB = db }

func (s *Server) SetScryfallClient(cl *scryfall.Client) { s.scryfallClient = cl }

// SetGameLogProvider attaches a game log ring buffer for /api/game-log-list and /api/game-log.
func (s *Server) SetGameLogProvider(p GameLogProvider) { s.gameLogProvider = p }

// SetDataResetter attaches a data reseter for resetting card library and game logs.
func (s *Server) SetDataResetter(d DataResetter) { s.dataResetter = d }

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
	s.mux.HandleFunc("/api/uploaded-decks", s.handleUploadedDecks)
	s.mux.HandleFunc("/api/card-recommendations", s.handleCardRecommendations)
	s.mux.HandleFunc("/api/sideboard-suggestions", s.handleSideboardSuggestions)
	s.mux.HandleFunc("/api/matchup-matrix", s.handleMatchupMatrix)
	s.mux.HandleFunc("/api/card-search", s.handleCardSearch)
	s.mux.HandleFunc("/api/db-status", s.handleDBStatus)
	s.mux.HandleFunc("/api/card-image", s.handleCardImage)
	s.mux.HandleFunc("/api/save-snapshot", s.handleSaveSnapshot)
	s.mux.HandleFunc("/api/snapshots", s.handleListSnapshots)
	s.mux.HandleFunc("/api/snapshot-comparison", s.handleSnapshotComparison)
	s.mux.HandleFunc("/api/meta-trends", s.handleMetaTrends)
	s.mux.HandleFunc("/api/game-log-list", s.handleGameLogList)
	s.mux.HandleFunc("/api/game-log", s.handleGameLog)
	s.mux.HandleFunc("/api/reset-card-library", s.handleResetCardLibrary)
	s.mux.HandleFunc("/api/reset-game-logs", s.handleResetGameLogs)
	s.mux.HandleFunc("/style.css", serveStatic("style.css", "text/css"))
	s.mux.HandleFunc("/app.js", serveStatic("app.js", "application/javascript"))
	s.mux.HandleFunc("/", s.handleIndex)
}

// Start starts the server (blocking).
func (s *Server) Start() error {
	s.registerRoutes()
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting dashboard server on http://localhost%s\n", addr)
	
	// Timeouts are generous because some API handlers (e.g. handleResults)
	// may block briefly on the simulation mutex before returning a cached
	// response.  Caching ensures most requests complete in under 100 ms so
	// pile-up is not a practical concern.
	server := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  10 * time.Second,
	}
	return server.ListenAndServe()
}

type deckRow struct {
	Name    string  `json:"name"`
	Wins    int     `json:"wins"`
	Losses  int     `json:"losses"`
	WinRate float64 `json:"win_rate"`
}

type winRateBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type resultsResponse struct {
	TotalGames  int       `json:"total_games"`
	UniqueDecks int       `json:"unique_decks"`
	Decks       []deckRow `json:"decks"`

	TotalDecks int  `json:"totalDecks,omitempty"`
	Truncated  bool `json:"truncated,omitempty"`

	// Server-computed win-rate histogram so the front-end doesn't need
	// to iterate the full deck list.
	WinRateBuckets []winRateBucket `json:"win_rate_buckets,omitempty"`
}

// handleResults returns deck win/loss aggregates derived from simulation.Results.
func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Try serving from the byte cache first (avoids provider lock, sort, and
	// JSON serialisation on every poll cycle).
	s.cacheMu.RLock()
	if s.resultsCache != nil && time.Since(s.resultsCached) < s.cacheMaxAge {
		_, _ = w.Write(s.resultsCache)
		s.cacheMu.RUnlock()
		return
	}
	s.cacheMu.RUnlock()

	snapshot := s.provider()

	totalRecords := 0
	rows := make([]deckRow, 0, len(snapshot))

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

	// Build win-rate histogram from the full dataset before truncation.
	buckets := computeWinRateBuckets(rows)

	// Default limit prevents blowing up JSON payload size for very large
	// result sets.  The front-end charts (win-rate histogram, top-N bar
	// chart) all work from the histogram or the first few entries.
	const defaultMaxDecks = 500
	totalDecks := len(rows)
	limit := defaultMaxDecks
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v >= 0 {
			limit = v
		}
	}
	truncated := limit > 0 && limit < len(rows)
	if truncated {
		rows = rows[:limit]
	}

	resp := resultsResponse{
		TotalGames:     totalRecords / 2,
		UniqueDecks:    totalDecks,
		TotalDecks:     totalDecks,
		Decks:          rows,
		Truncated:      truncated,
		WinRateBuckets: buckets,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "json marshal error", http.StatusInternalServerError)
		return
	}

	s.cacheMu.Lock()
	s.resultsCache = data
	s.resultsCached = time.Now()
	s.cacheMu.Unlock()

	_, _ = w.Write(data)
}

// computeWinRateBuckets partitions decks into the four win-rate quartiles
// used by the front-end doughnut chart.
func computeWinRateBuckets(decks []deckRow) []winRateBucket {
	var lt25, lt50, lt75, ge75 int
	for _, d := range decks {
		switch {
		case d.WinRate < 25:
			lt25++
		case d.WinRate < 50:
			lt50++
		case d.WinRate < 75:
			lt75++
		default:
			ge75++
		}
	}
	return []winRateBucket{
		{Label: "0-25%", Count: lt25},
		{Label: "25-50%", Count: lt50},
		{Label: "50-75%", Count: lt75},
		{Label: "75-100%", Count: ge75},
	}
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

	uploadedOnly := r.URL.Query().Get("uploaded") == "1"
	light := r.URL.Query().Get("light") == "1"
	deckFilter := r.URL.Query().Get("deck")

	if !light && deckFilter == "" {
		if uploadedOnly {
			s.cacheMu.RLock()
			if s.uploadedEdhCache != nil && time.Since(s.uploadedEdhCached) < s.cacheMaxAge {
				_, _ = w.Write(s.uploadedEdhCache)
				s.cacheMu.RUnlock()
				return
			}
			s.cacheMu.RUnlock()
		} else {
			s.cacheMu.RLock()
			if s.edhResultsCacheBytes != nil && time.Since(s.edhResultsBytesCached) < s.cacheMaxAge {
				_, _ = w.Write(s.edhResultsCacheBytes)
				s.cacheMu.RUnlock()
				return
			}
			s.cacheMu.RUnlock()
		}
	}

	allDecks := s.edhProvider()
	var decks []simulation.EDHDeckStats
	var totalDecks int

	if deckFilter != "" {
		for _, d := range allDecks {
			if d.DeckName == deckFilter {
				decks = append(decks, d)
				totalDecks = 1
				break
			}
		}
	} else if uploadedOnly && s.uploadedDecks != nil {
		names := s.uploadedDecks()
		nameSet := make(map[string]bool, len(names))
		for _, n := range names {
			nameSet[n] = true
		}
		for _, d := range allDecks {
			if nameSet[d.DeckName] {
				d2 := d
				if light { d2.CardStats = nil }
				decks = append(decks, d2)
			}
		}
		totalDecks = len(decks)
	} else {
		for _, d := range allDecks {
			d2 := d
			if light { d2.CardStats = nil }
			decks = append(decks, d2)
		}
		totalDecks = len(decks)
		limit := 100
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 { limit = v }
		}
		if deckFilter == "" && !uploadedOnly && len(decks) > limit {
			decks = decks[:limit]
		}
	}

	resp := edhResultsResponse{
		Enabled:    true,
		TotalDecks: totalDecks,
		Decks:      decks,
	}

	if s.edhSummary != nil {
		resp.Summary = s.edhSummary()
	}

	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "json marshal error", http.StatusInternalServerError)
		return
	}

	s.cacheMu.Lock()
	if uploadedOnly {
		s.uploadedEdhCache = data
		s.uploadedEdhCached = time.Now()
	} else {
		s.edhResultsCacheBytes = data
		s.edhResultsBytesCached = time.Now()
	}
	s.cacheMu.Unlock()

	_, _ = w.Write(data)
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
	CardDB  map[string]cardMeta              `json:"card_db,omitempty"`
}

type cardMeta struct {
	TypeLine string   `json:"type_line"`
	CMC      float32  `json:"cmc"`
	ManaCost string   `json:"mana_cost"`
	Colors   []string `json:"colors,omitempty"`
	ImageURL string   `json:"image_url,omitempty"`
}

func (s *Server) handleCardLibrary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cardLibrary == nil {
		_ = json.NewEncoder(w).Encode(cardLibraryResponse{Enabled: false})
		return
	}
	lib := s.cardLibrary()
	resp := cardLibraryResponse{Enabled: true, Cards: lib}
	if len(resp.Cards) > 500 {
		filtered := make(map[string]stats.GlobalCardStats, 500)
		type entry struct{ name string; casts int }
		var entries []entry
		for name, s := range resp.Cards { entries = append(entries, entry{name, s.Casts}) }
		sort.Slice(entries, func(i, j int) bool { return entries[i].casts > entries[j].casts })
		for i := 0; i < 500 && i < len(entries); i++ {
			filtered[entries[i].name] = resp.Cards[entries[i].name]
		}
		resp.Cards = filtered
	}
	if s.cardDB != nil {
		resp.CardDB = make(map[string]cardMeta)
		for name, card := range lib {
			if c, ok := s.cardDB.GetCardByName(name); ok {
				imageURL := card.ImageURL
				if imageURL == "" && c.ImageURIs != nil { imageURL = c.ImageURIs.Normal }
				resp.CardDB[name] = cardMeta{
					TypeLine: c.TypeLine, CMC: c.CMC, ManaCost: c.ManaCost,
					Colors: c.ColorIdentity, ImageURL: imageURL,
				}
			}
		}
	}
	_ = json.NewEncoder(w).Encode(resp)
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

func (s *Server) handleResetCardLibrary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST required"})
		return
	}
	if s.dataResetter == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "data reseter not configured"})
		return
	}
	s.dataResetter.ResetCardLibrary()
	_ = json.NewEncoder(w).Encode(map[string]any{"message": "card library reset"})
}

func (s *Server) handleResetGameLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST required"})
		return
	}
	if s.dataResetter == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "data reseter not configured"})
		return
	}
	s.dataResetter.ResetGameLogs()
	_ = json.NewEncoder(w).Encode(map[string]any{"message": "game logs cleared"})
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
	defer func() { _ = file.Close() }()
	
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "deck-*.txt")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "failed to create temp file: " + err.Error(),
		})
		return
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	
	// Copy uploaded file to temp file
	if _, err := io.Copy(tmpFile, file); err != nil {
		_ = tmpFile.Close()
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "failed to save file: " + err.Error(),
		})
		return
	}
	_ = tmpFile.Close()
	
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
	
	// Persist to uploaded decks table
	if s.recordUpload != nil {
		s.recordUpload(deckName)
	}
	
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "deck uploaded and set",
		"name":    handler.Filename,
	})
}

// handleUploadedDecks lists or deletes uploaded decks.
func (s *Server) handleUploadedDecks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodDelete {
		name := r.URL.Query().Get("name")
		if name == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "missing name"})
			return
		}
		if s.deleteUpload != nil {
			s.deleteUpload(name)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "deleted"})
		return
	}

	if r.Method == http.MethodGet {
		if s.cardLibrary != nil {
			// Return deck names from the provider (not implemented as separate list)
			_ = json.NewEncoder(w).Encode(map[string]any{"decks": []string{}})
		}
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": "method not allowed"})
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

func (s *Server) handleDBStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{"connected": false}
	if s.edhProvider != nil {
		allDecks := s.edhProvider()
		totalPods := 0
		for _, d := range allDecks {
			totalPods += d.Games
		}
		resp["connected"] = true
		resp["decks"] = len(allDecks)
		resp["total_pods"] = totalPods
	}
	if s.cardLibrary != nil {
		cards := s.cardLibrary()
		resp["total_cards_tracked"] = len(cards)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCardImage(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}
	var imageURL string
	if lib := s.cardLibrary; lib != nil {
		cards := lib()
		if c, ok := cards[name]; ok && c.ImageURL != "" {
			imageURL = c.ImageURL
		}
	}
	if imageURL == "" && s.cardDB != nil {
		if c, ok := s.cardDB.GetCardByName(name); ok && c.ImageURIs != nil {
			imageURL = c.ImageURIs.Normal
		}
	}
	if imageURL == "" {
		http.NotFound(w, r)
		return
	}
	if s.scryfallClient != nil {
		path, err := s.scryfallClient.DownloadAndCacheImage(imageURL)
		if err == nil {
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, path)
			return
		}
	}
	http.Redirect(w, r, imageURL, http.StatusFound)
}

// handleGameLogList returns a list of recent game summaries for the log viewer.
func (s *Server) handleGameLogList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.gameLogProvider == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false, "games": []any{}})
		return
	}
	summaries, _ := s.gameLogProvider()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"games":   summaries,
	})
}

// handleGameLog returns the full event log for a specific game.
func (s *Server) handleGameLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.gameLogProvider == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "missing id parameter", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	_, getGame := s.gameLogProvider()
	rec := getGame(id)
	if rec == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"game":    rec,
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
