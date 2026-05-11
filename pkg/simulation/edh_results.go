package simulation

import (
	"sort"
	"sync"
)

// KillSource categorizes how a player was eliminated from an EDH game
// (CR 704.5 / 704.5u for commander damage). The headless runner reports
// one source per eliminated player so aggregate metrics can attribute
// losses to the rule that triggered them.
type KillSource string

const (
	KillSourceLifeLoss        KillSource = "life_loss"
	KillSourceCommanderDamage KillSource = "commander_damage"
	KillSourceMill            KillSource = "mill"
	KillSourceDeckout         KillSource = "deckout"
	KillSourceEffect          KillSource = "effect"
	KillSourceTurnLimit       KillSource = "turn_limit"
	KillSourceUnknown         KillSource = "unknown"
)

// WinCondition categorizes how a player won the game.
type WinCondition string

const (
	WinConditionCombat         WinCondition = "combat"
	WinConditionCommanderDamage WinCondition = "commander_damage"
	WinConditionDeckout          WinCondition = "deckout"
	WinConditionEffect           WinCondition = "effect"
	WinConditionTurnLimit        WinCondition = "turn_limit"
	WinConditionUnknown          WinCondition = "unknown"
)

// CardPerformance tracks per-card aggregate performance for a deck.
type CardPerformance struct {
	Casts int `json:"casts"`
	Wins  int `json:"wins"`
}

// EDHPlayerRecord captures one seat in one game.
type EDHPlayerRecord struct {
	DeckName       string
	CommanderName  string
	Mulligans      int
	FinalLife      int
	CommanderCasts int
	CardsPlayed    int
	LandsPlayed    int
	SpellsCast     int
	CreaturesCast  int
	ManaSpent      int
	MaxStormCount  int
	CombatDamage   int
	Eliminations   int
	Eliminated     bool
	KillSource     KillSource
	CardStats      map[string]CardPerformance
}

// EDHGameRecord captures one completed multiplayer pod.
type EDHGameRecord struct {
	Turns             int
	Players           []EDHPlayerRecord
	Winner            string       // deck name; empty if draw / turn limit
	WinnerCondition   WinCondition // how the winner won (combat, effect, etc.)
	MaxStormCount     int
	TotalManaSpent    int
	TotalCardsPlayed  int
	TotalCombatDamage int
	TotalEliminations int
	// Events is the per-pod replay log. Populated only when
	// EDHRunOptions.RecordEvents is true.
	Events []EDHEvent
}

// EDHSummary captures global EDH aggregates for dashboard highlight cards.
type EDHSummary struct {
	TotalGames          int     `json:"total_games"`
	AverageTurns        float64 `json:"average_turns"`
	TotalManaSpent      int     `json:"total_mana_spent"`
	AverageManaSpent    float64 `json:"average_mana_spent"`
	TotalCardsPlayed    int     `json:"total_cards_played"`
	AverageCardsPlayed  float64 `json:"average_cards_played"`
	HighestStormCount   int     `json:"highest_storm_count"`
	TotalCombatDamage   int     `json:"total_combat_damage"`
	TotalEliminations   int     `json:"total_eliminations"`
	AverageEliminations float64 `json:"average_eliminations"`
	AverageCombatDamage float64 `json:"average_combat_damage"`
}

// EDHDeckStats is the aggregate row exposed to the dashboard.
type EDHDeckStats struct {
	DeckName           string                        `json:"deck_name"`
	CommanderName      string                        `json:"commander_name"`
	Games              int                           `json:"games"`
	Wins               int                           `json:"wins"`
	Losses             int                           `json:"losses"`
	WinRate            float64                       `json:"win_rate"`
	AvgFinalLife       float64                       `json:"avg_final_life"`
	AvgMulligans       float64                       `json:"avg_mulligans"`
	CommanderDamageKOs int                           `json:"commander_damage_kos"`
	LifeLossKOs        int                           `json:"life_loss_kos"`
	MillKOs            int                           `json:"mill_kos"`
	DeckoutKOs         int                           `json:"deckout_kos"`
	EffectKOs          int                           `json:"effect_kos"`
	CombatWins         int                           `json:"combat_wins"`
	EffectWins         int                           `json:"effect_wins"`
	DeckoutWins        int                           `json:"deckout_wins"`
	AvgCommanderCasts  float64                       `json:"avg_commander_casts"`
	AvgManaSpent       float64                       `json:"avg_mana_spent"`
	AvgCardsPlayed     float64                       `json:"avg_cards_played"`
	AvgLandsPlayed     float64                       `json:"avg_lands_played"`
	AvgSpellsCast      float64                       `json:"avg_spells_cast"`
	AvgCreaturesCast   float64                       `json:"avg_creatures_cast"`
	AvgCombatDamage    float64                       `json:"avg_combat_damage"`
	MaxStormCount      int                           `json:"max_storm_count"`
	TotalManaSpent     int                           `json:"total_mana_spent"`
	TotalCardsPlayed   int                           `json:"total_cards_played"`
	TotalCombatDamage  int                           `json:"total_combat_damage"`
	Eliminations       int                           `json:"eliminations"`
	CardStats          map[string]CardPerformance    `json:"card_stats"`
}

// EDHResults aggregates EDHGameRecord values across many simulated pods.
// It is safe for concurrent use.
type EDHResults struct {
	mu      sync.Mutex
	games   []EDHGameRecord
	byDeck  map[string]*deckAccumulator
	summary EDHSummary
}

type cardPerfAccumulator struct {
	casts int
	wins  int
}

type deckAccumulator struct {
	commanderName    string
	games            int
	wins             int
	losses           int
	finalLifeSum     int
	mulligansSum     int
	cmdDamageKOs     int
	lifeLossKOs      int
	millKOs          int
	deckoutKOs       int
	effectKOs        int
	combatWins       int
	effectWins       int
	deckoutWins      int
	commanderCastSum int
	manaSpentSum     int
	cardsPlayedSum   int
	landsPlayedSum   int
	spellsCastSum    int
	creaturesCastSum int
	combatDamageSum  int
	maxStormCount    int
	eliminations     int
	cardStats        map[string]*cardPerfAccumulator
}

// NewEDHResults constructs an empty aggregator.
func NewEDHResults() *EDHResults {
	return &EDHResults{byDeck: map[string]*deckAccumulator{}}
}

// RecordGame appends a completed pod and updates per-deck aggregates.
func (r *EDHResults) RecordGame(rec EDHGameRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.games = append(r.games, rec)
	r.summary.TotalGames++
	r.summary.TotalManaSpent += rec.TotalManaSpent
	r.summary.TotalCardsPlayed += rec.TotalCardsPlayed
	r.summary.TotalCombatDamage += rec.TotalCombatDamage
	r.summary.TotalEliminations += rec.TotalEliminations
	if rec.MaxStormCount > r.summary.HighestStormCount {
		r.summary.HighestStormCount = rec.MaxStormCount
	}
	for _, p := range rec.Players {
		acc := r.byDeck[p.DeckName]
		if acc == nil {
			acc = &deckAccumulator{commanderName: p.CommanderName}
			r.byDeck[p.DeckName] = acc
		}
		acc.games++
		acc.finalLifeSum += p.FinalLife
		acc.mulligansSum += p.Mulligans
		acc.commanderCastSum += p.CommanderCasts
		acc.manaSpentSum += p.ManaSpent
		acc.cardsPlayedSum += p.CardsPlayed
		acc.landsPlayedSum += p.LandsPlayed
		acc.spellsCastSum += p.SpellsCast
		acc.creaturesCastSum += p.CreaturesCast
		acc.combatDamageSum += p.CombatDamage
		acc.eliminations += p.Eliminations
		if p.MaxStormCount > acc.maxStormCount {
			acc.maxStormCount = p.MaxStormCount
		}
		if p.DeckName == rec.Winner {
			acc.wins++
		} else {
			acc.losses++
		}
		if p.Eliminated {
			switch p.KillSource {
			case KillSourceCommanderDamage:
				acc.cmdDamageKOs++
			case KillSourceLifeLoss:
				acc.lifeLossKOs++
			case KillSourceMill:
				acc.millKOs++
			case KillSourceDeckout:
				acc.deckoutKOs++
			case KillSourceEffect:
				acc.effectKOs++
			}
		}
		// Track how this deck won when it is the winner
		if p.DeckName == rec.Winner {
			switch rec.WinnerCondition {
			case WinConditionCombat:
				acc.combatWins++
			case WinConditionEffect:
				acc.effectWins++
			case WinConditionDeckout:
				acc.deckoutWins++
			}
		}
		for cardName, perf := range p.CardStats {
			if acc.cardStats == nil {
				acc.cardStats = map[string]*cardPerfAccumulator{}
			}
			cpa := acc.cardStats[cardName]
			if cpa == nil {
				cpa = &cardPerfAccumulator{}
				acc.cardStats[cardName] = cpa
			}
			cpa.casts += perf.Casts
			if p.DeckName == rec.Winner {
				cpa.wins += perf.Casts
			}
		}
	}
}

// GameCount returns the number of pods recorded.
func (r *EDHResults) GameCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.games)
}

// DeckStats returns a snapshot of per-deck aggregates sorted by win rate.
func (r *EDHResults) DeckStats() []EDHDeckStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]EDHDeckStats, 0, len(r.byDeck))
	for name, acc := range r.byDeck {
		games := acc.games
		row := EDHDeckStats{
			DeckName:           name,
			CommanderName:      acc.commanderName,
			Games:              games,
			Wins:               acc.wins,
			Losses:             acc.losses,
			CommanderDamageKOs: acc.cmdDamageKOs,
			LifeLossKOs:        acc.lifeLossKOs,
			MillKOs:            acc.millKOs,
			DeckoutKOs:         acc.deckoutKOs,
			EffectKOs:          acc.effectKOs,
			CombatWins:         acc.combatWins,
			EffectWins:         acc.effectWins,
			DeckoutWins:        acc.deckoutWins,
			MaxStormCount:      acc.maxStormCount,
			TotalManaSpent:     acc.manaSpentSum,
			TotalCardsPlayed:   acc.cardsPlayedSum,
			TotalCombatDamage:  acc.combatDamageSum,
			Eliminations:       acc.eliminations,
			CardStats:          make(map[string]CardPerformance, len(acc.cardStats)),
		}
		if games > 0 {
			row.WinRate = float64(acc.wins) / float64(games) * 100
			row.AvgFinalLife = float64(acc.finalLifeSum) / float64(games)
			row.AvgMulligans = float64(acc.mulligansSum) / float64(games)
			row.AvgCommanderCasts = float64(acc.commanderCastSum) / float64(games)
			row.AvgManaSpent = float64(acc.manaSpentSum) / float64(games)
			row.AvgCardsPlayed = float64(acc.cardsPlayedSum) / float64(games)
			row.AvgLandsPlayed = float64(acc.landsPlayedSum) / float64(games)
			row.AvgSpellsCast = float64(acc.spellsCastSum) / float64(games)
			row.AvgCreaturesCast = float64(acc.creaturesCastSum) / float64(games)
			row.AvgCombatDamage = float64(acc.combatDamageSum) / float64(games)
		}
		for cname, cpa := range acc.cardStats {
			row.CardStats[cname] = CardPerformance{Casts: cpa.casts, Wins: cpa.wins}
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WinRate > out[j].WinRate })
	return out
}

// Summary returns global EDH telemetry for dashboard highlight cards.
func (r *EDHResults) Summary() EDHSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.summary
	if len(r.games) == 0 {
		return out
	}
	turns := 0
	for _, g := range r.games {
		turns += g.Turns
	}
	games := float64(len(r.games))
	out.TotalGames = len(r.games)
	out.AverageTurns = float64(turns) / games
	out.AverageManaSpent = float64(out.TotalManaSpent) / games
	out.AverageCardsPlayed = float64(out.TotalCardsPlayed) / games
	out.AverageEliminations = float64(out.TotalEliminations) / games
	out.AverageCombatDamage = float64(out.TotalCombatDamage) / games
	return out
}

// RecentGames returns up to limit completed pods, newest first. The returned
// records are deep-copied enough for dashboard/API consumers to read without
// racing the simulator workers.
func (r *EDHResults) RecentGames(limit int) []EDHGameRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || len(r.games) == 0 {
		return nil
	}
	if limit > len(r.games) {
		limit = len(r.games)
	}
	out := make([]EDHGameRecord, 0, limit)
	for i := len(r.games) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, cloneEDHGameRecord(r.games[i]))
	}
	return out
}

func cloneEDHGameRecord(rec EDHGameRecord) EDHGameRecord {
	out := rec
	out.Players = make([]EDHPlayerRecord, len(rec.Players))
	for i, p := range rec.Players {
		out.Players[i] = p
		if p.CardStats != nil {
			out.Players[i].CardStats = make(map[string]CardPerformance, len(p.CardStats))
			for k, v := range p.CardStats {
				out.Players[i].CardStats[k] = v
			}
		}
	}
	out.Events = append([]EDHEvent(nil), rec.Events...)
	return out
}

// AverageTurns returns the mean turn count across recorded pods.
func (r *EDHResults) AverageTurns() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.games) == 0 {
		return 0
	}
	sum := 0
	for _, g := range r.games {
		sum += g.Turns
	}
	return float64(sum) / float64(len(r.games))
}
