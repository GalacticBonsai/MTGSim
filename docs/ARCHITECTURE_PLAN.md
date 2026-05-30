# MTGSim Architecture Modernization Plan

## Executive Summary

This plan outlines the transition of MTGSim from a monolithic game runner + dashboard to a decoupled service architecture:
- **Background Service**: Game simulation engine running continuously, persisting results to database
- **Dashboard Frontend**: Lightweight, long-running service that queries the database for metrics
- **Intelligent Deck Rotation**: Uploaded decks tested one-at-a-time against a stable collection of pre-downloaded decks, with automatic rotation between games

---

## Phase 1: Database Layer (Foundation)

### 1.1 Database Schema Design

Create a database (SQLite for simplicity, PostgreSQL for scalability) with these core tables:

```sql
-- Deck Registry
CREATE TABLE decks (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE,
  deck_path TEXT,
  is_preloaded BOOLEAN,  -- True for pre-downloaded, False for uploaded
  created_at TIMESTAMP,
  last_used TIMESTAMP
);

-- Game Results (1v1)
CREATE TABLE game_results (
  id INTEGER PRIMARY KEY,
  deck1_id INTEGER REFERENCES decks(id),
  deck2_id INTEGER REFERENCES decks(id),
  winner_id INTEGER REFERENCES decks(id),
  turns INT,
  duration_ms INT,
  created_at TIMESTAMP,
  FOREIGN KEY (deck1_id, deck2_id) REFERENCES decks(id)
);

-- EDH Pod Results
CREATE TABLE edh_pod_results (
  id INTEGER PRIMARY KEY,
  pod_number INT,
  player_count INT,
  created_at TIMESTAMP
);

CREATE TABLE edh_pod_players (
  id INTEGER PRIMARY KEY,
  pod_id INTEGER REFERENCES edh_pod_results(id),
  seat INT,
  deck_id INTEGER REFERENCES decks(id),
  final_life INT,
  eliminated_turn INT,
  commander_damage_ko BOOLEAN,
  created_at TIMESTAMP
);

-- Deck Rotation / Queue
CREATE TABLE deck_queue (
  id INTEGER PRIMARY KEY,
  uploaded_deck_id INTEGER REFERENCES decks(id) NOT NULL,
  rotation_order INT,  -- Round-robin index
  next_run_index INT,  -- Which preloaded deck to test against next
  games_completed INT,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

-- Game Statistics (Aggregated per Deck)
CREATE TABLE deck_stats (
  deck_id INTEGER PRIMARY KEY REFERENCES decks(id),
  total_games INT,
  wins INT,
  losses INT,
  win_rate FLOAT,
  avg_turns FLOAT,
  avg_final_life FLOAT,
  last_updated TIMESTAMP
);
```

### 1.2 Create `pkg/database` Package

**File**: `pkg/database/db.go`
- Implement database connection management
- Migrations system (schema versioning)
- Query helper functions for common operations

**File**: `pkg/database/models.go`
- Define Go structs mirroring schema tables
- Implement marshaling/unmarshaling logic

**File**: `pkg/database/operations.go`
- CRUD operations for decks, results, stats
- Batch insert functions for performance
- Stats aggregation queries (win rates, averages)

---

## Phase 2: Game Runner Service

### 2.1 Extract Game Simulation Logic into `pkg/simulation-engine`

**File**: `pkg/simulation-engine/engine.go`
- Decouple game simulation from I/O
- Accept deck instances, return structured game result
- No HTTP, no file I/O — pure game logic

**File**: `pkg/simulation-engine/edh_engine.go`
- EDH-specific simulation (pods, multiplayer logic)
- Return structured pod results

**File**: `pkg/simulation-engine/deck_rotation.go`
- Track which uploaded deck is "active" in rotation
- Determine which preloaded deck to test against next
- Implement round-robin or weighted strategy

### 2.2 Create Background Service: `cmd/mtgsim-runner`

**File**: `cmd/mtgsim-runner/main.go`
- Continuous background service (runs as daemon/systemd service)
- Command-line flags:
  - `--db-path` (SQLite) or `--db-url` (PostgreSQL)
  - `--preloaded-dir` (path to pre-downloaded decks)
  - `--uploaded-dir` (path to uploaded decks)
  - `--pod-size` (2-6 for EDH, or 0 to disable EDH mode)
  - `--games-per-batch` (how many games before persisting)
  - `--rotation-strategy` (round-robin, weighted, etc.)

**File**: `cmd/mtgsim-runner/worker.go`
- Main event loop:
  1. Load preloaded decks into memory
  2. Poll `deck_queue` for active uploaded deck
  3. Simulate N games between active deck and a preloaded deck
  4. Insert results into database
  5. Update `deck_queue` rotation state
  6. Sleep briefly, repeat

**File**: `cmd/mtgsim-runner/config.go`
- Configuration loading (flags + env vars)
- Graceful shutdown handling

**File**: `cmd/mtgsim-runner/stats_updater.go`
- Periodically recalculate aggregated stats in `deck_stats` table
- Runs after batch insertion to keep dashboards fresh

### 2.3 Implement Deck Rotation Logic

**File**: `pkg/simulation-engine/deck_rotation.go` functions:

```go
// SelectUploadedDeck() -> (deck_id, preloaded_opponent_index)
// SelectUploadedDeck picks the next uploaded deck in rotation,
// and determines which preloaded deck to test it against.
// Updates deck_queue.next_run_index and deck_queue.rotation_order

// AdvanceRotation() 
// After a batch completes, move to the next uploaded deck

// LoadDecksFromDirs(preloadedDir, uploadedDir) -> (*DeckSet, error)
// Scan and cache all deck files at startup
```

---

## Phase 3: Refactor Dashboard

### 3.1 Update `cmd/mtgsim-dashboard`

**Remove**:
- Game runner logic (no more `RunGames()`, `simulateGame()`, goroutines)
- In-memory results storage
- Deck loading and file I/O

**Add**:
- Database connection via `pkg/database`
- New flag: `--db-path` (or `--db-url`)

### 3.2 Create Dashboard Query Layer: `pkg/dashboard/queries.go`

```go
// GetDeckMetrics(deckID) -> (wins, losses, winRate, avgTurns, ...)
func GetDeckMetrics(db *sql.DB, deckID int) (*DeckMetrics, error)

// GetLeaderboard() -> []DeckMetrics (sorted by win rate)
func GetLeaderboard(db *sql.DB) ([]DeckMetrics, error)

// GetRecentGames(limit) -> []GameResult
func GetRecentGames(db *sql.DB, limit int) ([]GameResult, error)

// GetEDHPodStats() -> []EDHDeckStats
func GetEDHPodStats(db *sql.DB) ([]EDHDeckStats, error)

// GetUploadedDeckQueueStatus() -> []DeckQueueEntry
func GetUploadedDeckQueueStatus(db *sql.DB) ([]DeckQueueEntry, error)
```

### 3.3 Update HTTP Handlers in `pkg/dashboard/server.go`

```go
// GET /api/decks -> JSON list of all decks with stats
// GET /api/decks/:id -> Detailed metrics for one deck
// GET /api/leaderboard -> Top performers
// GET /api/recent-games -> Latest game results
// GET /api/edh-pods -> EDH pod stats
// GET /api/queue-status -> Status of deck rotation queue
// POST /api/decks/upload -> Add new uploaded deck (creates entry in deck_queue)
```

### 3.4 Update Dashboard HTML/JavaScript

- Replace hard-coded game runner UI with database-driven metrics
- Auto-refresh from `/api/*` endpoints every 5 seconds
- Add "Deck Rotation Status" widget:
  - Show current active uploaded deck
  - Show number of games completed
  - Show rotation order (which decks are queued next)
  - Estimated time to completion per deck

---

## Phase 4: Deck Upload & Management

### 4.1 Add Deck Upload Endpoint

**File**: `pkg/dashboard/upload.go`

```go
// HandleUploadDeck handles POST /api/decks/upload
// 1. Accept multipart form data (.deck or .txt file)
// 2. Write to uploaded-decks directory
// 3. Parse deck name and commanders
// 4. Insert into decks table (is_preloaded=false)
// 5. Insert into deck_queue with next rotation_order
// Return: {deckID, deckName, nextRunOrder}
```

### 4.2 Deck Management Routes

```
DELETE /api/decks/:id          # Remove a deck from rotation (only uploaded)
POST   /api/decks/:id/skip     # Skip an uploaded deck, move to next
GET    /api/decks/:id/history  # View all games for this deck
```

---

## Phase 5: Monitoring & Operations

### 5.1 Health Check Endpoint

**File**: `cmd/mtgsim-runner/health.go`

```go
// GET /health -> {status, games_completed_today, active_deck, uptime}
// Expose on a separate port (e.g., :9090)
```

### 5.2 Logging & Metrics

- Export Prometheus metrics from runner:
  - `mtgsim_games_total` (counter)
  - `mtgsim_games_duration_ms` (histogram)
  - `mtgsim_deck_queue_length` (gauge)
  - `mtgsim_active_deck` (label with deck name)

- Dashboard can scrape for monitoring/alerting

### 5.3 Create Operations Guide

**File**: `docs/OPERATIONS.md`
- How to start/stop runner service
- How to configure systemd service
- Database backup & recovery procedures
- Troubleshooting common issues

---

## Implementation Roadmap

### Week 1: Foundation
- [ ] Design & create database schema
- [ ] Implement `pkg/database` package with migrations
- [ ] Write unit tests for database layer

### Week 2: Game Runner Service
- [ ] Extract deck rotation logic into `pkg/simulation-engine`
- [ ] Create `cmd/mtgsim-runner` service
- [ ] Implement deck loading and rotation scheduler
- [ ] Add database persistence for results

### Week 3: Dashboard Refactor
- [ ] Add database query layer to dashboard (`pkg/dashboard/queries.go`)
- [ ] Update HTTP handlers to use database instead of in-memory storage
- [ ] Create deck upload endpoint
- [ ] Update HTML/JS UI to reflect new data sources

### Week 4: Polish & Testing
- [ ] Integration tests (runner + dashboard + database)
- [ ] Load testing (simulate 1000s of uploaded decks in queue)
- [ ] Create operations guide and systemd service file
- [ ] Document deck rotation algorithm

### Week 5: Deployment & Monitoring
- [ ] Add Prometheus metrics
- [ ] Health check endpoints
- [ ] Staging environment testing
- [ ] Performance profiling & optimization

---

## Deck Rotation Strategy (Detailed)

### Algorithm: Round-Robin with Preloaded Opponent Selection

```
STATE: deck_queue table tracks:
  - uploaded_deck_id (which user-submitted deck)
  - rotation_order (queue position, 0 = current)
  - next_run_index (which preloaded deck to test against next, 0-based)
  - games_completed (how many games this rotation)

LOOP:
  1. SELECT deck FROM deck_queue WHERE rotation_order = 0
  2. SELECT preloaded_deck FROM decks WHERE is_preloaded = true LIMIT 1 OFFSET next_run_index
  3. Run N games: uploaded_deck vs preloaded_deck
  4. INSERT results into game_results or edh_pod_results
  5. UPDATE deck_queue SET 
       next_run_index = (next_run_index + 1) % COUNT(preloaded_decks),
       games_completed = games_completed + N
  6. If next_run_index == 0 (completed full rotation):
       UPDATE deck_queue SET rotation_order = rotation_order - 1 WHERE rotation_order > 0
       INSERT INTO deck_queue (new uploaded deck with rotation_order = MAX)
  7. Sleep & repeat
```

### Example Flow
```
Preloaded Decks: [A, B, C, D]  (4 total)
Uploaded Queue:  [X, Y, Z]      (3 to test)

Round 1:
  X vs A (games 1-10)    → games_completed=10, next_run_index=1
  X vs B (games 11-20)   → games_completed=20, next_run_index=2
  X vs C (games 21-30)   → games_completed=30, next_run_index=3
  X vs D (games 31-40)   → games_completed=40, next_run_index=0 (rotate X to end)
  
  Y vs A (games 41-50)   → rotation_order now Y=0, X=1, Z=2
  ...
```

---

## Key Benefits

1. **Decoupling**: Game runner and dashboard are independent services
2. **Persistence**: Results survive service restarts
3. **Scalability**: Can run multiple runners against same database
4. **Transparency**: Dashboard always shows fresh, consistent data
5. **Fairness**: Every uploaded deck tested against same pool of preloaded decks
6. **Observability**: Database enables detailed analytics, filtering, and replay

---

## Backward Compatibility

- Keep `mtgsim` and `mtgsim-edh` CLI tools as-is (for ad-hoc testing)
- New `mtgsim-runner` is opt-in for continuous simulation
- Dashboard can support both modes (fallback to in-memory if no database)

---

## Future Enhancements

1. **Weighted Rotation**: Prioritize weaker decks for more testing
2. **A/B Testing**: Test two variants of an uploaded deck in parallel
3. **Multi-Region**: Distribute simulation across machines to same database
4. **Web UI for Configuration**: UI to adjust rotation strategy, pause/resume
5. **Export & Analysis**: CSV export, statistical significance tests
6. **Replay Browser**: Search and re-run specific game scenarios

---

## Questions & Decisions

- [ ] SQLite or PostgreSQL? (SQLite for MVP, migrate to Postgres if needed)
- [ ] Service discovery? (systemd for now, consider Kubernetes later)
- [ ] Real-time updates for dashboard? (polling for MVP, WebSocket if needed)
- [ ] Retention policy? (Keep all results, or archive after 30 days?)
