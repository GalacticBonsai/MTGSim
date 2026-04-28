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
	KillSourceTurnLimit       KillSource = "turn_limit"
	KillSourceUnknown         KillSource = "unknown"
)

// EDHPlayerRecord captures one seat in one game.
type EDHPlayerRecord struct {
	DeckName       string
	CommanderName  string
	Mulligans      int
	FinalLife      int
	CommanderCasts int
	Eliminated     bool
	KillSource     KillSource
}

// EDHGameRecord captures one completed multiplayer pod.
type EDHGameRecord struct {
	Turns   int
	Players []EDHPlayerRecord
	Winner  string // deck name; empty if draw / turn limit
}

// EDHDeckStats is the aggregate row exposed to the dashboard.
type EDHDeckStats struct {
	DeckName            string  `json:"deck_name"`
	CommanderName       string  `json:"commander_name"`
	Games               int     `json:"games"`
	Wins                int     `json:"wins"`
	Losses              int     `json:"losses"`
	WinRate             float64 `json:"win_rate"`
	AvgFinalLife        float64 `json:"avg_final_life"`
	AvgMulligans        float64 `json:"avg_mulligans"`
	CommanderDamageKOs  int     `json:"commander_damage_kos"`
	LifeLossKOs         int     `json:"life_loss_kos"`
	MillKOs             int     `json:"mill_kos"`
	AvgCommanderCasts   float64 `json:"avg_commander_casts"`
}

// EDHResults aggregates EDHGameRecord values across many simulated pods.
// It is safe for concurrent use.
type EDHResults struct {
	mu      sync.Mutex
	games   []EDHGameRecord
	byDeck  map[string]*deckAccumulator
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
	commanderCastSum int
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
		}
		if games > 0 {
			row.WinRate = float64(acc.wins) / float64(games) * 100
			row.AvgFinalLife = float64(acc.finalLifeSum) / float64(games)
			row.AvgMulligans = float64(acc.mulligansSum) / float64(games)
			row.AvgCommanderCasts = float64(acc.commanderCastSum) / float64(games)
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WinRate > out[j].WinRate })
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
