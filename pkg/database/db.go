// Package database provides SQLite persistence for MTGSim results and stats.
package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a sql.DB with MTGSim-specific helpers.
type DB struct {
	sqlDB *sql.DB
}

// Open opens (or creates) the SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	db := &DB{sqlDB: sqlDB}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.sqlDB.Close()
}

func (db *DB) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS decks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS games_1v1 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deck1_id INTEGER NOT NULL REFERENCES decks(id),
    deck2_id INTEGER NOT NULL REFERENCES decks(id),
    winner_id INTEGER REFERENCES decks(id),
    turns INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS edh_pods (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    total_turns INTEGER,
    winner TEXT,
    winner_condition TEXT,
    max_storm_count INTEGER DEFAULT 0,
    total_mana_spent INTEGER DEFAULT 0,
    total_mana_produced INTEGER DEFAULT 0,
    total_cards_played INTEGER DEFAULT 0,
    total_combat_damage INTEGER DEFAULT 0,
    total_eliminations INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS edh_pod_players (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pod_id INTEGER NOT NULL REFERENCES edh_pods(id),
    deck_id INTEGER NOT NULL REFERENCES decks(id),
    seat INTEGER,
    final_life INTEGER,
    eliminated INTEGER DEFAULT 0,
    kill_source TEXT,
    commander_name TEXT,
    commander_casts INTEGER DEFAULT 0,
    cards_played INTEGER DEFAULT 0,
    lands_played INTEGER DEFAULT 0,
    spells_cast INTEGER DEFAULT 0,
    creatures_cast INTEGER DEFAULT 0,
    mana_spent INTEGER DEFAULT 0,
    mana_produced INTEGER DEFAULT 0,
    combat_damage INTEGER DEFAULT 0,
    eliminations INTEGER DEFAULT 0,
    max_storm_count INTEGER DEFAULT 0,
    mulligans INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS card_global_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_name TEXT UNIQUE NOT NULL,
    casts INTEGER DEFAULT 0,
    wins INTEGER DEFAULT 0,
    image_url TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS deck_card_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deck_id INTEGER NOT NULL REFERENCES decks(id),
    card_name TEXT NOT NULL,
    casts INTEGER DEFAULT 0,
    wins INTEGER DEFAULT 0,
    UNIQUE(deck_id, card_name)
);

CREATE INDEX IF NOT EXISTS idx_games_1v1_winner ON games_1v1(winner_id);
CREATE INDEX IF NOT EXISTS idx_games_1v1_decks ON games_1v1(deck1_id, deck2_id);
CREATE INDEX IF NOT EXISTS idx_edh_pod_players_pod ON edh_pod_players(pod_id);
CREATE INDEX IF NOT EXISTS idx_edh_pod_players_deck ON edh_pod_players(deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_card_stats_deck ON deck_card_stats(deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_card_stats_name ON deck_card_stats(card_name);
`
	if _, err := db.sqlDB.Exec(schema); err != nil {
		return err
	}

	// Migrate columns that may not exist in older DBs
	alterStatements := []string{
		"ALTER TABLE edh_pods ADD COLUMN total_mana_produced INTEGER DEFAULT 0",
		"ALTER TABLE edh_pod_players ADD COLUMN mana_produced INTEGER DEFAULT 0",
	}
	for _, stmt := range alterStatements {
		if _, err := db.sqlDB.Exec(stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}
	return nil
}

// txHelper runs f inside a transaction and commits or rolls back.
func (db *DB) txHelper(f func(*sql.Tx) error) error {
	tx, err := db.sqlDB.Begin()
	if err != nil {
		return err
	}
	if err := f(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func now() time.Time {
	return time.Now().UTC()
}
