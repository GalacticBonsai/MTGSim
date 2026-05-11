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
	"github.com/mtgsim/mtgsim/pkg/stats"
)

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

// Server serves the dashboard.
type Server struct {
	provider     ResultsProvider
	edhProvider  EDHResultsProvider
	edhGames     EDHGamesProvider
	edhSummary   EDHSummaryProvider
	cardLibrary  CardLibraryProvider
	port         int
	mux          *http.ServeMux
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
	Enabled bool                      `json:"enabled"`
	Decks   []simulation.EDHDeckStats `json:"decks"`
	Summary simulation.EDHSummary     `json:"summary"`
}

// handleEDHResults returns per-deck EDH aggregates if an EDH provider
// is registered; otherwise responds with {"enabled": false}.
func (s *Server) handleEDHResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.edhProvider == nil {
		_ = json.NewEncoder(w).Encode(edhResultsResponse{Enabled: false})
		return
	}
	resp := edhResultsResponse{Enabled: true, Decks: s.edhProvider()}
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
	Enabled bool                          `json:"enabled"`
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
		header { padding: 20px 0; border-bottom: 1px solid #1a1f3a; margin-bottom: 20px; }
		h1 { font-size: 2.5em; margin-bottom: 5px; }
		.subtitle { color: #888; }
		.tabs { display: flex; gap: 8px; margin-bottom: 20px; border-bottom: 1px solid #1a1f3a; padding-bottom: 10px; }
		.tab-btn { background: #141829; border: 1px solid #1a1f3a; color: #888; padding: 10px 20px; border-radius: 6px; cursor: pointer; font-size: 0.9em; }
		.tab-btn:hover { background: #1a1f3a; color: #fff; }
		.tab-btn.active { background: #5a6dd8; color: #fff; border-color: #5a6dd8; }
		.tab-content { display: none; }
		.tab-content.active { display: block; }
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

		<div class="tabs">
			<button class="tab-btn active" onclick="showTab('overview', this)">Overview</button>
			<button class="tab-btn" onclick="showTab('edh-decks', this)">EDH Decks</button>
			<button class="tab-btn" onclick="showTab('top-cards', this)">Top Cards</button>
			<button class="tab-btn" onclick="showTab('recent-pods', this)">Recent Pods</button>
			<button class="tab-btn" onclick="showTab('card-library', this)">Card Library</button>
		</div>
		<div id="overview" class="tab-content active">

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

			<div id="edhSummarySection" style="display:none">
				<h3>EDH Tuning Highlights</h3>
				<div class="grid" id="edhSummary">
					<div class="loading">Loading EDH telemetry...</div>
				</div>
			</div>
		</div>
		<div id="edh-decks" class="tab-content">
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
							<th>Life KOs</th>
							<th>Mill KOs</th>
							<th>Avg Cmdr Casts</th>
					<th>Avg Mana</th>
					<th>Avg Cards</th>
					<th>Avg Lands</th>
					<th>Avg Spells</th>
					<th>Avg Creatures</th>
							<th>Avg Combat</th>
							<th>Max Storm</th>
							<th>KOs</th>
						<th>Avg Mulls</th>
					</tr>
				</thead>
				<tbody id="edhDecksBody">
					<tr><td colspan="20" class="loading">Loading...</td></tr>
				</tbody>
			</table>
		</div>

		<div id="top-cards" class="tab-content">
			<h3>Top Cards by Win Rate When Cast</h3>
			<p style="color:#888; margin-bottom:10px;">Cards with at least 5 casts. Sorted by win rate, then by cast volume.</p>
			<table class="table" id="topCards">
				<thead>
					<tr>
						<th>Deck</th>
						<th>Card</th>
						<th>Casts</th>
						<th>Wins</th>
						<th>Win Rate</th>
					</tr>
				</thead>
				<tbody id="topCardsBody">
					<tr><td colspan="5" class="loading">Loading...</td></tr>
				</tbody>
			</table>
		</div>

		<div id="recent-pods" class="tab-content">
				<h3>Recent EDH Pods</h3>
				<table class="table" id="edhGames">
					<thead>
						<tr><th>Winner</th><th>Turns</th><th>Max Storm</th><th>Mana</th><th>Cards</th><th>Combat</th><th>Players</th><th>Events</th><th>Last Event</th></tr>
					</thead>
					<tbody id="edhGamesBody">
						<tr><td colspan="9" class="loading">Loading...</td></tr>
					</tbody>
				</table>
		</div>

		<div id="card-library" class="tab-content">
			<h3>Global Card Library</h3>
			<p style="color:#888; margin-bottom:10px;">Aggregated across all simulation runs. Cards with at least 5 casts.</p>
			<table class="table" id="cardLibrary">
				<thead>
					<tr>
						<th>Card</th>
						<th>Casts</th>
						<th>Wins</th>
						<th>Win Rate</th>
					</tr>
				</thead>
				<tbody id="cardLibraryBody">
					<tr><td colspan="4" class="loading">Loading...</td></tr>
				</tbody>
			</table>
		</div>
	</div>

	<script>
		function showTab(tabId, btn) {
			document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
			document.querySelectorAll('.tab-btn').forEach(el => el.classList.remove('active'));
			document.getElementById(tabId).classList.add('active');
			btn.classList.add('active');
		}
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
					document.getElementById('edhSummarySection').style.display = '';
					renderEDHSummary(data.summary || {});
					renderEDH(data);
					renderTopCards(data.decks || []);
				}
			} catch (err) {
				console.error('Error loading EDH results:', err);
			}
			try {
				const res = await fetch('/api/card-library');
				const data = await res.json();
				if (data.enabled) {
					renderCardLibrary(data.cards || {});
				}
			} catch (err) {
				console.error('Error loading card library:', err);
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
					+ '<td>' + d.life_loss_kos + '</td>'
					+ '<td>' + d.mill_kos + '</td>'
					+ '<td>' + (d.avg_commander_casts || 0).toFixed(2) + '</td>'
					+ '<td>' + (d.avg_mana_spent || 0).toFixed(1) + '</td>'
					+ '<td>' + (d.avg_cards_played || 0).toFixed(1) + '</td>'
					+ '<td>' + (d.avg_lands_played || 0).toFixed(1) + '</td>'
					+ '<td>' + (d.avg_spells_cast || 0).toFixed(1) + '</td>'
					+ '<td>' + (d.avg_creatures_cast || 0).toFixed(1) + '</td>'
					+ '<td>' + (d.avg_combat_damage || 0).toFixed(1) + '</td>'
					+ '<td>' + (d.max_storm_count || 0) + '</td>'
					+ '<td>' + (d.eliminations || 0) + '</td>'
					+ '<td>' + (d.avg_mulligans || 0).toFixed(2) + '</td>'
					+ '</tr>';
			}
				document.getElementById('edhDecksBody').innerHTML = html || '<tr><td colspan="20">No data</td></tr>';
			}

			function renderTopCards(decks) {
			let cards = [];
			for (let d of decks) {
				let cs = d.card_stats || {};
				for (let [name, perf] of Object.entries(cs)) {
					if (perf.casts >= 5) {
						cards.push({
							deck: d.deck_name,
							name: name,
							casts: perf.casts,
							wins: perf.wins,
							winRate: perf.casts > 0 ? (perf.wins / perf.casts * 100) : 0,
							image_url: perf.image_url || ''
						});
					}
				}
			}
			cards.sort((a, b) => b.winRate - a.winRate || b.casts - a.casts);
			cards = cards.slice(0, 100);
			let html = '';
			for (let c of cards) {
				let img = c.image_url ? '<img src="' + c.image_url + '" height="40" style="vertical-align:middle;margin-right:8px;border-radius:4px;" alt="">' : '';
				html += '<tr><td>' + c.deck + '</td><td>' + img + c.name + '</td><td>' + c.casts + '</td><td>' + c.wins + '</td><td><strong>' + c.winRate.toFixed(1) + '%</strong></td></tr>';
			}
			document.getElementById('topCardsBody').innerHTML = html || '<tr><td colspan="5">No cards with enough sample size yet</td></tr>';
		}

			function renderEDHSummary(s) {
				let html = '';
				html += '<div class="card"><h2>EDH Pods</h2><div class="value">' + (s.total_games || 0) + '</div></div>';
				html += '<div class="card"><h2>Average Turns</h2><div class="value">' + (s.average_turns || 0).toFixed(1) + '</div></div>';
				html += '<div class="card"><h2>Highest Storm</h2><div class="value">' + (s.highest_storm_count || 0) + '</div></div>';
				html += '<div class="card"><h2>Total Mana Spent</h2><div class="value">' + (s.total_mana_spent || 0) + '</div><span class="unit">avg ' + (s.average_mana_spent || 0).toFixed(1) + '/pod</span></div>';
				html += '<div class="card"><h2>Total Cards Played</h2><div class="value">' + (s.total_cards_played || 0) + '</div><span class="unit">avg ' + (s.average_cards_played || 0).toFixed(1) + '/pod</span></div>';
				html += '<div class="card"><h2>Combat Damage</h2><div class="value">' + (s.total_combat_damage || 0) + '</div><span class="unit">avg ' + (s.average_combat_damage || 0).toFixed(1) + '/pod</span></div>';
				html += '<div class="card"><h2>Eliminations</h2><div class="value">' + (s.total_eliminations || 0) + '</div><span class="unit">avg ' + (s.average_eliminations || 0).toFixed(1) + '/pod</span></div>';
				document.getElementById('edhSummary').innerHTML = html;
			}

			function renderCardLibrary(cards) {
				let rows = [];
				for (let [name, perf] of Object.entries(cards)) {
					if (perf.casts >= 5) {
						rows.push({
							name: name,
							casts: perf.casts,
							wins: perf.wins,
							winRate: perf.casts > 0 ? (perf.wins / perf.casts * 100) : 0,
							image_url: perf.image_url || ''
						});
					}
				}
				rows.sort((a, b) => b.winRate - a.winRate || b.casts - a.casts);
				rows = rows.slice(0, 100);
				let html = '';
				for (let c of rows) {
					let img = c.image_url ? '<img src="' + c.image_url + '" height="40" style="vertical-align:middle;margin-right:8px;border-radius:4px;" alt="">' : '';
					html += '<tr><td>' + img + c.name + '</td><td>' + c.casts + '</td><td>' + c.wins + '</td><td><strong>' + c.winRate.toFixed(1) + '%</strong></td></tr>';
				}
				document.getElementById('cardLibraryBody').innerHTML = html || '<tr><td colspan="4">No cards with enough sample size yet</td></tr>';
			}

			async function loadEDHGames() {
				try {
					const res = await fetch('/api/edh-games');
					const data = await res.json();
					if (data.enabled) renderEDHGames(data.games || []);
				} catch (err) {
					console.error('Error loading EDH games:', err);
			}
			}

			function renderEDHGames(games) {
				let html = '';
				for (let g of games) {
					const players = (g.Players || []).map(p => p.DeckName + ' mana=' + (p.ManaSpent || 0) + ' cards=' + (p.CardsPlayed || 0) + (p.Eliminated ? ' ✗' : ' ✓')).join('<br>');
					const events = g.Events || [];
					const last = events.length ? events[events.length - 1].kind : '-';
					html += '<tr><td>' + (g.Winner || 'Draw') + '</td><td>' + g.Turns + '</td><td>' + (g.MaxStormCount || 0) + '</td><td>' + (g.TotalManaSpent || 0) + '</td><td>' + (g.TotalCardsPlayed || 0) + '</td><td>' + (g.TotalCombatDamage || 0) + '</td><td>' + players + '</td><td>' + events.length + '</td><td>' + last + '</td></tr>';
			}
				document.getElementById('edhGamesBody').innerHTML = html || '<tr><td colspan="9">No recent pods</td></tr>';
			}

		setInterval(loadResults, 5000);
			setInterval(loadEDHGames, 5000);
		loadResults();
			loadEDHGames();
	</script>
</body>
</html>
`
