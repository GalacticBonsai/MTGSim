package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mtgsim/mtgsim/pkg/simulation"
	"github.com/mtgsim/mtgsim/pkg/stats"
)

func snapshotFromResults(r *simulation.Results) ResultsProvider {
	return func() []simulation.Result { return r.GetResults() }
}

func TestServer_HandleResults(t *testing.T) {
	r := simulation.NewResults()
	r.AddWin("Deck A")
	r.AddLoss("Deck B")
	r.AddWin("Deck A")
	r.AddLoss("Deck B")

	server := NewServer(snapshotFromResults(r), 0)
	req := httptest.NewRequest("GET", "/api/results", nil)
	w := httptest.NewRecorder()
	server.handleResults(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var data resultsResponse
	body, _ := io.ReadAll(w.Body)
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.TotalGames != 2 {
		t.Errorf("expected 2 total games, got %d", data.TotalGames)
	}
	if data.UniqueDecks != 2 {
		t.Errorf("expected 2 unique decks, got %d", data.UniqueDecks)
	}
	if len(data.Decks) != 2 || data.Decks[0].Name != "Deck A" {
		t.Errorf("expected Deck A first, got %+v", data.Decks)
	}
	if data.Decks[0].WinRate != 100.0 {
		t.Errorf("expected 100%% win rate for Deck A, got %v", data.Decks[0].WinRate)
	}
}

func TestServer_HandleHealth(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var health map[string]interface{}
	body, _ := io.ReadAll(w.Body)
	if err := json.Unmarshal(body, &health); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if health["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", health["status"])
	}
}

func TestServer_HandleIndex(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	server.handleIndex(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html" {
		t.Errorf("expected text/html, got %s", ct)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "MTGSim") {
		t.Error("expected 'MTGSim' in HTML response")
	}
}

func TestServer_HandlerRoutes(t *testing.T) {
	r := simulation.NewResults()
	r.AddWin("Solo")
	server := NewServer(snapshotFromResults(r), 0)
	h := server.Handler()

	for _, path := range []string{"/", "/api/results", "/api/edh-games", "/api/health"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("path %s: expected 200, got %d", path, w.Code)
		}
	}
}

func TestServer_HandleEDHResults_Disabled(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	req := httptest.NewRequest("GET", "/api/edh-results", nil)
	w := httptest.NewRecorder()
	server.handleEDHResults(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp edhResultsResponse
	body, _ := io.ReadAll(w.Body)
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Enabled {
		t.Errorf("expected enabled=false when no EDH provider is registered")
	}
}

func TestServer_HandleEDHResults_Enabled(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	server.SetEDHProvider(func() []simulation.EDHDeckStats {
		return []simulation.EDHDeckStats{{
			DeckName: "Atraxa", CommanderName: "Atraxa, Praetors' Voice",
			Games: 3, Wins: 2, Losses: 1, WinRate: 66.6, AvgFinalLife: 12.0,
			CommanderDamageKOs: 1, AvgManaSpent: 12.3, MaxStormCount: 4,
		}}
	})
	server.SetEDHSummaryProvider(func() simulation.EDHSummary {
		return simulation.EDHSummary{TotalGames: 3, HighestStormCount: 4, TotalManaSpent: 37}
	})
	req := httptest.NewRequest("GET", "/api/edh-results", nil)
	w := httptest.NewRecorder()
	server.handleEDHResults(w, req)

	var resp edhResultsResponse
	body, _ := io.ReadAll(w.Body)
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Enabled {
		t.Fatalf("expected enabled=true with provider registered")
	}
	if len(resp.Decks) != 1 || resp.Decks[0].DeckName != "Atraxa" {
		t.Fatalf("unexpected EDH decks: %+v", resp.Decks)
	}
	if resp.Decks[0].CommanderDamageKOs != 1 {
		t.Errorf("expected 1 commander dmg KO, got %d", resp.Decks[0].CommanderDamageKOs)
	}
	if resp.Summary.HighestStormCount != 4 || resp.Summary.TotalManaSpent != 37 {
		t.Errorf("unexpected EDH summary: %+v", resp.Summary)
	}
}

func TestServer_HandleEDHGames_Enabled(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	server.SetEDHGamesProvider(func() []simulation.EDHGameRecord {
		return []simulation.EDHGameRecord{{
			Turns: 7, Winner: "Deck A", MaxStormCount: 3, TotalManaSpent: 18,
			Players: []simulation.EDHPlayerRecord{{DeckName: "Deck A"}},
			Events:  []simulation.EDHEvent{{Kind: simulation.EventGameEnd}},
		}}
	})
	req := httptest.NewRequest("GET", "/api/edh-games", nil)
	w := httptest.NewRecorder()
	server.handleEDHGames(w, req)

	var resp edhGamesResponse
	body, _ := io.ReadAll(w.Body)
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Enabled || len(resp.Games) != 1 {
		t.Fatalf("unexpected EDH games response: %+v", resp)
	}
	if resp.Games[0].Winner != "Deck A" || len(resp.Games[0].Events) != 1 {
		t.Fatalf("unexpected game payload: %+v", resp.Games[0])
	}
	if resp.Games[0].MaxStormCount != 3 || resp.Games[0].TotalManaSpent != 18 {
		t.Fatalf("missing tuning metrics: %+v", resp.Games[0])
	}
}

func TestNewServer(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 8080)
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.port != 8080 { //nolint:staticcheck
		t.Errorf("expected port 8080, got %d", server.port)
	}
	if server.provider == nil {
		t.Error("expected provider to be set")
	}
}

// ============================================================================
// Endpoint Timing Tests - Ensure responses complete within 1.5s timeout
// ============================================================================

// TestGameStatus_ResponseTime verifies /api/game-status completes quickly
func TestGameStatus_ResponseTime(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	
	req := httptest.NewRequest("GET", "/api/game-status", nil)
	w := httptest.NewRecorder()
	
	start := time.Now()
	server.handleGameStatus(w, req)
	elapsed := time.Since(start)
	
	maxLatency := 100 * time.Millisecond
	if elapsed > maxLatency {
		t.Errorf("gameStatus took %v (max %v)", elapsed, maxLatency)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if _, ok := result["running"]; !ok {
		t.Error("expected 'running' field in response")
	}
}

// TestMatchupMatrix_ResponseTime verifies /api/matchup-matrix completes within timeout
func TestMatchupMatrix_ResponseTime(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	
	// Set up EDH provider with sample decks
	server.SetEDHProvider(func() []simulation.EDHDeckStats {
		decks := make([]simulation.EDHDeckStats, 10)
		for i := 0; i < 10; i++ {
			decks[i] = simulation.EDHDeckStats{
				DeckName:      "Deck " + string(rune(i)),
				CommanderName: "Commander " + string(rune(i)),
				Games:         5 + i,
				Wins:          2,
				Losses:        3 + i,
				WinRate:       33.3,
			}
		}
		return decks
	})
	
	req := httptest.NewRequest("GET", "/api/matchup-matrix", nil)
	w := httptest.NewRecorder()
	
	start := time.Now()
	server.handleMatchupMatrix(w, req)
	elapsed := time.Since(start)
	
	maxLatency := 200 * time.Millisecond
	if elapsed > maxLatency {
		t.Errorf("matchupMatrix took %v (max %v)", elapsed, maxLatency)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if enabled, ok := result["enabled"].(bool); !ok || !enabled {
		t.Error("expected 'enabled'=true in response")
	}
}

// TestEDHResults_ResponseTime verifies /api/edh-results responds within 1.5s timeout
func TestEDHResults_ResponseTime(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	
	// Set provider that returns 50 decks (realistic load)
	server.SetEDHProvider(func() []simulation.EDHDeckStats {
		decks := make([]simulation.EDHDeckStats, 50)
		for i := 0; i < 50; i++ {
			decks[i] = simulation.EDHDeckStats{
				DeckName:      "Deck " + string(rune(i%26 + 'A')),
				CommanderName: "Commander " + string(rune(i%26 + 'A')),
				Games:         10,
				Wins:          5,
				Losses:        5,
				WinRate:       50.0,
			}
		}
		return decks
	})
	
	server.SetEDHSummaryProvider(func() simulation.EDHSummary {
		return simulation.EDHSummary{
			TotalGames:       500,
			HighestStormCount: 10,
			TotalManaSpent:    5000,
		}
	})
	
	req := httptest.NewRequest("GET", "/api/edh-results", nil)
	w := httptest.NewRecorder()
	
	start := time.Now()
	server.handleEDHResults(w, req)
	elapsed := time.Since(start)
	
	maxLatency := 300 * time.Millisecond
	if elapsed > maxLatency {
		t.Errorf("edhResults took %v (max %v)", elapsed, maxLatency)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	
	var result edhResultsResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !result.Enabled || len(result.Decks) != 50 {
		t.Errorf("expected 50 enabled decks, got %d enabled=%v", len(result.Decks), result.Enabled)
	}
}

// TestCardLibrary_ResponseTime verifies /api/card-library responds quickly
func TestCardLibrary_ResponseTime(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	
	// Set up card library provider with sample data
	server.SetCardLibraryProvider(func() map[string]stats.GlobalCardStats {
		cards := make(map[string]stats.GlobalCardStats)
		for i := 0; i < 100; i++ {
			cards["Card "+string(rune(i))] = stats.GlobalCardStats{
				Casts:             5,
				Wins:              2,
				WinRate:           40.0,
				TotalGamesTracked: 100,
			}
		}
		return cards
	})
	
	req := httptest.NewRequest("GET", "/api/card-library", nil)
	w := httptest.NewRecorder()
	
	start := time.Now()
	server.handleCardLibrary(w, req)
	elapsed := time.Since(start)
	
	maxLatency := 200 * time.Millisecond
	if elapsed > maxLatency {
		t.Errorf("cardLibrary took %v (max %v)", elapsed, maxLatency)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestConcurrentEndpointCalls verifies endpoints don't block each other
func TestConcurrentEndpointCalls(t *testing.T) {
	r := simulation.NewResults()
	server := NewServer(snapshotFromResults(r), 0)
	
	server.SetEDHProvider(func() []simulation.EDHDeckStats {
		decks := make([]simulation.EDHDeckStats, 30)
		for i := 0; i < 30; i++ {
			decks[i] = simulation.EDHDeckStats{
				DeckName:      "Deck " + string(rune(i)),
				CommanderName: "Cmd " + string(rune(i)),
				Games:         5,
				Wins:          2,
				Losses:        3,
				WinRate:       40.0,
			}
		}
		return decks
	})
	
	// Simulate concurrent polling (like the dashboard does every 2-5 seconds)
	done := make(chan time.Duration, 3)
	
	// Game status (2s poll interval)
	go func() {
		start := time.Now()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/game-status", nil)
		server.handleGameStatus(w, req)
		done <- time.Since(start)
	}()
	
	// Matchup matrix (5s poll interval)
	go func() {
		start := time.Now()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/matchup-matrix", nil)
		server.handleMatchupMatrix(w, req)
		done <- time.Since(start)
	}()
	
	// EDH results (5s poll interval)
	go func() {
		start := time.Now()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/edh-results", nil)
		server.handleEDHResults(w, req)
		done <- time.Since(start)
	}()
	
	// Collect results
	maxConcurrentLatency := 500 * time.Millisecond
	for i := 0; i < 3; i++ {
		elapsed := <-done
		if elapsed > maxConcurrentLatency {
			t.Errorf("concurrent call %d took %v (max %v)", i, elapsed, maxConcurrentLatency)
		}
	}
}

// ============================================================================
// handleResults Benchmarks – measure the uncached (cold) path at scale
// ============================================================================

func benchHandleResults(b *testing.B, n int) {
	// Build N results with realistic win/loss distributions.
	results := make([]simulation.Result, n)
	for i := range results {
		results[i] = simulation.Result{
			Name:   fmt.Sprintf("Deck-%06d", i),
			Wins:   rand.Intn(50),
			Losses: rand.Intn(50),
		}
	}

	provider := func() []simulation.Result { return results }

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create a fresh server each iteration so the cache is always cold.
		s := NewServer(provider, 0)
		req := httptest.NewRequest("GET", "/api/results", nil)
		w := httptest.NewRecorder()
		s.handleResults(w, req)
	}
}

func BenchmarkHandleResults_100(b *testing.B)    { benchHandleResults(b, 100) }
func BenchmarkHandleResults_1k(b *testing.B)     { benchHandleResults(b, 1000) }
func BenchmarkHandleResults_10k(b *testing.B)    { benchHandleResults(b, 10000) }
func BenchmarkHandleResults_50k(b *testing.B)    { benchHandleResults(b, 50000) }
func BenchmarkHandleResults_100k(b *testing.B)   { benchHandleResults(b, 100000) }

// Sub-benchmarks that measure the individual phases of handleResults so we
// can pinpoint where the time goes.

func benchHandleResultsPhases(b *testing.B, n int) {
	results := make([]simulation.Result, n)
	for i := range results {
		results[i] = simulation.Result{
			Name:   fmt.Sprintf("Deck-%06d", i),
			Wins:   rand.Intn(50),
			Losses: rand.Intn(50),
		}
	}

	server := NewServer(func() []simulation.Result { return results }, 0)
	snapshot := server.provider()

	b.Run("iterate", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
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
			_ = rows
			_ = totalRecords
		}
	})

	rows := make([]deckRow, len(snapshot))
	for i, res := range snapshot {
		rows[i] = deckRow{
			Name:    res.Name,
			Wins:    res.Wins,
			Losses:  res.Losses,
			WinRate: res.WinPercentage(),
		}
	}

	b.Run("sort", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cp := make([]deckRow, len(rows))
			copy(cp, rows)
			sort.Slice(cp, func(i, j int) bool {
				return cp[i].WinRate > cp[j].WinRate
			})
		}
	})

	b.Run("marshal", func(b *testing.B) {
		resp := resultsResponse{
			TotalGames:  n,
			UniqueDecks: n,
			Decks:       rows,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, _ := json.Marshal(resp)
			_ = data
		}
	})
}

func BenchmarkHandleResultsPhases_1k(b *testing.B)   { benchHandleResultsPhases(b, 1000) }
func BenchmarkHandleResultsPhases_10k(b *testing.B)  { benchHandleResultsPhases(b, 10000) }
func BenchmarkHandleResultsPhases_100k(b *testing.B) { benchHandleResultsPhases(b, 100000) }
