// Package dashboard serves a web dashboard for browsing simulation results.
package dashboard

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/mtgsim/mtgsim/pkg/simulation"
)

// ResultsProvider returns a snapshot of the current simulation results.
// The caller is responsible for any synchronization needed to produce the snapshot.
type ResultsProvider func() []simulation.Result

// EDHResultsProvider returns a snapshot of EDH-format aggregate stats
// (per-deck wins, losses, commander damage KOs, etc.). It is optional;
// when nil the dashboard falls back to the 1v1 legacy view only.
type EDHResultsProvider func() []simulation.EDHDeckStats

// Server serves the dashboard.
type Server struct {
	provider    ResultsProvider
	edhProvider EDHResultsProvider
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

// Handler returns the underlying http.Handler (useful for tests).
func (s *Server) Handler() http.Handler {
	s.registerRoutes()
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/results", s.handleResults)
	s.mux.HandleFunc("/api/edh-results", s.handleEDHResults)
	s.mux.HandleFunc("/api/health", s.handleHealth)
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
	sort.Slice(rows, func(i, j int) bool { return rows[i].WinRate > rows[j].WinRate })
	// Each game contributes one win and one loss across the deck pool.
	json.NewEncoder(w).Encode(resultsResponse{
		TotalGames:  totalRecords / 2,
		UniqueDecks: len(rows),
		Decks:       rows,
	})
}

type edhResultsResponse struct {
	Enabled bool                       `json:"enabled"`
	Decks   []simulation.EDHDeckStats  `json:"decks"`
}

// handleEDHResults returns per-deck EDH aggregates if an EDH provider
// is registered; otherwise responds with {"enabled": false}.
func (s *Server) handleEDHResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.edhProvider == nil {
		_ = json.NewEncoder(w).Encode(edhResultsResponse{Enabled: false})
		return
	}
	_ = json.NewEncoder(w).Encode(edhResultsResponse{Enabled: true, Decks: s.edhProvider()})
}

// handleHealth returns server health.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}

// handleIndex returns the HTML dashboard.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(htmlDashboard))
}

const htmlDashboard = `
<!DOCTYPE html>
<html>
<head>
	<title>MTGSim Dashboard</title>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		* { margin: 0; padding: 0; box-sizing: border-box; }
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #0a0e27; color: #fff; }
		.container { max-width: 1200px; margin: 0 auto; padding: 20px; }
		header { padding: 20px 0; border-bottom: 1px solid #1a1f3a; margin-bottom: 30px; }
		h1 { font-size: 2.5em; margin-bottom: 5px; }
		.subtitle { color: #888; }
		.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; margin-bottom: 30px; }
		.card { background: #141829; border: 1px solid #1a1f3a; border-radius: 8px; padding: 20px; }
		.card h2 { font-size: 0.9em; color: #888; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 10px; }
		.card .value { font-size: 2.5em; font-weight: bold; color: #5a6dd8; }
		.card .unit { color: #666; font-size: 0.8em; }
		.table { width: 100%; background: #141829; border: 1px solid #1a1f3a; border-radius: 8px; overflow: hidden; }
		.table th { background: #0a0e27; padding: 15px; text-align: left; border-bottom: 1px solid #1a1f3a; font-weight: 600; color: #888; }
		.table td { padding: 15px; border-bottom: 1px solid #1a1f3a; }
		.table tr:last-child td { border-bottom: none; }
		.table tbody tr:hover { background: #1a1f3a; }
		.bar { height: 20px; background: #5a6dd8; border-radius: 4px; position: relative; }
		.bar-label { position: absolute; left: 5px; top: 50%; transform: translateY(-50%); color: white; font-size: 0.85em; font-weight: bold; }
		.chart-container { background: #141829; border: 1px solid #1a1f3a; border-radius: 8px; padding: 20px; margin-top: 20px; }
		h3 { margin: 20px 0 10px 0; }
		.loading { text-align: center; color: #666; }
	</style>
</head>
<body>
	<div class="container">
		<header>
			<h1>🧙 MTGSim Dashboard</h1>
			<p class="subtitle">Magic: The Gathering Simulation Statistics</p>
		</header>

		<div class="grid" id="summary">
			<div class="loading">Loading...</div>
		</div>

		<h3>Deck Performance</h3>
		<table class="table" id="decks">
			<thead>
				<tr>
					<th>Deck Name</th>
					<th>Wins</th>
					<th>Losses</th>
					<th>Win Rate</th>
				</tr>
			</thead>
			<tbody id="decksBody">
				<tr><td colspan="4" class="loading">Loading...</td></tr>
			</tbody>
		</table>

		<div id="edhSection" style="display:none">
			<h3>EDH / Commander Performance</h3>
			<table class="table" id="edhDecks">
				<thead>
					<tr>
						<th>Deck</th>
						<th>Commander</th>
						<th>Games</th>
						<th>Wins</th>
						<th>Losses</th>
						<th>Win Rate</th>
						<th>Avg Life</th>
						<th>Cmdr Dmg KOs</th>
						<th>Avg Mulls</th>
					</tr>
				</thead>
				<tbody id="edhDecksBody">
					<tr><td colspan="9" class="loading">Loading...</td></tr>
				</tbody>
			</table>
		</div>
	</div>

	<script>
		async function loadResults() {
			try {
				const res = await fetch('/api/results');
				const data = await res.json();
				renderSummary(data);
				renderDecks(data);
			} catch (err) {
				console.error('Error loading results:', err);
			}
			try {
				const res = await fetch('/api/edh-results');
				const data = await res.json();
				if (data.enabled) {
					document.getElementById('edhSection').style.display = '';
					renderEDH(data);
				}
			} catch (err) {
				console.error('Error loading EDH results:', err);
			}
		}

		function renderSummary(data) {
			let html = '';
			html += '<div class="card"><h2>Total Games</h2><div class="value">' + (data.total_games || 0) + '</div></div>';
			html += '<div class="card"><h2>Unique Decks</h2><div class="value">' + (data.unique_decks || 0) + '</div></div>';
			document.getElementById('summary').innerHTML = html;
		}

		function renderDecks(data) {
			const decks = data.decks || [];
			let html = '';
			for (let d of decks) {
				html += '<tr><td>' + d.name + '</td><td>' + d.wins + '</td><td>' + d.losses + '</td><td><strong>' + (d.win_rate || 0).toFixed(1) + '%</strong></td></tr>';
			}
			document.getElementById('decksBody').innerHTML = html || '<tr><td colspan="4">No data</td></tr>';
		}

		function renderEDH(data) {
			const decks = data.decks || [];
			let html = '';
			for (let d of decks) {
				html += '<tr>'
					+ '<td>' + d.deck_name + '</td>'
					+ '<td>' + (d.commander_name || '-') + '</td>'
					+ '<td>' + d.games + '</td>'
					+ '<td>' + d.wins + '</td>'
					+ '<td>' + d.losses + '</td>'
					+ '<td><strong>' + (d.win_rate || 0).toFixed(1) + '%</strong></td>'
					+ '<td>' + (d.avg_final_life || 0).toFixed(1) + '</td>'
					+ '<td>' + d.commander_damage_kos + '</td>'
					+ '<td>' + (d.avg_mulligans || 0).toFixed(2) + '</td>'
					+ '</tr>';
			}
			document.getElementById('edhDecksBody').innerHTML = html || '<tr><td colspan="9">No data</td></tr>';
		}

		setInterval(loadResults, 5000);
		loadResults();
	</script>
</body>
</html>
`
