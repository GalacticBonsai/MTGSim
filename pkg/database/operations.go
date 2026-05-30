package database

import (
	"database/sql"
	"fmt"
)

// GetOrCreateDeck returns the ID for a deck name, inserting if necessary.
func (db *DB) GetOrCreateDeck(name, path string) (int64, error) {
	var id int64
	err := db.sqlDB.QueryRow("SELECT id FROM decks WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	res, err := db.sqlDB.Exec("INSERT INTO decks(name, path) VALUES (?, ?)", name, path)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Record1v1Game inserts a single 1v1 game result.
func (db *DB) Record1v1Game(deck1, deck2, winner string, turns int) error {
	return db.txHelper(func(tx *sql.Tx) error {
		d1, err := db.getOrCreateDeckTx(tx, deck1, "")
		if err != nil {
			return err
		}
		d2, err := db.getOrCreateDeckTx(tx, deck2, "")
		if err != nil {
			return err
		}
		var wID sql.NullInt64
		if winner != "" {
			wid, err := db.getOrCreateDeckTx(tx, winner, "")
			if err != nil {
				return err
			}
			wID.Int64 = wid
			wID.Valid = true
		}
		_, err = tx.Exec(
			"INSERT INTO games_1v1(deck1_id, deck2_id, winner_id, turns, created_at) VALUES (?, ?, ?, ?, ?)",
			d1, d2, wID, turns, now())
		return err
	})
}

func (db *DB) getOrCreateDeckTx(tx *sql.Tx, name, path string) (int64, error) {
	var id int64
	err := tx.QueryRow("SELECT id FROM decks WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	res, err := tx.Exec("INSERT INTO decks(name, path) VALUES (?, ?)", name, path)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// EDHPodRecord mirrors simulation.EDHGameRecord for DB insertion.
type EDHPodRecord struct {
	TotalTurns         int
	Winner             string
	WinnerCondition    string
	MaxStormCount      int
	TotalManaSpent     int
	TotalManaProduced  int
	TotalCardsPlayed   int
	TotalCombatDamage  int
	TotalEliminations  int
}

// EDHPlayerRecord mirrors simulation.EDHPlayerRecord for DB insertion.
type EDHPlayerRecord struct {
	DeckName        string
	CommanderName   string
	Mulligans       int
	FinalLife       int
	CommanderCasts  int
	CardsPlayed     int
	LandsPlayed     int
	SpellsCast      int
	CreaturesCast   int
	ManaSpent       int
	ManaProduced    int
	CombatDamage    int
	Eliminations    int
	MaxStormCount   int
	Eliminated      bool
	KillSource      string
}

// RecordEDHPod inserts a pod and its players, plus per-deck card stats.
func (db *DB) RecordEDHPod(pod EDHPodRecord, players []EDHPlayerRecord, cardStats map[string]map[string]struct{ Casts, Wins int }) error {
	return db.txHelper(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO edh_pods(total_turns, winner, winner_condition, max_storm_count,
			total_mana_spent, total_mana_produced, total_cards_played, total_combat_damage, total_eliminations, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pod.TotalTurns, pod.Winner, pod.WinnerCondition, pod.MaxStormCount,
			pod.TotalManaSpent, pod.TotalManaProduced, pod.TotalCardsPlayed, pod.TotalCombatDamage, pod.TotalEliminations, now())
		if err != nil {
			return fmt.Errorf("insert pod: %w", err)
		}
		podID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for i, p := range players {
			deckID, err := db.getOrCreateDeckTx(tx, p.DeckName, "")
			if err != nil {
				return err
			}
			eliminated := 0
			if p.Eliminated {
				eliminated = 1
			}
			_, err = tx.Exec(
				`INSERT INTO edh_pod_players(pod_id, deck_id, seat, final_life, eliminated, kill_source,
				commander_name, commander_casts, cards_played, lands_played, spells_cast, creatures_cast,
				mana_spent, mana_produced, combat_damage, eliminations, max_storm_count, mulligans)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				podID, deckID, i, p.FinalLife, eliminated, p.KillSource,
				p.CommanderName, p.CommanderCasts, p.CardsPlayed, p.LandsPlayed, p.SpellsCast, p.CreaturesCast,
				p.ManaSpent, p.ManaProduced, p.CombatDamage, p.Eliminations, p.MaxStormCount, p.Mulligans)
			if err != nil {
				return fmt.Errorf("insert player: %w", err)
			}
			if stats, ok := cardStats[p.DeckName]; ok {
				for cName, perf := range stats {
					if err := db.recordDeckCardStatTx(tx, deckID, cName, perf.Casts, perf.Wins); err != nil {
						return err
					}
					if err := db.recordGlobalCardStatTx(tx, cName, perf.Casts, perf.Wins); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (db *DB) recordDeckCardStatTx(tx *sql.Tx, deckID int64, cardName string, casts, wins int) error {
	_, err := tx.Exec(
		`INSERT INTO deck_card_stats(deck_id, card_name, casts, wins)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(deck_id, card_name) DO UPDATE SET
		casts = casts + excluded.casts,
		wins = wins + excluded.wins`,
		deckID, cardName, casts, wins)
	return err
}

func (db *DB) recordGlobalCardStatTx(tx *sql.Tx, cardName string, casts, wins int) error {
	_, err := tx.Exec(
		`INSERT INTO card_global_stats(card_name, casts, wins, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(card_name) DO UPDATE SET
		casts = casts + excluded.casts,
		wins = wins + excluded.wins,
		updated_at = excluded.updated_at`,
		cardName, casts, wins, now())
	return err
}

// Deck1v1Stats holds aggregated 1v1 results for a deck.
type Deck1v1Stats struct {
	Name    string
	Wins    int
	Losses  int
	WinRate float64
}

// Get1v1DeckStats returns aggregated win/loss stats per deck.
func (db *DB) Get1v1DeckStats() ([]Deck1v1Stats, error) {
	rows, err := db.sqlDB.Query(`
		SELECT d.name,
			SUM(CASE WHEN g.winner_id = d.id THEN 1 ELSE 0 END) AS wins,
			SUM(CASE WHEN g.winner_id IS NOT NULL AND g.winner_id != d.id THEN 1 ELSE 0 END) AS losses
		FROM decks d
		LEFT JOIN games_1v1 g ON d.id IN (g.deck1_id, g.deck2_id)
		GROUP BY d.id, d.name
		HAVING wins + losses > 0
		ORDER BY wins DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Deck1v1Stats
	for rows.Next() {
		var s Deck1v1Stats
		if err := rows.Scan(&s.Name, &s.Wins, &s.Losses); err != nil {
			return nil, err
		}
		total := s.Wins + s.Losses
		if total > 0 {
			s.WinRate = float64(s.Wins) / float64(total) * 100
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// EDHDeckStats holds aggregated EDH stats for a deck.
type EDHDeckStats struct {
	DeckName           string
	CommanderName      string
	Games              int
	Wins               int
	Losses             int
	WinRate            float64
	AvgFinalLife       float64
	AvgMulligans       float64
	CommanderDamageKOs int
	LifeLossKOs        int
	MillKOs            int
	DeckoutKOs         int
	EffectKOs          int
	CombatWins         int
	EffectWins         int
	DeckoutWins        int
	AvgCommanderCasts  float64
	AvgManaSpent       float64
	AvgManaProduced    float64
	AvgCardsPlayed     float64
	AvgLandsPlayed     float64
	AvgSpellsCast      float64
	AvgCreaturesCast   float64
	AvgCombatDamage    float64
	MaxStormCount      int
	TotalManaSpent     int
	TotalManaProduced  int
	TotalCardsPlayed   int
	TotalCombatDamage  int
	Eliminations       int
	CardStats          map[string]struct{ Casts, Wins int }
}

// GetEDHDeckStats returns aggregated EDH stats per deck.
func (db *DB) GetEDHDeckStats() ([]EDHDeckStats, error) {
	rows, err := db.sqlDB.Query(`
		SELECT d.name,
			MAX(ep.commander_name) AS commander_name,
			COUNT(*) AS games,
			SUM(CASE WHEN p.winner = d.name THEN 1 ELSE 0 END) AS wins,
			SUM(CASE WHEN p.winner != d.name OR p.winner IS NULL THEN 1 ELSE 0 END) AS losses,
			AVG(ep.final_life) AS avg_final_life,
			AVG(ep.mulligans) AS avg_mulligans,
			SUM(CASE WHEN ep.kill_source = 'commander_damage' THEN 1 ELSE 0 END) AS cmdr_kos,
			SUM(CASE WHEN ep.kill_source = 'life_loss' THEN 1 ELSE 0 END) AS life_kos,
			SUM(CASE WHEN ep.kill_source = 'mill' THEN 1 ELSE 0 END) AS mill_kos,
			SUM(CASE WHEN ep.kill_source = 'deckout' THEN 1 ELSE 0 END) AS deckout_kos,
			SUM(CASE WHEN ep.kill_source = 'effect' THEN 1 ELSE 0 END) AS effect_kos,
			SUM(CASE WHEN p.winner = d.name AND p.winner_condition = 'combat' THEN 1 ELSE 0 END) AS combat_wins,
			SUM(CASE WHEN p.winner = d.name AND p.winner_condition = 'effect' THEN 1 ELSE 0 END) AS effect_wins,
			SUM(CASE WHEN p.winner = d.name AND p.winner_condition = 'deckout' THEN 1 ELSE 0 END) AS deckout_wins,
			AVG(ep.commander_casts) AS avg_cmdr_casts,
			AVG(ep.mana_spent) AS avg_mana,
			AVG(CASE WHEN ep.mana_produced > 0 THEN ep.mana_produced END) AS avg_mana_produced,
			AVG(ep.cards_played) AS avg_cards,
			AVG(ep.lands_played) AS avg_lands,
			AVG(ep.spells_cast) AS avg_spells,
			AVG(ep.creatures_cast) AS avg_creatures,
			AVG(ep.combat_damage) AS avg_combat,
			MAX(ep.max_storm_count) AS max_storm,
			SUM(ep.mana_spent) AS total_mana,
			SUM(ep.mana_produced) AS total_mana_produced,
			SUM(ep.cards_played) AS total_cards,
			SUM(ep.combat_damage) AS total_combat,
			SUM(ep.eliminations) AS eliminations
		FROM decks d
		JOIN edh_pod_players ep ON ep.deck_id = d.id
		JOIN edh_pods p ON p.id = ep.pod_id
		GROUP BY d.id, d.name
		ORDER BY wins DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EDHDeckStats
	for rows.Next() {
		var s EDHDeckStats
		var avgLife, avgMulls, avgCmdr, avgMana, avgManaProduced, avgCards, avgLands, avgSpells, avgCreatures, avgCombat sql.NullFloat64
		if err := rows.Scan(
			&s.DeckName, &s.CommanderName, &s.Games, &s.Wins, &s.Losses,
			&avgLife, &avgMulls, &s.CommanderDamageKOs, &s.LifeLossKOs, &s.MillKOs, &s.DeckoutKOs, &s.EffectKOs,
			&s.CombatWins, &s.EffectWins, &s.DeckoutWins,
			&avgCmdr, &avgMana, &avgManaProduced, &avgCards, &avgLands, &avgSpells, &avgCreatures, &avgCombat,
			&s.MaxStormCount, &s.TotalManaSpent, &s.TotalManaProduced, &s.TotalCardsPlayed, &s.TotalCombatDamage, &s.Eliminations,
		); err != nil {
			return nil, err
		}
		s.AvgFinalLife = avgLife.Float64
		s.AvgMulligans = avgMulls.Float64
		s.AvgCommanderCasts = avgCmdr.Float64
		s.AvgManaSpent = avgMana.Float64
		s.AvgManaProduced = avgManaProduced.Float64
		s.AvgCardsPlayed = avgCards.Float64
		s.AvgLandsPlayed = avgLands.Float64
		s.AvgSpellsCast = avgSpells.Float64
		s.AvgCreaturesCast = avgCreatures.Float64
		s.AvgCombatDamage = avgCombat.Float64
		if s.Games > 0 {
			s.WinRate = float64(s.Wins) / float64(s.Games) * 100
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load per-deck card stats
	for i := range out {
		cs, err := db.getDeckCardStatsByName(out[i].DeckName)
		if err != nil {
			return nil, err
		}
		out[i].CardStats = cs
	}
	return out, nil
}

func (db *DB) getDeckCardStatsByName(deckName string) (map[string]struct{ Casts, Wins int }, error) {
	rows, err := db.sqlDB.Query(`
		SELECT dcs.card_name, dcs.casts, dcs.wins
		FROM deck_card_stats dcs
		JOIN decks d ON d.id = dcs.deck_id
		WHERE d.name = ?`, deckName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]struct{ Casts, Wins int })
	for rows.Next() {
		var name string
		var c, w int
		if err := rows.Scan(&name, &c, &w); err != nil {
			return nil, err
		}
		out[name] = struct{ Casts, Wins int }{Casts: c, Wins: w}
	}
	return out, rows.Err()
}

// EDHSummary holds global EDH aggregates.
type EDHSummary struct {
	TotalGames           int
	AverageTurns         float64
	TotalManaSpent       int
	AverageManaSpent     float64
	TotalManaProduced    int
	AverageManaProduced  float64
	TotalCardsPlayed     int
	AverageCardsPlayed   float64
	HighestStormCount    int
	TotalCombatDamage    int
	TotalEliminations    int
	AverageEliminations  float64
	AverageCombatDamage  float64
}

// GetEDHSummary returns global EDH aggregates.
func (db *DB) GetEDHSummary() (EDHSummary, error) {
	var s EDHSummary
	var avgTurns sql.NullFloat64
	var manaProducedGames int64
	err := db.sqlDB.QueryRow(`
		SELECT COUNT(*), AVG(total_turns), SUM(total_mana_spent), SUM(total_mana_produced), SUM(total_cards_played),
			MAX(max_storm_count), SUM(total_combat_damage), SUM(total_eliminations),
			COUNT(CASE WHEN total_mana_produced > 0 THEN 1 END)
		FROM edh_pods`).Scan(
		&s.TotalGames, &avgTurns, &s.TotalManaSpent, &s.TotalManaProduced, &s.TotalCardsPlayed,
		&s.HighestStormCount, &s.TotalCombatDamage, &s.TotalEliminations, &manaProducedGames)
	if err != nil {
		return s, err
	}
	s.AverageTurns = avgTurns.Float64
	if s.TotalGames > 0 {
		s.AverageManaSpent = float64(s.TotalManaSpent) / float64(s.TotalGames)
	}
	if manaProducedGames > 0 {
		s.AverageManaProduced = float64(s.TotalManaProduced) / float64(manaProducedGames)
	}
	if s.TotalGames > 0 {
		s.AverageCardsPlayed = float64(s.TotalCardsPlayed) / float64(s.TotalGames)
		s.AverageEliminations = float64(s.TotalEliminations) / float64(s.TotalGames)
		s.AverageCombatDamage = float64(s.TotalCombatDamage) / float64(s.TotalGames)
	}
	return s, nil
}

// EDHRecentPod holds a lightweight recent pod record.
type EDHRecentPod struct {
	ID                int64
	TotalTurns        int
	Winner            string
	WinnerCondition   string
	MaxStormCount     int
	TotalManaSpent    int
	TotalManaProduced int
	TotalCardsPlayed  int
	TotalCombatDamage int
	TotalEliminations int
	CreatedAt         string
	Players           []EDHRecentPlayer
}

// EDHRecentPlayer holds a lightweight player record for recent pods.
type EDHRecentPlayer struct {
	DeckName       string
	CommanderName  string
	FinalLife      int
	Eliminated     bool
	KillSource     string
	ManaSpent      int
	ManaProduced   int
	CardsPlayed    int
	CombatDamage   int
	Eliminations   int
}

// GetRecentEDHPods returns the most recent pods with player details.
func (db *DB) GetRecentEDHPods(limit int) ([]EDHRecentPod, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := db.sqlDB.Query(`
		SELECT id, total_turns, winner, winner_condition, max_storm_count,
			total_mana_spent, total_mana_produced, total_cards_played, total_combat_damage, total_eliminations, created_at
		FROM edh_pods
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pods []EDHRecentPod
	for rows.Next() {
		var p EDHRecentPod
		var created sql.NullString
		if err := rows.Scan(&p.ID, &p.TotalTurns, &p.Winner, &p.WinnerCondition, &p.MaxStormCount,
			&p.TotalManaSpent, &p.TotalManaProduced, &p.TotalCardsPlayed, &p.TotalCombatDamage, &p.TotalEliminations, &created); err != nil {
			return nil, err
		}
		if created.Valid {
			p.CreatedAt = created.String
		}
		pods = append(pods, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range pods {
		pRows, err := db.sqlDB.Query(`
			SELECT d.name, ep.commander_name, ep.final_life, ep.eliminated, ep.kill_source,
				ep.mana_spent, ep.mana_produced, ep.cards_played, ep.combat_damage, ep.eliminations
			FROM edh_pod_players ep
			JOIN decks d ON d.id = ep.deck_id
			WHERE ep.pod_id = ?
			ORDER BY ep.seat`, pods[i].ID)
		if err != nil {
			return nil, err
		}
		for pRows.Next() {
			var pl EDHRecentPlayer
			var elim int
			if err := pRows.Scan(&pl.DeckName, &pl.CommanderName, &pl.FinalLife, &elim, &pl.KillSource,
				&pl.ManaSpent, &pl.ManaProduced, &pl.CardsPlayed, &pl.CombatDamage, &pl.Eliminations); err != nil {
				pRows.Close()
				return nil, err
			}
			pl.Eliminated = elim != 0
			pods[i].Players = append(pods[i].Players, pl)
		}
		pRows.Close()
		if err := pRows.Err(); err != nil {
			return nil, err
		}
	}
	return pods, nil
}

// GlobalCardStats holds aggregated global card performance.
type GlobalCardStats struct {
	CardName string
	Casts    int
	Wins     int
	WinRate  float64
	ImageURL string
}

// GetGlobalCardStats returns all global card stats.
func (db *DB) GetGlobalCardStats() ([]GlobalCardStats, error) {
	rows, err := db.sqlDB.Query(`
		SELECT card_name, casts, wins, IFNULL(image_url, '')
		FROM card_global_stats
		WHERE casts > 0
		ORDER BY wins DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GlobalCardStats
	for rows.Next() {
		var s GlobalCardStats
		if err := rows.Scan(&s.CardName, &s.Casts, &s.Wins, &s.ImageURL); err != nil {
			return nil, err
		}
		if s.Casts > 0 {
			s.WinRate = float64(s.Wins) / float64(s.Casts) * 100
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateCardImageURL sets the image URL for a card.
func (db *DB) UpdateCardImageURL(cardName, url string) error {
	_, err := db.sqlDB.Exec(
		`INSERT INTO card_global_stats(card_name, image_url, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(card_name) DO UPDATE SET
		image_url = excluded.image_url,
		updated_at = excluded.updated_at`,
		cardName, url, now())
	return err
}
