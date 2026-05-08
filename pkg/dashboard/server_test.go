package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/simulation"
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
	if server.port != 8080 {
		t.Errorf("expected port 8080, got %d", server.port)
	}
	if server.provider == nil {
		t.Error("expected provider to be set")
	}
}
