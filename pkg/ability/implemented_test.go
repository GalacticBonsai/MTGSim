package ability

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
)

func TestEvaluateCard_BasicLand(t *testing.T) {
	tracker := NewImplementationTracker()
	impl, reason := tracker.EvaluateCard(card.Card{
		Name:       "Forest",
		TypeLine:   "Basic Land — Forest",
		OracleText: "",
	})
	if !impl {
		t.Errorf("Forest should be implemented, got: %s", reason)
	}
}

func TestEvaluateCard_VanillaCreature(t *testing.T) {
	tracker := NewImplementationTracker()
	impl, reason := tracker.EvaluateCard(card.Card{
		Name:       "Grizzly Bears",
		TypeLine:   "Creature — Bear",
		OracleText: "",
	})
	if !impl {
		t.Errorf("Vanilla creature should be implemented, got: %s", reason)
	}
}

func TestEvaluateCard_ImplementedSpell(t *testing.T) {
	tracker := NewImplementationTracker()
	impl, reason := tracker.EvaluateCard(card.Card{
		Name:       "Shock",
		TypeLine:   "Instant",
		OracleText: "Shock deals 2 damage to any target.",
	})
	if !impl {
		t.Errorf("Shock should be implemented, got: %s", reason)
	}
}

func TestEvaluateCard_UnimplementedSorcery(t *testing.T) {
	tracker := NewImplementationTracker()
	impl, reason := tracker.EvaluateCard(card.Card{
		Name:       "Very Complex Spell",
		TypeLine:   "Sorcery",
		OracleText: "Do something completely unprecedented that no parser could ever understand.",
	})
	if impl {
		t.Error("Nonsense sorcery should be unimplemented")
	}
	if reason == "" {
		t.Error("Expected a reason for unimplemented card")
	}
}

func TestImplementationTracker_Persistence(t *testing.T) {
	// Use a temporary cache file to avoid clobbering the real one.
	orig := implementationCacheFile
	implementationCacheFile = ".cache/implemented_test.json"
	defer func() { implementationCacheFile = orig }()

	tracker := NewImplementationTracker()
	tracker.EvaluateCard(card.Card{
		Name:       "TestCard123",
		TypeLine:   "Instant",
		OracleText: "Draw a card.",
	})
	if err := tracker.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	tracker2 := NewImplementationTracker()
	all := tracker2.GetAll()
	if _, ok := all["TestCard123"]; !ok {
		t.Error("Expected TestCard123 to survive reload")
	}

	os.Remove(implementationCacheFile)
}

func TestUpdateImplementationCache(t *testing.T) {
	paths := []string{card.CardDBFile, "../../" + card.CardDBFile}
	var dbPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			dbPath = p
			break
		}
	}
	if dbPath == "" {
		t.Skip("cardDB.json not present, skipping integration test")
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("Failed to read card database: %v", err)
	}
	var cards []card.Card
	if err := json.Unmarshal(data, &cards); err != nil {
		t.Fatalf("Failed to parse card database: %v", err)
	}
	db := card.NewCardDB(cards)
	if db == nil {
		t.Fatal("Failed to create card database")
	}

	tracker := NewImplementationTracker()
	t.Logf("Evaluating %d cards in database...", db.Size())
	tracker.EvaluateAllInDB(db)

	all := tracker.GetAll()
	var impl, unimpl int
	for _, s := range all {
		if s.Implemented {
			impl++
		} else {
			unimpl++
		}
	}
	t.Logf("Implemented: %d, Unimplemented: %d (%.1f%%)", impl, unimpl, float64(impl)*100/float64(len(all)))

	if err := tracker.Save(); err != nil {
		t.Fatalf("Failed to save implementation cache: %v", err)
	}
}

func TestComputeImplementationReport(t *testing.T) {
	paths := []string{card.CardDBFile, "../../" + card.CardDBFile}
	var dbPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			dbPath = p
			break
		}
	}
	if dbPath == "" {
		t.Skip("cardDB.json not present, skipping integration test")
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("Failed to read card database: %v", err)
	}
	var cards []card.Card
	if err := json.Unmarshal(data, &cards); err != nil {
		t.Fatalf("Failed to parse card database: %v", err)
	}
	db := card.NewCardDB(cards)
	if db == nil {
		t.Fatal("Failed to create card database")
	}

	tracker := NewImplementationTracker()
	report, err := card.ComputeImplementationStatus(db, tracker)
	if err != nil {
		t.Fatalf("ComputeImplementationStatus failed: %v", err)
	}

	t.Logf("Total: %d, Implemented: %d, Unimplemented: %d (%.1f%%)",
		report.TotalCards, report.ImplementedCount, report.UnimplementedCount, report.Percentage)

	if report.TotalCards == 0 {
		t.Fatal("expected non-zero total cards")
	}
	if len(report.ByColor) == 0 {
		t.Error("expected non-empty ByColor")
	}
	if len(report.ByType) == 0 {
		t.Error("expected non-empty ByType")
	}
	if len(report.BySet) == 0 {
		t.Error("expected non-empty BySet")
	}

	if err := tracker.Save(); err != nil {
		t.Fatalf("Failed to save implementation cache: %v", err)
	}
}

