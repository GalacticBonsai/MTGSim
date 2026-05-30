# MTGSim Results & Card Statistics Persistence Plan

## Executive Summary

This plan outlines comprehensive persistence of:
1. **Game Results**: Detailed logs of every game (participants, turns, final state, replay data)
2. **Card Statistics**: Per-card metrics (cast counts, win rates, average performance)
3. **Aggregate Metrics**: Deck performance, meta analysis, format trends
4. **Replay Data**: Full event logs for post-game analysis and replay

This enables deep analytics, debugging, and long-term meta tracking.

---

## Phase 1: Enhanced Schema for Game Results

### 1.1 Core Game Result Tables

```sql
-- 1v1 Game Results (detailed)
CREATE TABLE game_results_1v1 (
  id INTEGER PRIMARY KEY,
  deck1_id INTEGER NOT NULL REFERENCES decks(id),
  deck2_id INTEGER NOT NULL REFERENCES decks(id),
  winner_id INTEGER NOT NULL REFERENCES decks(id),
  loser_id INTEGER NOT NULL REFERENCES decks(id),
  turns_played INT,
  winner_final_life INT,
  loser_final_life INT,
  winner_hand_size INT,
  loser_hand_size INT,
  duration_ms INT,
  random_seed BIGINT,  -- For replay
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  INDEX idx_deck1(deck1_id),
  INDEX idx_deck2(deck2_id),
  INDEX idx_winner(winner_id),
  INDEX idx_created(completed_at)
);

-- EDH Pod Results
CREATE TABLE edh_pod_results (
  id INTEGER PRIMARY KEY,
  pod_number INT,
  player_count INT,
  pod_seed BIGINT,
  total_turns INT,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  INDEX idx_pod_number(pod_number),
  INDEX idx_created(completed_at)
);

-- EDH Pod Seats (each player in a pod)
CREATE TABLE edh_pod_seats (
  id INTEGER PRIMARY KEY,
  pod_id INTEGER NOT NULL REFERENCES edh_pod_results(id),
  seat INT,  -- 0-5 (seat order at table)
  deck_id INTEGER NOT NULL REFERENCES decks(id),
  final_life INT,
  eliminated_turn INT,  -- NULL if survived
  elimination_reason TEXT,  -- 'damage', 'commander_tax', 'milled_out', etc.
  commander_damage_from_id INTEGER REFERENCES decks(id),  -- Who dealt the KO if relevant
  damage_dealt INT,  -- Total damage this deck dealt
  damage_taken INT,  -- Total damage this deck took
  creatures_at_end INT,
  permanents_at_end INT,
  hand_size_at_end INT,
  created_at TIMESTAMP,
  INDEX idx_pod(pod_id),
  INDEX idx_deck(deck_id)
);
```

### 1.2 Replay & Event Logs

```sql
-- Game Events (turn-by-turn log for replay)
CREATE TABLE game_events (
  id INTEGER PRIMARY KEY,
  game_id INT,  -- References game_results_1v1.id OR NULL for EDH
  pod_id INT REFERENCES edh_pod_results(id),  -- For EDH pods
  event_type TEXT,  -- 'turn_start', 'cast_spell', 'attack', 'block', 'damage', 'elimination', etc.
  player_id INTEGER,  -- Which player this event belongs to
  turn_number INT,
  step_name TEXT,  -- 'main1', 'combat', 'main2', 'endstep'
  card_name TEXT,  -- If spell cast or ability triggered
  details JSON,  -- Extra data: {targets: [...], damage: N, source: cardName, ...}
  sequence_number INT,  -- Order within turn
  created_at TIMESTAMP,
  INDEX idx_game(game_id),
  INDEX idx_pod(pod_id),
  INDEX idx_player(player_id),
  INDEX idx_turn(turn_number)
);

-- Replay Snapshots (optional: full game state at key points)
CREATE TABLE replay_snapshots (
  id INTEGER PRIMARY KEY,
  game_id INT,
  pod_id INT,
  turn_number INT,
  player_state JSON,  -- {hand: [...], battlefield: [...], graveyard: [...], library_size: N, ...} per player
  captured_at TIMESTAMP
);
```

### 1.3 Aggregate Statistics Tables

```sql
-- Per-Deck Statistics (updated periodically)
CREATE TABLE deck_stats (
  deck_id INTEGER PRIMARY KEY REFERENCES decks(id),
  -- 1v1 Stats
  games_1v1_total INT DEFAULT 0,
  games_1v1_wins INT DEFAULT 0,
  games_1v1_losses INT DEFAULT 0,
  games_1v1_win_rate FLOAT,  -- wins / total
  games_1v1_avg_turns FLOAT,
  games_1v1_avg_winner_life FLOAT,
  games_1v1_avg_loser_life FLOAT,
  -- EDH Stats
  games_edh_total INT DEFAULT 0,
  games_edh_top4 INT DEFAULT 0,  -- Survived to 4 or fewer players
  games_edh_top2 INT DEFAULT 0,  -- Final 2 standing
  games_edh_wins INT DEFAULT 0,  -- Sole survivor
  games_edh_avg_placement FLOAT,  -- 1-6 scale
  games_edh_avg_final_life FLOAT,
  games_edh_cmdr_damage_kos INT,
  games_edh_avg_turns FLOAT,
  -- Combined
  total_games INT GENERATED AS (games_1v1_total + games_edh_total),
  last_updated TIMESTAMP,
  INDEX idx_updated(last_updated)
);

-- Head-to-Head Records (for common matchups)
CREATE TABLE matchup_records (
  id INTEGER PRIMARY KEY,
  deck1_id INTEGER NOT NULL REFERENCES decks(id),
  deck2_id INTEGER NOT NULL REFERENCES decks(id),
  games_played INT,
  deck1_wins INT,
  deck2_wins INT,
  win_rate_d1 FLOAT,  -- deck1 wins / total
  avg_turns FLOAT,
  last_played TIMESTAMP,
  UNIQUE(deck1_id, deck2_id),
  INDEX idx_d1(deck1_id),
  INDEX idx_d2(deck2_id)
);
```

---

## Phase 2: Card-Level Statistics

### 2.1 Card Performance Tables

```sql
-- Card Cast Statistics (per deck)
CREATE TABLE card_casts (
  id INTEGER PRIMARY KEY,
  deck_id INTEGER NOT NULL REFERENCES decks(id),
  card_name TEXT,
  times_cast INT,
  times_cast_in_winning_game INT,
  times_cast_in_losing_game INT,
  win_rate_when_cast FLOAT,  -- (wins_with_cast / games_with_cast)
  avg_turn_cast INT,  -- Average turn card is cast
  last_cast_at TIMESTAMP,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  UNIQUE(deck_id, card_name),
  INDEX idx_deck(deck_id),
  INDEX idx_win_rate(win_rate_when_cast DESC)
);

-- Card Global Statistics (across all decks using that card)
CREATE TABLE card_global_stats (
  id INTEGER PRIMARY KEY,
  card_name TEXT UNIQUE,
  total_casts INT,
  total_casts_in_winning_games INT,
  total_casts_in_losing_games INT,
  global_win_rate FLOAT,  -- Overall win rate when this card is cast
  num_decks_using INT,  -- How many decks run this card
  avg_deck_count FLOAT,  -- Average number of copies per deck
  format TEXT,  -- '1v1', 'edh', 'all'
  last_updated TIMESTAMP
);

-- Card Mulligan Decisions (optional: track if card influences mulligan decisions)
CREATE TABLE card_mulligan_impact (
  card_name TEXT PRIMARY KEY,
  times_hand_kept_with_card INT,
  times_hand_mulliganed_with_card INT,
  keep_rate FLOAT,  -- kept / (kept + mulliganed)
  updated_at TIMESTAMP
);
```

### 2.2 Ability & Trigger Statistics (optional, advanced)

```sql
-- Track which abilities/triggers fired most often
CREATE TABLE ability_triggers (
  id INTEGER PRIMARY KEY,
  game_id INT,
  pod_id INT,
  card_name TEXT,
  ability_description TEXT,  -- "Draw a card", "Deal 1 damage", etc.
  trigger_count INT,
  turn_number INT,
  created_at TIMESTAMP
);

-- Synergy Metrics (optional: which card combinations work well together)
CREATE TABLE card_synergies (
  id INTEGER PRIMARY KEY,
  deck_id INTEGER NOT NULL REFERENCES decks(id),
  card1 TEXT,
  card2 TEXT,
  times_both_in_play INT,
  times_both_in_winning_game INT,
  synergy_score FLOAT,  -- How much this pair contributes to wins
  last_updated TIMESTAMP,
  UNIQUE(deck_id, card1, card2)
);
```

---

## Phase 3: Data Collection & Insertion

### 3.1 Enhanced Game Engine: `pkg/simulation-engine/result_recorder.go`

```go
package simulationengine

import (
	"database/sql"
	"time"
)

// ResultRecorder captures game events and persists to database
type ResultRecorder struct {
	db *sql.DB
	gameID int
	podID int
	events []GameEvent
	eventMutex sync.Mutex
}

type GameEvent struct {
	Type           string    // 'turn_start', 'cast_spell', 'attack', 'damage', 'elimination'
	TurnNumber     int
	StepName       string    // 'main1', 'combat', 'main2', 'endstep'
	PlayerID       int
	CardName       string
	Details        json.RawMessage  // Extra context
	SequenceNumber int
	Timestamp      time.Time
}

// RecordEvent appends a game event
func (rr *ResultRecorder) RecordEvent(event GameEvent) {
	rr.eventMutex.Lock()
	defer rr.eventMutex.Unlock()
	rr.events = append(rr.events, event)
}

// RecordGameResult persists 1v1 result to database
func (rr *ResultRecorder) RecordGameResult1v1(
	deck1ID, deck2ID, winnerID int,
	turns, winnerLife, loserLife int,
	duration time.Duration,
	seed int64,
) error {
	// 1. INSERT into game_results_1v1
	// 2. Get returned game_id
	// 3. Batch INSERT all events into game_events
	// 4. Trigger stats aggregation
}

// RecordEDHPodResult persists EDH pod to database
func (rr *ResultRecorder) RecordEDHPodResult(
	seats []*EDHSeat,
	totalTurns int,
	seed int64,
) error {
	// 1. INSERT into edh_pod_results
	// 2. INSERT each seat into edh_pod_seats
	// 3. Batch INSERT all events
	// 4. Trigger stats aggregation
}

// FlushToDB writes all buffered events to database
func (rr *ResultRecorder) FlushToDB() error {
	// Batch insert events for efficiency
}
```

### 3.2 Card Tracking: `pkg/simulation-engine/card_tracker.go`

```go
package simulationengine

import (
	"sync"
)

// CardTracker tracks per-deck, per-game card statistics
type CardTracker struct {
	deckID int
	gameWon bool
	casts map[string]CastInfo  // card name -> cast metadata
	mu sync.Mutex
}

type CastInfo struct {
	TimesCast      int
	TurnsCast      []int  // Which turns were this cast
	WasCastInWin    bool
	WasCastInLoss   bool
}

// RecordCast logs a spell cast
func (ct *CardTracker) RecordCast(cardName string, turnNumber int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if ct.casts[cardName] == nil {
		ct.casts[cardName] = &CastInfo{}
	}
	ct.casts[cardName].TimessCast++
	ct.casts[cardName].TurnssCast = append(ct.casts[cardName].TurnssCast, turnNumber)
}

// FinalizGame marks the game as won/lost and marks all casts accordingly
func (ct *CardTracker) FinalizeGame(gameWon bool) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	for _, info := range ct.casts {
		if gameWon {
			info.WasCastInWin = true
		} else {
			info.WasCastInLoss = true
		}
	}
	ct.gameWon = gameWon
}

// PersistToDB writes card stats for this game to database
func (ct *CardTracker) PersistToDB(db *sql.DB) error {
	// For each card cast:
	// 1. INSERT or UPDATE card_casts
	// 2. INSERT or UPDATE card_global_stats
	// Batch operation for efficiency
}
```

### 3.3 Integration in Game Runner: `cmd/mtgsim-runner/worker.go`

```go
func (w *Worker) runBatch() {
	for i := 0; i < w.gamesPerBatch; i++ {
		// Load decks
		d1, _ := w.engine.LoadDeckFromDB(w.db, uploadedDeckID)
		d2, _ := w.engine.LoadDeckFromDB(w.db, preloadedDeckID)

		// Create recorders
		recorder := simulationengine.NewResultRecorder(w.db)
		tracker1 := simulationengine.NewCardTracker(uploadedDeckID)
		tracker2 := simulationengine.NewCardTracker(preloadedDeckID)

		// Run game (pass recorder to hook into events)
		winner, loser := w.engine.SimulateGame(d1, d2, recorder, []*CardTracker{tracker1, tracker2})

		// Finalize and persist
		tracker1.FinalizeGame(winner == p1)
		tracker2.FinalizeGame(winner == p2)
		
		recorder.RecordGameResult1v1(d1.ID, d2.ID, winner.ID, turns, ...)
		recorder.FlushToDB()

		tracker1.PersistToDB(w.db)
		tracker2.PersistToDB(w.db)

		w.updateStats()  // Recalculate aggregates
	}
}
```

---

## Phase 4: Statistics Aggregation

### 4.1 Stats Aggregator: `pkg/stats-aggregator/aggregator.go`

```go
package statsaggregator

import (
	"database/sql"
	"time"
)

// StatsAggregator recalculates all deck and card statistics
type StatsAggregator struct {
	db *sql.DB
}

// AggregateAllStats recalculates all statistics (run periodically)
func (sa *StatsAggregator) AggregateAllStats() error {
	// 1. Get all decks
	// 2. For each deck:
	//    - Count wins/losses in game_results_1v1
	//    - Count EDH placements
	//    - Calculate averages
	//    - UPDATE deck_stats
	// 3. Recalculate matchup_records
	// 4. Recalculate card_global_stats
}

// AggregateDeckStats recalculates stats for a single deck
func (sa *StatsAggregator) AggregateDeckStats(deckID int) error {
	// 1v1 Stats
	var games1v1Total, wins1v1 int
	row := sa.db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN winner_id = ? THEN 1 ELSE 0 END)
		FROM game_results_1v1
		WHERE deck1_id = ? OR deck2_id = ?
	`, deckID, deckID, deckID)
	row.Scan(&games1v1Total, &wins1v1)

	// EDH Stats
	var gamesEdhTotal, edhWins int
	row = sa.db.QueryRow(`
		SELECT COUNT(DISTINCT pod_id), SUM(CASE WHEN eliminated_turn IS NULL THEN 1 ELSE 0 END)
		FROM edh_pod_seats
		WHERE deck_id = ?
	`, deckID)
	row.Scan(&gamesEdhTotal, &edhWins)

	// Average turns, life, etc.
	// ...

	// UPDATE deck_stats
}

// AggregateMatchupRecords updates head-to-head stats
func (sa *StatsAggregator) AggregateMatchupRecords() error {
	// For each pair of decks that have played:
	// - Count games between them
	// - Calculate win rate for deck1 vs deck2
	// - INSERT or UPDATE matchup_records
}

// AggregateCardStats recalculates global card statistics
func (sa *StatsAggregator) AggregateCardStats(format string) error {
	// For each card:
	// - Count total times cast
	// - Count wins when cast / losses when cast
	// - Calculate win rate
	// - Count decks using it
	// - UPDATE card_global_stats
}
```

### 4.2 Periodic Aggregation: `cmd/mtgsim-runner/stats_updater.go`

```go
// Run periodically (every 5-10 minutes, or after each batch)
func (w *Worker) updateStats() {
	aggregator := statsaggregator.New(w.db)
	if err := aggregator.AggregateAllStats(); err != nil {
		logger.LogMeta("Stats aggregation failed: %v", err)
	}
}
```

---

## Phase 5: Results & Analytics API

### 5.1 Dashboard API Endpoints: `pkg/dashboard/results_handlers.go`

```go
// GET /api/results/games?limit=100&offset=0&deck_id=?&format=1v1|edh
// → Recent game results with basic info

// GET /api/results/games/:id
// → Detailed game result with full event log

// GET /api/results/replays/:game_id
// → Full replay with event sequence (for watching/debugging)

// GET /api/results/stats/deck/:deck_id
// → All statistics for a single deck

// GET /api/results/stats/leaderboard?format=1v1|edh|all
// → Top performers sorted by win rate

// GET /api/results/stats/matchup/:deck1_id/:deck2_id
// → Head-to-head record between two decks

// GET /api/results/cards/:card_name
// → Global statistics for a card (cast rate, win rate, etc.)

// GET /api/results/cards/:card_name/in-deck/:deck_id
// → Card performance within a specific deck

// GET /api/results/meta-analysis?format=edh
// → Meta analysis: most-played cards, color distribution, etc.

// GET /api/results/trends?days=30
// → Win rate trends over time, emerging decks, etc.
```

### 5.2 Response Examples

**GET /api/results/games/123**
```json
{
  "id": 123,
  "format": "1v1",
  "deck1": {
    "id": 5,
    "name": "Krenko EDH",
    "final_life": 20
  },
  "deck2": {
    "id": 8,
    "name": "Chandra EDH",
    "final_life": 0
  },
  "winner_id": 5,
  "turns": 8,
  "duration_ms": 245,
  "completed_at": "2026-05-29T14:32:00Z",
  "events": [
    {
      "turn": 1,
      "step": "main1",
      "type": "cast_spell",
      "card": "Llanowar Elves",
      "player": "Krenko"
    },
    {
      "turn": 2,
      "step": "main1",
      "type": "cast_spell",
      "card": "Goblin Recruiter",
      "player": "Krenko"
    }
    // ... more events
  ]
}
```

**GET /api/results/cards/Lightning Bolt**
```json
{
  "card_name": "Lightning Bolt",
  "total_casts": 4521,
  "total_casts_in_winning_games": 2887,
  "total_casts_in_losing_games": 1634,
  "global_win_rate": 0.639,
  "num_decks_using": 287,
  "avg_copies_per_deck": 3.1,
  "format": "1v1",
  "last_updated": "2026-05-29T15:00:00Z",
  "by_deck": [
    {
      "deck_id": 5,
      "deck_name": "Red Aggressive",
      "times_cast": 145,
      "win_rate_when_cast": 0.72,
      "avg_turn_cast": 2.1
    }
  ]
}
```

**GET /api/results/stats/leaderboard?format=edh**
```json
{
  "leaderboard": [
    {
      "rank": 1,
      "deck_id": 42,
      "deck_name": "Zur Control",
      "total_games": 87,
      "edh_wins": 34,
      "avg_placement": 1.8,
      "win_rate": 0.391,
      "avg_final_life": 32.5
    }
  ]
}
```

---

## Phase 6: Data Retention & Archival

### 6.1 Data Retention Policy

```sql
-- Keep detailed event logs for 90 days
DELETE FROM game_events 
WHERE created_at < datetime('now', '-90 days');

-- Keep detailed game results for 1 year
DELETE FROM game_results_1v1
WHERE completed_at < datetime('now', '-1 year');

-- Keep aggregated stats forever (small table)
-- Keep card stats forever (useful for meta tracking)

-- Archive old replays to cold storage (optional)
CREATE TABLE replay_archives (
  id INTEGER PRIMARY KEY,
  game_id INT,
  archive_path TEXT,  -- S3, GCS, local disk
  archived_at TIMESTAMP
);
```

### 6.2 Maintenance Routine

**File**: `cmd/mtgsim-runner/maintenance.go`

```go
// Run daily (e.g., 2 AM)
func (w *Worker) runMaintenanceCycle() {
	// 1. Delete old event logs (> 90 days)
	// 2. Vacuum database (optimize disk space)
	// 3. Backup database to external storage
	// 4. Recalculate all statistics
	// 5. Log summary
}
```

---

## Phase 7: Database Optimization

### 7.1 Indexing Strategy

```sql
-- Fast lookups for common queries
CREATE INDEX idx_game_results_1v1_winner ON game_results_1v1(winner_id);
CREATE INDEX idx_game_results_1v1_loser ON game_results_1v1(loser_id);
CREATE INDEX idx_game_results_1v1_decks ON game_results_1v1(deck1_id, deck2_id);
CREATE INDEX idx_game_results_1v1_created ON game_results_1v1(completed_at DESC);

CREATE INDEX idx_edh_pod_seats_deck ON edh_pod_seats(deck_id);
CREATE INDEX idx_edh_pod_seats_pod ON edh_pod_seats(pod_id);

CREATE INDEX idx_card_casts_deck_card ON card_casts(deck_id, card_name);
CREATE INDEX idx_card_casts_win_rate ON card_casts(win_rate_when_cast DESC);

CREATE INDEX idx_game_events_game ON game_events(game_id);
CREATE INDEX idx_game_events_pod ON game_events(pod_id);
CREATE INDEX idx_game_events_turn ON game_events(turn_number);

-- Unique constraints (prevent duplicates)
CREATE UNIQUE INDEX idx_matchup_records ON matchup_records(deck1_id, deck2_id);
CREATE UNIQUE INDEX idx_card_casts ON card_casts(deck_id, card_name);
```

### 7.2 Partitioning (for large datasets)

For >1M game results, partition by date:

```sql
-- SQLite doesn't support partitioning natively, but PostgreSQL does:
CREATE TABLE game_results_1v1_2026_05 PARTITION OF game_results_1v1
  FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
```

---

## Phase 8: Reporting & Dashboards

### 8.1 New Dashboard Pages

1. **Game Browser**
   - Filter by deck, date range, format
   - View game replays with event log
   - Compare win rates across matchups

2. **Card Statistics**
   - Most cast cards (global)
   - Highest win-rate cards
   - Cards by color/type
   - Card synergies

3. **Meta Report**
   - Top decks by win rate
   - Color distribution
   - Format-specific breakdowns
   - Trends over time

4. **Leaderboards**
   - All-time best decks (by format)
   - Recent winners
   - Head-to-head matchup records

5. **Analytics**
   - Win rate trends (30-day rolling average)
   - Emerging decks (gaining win rate)
   - Declining decks
   - Color balance analysis

---

## Implementation Roadmap

### Week 1: Schema & Ingestion
- [ ] Design and create database schema for results (Phase 1)
- [ ] Implement ResultRecorder in game engine
- [ ] Test event capture with sample games

### Week 2: Card Tracking
- [ ] Implement CardTracker
- [ ] Integrate into game engine
- [ ] Verify card statistics are captured correctly

### Week 3: Aggregation
- [ ] Implement StatsAggregator
- [ ] Schedule periodic aggregation
- [ ] Verify stats are calculated correctly

### Week 4: API & Dashboard
- [ ] Add API endpoints for results querying
- [ ] Create game browser UI
- [ ] Create card statistics UI

### Week 5: Advanced Analytics
- [ ] Add meta-analysis endpoints
- [ ] Create reporting dashboard
- [ ] Implement data retention policies
- [ ] Performance testing & optimization

---

## Estimated Storage Requirements

```
Assumptions:
- 10,000 games/day
- 100 decks
- ~500 cards per game
- ~50 events per game

Daily Storage:
- game_results_1v1:      ~500 KB (10K records × 50 bytes)
- edh_pod_results:       ~200 KB (if running EDH)
- game_events:           ~2.5 MB (10K × 50 events × 50 bytes)
- card_casts:            ~300 KB (updates to existing records)
Total per day:           ~3.5 MB

Monthly:                ~105 MB
Yearly (with 90-day event retention): ~500 MB

This is manageable with SQLite, or scale to PostgreSQL if needed.
```

---

## Key Benefits

| Capability | Before | After |
|------------|--------|-------|
| **View Past Games** | Not possible | Browse + replay any game |
| **Card Analysis** | Rough tracking only | Per-deck and global card stats |
| **Matchup History** | Unknown | Detailed head-to-head records |
| **Meta Trends** | No data | Color distribution, emerging decks |
| **Debugging** | Run game again | Replay exact game sequence |
| **Long-term Analytics** | No history | 1+ year of historical data |
| **Leaderboards** | Per-batch only | Persistent rankings |

---

## Questions & Decisions

- [ ] **Event Log Retention**: Keep all events forever, or just 90 days?
- [ ] **Replay Granularity**: Store full game state snapshots or just events?
- [ ] **Real-time Queries**: Dashboard polls for updates, or WebSocket streaming?
- [ ] **Card Name Normalization**: Handle alternate names, printings, etc.?
- [ ] **Custom Analytics**: Support user-defined queries/reports?

---

## Integration with Other Plans

This Results Persistence Plan works alongside:
- **ARCHITECTURE_PLAN.md**: Game runner service persists results to database
- **DECK_MIGRATION_PLAN.md**: Deck IDs used in all results records
- Both are required for a complete modern architecture

---

## Next Steps

1. Review schema design for potential optimizations
2. Implement Phase 1 (schema creation)
3. Integrate ResultRecorder into existing game engine
4. Build aggregation pipeline
5. Create API endpoints and UI for browsing results
6. Monitor database size and adjust retention policies as needed
