package card

import (
	"testing"
)

type mockEvaluator struct {
	results map[string]bool
	reasons map[string]string
}

func (m *mockEvaluator) EvaluateCard(c Card) (bool, string) {
	return m.results[c.Name], m.reasons[c.Name]
}

func TestComputeImplementationStatus(t *testing.T) {
	db := NewCardDB([]Card{
		{Name: "Forest", TypeLine: "Basic Land — Forest", ColorIdentity: []string{}, Set: "lea"},
		{Name: "Lightning Bolt", TypeLine: "Instant", ColorIdentity: []string{"R"}, Set: "lea"},
		{Name: "Counterspell", TypeLine: "Instant", ColorIdentity: []string{"U"}, Set: "lea"},
		{Name: "Unsupported Spell", TypeLine: "Sorcery", ColorIdentity: []string{"B"}, Set: "lea"},
		{Name: "Multicolor Guy", TypeLine: "Creature", ColorIdentity: []string{"W", "U"}, Set: "lea"},
	})

	m := &mockEvaluator{
		results: map[string]bool{
			"Forest":            true,
			"Lightning Bolt":    true,
			"Counterspell":      true,
			"Unsupported Spell": false,
			"Multicolor Guy":    true,
		},
		reasons: map[string]string{
			"Unsupported Spell": "parser failed to extract abilities from oracle text",
		},
	}

	report, err := ComputeImplementationStatus(db, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalCards != 5 {
		t.Errorf("expected TotalCards=5, got %d", report.TotalCards)
	}
	if report.ImplementedCount != 4 {
		t.Errorf("expected ImplementedCount=4, got %d", report.ImplementedCount)
	}
	if report.UnimplementedCount != 1 {
		t.Errorf("expected UnimplementedCount=1, got %d", report.UnimplementedCount)
	}
	if len(report.UnimplementedCards) != 1 || report.UnimplementedCards[0].Name != "Unsupported Spell" {
		t.Errorf("expected unimplemented list with Unsupported Spell, got %v", report.UnimplementedCards)
	}

	// Verify color buckets
	colorMap := map[string]Bucket{}
	for _, b := range report.ByColor {
		colorMap[b.Name] = b
	}
	if colorMap["R"].Implemented != 1 || colorMap["R"].Total != 1 {
		t.Errorf("R bucket wrong: %+v", colorMap["R"])
	}
	if colorMap["Multicolor"].Implemented != 1 || colorMap["Multicolor"].Total != 1 {
		t.Errorf("Multicolor bucket wrong: %+v", colorMap["Multicolor"])
	}
	if colorMap["Colorless"].Implemented != 1 || colorMap["Colorless"].Total != 1 {
		t.Errorf("Colorless bucket wrong: %+v", colorMap["Colorless"])
	}

	// Verify type buckets
	typeMap := map[string]Bucket{}
	for _, b := range report.ByType {
		typeMap[b.Name] = b
	}
	if typeMap["Land"].Total != 1 {
		t.Errorf("Land bucket wrong: %+v", typeMap["Land"])
	}
	if typeMap["Instant"].Total != 2 {
		t.Errorf("Instant bucket wrong: %+v", typeMap["Instant"])
	}
	if typeMap["Sorcery"].Implemented != 0 {
		t.Errorf("Sorcery bucket wrong: %+v", typeMap["Sorcery"])
	}
}
