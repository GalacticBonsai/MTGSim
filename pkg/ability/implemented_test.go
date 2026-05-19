package ability

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
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

func TestEvaluateCard_UnsupportedActivatedAbilityIsNotImplemented(t *testing.T) {
	impl, reason := testCardImplementation(card.Card{
		Name:       "Mystery Engine",
		TypeLine:   "Artifact",
		OracleText: "{2}: Do something unusual.",
	})
	if impl {
		t.Fatal("unsupported activated ability should not count as fully implemented")
	}
	if !strings.Contains(reason, "parser failed") {
		t.Fatalf("expected parser failure reason, got %q", reason)
	}
}

func TestEvaluateCard_RuntimePlaceholderIsNotFullyImplemented(t *testing.T) {
	impl, reason := testCardImplementation(card.Card{
		Name:       "Simple Charm",
		TypeLine:   "Instant",
		OracleText: "Choose one — Draw a card.",
	})
	if impl {
		t.Fatal("modal placeholder should not count as fully implemented")
	}
	if !strings.Contains(reason, "modal choices") {
		t.Fatalf("expected modal placeholder reason, got %q", reason)
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

	_ = os.Remove(implementationCacheFile)
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

	for _, bucket := range report.FailureReasons {
		t.Logf("Failure bucket: %s = %d", bucket.Category, bucket.Count)
	}

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
func TestAnalyzeParserFailures(t *testing.T) {
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

	firstWordCounts := make(map[string]int)
	keywordCounts := make(map[string]int)
	sampleTexts := make(map[string][]string)
	const maxSamples = 3

	for _, c := range cards {
		impl, reason := testCardImplementation(c)
		if !impl && strings.Contains(reason, "parser failed") {
			oracle := strings.TrimSpace(c.OracleText)
			if oracle == "" {
				continue
			}
			// Get first word of first sentence
			sentences := strings.Split(oracle, ".")
			firstSentence := strings.TrimSpace(sentences[0])
			words := strings.Fields(firstSentence)
			if len(words) > 0 {
				firstWord := strings.ToLower(words[0])
				firstWordCounts[firstWord]++
				if len(sampleTexts[firstWord]) < maxSamples {
					sampleTexts[firstWord] = append(sampleTexts[firstWord], firstSentence)
				}
			}

			// Check for common keywords
			lower := strings.ToLower(oracle)
			keywords := []string{"mill", "exile", "scry", "counter", "kicker", "flashback", "equip", "lifelink", "trample", "flying", "haste", "vigilance", "deathtouch", "menace", "reach", "hexproof", "indestructible", "ward", "prowess", "cascade", "convoke", "delve", "dredge", "persist", "undying", "unearth", "bloodthirst", "annihilator", "morph", "manifest", "embalm", "eternalize", "aftermath", "adventure", "mutate", "foretell", "strive", "rebound", "suspend", "madness", "flash", "defender", "double strike", "first strike", "protection", "regenerate", "fear", "intimidate", "shadow", "shroud", "infect", "wither", "poisonous", "storm", "buyback", "replicate", "splice", "transmute"}
			for _, kw := range keywords {
				if strings.Contains(lower, kw) {
					keywordCounts[kw]++
				}
			}
		}
	}

	// Sort and log top first words
	t.Logf("=== Top first words in parser failures ===")
	type wordCount struct {
		word  string
		count int
	}
	var wcs []wordCount
	for w, c := range firstWordCounts {
		wcs = append(wcs, wordCount{w, c})
	}
	sort.Slice(wcs, func(i, j int) bool { return wcs[i].count > wcs[j].count })
	for i := 0; i < 30 && i < len(wcs); i++ {
		t.Logf("  %s: %d", wcs[i].word, wcs[i].count)
		for _, sample := range sampleTexts[wcs[i].word] {
			t.Logf("    - %s", sample)
		}
	}

	t.Logf("=== Top keywords in parser failures ===")
	var kcs []wordCount
	for k, c := range keywordCounts {
		kcs = append(kcs, wordCount{k, c})
	}
	sort.Slice(kcs, func(i, j int) bool { return kcs[i].count > kcs[j].count })
	for i := 0; i < 20 && i < len(kcs); i++ {
		t.Logf("  %s: %d", kcs[i].word, kcs[i].count)
	}
}
