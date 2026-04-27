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

	for _, path := range []string{"/", "/api/results", "/api/health"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("path %s: expected 200, got %d", path, w.Code)
		}
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
