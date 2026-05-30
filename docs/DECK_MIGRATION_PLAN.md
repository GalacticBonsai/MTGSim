# MTGSim Deck Migration Plan

## Executive Summary

This plan outlines the migration of all deck files from the filesystem (`/decks/` directory) into the database, enabling:
- **Centralized deck storage** in the database instead of scattered `.deck` and `.txt` files
- **No filesystem dependency** for game simulation (only database access needed)
- **Deck versioning** (optional: track deck edits over time)
- **Metadata extraction** (commanders, card counts, format, estimated power level)
- **API-driven deck management** (browse, upload, edit, delete)

---

## Phase A: Schema Enhancement

### A.1 Extend Database Schema

Add these tables to the existing schema from `ARCHITECTURE_PLAN.md`:

```sql
-- Deck Mainboard and Sideboard Cards
CREATE TABLE deck_cards (
  id INTEGER PRIMARY KEY,
  deck_id INTEGER NOT NULL REFERENCES decks(id),
  card_name TEXT,
  quantity INT,
  is_sideboard BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP,
  FOREIGN KEY (deck_id) REFERENCES decks(id)
);

-- Deck Metadata (commanders, format, tags, etc.)
CREATE TABLE deck_metadata (
  deck_id INTEGER PRIMARY KEY REFERENCES decks(id),
  format TEXT,  -- '1v1', 'edh', 'vintage', 'casual', etc.
  commanders TEXT,  -- JSON array of commander names
  primary_colors TEXT,  -- JSON array: ['W', 'U', 'B', 'R', 'G']
  mainboard_size INT,
  sideboard_size INT,
  estimated_power_level INT,  -- 1-10 scale
  notes TEXT,  -- User notes/description
  source TEXT,  -- 'preloaded', 'uploaded', 'migrated'
  imported_from_file TEXT,  -- Original filename (for audit trail)
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

-- Deck Revisions (optional: track changes over time)
CREATE TABLE deck_revisions (
  id INTEGER PRIMARY KEY,
  deck_id INTEGER NOT NULL REFERENCES decks(id),
  revision_number INT,
  mainboard_snapshot TEXT,  -- JSON array of {name, quantity}
  sideboard_snapshot TEXT,  -- JSON array of {name, quantity}
  change_description TEXT,  -- "Added 2x Counterspell"
  created_by TEXT,  -- username if auth is added later
  created_at TIMESTAMP
);
```

### A.2 Update `decks` Table

Enhance the base `decks` table from Phase 1:

```sql
ALTER TABLE decks ADD COLUMN format TEXT DEFAULT 'edh';
ALTER TABLE decks ADD COLUMN metadata_id INTEGER REFERENCES deck_metadata(deck_id);
```

---

## Phase B: Deck Parser & Ingestion

### B.1 Create Deck Parser: `pkg/deck-importer/parser.go`

```go
package deckimporter

import (
	"bufio"
	"io"
	"strings"
)

// ParsedDeck represents a deck parsed from a file
type ParsedDeck struct {
	Name         string
	Mainboard    []CardEntry
	Sideboard    []CardEntry
	Commanders   []string
	Format       string  // inferred or explicit
	SourcePath   string
}

type CardEntry struct {
	Name     string
	Quantity int
}

// ParseDeckFile parses .deck or .txt file format
// Supports: simple text lists, Cockatrice .dck, Moxfield sections
func ParseDeckFile(filename string, content io.Reader) (*ParsedDeck, error) {
	// Logic:
	// 1. Detect format by file extension and content structure
	// 2. If "Sideboard" section exists -> parse sideboard
	// 3. If "Commander:" line exists -> extract commanders
	// 4. Extract name from filename or metadata
	// 5. Infer format from directory path (edh/, 1v1/, etc.)
}

// InferFormat infers the deck format from deck content
// - If has 100-card mainboard + commander -> 'edh'
// - If has 60-card mainboard + no commander -> '1v1'
// - Default to 'edh'
func InferFormat(mainboard, sideboard []CardEntry, commanders []string) string {
	// Implementation
}

// ExtractCommanders attempts to find commander(s) in the deck
// Looks for:
// - Explicit "Commander:" lines
// - Card titles that are legendary creatures (requires card DB lookup)
func ExtractCommanders(mainboard []CardEntry, cardDB *card.CardDB) ([]string, error) {
	// Implementation
}
```

### B.2 Create Database Ingestion: `pkg/deck-importer/ingester.go`

```go
package deckimporter

import (
	"database/sql"
)

// IngestDeckFromFile parses a file and inserts into database
func IngestDeckFromFile(db *sql.DB, filePath string, isPreloaded bool) (deckID int, err error) {
	// 1. Parse file using ParseDeckFile()
	// 2. Extract deck name from filename (strip path, extension)
	// 3. Insert into decks table
	// 4. Insert cards into deck_cards table
	// 5. Insert metadata into deck_metadata table
	// 6. Return deck ID
}

// IngestBulkDecksFromDirectory scans a directory and ingests all .deck/.txt files
func IngestBulkDecksFromDirectory(db *sql.DB, dirPath string, isPreloaded bool, format string) ([]int, error) {
	// 1. Walk directory recursively
	// 2. For each .deck or .txt file:
	//    - Call IngestDeckFromFile()
	//    - Collect returned deck IDs
	// 3. Log summary (X decks ingested, Y errors)
	// 4. Return slice of deck IDs
}

// DeckToJSON converts a parsed deck to JSON for storage
func DeckToJSON(parsed *ParsedDeck) (string, error) {
	// Serialize Mainboard and Sideboard to JSON
}
```

### B.3 Create Migration Script: `cmd/deck-migrator/main.go`

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mtgsim/mtgsim/pkg/database"
	"github.com/mtgsim/mtgsim/pkg/deck-importer"
)

func main() {
	var (
		dbPath      = flag.String("db", "mtgsim.db", "Path to database")
		decksDir    = flag.String("decks-dir", "decks", "Root directory of deck files")
		dryRun      = flag.Bool("dry-run", false, "Parse files without writing to DB")
	)
	flag.Parse()

	// 1. Connect to database
	db, err := database.Connect(*dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Scan directory structure
	// Expected structure:
	//   decks/
	//     1v1/        -> preloaded, format='1v1'
	//     edh/        -> preloaded, format='edh'
	//     novelty/    -> preloaded, format='edh'
	//     test/       -> preloaded, format='edh'
	//     vanilla/    -> preloaded, format='edh'
	//     welcome/    -> preloaded, format='edh'
	//     uploaded/   -> uploaded, auto-detect format

	fmt.Println("Starting deck migration...")

	formatMap := map[string]string{
		"1v1":      "1v1",
		"edh":      "edh",
		"novelty":  "edh",
		"test":     "edh",
		"vanilla":  "edh",
		"welcome":  "edh",
		"uploaded": "",  // auto-detect
	}

	totalIngested := 0
	totalErrors := 0

	for dirName, format := range formatMap {
		dirPath := filepath.Join(*decksDir, dirName)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			fmt.Printf("Skipping %s (directory not found)\n", dirName)
			continue
		}

		fmt.Printf("\nProcessing %s/ (format: %s)\n", dirName, format)

		if *dryRun {
			fmt.Printf("  [DRY RUN] Would ingest decks from %s\n", dirPath)
			continue
		}

		deckIDs, err := deckimporter.IngestBulkDecksFromDirectory(
			db, dirPath, dirName != "uploaded", format,
		)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			totalErrors++
		}

		fmt.Printf("  Ingested %d decks\n", len(deckIDs))
		totalIngested += len(deckIDs)
	}

	fmt.Printf("\n=== Migration Summary ===\n")
	fmt.Printf("Total ingested: %d\n", totalIngested)
	fmt.Printf("Total errors: %d\n", totalErrors)

	if !*dryRun {
		fmt.Printf("\nMigration complete! Verify with:\n")
		fmt.Printf("  SELECT COUNT(*) FROM decks;\n")
	}
}
```

---

## Phase C: API Endpoints for Deck Management

### C.1 New Dashboard Endpoints in `pkg/dashboard/deck_handlers.go`

```go
// GET /api/decks                       - List all decks with basic info
// GET /api/decks?format=edh            - Filter by format
// GET /api/decks?preloaded=true        - Filter by preloaded status
// GET /api/decks/:id                   - Get full deck with all cards
// GET /api/decks/:id/cards             - Get mainboard + sideboard cards
// GET /api/decks/:id/metadata          - Get metadata (commander, colors, etc.)
// GET /api/decks/:id/stats             - Get performance stats (wins/losses/etc.)
// GET /api/decks/:id/revisions         - Get revision history (if versioning enabled)

// POST /api/decks                      - Upload new deck file (multipart)
// PUT  /api/decks/:id                  - Edit deck metadata (commander, notes, etc.)
// DELETE /api/decks/:id                - Remove deck (preloaded or uploaded)

// POST /api/decks/:id/cards            - Add cards to mainboard
// PUT  /api/decks/:id/cards/:cardName  - Update quantity
// DELETE /api/decks/:id/cards/:cardName - Remove card
```

### C.2 Deck List Response Example

```json
{
  "decks": [
    {
      "id": 1,
      "name": "Krenko EDH",
      "format": "edh",
      "is_preloaded": true,
      "mainboard_size": 100,
      "sideboard_size": 0,
      "commanders": ["Krenko, Mob Boss"],
      "primary_colors": ["R"],
      "stats": {
        "total_games": 147,
        "wins": 89,
        "losses": 58,
        "win_rate": 0.605
      },
      "created_at": "2026-05-20T10:00:00Z"
    }
  ]
}
```

---

## Phase D: Update Game Runner to Use Database Decks

### D.1 Modify `pkg/simulation-engine/engine.go`

**Before** (filesystem):
```go
func (e *Engine) LoadDeck(filePath string) (*Deck, error) {
	content, err := os.ReadFile(filePath)  // Read from filesystem
	// ...
}
```

**After** (database):
```go
func (e *Engine) LoadDeckFromDB(db *sql.DB, deckID int) (*Deck, error) {
	// 1. Query deck_cards WHERE deck_id = ?
	// 2. Query deck_metadata WHERE deck_id = ?
	// 3. Reconstruct Deck struct from DB rows
	// 4. Return *Deck
}

func (e *Engine) LoadDeckByName(db *sql.DB, deckName string) (*Deck, error) {
	// 1. SELECT id FROM decks WHERE name = ?
	// 2. Call LoadDeckFromDB()
}
```

### D.2 Update `cmd/mtgsim-runner/worker.go`

**Before**:
```go
func (w *Worker) runBatch() {
	d1, _ := deck.ImportDeckfile(d1Path, cardDB)  // Filesystem
	d2, _ := deck.ImportDeckfile(d2Path, cardDB)  // Filesystem
}
```

**After**:
```go
func (w *Worker) runBatch() {
	d1, _ := engine.LoadDeckFromDB(w.db, uploadedDeckID)    // Database
	d2, _ := engine.LoadDeckFromDB(w.db, preloadedDeckID)   // Database
}
```

### D.3 Update `cmd/mtgsim-dashboard/main.go`

Similarly, update dashboard to load decks from database instead of scanning filesystem.

---

## Phase E: Deck Versioning (Optional)

### E.1 Track Deck Changes Over Time

When a deck is edited via PUT `/api/decks/:id`, create a revision:

```go
func (h *DeckHandler) UpdateDeck(w http.ResponseWriter, r *http.Request) {
	// 1. Parse request body (mainboard changes, metadata changes)
	// 2. Query current deck_cards and deck_metadata
	// 3. Diff old vs new
	// 4. Insert into deck_revisions with change description
	// 5. Update deck_cards and deck_metadata
}
```

This enables:
- Rollback to previous deck versions
- Audit trail of who changed what
- A/B testing different deck variants

---

## Migration Execution Plan

### Step 1: Prepare Database (1-2 hours)

```bash
# 1. Back up existing database (if it exists)
cp mtgsim.db mtgsim.db.backup

# 2. Run migrations to create new tables
./cmd/deck-migrator/migrate.go --dry-run

# 3. Verify schema
sqlite3 mtgsim.db ".schema"
```

### Step 2: Run Migration Script (1-2 hours)

```bash
# 1. Build migrator
go build -o deck-migrator ./cmd/deck-migrator

# 2. Test with dry-run first
./deck-migrator --db mtgsim.db --decks-dir ./decks --dry-run

# 3. Run actual migration
./deck-migrator --db mtgsim.db --decks-dir ./decks

# 4. Verify counts
sqlite3 mtgsim.db "SELECT COUNT(*) FROM decks; SELECT COUNT(*) FROM deck_cards;"
```

### Step 3: Validate Data (1 hour)

```bash
# Check a few decks were imported correctly
sqlite3 mtgsim.db "SELECT d.name, COUNT(c.id) as card_count FROM decks d 
  LEFT JOIN deck_cards c ON d.id = c.deck_id 
  GROUP BY d.id LIMIT 10;"

# Verify commanders were extracted
sqlite3 mtgsim.db "SELECT name, (SELECT commanders FROM deck_metadata WHERE deck_id = decks.id) as commanders FROM decks LIMIT 5;"
```

### Step 4: Update Code (2-4 hours)

- [ ] Update `pkg/simulation-engine` to use DB instead of files
- [ ] Update `cmd/mtgsim-runner` to call new DB functions
- [ ] Update `cmd/mtgsim-dashboard` to load decks from DB
- [ ] Update `pkg/deck` importers to optionally accept pre-parsed cards
- [ ] Add new API endpoints to dashboard

### Step 5: Testing (2-3 hours)

- [ ] Unit tests for deck parser with sample files
- [ ] Integration test: migrate sample decks, load from DB, verify they match
- [ ] End-to-end test: run game simulation with DB-loaded decks
- [ ] Performance test: measure DB query time vs filesystem I/O

### Step 6: Cleanup (1 hour)

- [ ] Once verified, optionally archive `/decks/` directory
- [ ] Update `.gitignore` to exclude `/decks/` (no longer in repo)
- [ ] Update README with new deployment instructions

---

## Benefits After Migration

| Aspect | Before | After |
|--------|--------|-------|
| **Deck Storage** | Filesystem files, version controlled | Database records, independent of repo |
| **Access** | File I/O, path-based loading | SQL queries, ID-based loading |
| **Scaling** | Adding 100 decks = 100 files | Adding 100 decks = 100 rows |
| **Metadata** | Filename parsing + comments | Structured `deck_metadata` table |
| **Auditing** | Git history only | Deck revisions table + timestamps |
| **Multi-Service** | All services need filesystem | Only need database connection |
| **Backup** | `git push` | Database dump / backup |

---

## Data Safety & Rollback Plan

### Backup Strategy

Before running migration:
```bash
# SQLite backup
sqlite3 mtgsim.db ".backup mtgsim.db.backup-$(date +%Y%m%d)"

# Filesystem backup (optional)
tar -czf decks-backup-$(date +%Y%m%d).tar.gz decks/
```

### Rollback Procedure

If migration fails:
```bash
# 1. Restore database
cp mtgsim.db.backup-YYYYMMDD mtgsim.db

# 2. Restart services
systemctl restart mtgsim-runner
systemctl restart mtgsim-dashboard

# 3. Investigate errors in migration log
```

---

## File Structure After Migration

```
MTGSim/
├── cmd/
│   ├── mtgsim/
│   ├── mtgsim-dashboard/
│   ├── mtgsim-edh/
│   ├── mtgsim-runner/
│   └── deck-migrator/           # NEW
├── pkg/
│   ├── deck-importer/           # NEW (parser + ingester)
│   ├── database/
│   ├── dashboard/
│   ├── simulation-engine/
│   └── ...
├── decks/                        # ARCHIVED or removed from version control
│   ├── 1v1/
│   ├── edh/
│   └── ...
├── mtgsim.db                     # DATABASE (now the source of truth)
└── DECK_MIGRATION_PLAN.md        # This file
```

---

## Questions & Decisions

- [ ] **Versioning**: Do we want to track deck edits over time? (Adds complexity)
- [ ] **Soft Delete**: When deleting a deck, archive or hard delete? (Archive = audit trail)
- [ ] **Deck Variants**: Support A/B testing (e.g., "Krenko EDH v1" vs "Krenko EDH v2")?
- [ ] **Power Level**: Manually assigned or calculated from card values?
- [ ] **Sync Back to Filesystem**: Export deck from DB to .txt/.dck format if needed?

---

## Next Steps

1. Review this plan with stakeholders
2. Implement Phase B (parser + migrator) and test with sample decks
3. Run Phase C (API endpoints) to make decks queryable
4. Update game runner (Phase D) to use DB instead of filesystem
5. Optionally implement versioning (Phase E)
6. Archive `/decks/` directory from version control

Once complete, the project will be filesystem-independent for deck storage and access!
