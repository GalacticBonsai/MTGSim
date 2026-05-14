// Package dashboard serves a web dashboard for browsing simulation results.
package dashboard

import (
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

// Server serves the dashboard.
type Server struct {
	provider    ResultsProvider
	edhProvider EDHResultsProvider
	edhGames    EDHGamesProvider
	edhSummary  EDHSummaryProvider
	cardLibrary CardLibraryProvider
	implReport  ImplementationReportProvider
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
		.th-sort { cursor: pointer; user-select: none; }
		.th-sort:hover { color: #5a6dd8; }
		.th-sort::after { content: ' ⇅'; font-size: 0.8em; opacity: 0.5; }
		.th-sort.asc::after { content: ' ▲'; opacity: 1; }
		.th-sort.desc::after { content: ' ▼'; opacity: 1; }
		.bar { height: 20px; background: #5a6dd8; border-radius: 4px; position: relative; }
		.bar-label { position: absolute; left: 5px; top: 50%; transform: translateY(-50%); color: white; font-size: 0.85em; font-weight: bold; }
		.chart-container { background: #141829; border: 1px solid #1a1f3a; border-radius: 8px; padding: 20px; margin-top: 20px; }
		h3 { margin: 20px 0 10px 0; }
		.loading { text-align: center; color: #666; }
		.search-box { width: 100%; max-width: 400px; background: #0a0e27; border: 1px solid #1a1f3a; color: #fff; padding: 10px 14px; border-radius: 6px; font-size: 0.95em; margin-bottom: 10px; }
		.search-box::placeholder { color: #666; }
		.search-box:focus { outline: none; border-color: #5a6dd8; }
		.search-meta { color: #888; font-size: 0.85em; margin-bottom: 10px; }
		.card-thumb-wrapper {
			position: relative;
			display: inline-block;
		}

		.card-thumb {
			height: 64px;
			border-radius: 6px;
			transition: transform 0.15s ease;
		}

		.card-thumb:hover {
			transform: scale(1.05);
		}

		.card-preview {
			display: none;
			position: absolute;
			z-index: 1000;
			left: 80px;
			top: -40px;
			background: #000;
			padding: 8px;
			border-radius: 10px;
			box-shadow: 0 8px 32px rgba(0,0,0,0.5);
		}

		.card-thumb-wrapper:hover .card-preview {
			display: block;
		}

		.card-preview-img {
			width: 320px;
			border-radius: 12px;
		}
	</style>
</head>
<body>
	<div class="container">
		<header>
			<h1>🧙 MTGSim Dashboard</h1>
			<p class="subtitle">Magic: The Gathering Simulation Statistics</p>
		</header>

		<div class="tabs">
			<button class="tab-btn active" onclick="showTab('overview', this)" title="Aggregate 1v1 simulation results and deck win rates">Overview</button>
			<button class="tab-btn" onclick="showTab('edh-decks', this)" title="Commander deck performance across all pods">EDH Decks</button>
			<button class="tab-btn" onclick="showTab('top-cards', this)" title="Highest win-rate cards when cast (minimum 5 casts)">Top Cards</button>
			<button class="tab-btn" onclick="showTab('recent-pods', this)" title="Last 10 EDH pods with per-player breakdowns">Recent Pods</button>
			<button class="tab-btn" onclick="showTab('card-library', this)" title="Global card statistics across all simulation runs">Card Library</button>
			<button class="tab-btn" onclick="showTab('implementation', this)" title="Card implementation status by parser and engine support">Implementation</button>
		</div>
		<div id="overview" class="tab-content active">

		<div class="grid" id="summary">
			<div class="loading">Loading...</div>
		</div>

		<h3>Deck Performance</h3>
		<table class="table" id="decks">
			<thead>
				<tr>
					<th title="Deck file name">Deck Name</th>
					<th title="Games won">Wins</th>
					<th title="Games lost">Losses</th>
					<th title="Win percentage (Wins / Total Games)">Win Rate</th>
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
						<th class="th-sort" data-sort-key="deck_name" onclick="sortEDH('deck_name')" title="Deck file name">Deck</th>
						<th class="th-sort" data-sort-key="commander_name" onclick="sortEDH('commander_name')" title="Commander card name">Commander</th>
						<th class="th-sort" data-sort-key="games" onclick="sortEDH('games')" title="Total pods played">Games</th>
						<th class="th-sort" data-sort-key="wins" onclick="sortEDH('wins')" title="Pods won">Wins</th>
						<th class="th-sort" data-sort-key="losses" onclick="sortEDH('losses')" title="Pods lost">Losses</th>
						<th class="th-sort" data-sort-key="win_rate" onclick="sortEDH('win_rate')" title="Win percentage (Wins / Games)">Win Rate</th>
						<th class="th-sort" data-sort-key="avg_final_life" onclick="sortEDH('avg_final_life')" title="Average remaining life total at pod end">Avg Life</th>
						<th class="th-sort" data-sort-key="commander_damage_kos" onclick="sortEDH('commander_damage_kos')" title="Eliminations via commander damage">Cmdr Dmg KOs</th>
							<th class="th-sort" data-sort-key="life_loss_kos" onclick="sortEDH('life_loss_kos')" title="Eliminations via life loss (non-commander)">Life KOs</th>
							<th class="th-sort" data-sort-key="mill_kos" onclick="sortEDH('mill_kos')" title="Eliminations via decking or mill">Mill KOs</th>
							<th class="th-sort" data-sort-key="avg_commander_casts" onclick="sortEDH('avg_commander_casts')" title="Average commander casts per pod">Avg Cmdr Casts</th>
					<th class="th-sort" data-sort-key="avg_mana_spent" onclick="sortEDH('avg_mana_spent')" title="Average mana spent per pod">Avg Mana</th>
					<th class="th-sort" data-sort-key="avg_cards_played" onclick="sortEDH('avg_cards_played')" title="Average cards played per pod">Avg Cards</th>
					<th class="th-sort" data-sort-key="avg_lands_played" onclick="sortEDH('avg_lands_played')" title="Average lands played per pod">Avg Lands</th>
					<th class="th-sort" data-sort-key="avg_spells_cast" onclick="sortEDH('avg_spells_cast')" title="Average non-creature spells cast per pod">Avg Spells</th>
					<th class="th-sort" data-sort-key="avg_creatures_cast" onclick="sortEDH('avg_creatures_cast')" title="Average creatures cast per pod">Avg Creatures</th>
							<th class="th-sort" data-sort-key="avg_combat_damage" onclick="sortEDH('avg_combat_damage')" title="Average combat damage dealt per pod">Avg Combat</th>
							<th class="th-sort" data-sort-key="max_storm_count" onclick="sortEDH('max_storm_count')" title="Highest storm count reached">Max Storm</th>
							<th class="th-sort" data-sort-key="eliminations" onclick="sortEDH('eliminations')" title="Total eliminations caused">KOs</th>
						<th class="th-sort" data-sort-key="avg_mulligans" onclick="sortEDH('avg_mulligans')" title="Average mulligans per pod">Avg Mulls</th>
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
						<th title="Deck that played this card">Deck</th>
						<th title="Card name">Card</th>
						<th title="Total times this card was cast">Casts</th>
						<th title="Games won when this card was cast">Wins</th>
						<th title="Win percentage when cast (Wins / Casts)">Win Rate</th>
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
						<tr>
							<th title="Deck that won the pod (or Draw)">Winner</th>
							<th title="Number of turns the pod lasted">Turns</th>
							<th title="Highest storm count reached">Max Storm</th>
							<th title="Total mana spent by all players">Mana</th>
							<th title="Total cards played by all players">Cards</th>
							<th title="Total combat damage dealt">Combat</th>
							<th title="Per-player pod summary">Players</th>
							<th title="Number of game events logged">Events</th>
							<th title="Type of the final game event">Last Event</th>
						</tr>
					</thead>
					<tbody id="edhGamesBody">
						<tr><td colspan="9" class="loading">Loading...</td></tr>
					</tbody>
				</table>
		</div>

		<div id="card-library" class="tab-content">
			<h3>Global Card Library</h3>
			<div style="display:flex;gap:12px;flex-wrap:wrap;align-items:center;margin-bottom:12px;">
	<input
		type="text"
		id="cardLibrarySearch"
		class="search-box"
		placeholder="Search cards..."
		oninput="filterCardLibrary()"
		style="max-width:300px;"
	>

	<label style="color:#888;font-size:0.9em;">
		Min Casts:
		<input
			type="number"
			id="cardLibraryMinCasts"
			value="5"
			min="1"
			oninput="filterCardLibrary()"
			style="width:80px;margin-left:6px;background:#0a0e27;color:#fff;border:1px solid #1a1f3a;padding:6px;border-radius:4px;"
		>
	</label>

	<label style="color:#888;font-size:0.9em;">
		Limit:
		<select
			id="cardLibraryLimit"
			onchange="filterCardLibrary()"
			style="margin-left:6px;background:#0a0e27;color:#fff;border:1px solid #1a1f3a;padding:6px;border-radius:4px;"
		>
			<option value="50">50</option>
			<option value="100" selected>100</option>
			<option value="250">250</option>
			<option value="500">500</option>
		</select>
	</label>
</div>

<div id="cardLibraryMeta" class="search-meta"></div>
			<table class="table" id="cardLibrary">
				<thead>
					<tr>
						<th class="th-sort" onclick="sortCardLibrary('name')">Card</th>
						<th class="th-sort" onclick="sortCardLibrary('casts')">Casts</th>
						<th class="th-sort" onclick="sortCardLibrary('wins')">Wins</th>
						<th class="th-sort" onclick="sortCardLibrary('winRate')">Win Rate</th>
					</tr>
				</thead>
				<tbody id="cardLibraryBody">
					<tr><td colspan="4" class="loading">Loading...</td></tr>
				</tbody>
			</table>
		</div>

		<div id="implementation" class="tab-content">
			<h3>Implementation</h3>
			<p style="color:#888; margin-bottom:10px;">Cards evaluated against the ability parser and execution engine.</p>
			<div class="grid" id="implSummary">
				<div class="loading">Loading...</div>
			</div>
			<h3>By Color</h3>
			<div class="chart-container" id="implByColor" style="max-width:600px;margin:0 auto;">
				<div class="loading">Loading...</div>
			</div>
			<h3>By Set</h3>
			<div class="chart-container" id="implBySet" style="max-width:600px;margin:0 auto;">
				<div class="loading">Loading...</div>
			</div>
			<h3>By Type</h3>
			<div class="chart-container" id="implByType" style="max-width:600px;margin:0 auto;">
				<div class="loading">Loading...</div>
			</div>
			<h3>Unimplemented Cards</h3>
			<input type="text" id="implSearch" class="search-box" placeholder="Search unimplemented cards by name..." oninput="filterImplCards()">
			<div class="search-meta" id="implSearchMeta"></div>
			<table class="table" id="implCards">
				<thead>
					<tr>
						<th onclick="sortImplTable('name')" title="Click to sort by card name">Card ▲▼</th>
						<th onclick="sortImplTable('type')" title="Click to sort by card type">Type ▲▼</th>
						<th onclick="sortImplTable('set')" title="Click to sort by set code">Set ▲▼</th>
						<th onclick="sortImplTable('colors')" title="Click to sort by color identity">Color ▲▼</th>
						<th title="Why the card is not yet fully implemented">Reason</th>
					</tr>
				</thead>
				<tbody id="implCardsBody">
					<tr><td colspan="5" class="loading">Loading...</td></tr>
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

		// globals
		let currentEDHData = null;
		let edhSortState = { key: 'win_rate', dir: 'desc' };
		let cardLibrarySortKey = 'winRate';
		let cardLibrarySortAsc = false;

		function sortEDH(key) {
			if (!currentEDHData || !currentEDHData.decks) return;
			if (edhSortState.key === key) {
				edhSortState.dir = edhSortState.dir === 'asc' ? 'desc' : 'asc';
			} else {
				edhSortState.key = key;
				edhSortState.dir = 'asc';
			}
			document.querySelectorAll('#edhDecks .th-sort').forEach(th => {
				th.classList.remove('asc', 'desc');
				if (th.dataset.sortKey === key) {
					th.classList.add(edhSortState.dir);
				}
			});
			renderEDH(currentEDHData);
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
			try {
				const res = await fetch('/api/implementation');
				const data = await res.json();
				if (data.enabled) {
					renderImplementationStatus(data.report || {});
				}
			} catch (err) {
				console.error('Error loading implementation status:', err);
			}
		}

		function renderSummary(data) {
			let html = '';
			html += '<div class="card" title="Total number of 1v1 games simulated"><h2>Total Games</h2><div class="value">' + (data.total_games || 0) + '</div></div>';
			html += '<div class="card" title="Number of distinct decklists tested"><h2>Unique Decks</h2><div class="value">' + (data.unique_decks || 0) + '</div></div>';
			document.getElementById('summary').innerHTML = html;
		}

		function renderDecks(data) {
			const decks = data.decks || [];

			let html = '';

			if (data.truncated) {
				html += '<tr><td colspan="4" style="color:#888;">'
					+ 'Showing first '
					+ decks.length
					+ ' of '
					+ data.totalDecks
					+ ' decks'
					+ '</td></tr>';
			}

			for (let d of decks) {
				html += '<tr title="' + d.name + ': ' + d.wins + 'W / ' + d.losses + 'L (' + (d.win_rate || 0).toFixed(1) + '% win rate)"><td>' + d.name + '</td><td>' + d.wins + '</td><td>' + d.losses + '</td><td><strong>' + (d.win_rate || 0).toFixed(1) + '%</strong></td></tr>';
			}

			document.getElementById('decksBody').innerHTML =
				html || '<tr><td colspan="4">No data</td></tr>';
		}

		function renderEDH(data) {
			currentEDHData = data;
			const decks = (data.decks || []).slice();
			const key = edhSortState.key;
			const dir = edhSortState.dir;
			const numericKeys = ['games','wins','losses','win_rate','avg_final_life','commander_damage_kos','life_loss_kos','mill_kos','avg_commander_casts','avg_mana_spent','avg_cards_played','avg_lands_played','avg_spells_cast','avg_creatures_cast','avg_combat_damage','max_storm_count','eliminations','avg_mulligans'];
			const isNum = numericKeys.includes(key);
			decks.sort((a, b) => {
				let av = a[key] !== undefined ? a[key] : (isNum ? 0 : '');
				let bv = b[key] !== undefined ? b[key] : (isNum ? 0 : '');
				if (typeof av === 'string') av = av.toLowerCase();
				if (typeof bv === 'string') bv = bv.toLowerCase();
				if (av < bv) return dir === 'asc' ? -1 : 1;
				if (av > bv) return dir === 'asc' ? 1 : -1;
				return 0;
			});
			document.querySelectorAll('#edhDecks .th-sort').forEach(th => {
				th.classList.remove('asc', 'desc');
				if (th.dataset.sortKey === key) {
					th.classList.add(dir);
				}
			});
			let html = '';
			for (let d of decks) {
				let tooltip = d.deck_name + ' (' + (d.commander_name || 'No Commander') + ') — ' + d.games + ' pods, ' + d.wins + 'W / ' + d.losses + 'L';
				html += '<tr title="' + tooltip + '">'
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
				let tooltip = c.name + ' in ' + c.deck + ': ' + c.casts + ' casts, ' + c.wins + ' wins (' + c.winRate.toFixed(1) + '% win rate)';
				html += '<tr title="' + tooltip + '"><td>' + c.deck + '</td><td>' + img + c.name + '</td><td>' + c.casts + '</td><td>' + c.wins + '</td><td><strong>' + c.winRate.toFixed(1) + '%</strong></td></tr>';
			}
			document.getElementById('topCardsBody').innerHTML = html || '<tr><td colspan="5">No cards with enough sample size yet</td></tr>';
		}

			function renderEDHSummary(s) {
				let html = '';
				html += '<div class="card" title="Total number of EDH pods simulated"><h2>EDH Pods</h2><div class="value">' + (s.total_games || 0) + '</div></div>';
				html += '<div class="card" title="Average number of turns per pod"><h2>Average Turns</h2><div class="value">' + (s.average_turns || 0).toFixed(1) + '</div></div>';
				html += '<div class="card" title="Highest storm count reached in any pod"><h2>Highest Storm</h2><div class="value">' + (s.highest_storm_count || 0) + '</div></div>';
				html += '<div class="card" title="Total mana spent by all players across all pods"><h2>Total Mana Spent</h2><div class="value">' + (s.total_mana_spent || 0) + '</div><span class="unit">avg ' + (s.average_mana_spent || 0).toFixed(1) + '/pod</span></div>';
				html += '<div class="card" title="Total cards played by all players across all pods"><h2>Total Cards Played</h2><div class="value">' + (s.total_cards_played || 0) + '</div><span class="unit">avg ' + (s.average_cards_played || 0).toFixed(1) + '/pod</span></div>';
				html += '<div class="card" title="Total combat damage dealt across all pods"><h2>Combat Damage</h2><div class="value">' + (s.total_combat_damage || 0) + '</div><span class="unit">avg ' + (s.average_combat_damage || 0).toFixed(1) + '/pod</span></div>';
				html += '<div class="card" title="Total player eliminations across all pods"><h2>Eliminations</h2><div class="value">' + (s.total_eliminations || 0) + '</div><span class="unit">avg ' + (s.average_eliminations || 0).toFixed(1) + '/pod</span></div>';
				document.getElementById('edhSummary').innerHTML = html;
			}

			function renderCardLibrary(cards) {
				currentCardLibraryRows = [];

				for (let [name, perf] of Object.entries(cards)) {
					currentCardLibraryRows.push({
						name: name,
						casts: perf.casts || 0,
						wins: perf.wins || 0,
						winRate: perf.casts > 0 ? (perf.wins / perf.casts * 100) : 0,
						image_url: perf.image_url || ''
					});
				}

				// IMPORTANT: ensure UI renders even if inputs are missing
				if (!document.getElementById('cardLibrarySearch')) {
					document.getElementById('cardLibraryBody').innerHTML =
						'<tr><td colspan="4">Loading controls...</td></tr>';
					return;
				}

				filterCardLibrary();
			}
			
			function sortCardLibrary(key) {
				if (cardLibrarySortKey === key) {
					cardLibrarySortAsc = !cardLibrarySortAsc;
				} else {
					cardLibrarySortKey = key;
					cardLibrarySortAsc = false;
				}

				filterCardLibrary();
			}

			function filterCardLibrary() {
				let rows = [...currentCardLibraryRows];

				const searchEl = document.getElementById('cardLibrarySearch');
				const minEl = document.getElementById('cardLibraryMinCasts');
				const limitEl = document.getElementById('cardLibraryLimit');

				const search = (searchEl ? searchEl.value : '').toLowerCase();
				const minCasts = parseInt(minEl ? minEl.value : '5');
				const limit = parseInt(limitEl ? limitEl.value : '100');

				rows = rows.filter(r => {
					return r.casts >= minCasts &&
						r.name.toLowerCase().includes(search);
				});

				rows.sort((a, b) => {
					let av = a[cardLibrarySortKey];
					let bv = b[cardLibrarySortKey];

					if (typeof av === 'string') av = av.toLowerCase();
					if (typeof bv === 'string') bv = bv.toLowerCase();

					if (av < bv) return cardLibrarySortAsc ? -1 : 1;
					if (av > bv) return cardLibrarySortAsc ? 1 : -1;
					return 0;
				});

				const total = rows.length;

				rows = rows.slice(0, limit);

				let html = '';

				for (let c of rows) {
					let img = '';

					if (c.image_url) {
						img =
							'<div class="card-thumb-wrapper">' +
								'<img src="' + c.image_url + '" class="card-thumb">' +
								'<div class="card-preview">' +
									'<img src="' + c.image_url + '" class="card-preview-img">' +
								'</div>' +
							'</div>';
					}

					let tooltip =
						c.name + ': ' +
						c.casts + ' casts, ' +
						c.wins + ' wins (' +
						c.winRate.toFixed(1) + '% win rate)';

					html +=
						'<tr title="' + tooltip + '">' +
							'<td>' +
								'<div style="display:flex;align-items:center;gap:12px;">' +
									img +
									'<span>' + c.name + '</span>' +
								'</div>' +
							'</td>' +
							'<td>' + c.casts + '</td>' +
							'<td>' + c.wins + '</td>' +
							'<td><strong>' + c.winRate.toFixed(1) + '%</strong></td>' +
						'</tr>';
				}

				document.getElementById('cardLibraryBody').innerHTML =
					html || '<tr><td colspan="4">No matching cards</td></tr>';

				document.getElementById('cardLibraryMeta').textContent =
					'Showing ' + rows.length + ' of ' + total + ' matching cards';
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
					let tooltip = 'Pod winner: ' + (g.Winner || 'Draw') + ', Turns: ' + g.Turns + ', Storm: ' + (g.MaxStormCount || 0);
					html += '<tr title="' + tooltip + '"><td>' + (g.Winner || 'Draw') + '</td><td>' + g.Turns + '</td><td>' + (g.MaxStormCount || 0) + '</td><td>' + (g.TotalManaSpent || 0) + '</td><td>' + (g.TotalCardsPlayed || 0) + '</td><td>' + (g.TotalCombatDamage || 0) + '</td><td>' + players + '</td><td>' + events.length + '</td><td>' + last + '</td></tr>';
			}
				document.getElementById('edhGamesBody').innerHTML = html || '<tr><td colspan="9">No recent pods</td></tr>';
			}

			function renderStackedBar(label, impl, total) {
				const pct = total > 0 ? (impl / total * 100) : 0;
				const unimpl = total - impl;
				const implPct = pct.toFixed(1);
				const unimplPct = total > 0 ? ((unimpl / total) * 100).toFixed(1) : '0.0';
				let tooltip = label + ': ' + impl + ' implemented out of ' + total + ' cards (' + implPct + '%)';
				return '<div style="margin-bottom:10px;" title="' + tooltip + '">' +
					'<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:4px;">' +
						'<span style="font-size:13px;color:#fff;font-weight:600;">' + label + '</span>' +
						'<span style="font-size:12px;color:#888;">' + impl + '/' + total + ' (' + implPct + '%)</span>' +
					'</div>' +
					'<div style="width:100%;height:18px;background:#141829;border-radius:4px;overflow:hidden;display:flex;">' +
						'<div style="width:' + implPct + '%;height:100%;background:#5a6dd8;display:flex;align-items:center;justify-content:center;font-size:10px;color:#fff;font-weight:bold;white-space:nowrap;overflow:hidden;">' + (impl > 0 ? impl : '') + '</div>' +
						'<div style="width:' + unimplPct + '%;height:100%;background:#e74c3c;display:flex;align-items:center;justify-content:center;font-size:10px;color:#fff;font-weight:bold;white-space:nowrap;overflow:hidden;">' + (unimpl > 0 ? unimpl : '') + '</div>' +
					'</div>' +
					'<div style="display:flex;justify-content:space-between;margin-top:2px;">' +
						'<span style="font-size:10px;color:#5a6dd8;">implemented</span>' +
						'<span style="font-size:10px;color:#e74c3c;">remaining: ' + unimpl + '</span>' +
					'</div>' +
				'</div>';
			}

			let implSortKey = 'name', implSortAsc = true;
			let currentImplData = {};
			let currentImplRows = [];
			let implFilterText = '';
			function applyImplFilterAndSort() {
				let rows = (currentImplData.unimplemented_cards || []).slice();
				let filter = implFilterText.trim().toLowerCase();
				if (filter) {
					rows = rows.filter(c => {
						return (c.name || '').toLowerCase().includes(filter) ||
							(c.type || '').toLowerCase().includes(filter) ||
							(c.set || '').toLowerCase().includes(filter) ||
							(c.colors || '').toLowerCase().includes(filter) ||
							(c.reason || '').toLowerCase().includes(filter);
					});
				}
				rows.sort((a, b) => {
					let av = a[implSortKey] || '', bv = b[implSortKey] || '';
					if (av < bv) return implSortAsc ? -1 : 1;
					if (av > bv) return implSortAsc ? 1 : -1;
					return 0;
				});
				currentImplRows = rows;
				renderImplementationTable(rows);
			}
			function sortImplTable(key) {
				if (implSortKey === key) implSortAsc = !implSortAsc;
				else { implSortKey = key; implSortAsc = true; }
				applyImplFilterAndSort();
			}
			function filterImplCards() {
				implFilterText = document.getElementById('implSearch').value || '';
				applyImplFilterAndSort();
			}
			function renderImplementationTable(rows) {
				let html = '';
				for (let c of rows.slice(0, 500)) {
					let tooltip = (c.name || 'Unknown') + ' — ' + (c.type || 'Unknown') + ' — ' + (c.reason || 'No reason given');
					html += '<tr title="' + tooltip + '"><td>' + (c.name || '') + '</td><td>' + (c.type || '') + '</td><td>' + (c.set || '') + '</td><td>' + (c.colors || 'C') + '</td><td>' + (c.reason || '') + '</td></tr>';
				}
				document.getElementById('implCardsBody').innerHTML = html || '<tr><td colspan="5">No unimplemented cards match this search. The card may be fully implemented or not in the database.</td></tr>';
				let total = (currentImplData.unimplemented_cards || []).length;
				let meta = '';
				if (implFilterText.trim()) {
					meta = 'Showing ' + rows.length + ' of ' + total + ' unimplemented cards (search: "' + implFilterText.trim() + '")';
					if (rows.length > 500) meta += '. Display limited to first 500 results.';
				} else {
					meta = 'Showing ' + Math.min(rows.length, 500) + ' of ' + total + ' unimplemented cards';
					if (rows.length > 500) meta += '. Scroll or refine search to see more.';
				}
				document.getElementById('implSearchMeta').textContent = meta;
			}
			function renderImplementationStatus(report) {
				currentImplData = report;
				implFilterText = '';
				let total = report.total_cards || 0;
				let impl = report.implemented_count || 0;
				let unimpl = report.unimplemented_count || 0;
				let pct = report.percentage || 0;
				let summaryHtml = '<div class="card" title="Total cards in the card database"><h2>Total Cards</h2><div class="value">' + total + '</div></div>';
				summaryHtml += '<div class="card" title="Cards fully supported by the ability parser and execution engine"><h2>Implemented</h2><div class="value">' + impl + '</div><span class="unit">' + pct.toFixed(1) + '%</span></div>';
				summaryHtml += '<div class="card" title="Cards the engine cannot yet fully execute"><h2>Unimplemented</h2><div class="value">' + unimpl + '</div><span class="unit">' + (100 - pct).toFixed(1) + '%</span></div>';
				document.getElementById('implSummary').innerHTML = summaryHtml;

				let colorHtml = '';
				for (let b of (report.by_color || [])) {
					colorHtml += renderStackedBar(b.name, b.implemented, b.total);
				}
				document.getElementById('implByColor').innerHTML = colorHtml || '<div class="loading">No data</div>';

				let setHtml = '';
				for (let b of (report.by_set || [])) {
					setHtml += renderStackedBar(b.name, b.implemented, b.total);
				}
				document.getElementById('implBySet').innerHTML = setHtml || '<div class="loading">No data</div>';

				let typeHtml = '';
				for (let b of (report.by_type || [])) {
					typeHtml += renderStackedBar(b.name, b.implemented, b.total);
				}
				document.getElementById('implByType').innerHTML = typeHtml || '<div class="loading">No data</div>';

				applyImplFilterAndSort();
			}

		setInterval(loadResults, 5000);
			setInterval(loadEDHGames, 5000);
		loadResults();
			loadEDHGames();
	</script>
</body>
</html>
`
