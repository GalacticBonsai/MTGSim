package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mtgsim/mtgsim/pkg/simulation"
	"github.com/mtgsim/mtgsim/pkg/stats"
)

// MetaSnapshot captures the state of all decks at a point in time.
type MetaSnapshot struct {
	Timestamp   time.Time                     `json:"timestamp"`
	Name        string                        `json:"name"`
	Decks       []simulation.EDHDeckStats     `json:"decks"`
	CardLibrary map[string]stats.GlobalCardStats `json:"card_library"`
}

// SnapshotManager manages saving and loading deck performance snapshots.
type SnapshotManager struct {
	snapshotsDir string
	snapshots    []MetaSnapshot
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(snapshotsDir string) *SnapshotManager {
	_ = os.MkdirAll(snapshotsDir, 0o755)
	return &SnapshotManager{
		snapshotsDir: snapshotsDir,
		snapshots:    []MetaSnapshot{},
	}
}

// SaveSnapshot saves the current meta state.
func (sm *SnapshotManager) SaveSnapshot(name string, decks []simulation.EDHDeckStats, cardLib map[string]stats.GlobalCardStats) error {
	snapshot := MetaSnapshot{
		Timestamp:   time.Now(),
		Name:        name,
		Decks:       decks,
		CardLibrary: cardLib,
	}

	// Create filename from timestamp
	filename := fmt.Sprintf("%s_%d.json",
		time.Now().Format("2006-01-02_150405"),
		time.Now().UnixNano())

	filepath := filepath.Join(sm.snapshotsDir, filename)

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0o644)
}

// LoadSnapshots loads all available snapshots.
func (sm *SnapshotManager) LoadSnapshots() ([]MetaSnapshot, error) {
	files, err := os.ReadDir(sm.snapshotsDir)
	if err != nil {
		return nil, err
	}

	snapshots := []MetaSnapshot{}
	
	for _, f := range files {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sm.snapshotsDir, f.Name()))
		if err != nil {
			continue
		}

		var snapshot MetaSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	// Sort by timestamp descending
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	sm.snapshots = snapshots
	return snapshots, nil
}

// ComparSnapshots compares two snapshots to show meta evolution.
type SnapshotComparison struct {
	Before               MetaSnapshot                `json:"before"`
	After                MetaSnapshot                `json:"after"`
	DeckWinRateChanges   map[string]float64          `json:"deck_win_rate_changes"`
	NewDecks             []string                    `json:"new_decks"`
	RemovedDecks         []string                    `json:"removed_decks"`
	CardPerformanceShift map[string]interface{}      `json:"card_performance_shift"`
}

// CompareSnapshots compares two snapshots to track meta changes.
func CompareSnapshots(before, after MetaSnapshot) SnapshotComparison {
	comp := SnapshotComparison{
		Before:              before,
		After:               after,
		DeckWinRateChanges:  make(map[string]float64),
		CardPerformanceShift: make(map[string]interface{}),
	}

	// Map decks by name for comparison
	beforeDecks := make(map[string]*simulation.EDHDeckStats)
	for i := range before.Decks {
		beforeDecks[before.Decks[i].DeckName] = &before.Decks[i]
	}

	afterDecks := make(map[string]*simulation.EDHDeckStats)
	for i := range after.Decks {
		afterDecks[after.Decks[i].DeckName] = &after.Decks[i]
	}

	// Calculate win rate changes for existing decks
	for name, afterDeck := range afterDecks {
		if beforeDeck, exists := beforeDecks[name]; exists {
			change := afterDeck.WinRate - beforeDeck.WinRate
			comp.DeckWinRateChanges[name] = change
		}
	}

	// Find new and removed decks
	for name := range afterDecks {
		if _, exists := beforeDecks[name]; !exists {
			comp.NewDecks = append(comp.NewDecks, name)
		}
	}

	for name := range beforeDecks {
		if _, exists := afterDecks[name]; !exists {
			comp.RemovedDecks = append(comp.RemovedDecks, name)
		}
	}

	// Compare card performance shifts between snapshots
	for cardName, beforeStats := range before.CardLibrary {
		if afterStats, exists := after.CardLibrary[cardName]; exists {
			beforeWR := 0.0
			if beforeStats.Casts > 0 {
				beforeWR = float64(beforeStats.Wins) / float64(beforeStats.Casts) * 100
			}
			afterWR := 0.0
			if afterStats.Casts > 0 {
				afterWR = float64(afterStats.Wins) / float64(afterStats.Casts) * 100
			}
			change := afterWR - beforeWR
			// Only include cards with meaningful sample size and notable change
			if afterStats.Casts >= 5 && (change >= 5 || change <= -5) {
				comp.CardPerformanceShift[cardName] = map[string]interface{}{
					"change": change,
					"casts":  afterStats.Casts,
					"wins":   afterStats.Wins,
					"before_wr": beforeWR,
					"after_wr":  afterWR,
				}
			}
		}
	}

	return comp
}

// GetMetaTrends analyzes multiple snapshots to show overall trends.
type MetaTrend struct {
	Timestamp   time.Time                   `json:"timestamp"`
	TopDecks    []DeckTrendData              `json:"top_decks"`
	AverageWR   float64                     `json:"average_win_rate"`
	HighestWR   float64                     `json:"highest_win_rate"`
	LowestWR    float64                     `json:"lowest_win_rate"`
	Diversity   float64                     `json:"deck_diversity"` // Number of unique decks
}

// DeckTrendData captures one deck's trend over time.
type DeckTrendData struct {
	Name    string  `json:"name"`
	WinRate float64 `json:"win_rate"`
	Trend   string  `json:"trend"` // "up", "down", "stable"
}

// AnalyzeTrends analyzes trends across multiple snapshots.
func AnalyzeTrends(snapshots []MetaSnapshot) []MetaTrend {
	trends := []MetaTrend{}

	for _, snapshot := range snapshots {
		trend := MetaTrend{
			Timestamp: snapshot.Timestamp,
		}

		if len(snapshot.Decks) == 0 {
			continue
		}

		// Calculate statistics
		totalWR := 0.0
		maxWR := 0.0
		minWR := 100.0

		topDecks := []DeckTrendData{}
		for _, deck := range snapshot.Decks {
			if deck.Games > 0 {
				totalWR += deck.WinRate
				if deck.WinRate > maxWR {
					maxWR = deck.WinRate
				}
				if deck.WinRate < minWR {
					minWR = deck.WinRate
				}

				topDecks = append(topDecks, DeckTrendData{
					Name:    deck.DeckName,
					WinRate: deck.WinRate,
				})
			}
		}

		// Sort top decks by win rate
		sort.Slice(topDecks, func(i, j int) bool {
			return topDecks[i].WinRate > topDecks[j].WinRate
		})

		trend.TopDecks = topDecks[:minInt(5, len(topDecks))]
		trend.AverageWR = totalWR / float64(len(snapshot.Decks))
		trend.HighestWR = maxWR
		trend.LowestWR = minWR
		trend.Diversity = float64(len(snapshot.Decks))

		trends = append(trends, trend)
	}

	return trends
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
