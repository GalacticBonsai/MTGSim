package ability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mtgsim/mtgsim/pkg/card"
)

var implementationCacheFile = ".cache/implemented.json"

// sharedParser is a singleton AbilityParser reused across card evaluations to
// avoid recompiling ~500 regex patterns for every card.
var sharedParser = NewAbilityParser()

// ImplementationStatus tracks whether a card is fully supported by the engine.
type ImplementationStatus struct {
	Implemented bool   `json:"implemented"`
	Reason      string `json:"reason,omitempty"`
	ColorID     string `json:"color_id,omitempty"`
	Set         string `json:"set,omitempty"`
	Type        string `json:"type,omitempty"`
}

// ImplementationTracker persists and queries card implementation status.
type ImplementationTracker struct {
	mu      sync.RWMutex
	entries map[string]ImplementationStatus
}

// NewImplementationTracker loads any existing cache and returns a tracker.
func NewImplementationTracker() *ImplementationTracker {
	t := &ImplementationTracker{entries: map[string]ImplementationStatus{}}
	t.load()
	return t
}

func (t *ImplementationTracker) load() {
	data, err := os.ReadFile(implementationCacheFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &t.entries)
}

// Save writes the current cache to disk.
func (t *ImplementationTracker) Save() error {
	if err := os.MkdirAll(filepath.Dir(implementationCacheFile), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(implementationCacheFile, data, 0o644)
}

// EvaluateCard tests a single card against the ability parser/engine and
// records its status. It returns (implemented, reason).
func (t *ImplementationTracker) EvaluateCard(c card.Card) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if existing, ok := t.entries[c.Name]; ok {
		return existing.Implemented, existing.Reason
	}

	impl, reason := testCardImplementation(c)
	t.entries[c.Name] = ImplementationStatus{
		Implemented: impl,
		Reason:      reason,
		ColorID:     colorIDString(c.ColorIdentity),
		Set:         c.Set,
		Type:        simplifyType(c.TypeLine),
	}
	return impl, reason
}

// CheckDeck returns the names of unimplemented cards in a deck list.
func (t *ImplementationTracker) CheckDeck(deckCards []card.Card, db *card.CardDB) []string {
	var unimpl []string
	seen := map[string]bool{}
	for _, dc := range deckCards {
		if seen[dc.Name] {
			continue
		}
		seen[dc.Name] = true
		cd, ok := db.GetCardByName(dc.Name)
		if !ok {
			continue
		}
		impl, _ := t.EvaluateCard(cd)
		if !impl {
			unimpl = append(unimpl, cd.Name)
		}
	}
	sort.Strings(unimpl)
	return unimpl
}

// EvaluateAllInDB scans every card in the database and records its status.
// It forces a full re-evaluation, ignoring any cached results.
func (t *ImplementationTracker) EvaluateAllInDB(db *card.CardDB) {
	t.mu.Lock()
	t.entries = make(map[string]ImplementationStatus)
	t.mu.Unlock()
	for _, c := range db.ListAll() {
		impl, reason := testCardImplementation(c)
		t.mu.Lock()
		t.entries[c.Name] = ImplementationStatus{
			Implemented: impl,
			Reason:      reason,
			ColorID:     colorIDString(c.ColorIdentity),
			Set:         c.Set,
			Type:        simplifyType(c.TypeLine),
		}
		t.mu.Unlock()
	}
}

// GetAll returns a snapshot of all entries.
func (t *ImplementationTracker) GetAll() map[string]ImplementationStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]ImplementationStatus, len(t.entries))
	for k, v := range t.entries {
		out[k] = v
	}
	return out
}

// Stats returns aggregate counts and bucketed totals for percentage calculations.
func (t *ImplementationTracker) Stats() (total, implemented int, byColor, bySet, byType map[string]struct{ Total, Impl int }) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	byColor = map[string]struct{ Total, Impl int }{}
	bySet = map[string]struct{ Total, Impl int }{}
	byType = map[string]struct{ Total, Impl int }{}

	for _, s := range t.entries {
		total++
		byColor[s.ColorID] = struct{ Total, Impl int }{byColor[s.ColorID].Total + 1, byColor[s.ColorID].Impl}
		bySet[s.Set] = struct{ Total, Impl int }{bySet[s.Set].Total + 1, bySet[s.Set].Impl}
		byType[s.Type] = struct{ Total, Impl int }{byType[s.Type].Total + 1, byType[s.Type].Impl}
		if s.Implemented {
			implemented++
			byColor[s.ColorID] = struct{ Total, Impl int }{byColor[s.ColorID].Total, byColor[s.ColorID].Impl + 1}
			bySet[s.Set] = struct{ Total, Impl int }{bySet[s.Set].Total, bySet[s.Set].Impl + 1}
			byType[s.Type] = struct{ Total, Impl int }{byType[s.Type].Total, byType[s.Type].Impl + 1}
		}
	}
	return
}

func testCardImplementation(c card.Card) (bool, string) {
	oracle := strings.TrimSpace(c.OracleText)
	if oracle == "" {
		return true, ""
	}
	if isBasicLand(c.TypeLine) {
		return true, ""
	}

	abilities, err := sharedParser.ParseAbilities(oracle, c)
	if err != nil {
		return false, fmt.Sprintf("parse error: %v", err)
	}

	// A card with oracle text inherently has abilities. If the parser returns
	// zero abilities, that is a parser failure, not a missing-ability card.
	if len(abilities) == 0 {
		trimmed := strings.TrimSpace(oracle)
		// Silver-border reminder/flavor-only cards have no gameplay text
		if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
			return true, ""
		}
		if strings.Contains(strings.ToLower(trimmed), "theme color") {
			return true, ""
		}
		return false, "parser failed to extract abilities from oracle text"
	}

	// Verify every parsed effect is supported by the execution engine.
	for _, ab := range abilities {
		if ab.Approximate {
			return false, ab.ApproximationReason
		}
		for _, eff := range ab.Effects {
			if !CanExecuteEffect(eff.Type) {
				return false, fmt.Sprintf("unsupported effect: %s", eff.Type.String())
			}
			if eff.Approximate {
				return false, eff.ApproximationReason
			}
			if reason := approximateRuntimeReason(eff.Type); reason != "" {
				return false, reason
			}
			for _, cond := range eff.Conditions {
				if !CanExecuteCondition(cond.Type) {
					return false, fmt.Sprintf("unsupported condition: %s", cond.Type.String())
				}
			}
			for _, tgt := range eff.Targets {
				if tgt.Enhanced != nil {
					for _, r := range tgt.Enhanced.Restrictions {
						if !CanExecuteTargetRestriction(r.Type) {
							return false, fmt.Sprintf("unsupported target restriction: %v", r.Type)
						}
					}
				}
				for _, r := range tgt.Restrictions {
					// Basic restrictions are strings; only Enhanced restrictions carry typed data.
					_ = r
				}
			}
		}
	}

	return true, ""
}

func approximateRuntimeReason(effectType EffectType) string {
	return approximateRuntimeReasons[effectType]
}

func isBasicLand(typeLine string) bool {
	lower := strings.ToLower(typeLine)
	return strings.Contains(lower, "basic") && strings.Contains(lower, "land")
}

func colorIDString(colors []string) string {
	order := []string{"W", "U", "B", "R", "G"}
	present := map[string]bool{}
	for _, c := range colors {
		present[c] = true
	}
	var out []string
	for _, c := range order {
		if present[c] {
			out = append(out, c)
		}
	}
	return strings.Join(out, "")
}

func simplifyType(typeLine string) string {
	switch {
	case strings.Contains(typeLine, "Creature"):
		return "Creature"
	case strings.Contains(typeLine, "Instant"):
		return "Instant"
	case strings.Contains(typeLine, "Sorcery"):
		return "Sorcery"
	case strings.Contains(typeLine, "Artifact"):
		return "Artifact"
	case strings.Contains(typeLine, "Enchantment"):
		return "Enchantment"
	case strings.Contains(typeLine, "Planeswalker"):
		return "Planeswalker"
	case strings.Contains(typeLine, "Land"):
		return "Land"
	default:
		return "Other"
	}
}
