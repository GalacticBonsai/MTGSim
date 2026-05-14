package stats

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
)

// GlobalCardStats tracks aggregate cast and win data for a single card
// across all simulations and decks.
type GlobalCardStats struct {
	Casts    int    `json:"casts"`
	Wins     int    `json:"wins"`
	ImageURL string `json:"image_url,omitempty"`
}

// CardLibrary is a thread-safe, persistent collection of global card stats.
type CardLibrary struct {
	mu    sync.Mutex
	path  string
	Cards map[string]GlobalCardStats `json:"cards"`
}

// LoadCardLibrary reads a card library from disk. If the file does not exist,
// an empty library is returned without error.
func LoadCardLibrary(path string) (*CardLibrary, error) {
	cl := &CardLibrary{path: path, Cards: map[string]GlobalCardStats{}}
	if path == "" {
		return cl, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cl, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &cl.Cards); err != nil {
		return nil, err
	}
	return cl, nil
}

// Save persists the library to its path atomically (write to temp, then rename).
func (cl *CardLibrary) Save() error {
	if cl.path == "" {
		return nil
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	data, err := json.MarshalIndent(cl.Cards, "", "  ")
	if err != nil {
		return err
	}
	tmp := cl.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, cl.path)
}

// RecordCounts adds cast and win tallies for a specific card.
func (cl *CardLibrary) RecordCounts(cardName string, casts, wins int) {
	if cardName == "" || casts <= 0 {
		return
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	s := cl.Cards[cardName]
	s.Casts += casts
	s.Wins += wins
	cl.Cards[cardName] = s
}

// SetImageURL stores the canonical image URL for a card name.
func (cl *CardLibrary) SetImageURL(cardName, url string) {
	if cardName == "" || url == "" {
		return
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	s := cl.Cards[cardName]
	s.ImageURL = url
	cl.Cards[cardName] = s
}

// Snapshot returns a shallow copy of the underlying card map. The copy is safe
// to iterate concurrently with further mutations to the library.
func (cl *CardLibrary) Snapshot() map[string]GlobalCardStats {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	out := make(map[string]GlobalCardStats, len(cl.Cards))
	for k, v := range cl.Cards {
		out[k] = v
	}
	return out
}

// Entry is a single row for TopCards results.
type Entry struct {
	Name    string
	Casts   int
	Wins    int
	WinRate float64
}

// TopCards returns cards sorted by win rate descending, filtered to those with
// at least minCasts observations. Limit caps the result length (0 = unlimited).
func (cl *CardLibrary) TopCards(minCasts, limit int) []Entry {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	var out []Entry
	for name, s := range cl.Cards {
		if s.Casts >= minCasts {
			out = append(out, Entry{
				Name: name, Casts: s.Casts, Wins: s.Wins,
				WinRate: 100 * float64(s.Wins) / float64(s.Casts),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].WinRate != out[j].WinRate {
			return out[i].WinRate > out[j].WinRate
		}
		return out[i].Casts > out[j].Casts
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
