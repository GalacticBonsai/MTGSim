package card

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestCardIsLand(t *testing.T) {
	land := Card{Name: "Forest", TypeLine: "Basic Land — Forest"}
	if !land.IsLand() {
		t.Errorf("expected Forest to be a land")
	}
	creature := Card{Name: "Grizzly Bears", TypeLine: "Creature — Bear"}
	if creature.IsLand() {
		t.Errorf("expected Grizzly Bears not to be a land")
	}
}

func TestCardIsCreature(t *testing.T) {
	creature := Card{Name: "Grizzly Bears", TypeLine: "Creature — Bear"}
	if !creature.IsCreature() {
		t.Errorf("expected Grizzly Bears to be a creature")
	}
	instant := Card{Name: "Lightning Bolt", TypeLine: "Instant"}
	if instant.IsCreature() {
		t.Errorf("expected Lightning Bolt not to be a creature")
	}
}

func TestCardIsInstant(t *testing.T) {
	instant := Card{Name: "Lightning Bolt", TypeLine: "Instant"}
	if !instant.IsInstant() {
		t.Errorf("expected Lightning Bolt to be an instant")
	}
	sorcery := Card{Name: "Wrath of God", TypeLine: "Sorcery"}
	if sorcery.IsInstant() {
		t.Errorf("expected Wrath of God not to be an instant")
	}
}

func TestCardIsSorcery(t *testing.T) {
	sorcery := Card{Name: "Wrath of God", TypeLine: "Sorcery"}
	if !sorcery.IsSorcery() {
		t.Errorf("expected Wrath of God to be a sorcery")
	}
	instant := Card{Name: "Lightning Bolt", TypeLine: "Instant"}
	if instant.IsSorcery() {
		t.Errorf("expected Lightning Bolt not to be a sorcery")
	}
}

func TestCardIsArtifact(t *testing.T) {
	artifact := Card{Name: "Sol Ring", TypeLine: "Artifact"}
	if !artifact.IsArtifact() {
		t.Errorf("expected Sol Ring to be an artifact")
	}
	creature := Card{Name: "Grizzly Bears", TypeLine: "Creature — Bear"}
	if creature.IsArtifact() {
		t.Errorf("expected Grizzly Bears not to be an artifact")
	}
}

func TestCardIsEnchantment(t *testing.T) {
	enchantment := Card{Name: "Oblivion Ring", TypeLine: "Enchantment"}
	if !enchantment.IsEnchantment() {
		t.Errorf("expected Oblivion Ring to be an enchantment")
	}
	creature := Card{Name: "Grizzly Bears", TypeLine: "Creature — Bear"}
	if creature.IsEnchantment() {
		t.Errorf("expected Grizzly Bears not to be an enchantment")
	}
}

func TestCardIsPlaneswalker(t *testing.T) {
	pw := Card{Name: "Jace", TypeLine: "Planeswalker — Jace"}
	if !pw.IsPlaneswalker() {
		t.Errorf("expected Jace to be a planeswalker")
	}
	creature := Card{Name: "Grizzly Bears", TypeLine: "Creature — Bear"}
	if creature.IsPlaneswalker() {
		t.Errorf("expected Grizzly Bears not to be a planeswalker")
	}
}

func TestCardHasKeyword(t *testing.T) {
	c := Card{Name: "Birds of Paradise", Keywords: []string{"Flying"}}
	if !c.HasKeyword("Flying") {
		t.Errorf("expected Birds of Paradise to have Flying")
	}
	if c.HasKeyword("Trample") {
		t.Errorf("expected Birds of Paradise not to have Trample")
	}
	// Case insensitive
	if !c.HasKeyword("flying") {
		t.Errorf("expected HasKeyword to be case insensitive")
	}
}

func TestCardCast(t *testing.T) {
	creature := Card{Name: "Grizzly Bears", TypeLine: "Creature — Bear", Power: "2", Toughness: "2"}
	// Should not panic
	creature.Cast(nil, nil)

	spell := Card{Name: "Lightning Bolt", TypeLine: "Instant"}
	spell.Cast(nil, nil)
}

func TestCardDisplay(t *testing.T) {
	land := Card{Name: "Forest", TypeLine: "Basic Land — Forest"}
	creature := Card{Name: "Grizzly Bears", TypeLine: "Creature — Bear", CMC: 2, Power: "2", Toughness: "2"}
	spell := Card{Name: "Lightning Bolt", TypeLine: "Instant", CMC: 1}

	// These use log.Printf; just ensure they don't panic
	land.Display()
	creature.Display()
	spell.Display()
}

func TestDisplayCards(t *testing.T) {
	cards := []Card{
		{Name: "Forest", TypeLine: "Basic Land — Forest"},
		{Name: "Grizzly Bears", TypeLine: "Creature — Bear", CMC: 2, Power: "2", Toughness: "2"},
	}
	// Should not panic
	DisplayCards(cards)
	DisplayCards(nil)
}

func TestCardTypeEdgeCases(t *testing.T) {
	// Artifact Creature counts as both but our functions check Contains
	artCreature := Card{Name: "Solemn Simulacrum", TypeLine: "Artifact Creature — Golem"}
	if !artCreature.IsCreature() {
		t.Errorf("expected Artifact Creature to be a creature")
	}
	if !artCreature.IsArtifact() {
		t.Errorf("expected Artifact Creature to be an artifact")
	}

	// Creature Land
	creatureLand := Card{Name: "Treetop Village", TypeLine: "Land — Forest"}
	if !creatureLand.IsLand() {
		t.Errorf("expected Land to be a land")
	}
	if creatureLand.IsCreature() {
		t.Errorf("expected Land not to be a creature")
	}

	// Enchantment Artifact
	enchArt := Card{Name: "Liquimetal Coating", TypeLine: "Artifact — Equipment"}
	if !enchArt.IsArtifact() {
		t.Errorf("expected Artifact to be an artifact")
	}
	if enchArt.IsEnchantment() {
		t.Errorf("expected Artifact not to be an enchantment")
	}
}

func TestCardCastWithNonNumericPT(t *testing.T) {
	// Creature with */* power/toughness should not panic
	starCreature := Card{Name: "Tarmogoyf", TypeLine: "Creature — Lhurgoyf", Power: "*", Toughness: "1+*"}
	starCreature.Cast(nil, nil)
}

func TestCardSimplifyType(t *testing.T) {
	tests := []struct {
		typ      string
		expected string
	}{
		{"Creature — Bear", "Creature"},
		{"Instant", "Instant"},
		{"Sorcery", "Sorcery"},
		{"Artifact — Equipment", "Artifact"},
		{"Enchantment — Aura", "Enchantment"},
		{"Land — Forest", "Land"},
		{"Planeswalker — Jace", "Planeswalker"},
		{"Tribal", "Other"},
		{"Scheme", "Other"},
	}
	for _, tt := range tests {
		c := Card{TypeLine: tt.typ}
		got := simplifyType(c.TypeLine)
		if got != tt.expected {
			t.Errorf("simplifyType(%q) = %q, want %q", tt.typ, got, tt.expected)
		}
	}
}

func TestCardClassifyColor(t *testing.T) {
	tests := []struct {
		colors   []string
		expected string
	}{
		{[]string{}, "Colorless"},
		{[]string{"W"}, "W"},
		{[]string{"U"}, "U"},
		{[]string{"B"}, "B"},
		{[]string{"R"}, "R"},
		{[]string{"G"}, "G"},
		{[]string{"W", "U"}, "Multicolor"},
		{[]string{"X"}, "Colorless"},
	}
	for _, tt := range tests {
		got := classifyColor(tt.colors)
		if got != tt.expected {
			t.Errorf("classifyColor(%v) = %q, want %q", tt.colors, got, tt.expected)
		}
	}
}

func TestCardSafePct(t *testing.T) {
	if safePct(5, 10) != 50.0 {
		t.Errorf("safePct(5,10) = %f, want 50.0", safePct(5, 10))
	}
	if safePct(0, 0) != 0.0 {
		t.Errorf("safePct(0,0) = %f, want 0.0", safePct(0, 0))
	}
}

func TestCardCategorizeReason(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"parser failed to extract abilities", "Parser Failure"},
		{"parse error in oracle text", "Parser Failure"},
		{"unsupported effect damage to player", "Unsupported Effect"},
		{"unsupported condition hellbent", "Unsupported Condition"},
		{"unsupported target restriction planeswalker", "Unsupported Target Restriction"},
		{"some other reason", "Other"},
	}
	for _, tt := range tests {
		got := categorizeReason(tt.input)
		if got != tt.expected {
			t.Errorf("categorizeReason(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCardColorString(t *testing.T) {
	if colorString([]string{"G", "W", "U"}) != "WUG" {
		t.Errorf("colorString({G,W,U}) = %q, want WUG", colorString([]string{"G", "W", "U"}))
	}
	if colorString([]string{}) != "C" {
		t.Errorf("colorString({}) = %q, want C", colorString([]string{}))
	}
	if colorString([]string{"R", "B"}) != "BR" {
		t.Errorf("colorString({R,B}) = %q, want BR", colorString([]string{"R", "B"}))
	}
	// Verify ordering
	if colorString([]string{"R", "G", "W"}) != "WRG" {
		t.Errorf("colorString({R,G,W}) = %q, want WRG", colorString([]string{"R", "G", "W"}))
	}
}

func TestNewCardDBEmpty(t *testing.T) {
	db := NewCardDB([]Card{})
	if db != nil {
		t.Errorf("NewCardDB with empty slice should return nil, got %v", db)
	}
}

func TestManaAddNilPool(t *testing.T) {
	var m Mana
	m.Add(game.Red, 1)
	if m.Get(game.Red) != 1 {
		t.Errorf("expected 1 red mana after Add to nil pool, got %d", m.Get(game.Red))
	}
}

func TestCheckManaProducerNoAdd(t *testing.T) {
	isProducer, manaTypes := CheckManaProducer("Deal 3 damage to any target.")
	if isProducer {
		t.Errorf("expected CheckManaProducer to return false for non-mana text")
	}
	if len(manaTypes) != 0 {
		t.Errorf("expected empty mana types, got %v", manaTypes)
	}
}

func TestManaPoolPayFail(t *testing.T) {
	pool := NewManaPool()
	pool.Add(game.Red, 1)
	cost := ParseManaCost("{R}{R}")
	err := pool.Pay(cost)
	if err == nil {
		t.Errorf("expected Pay to fail with insufficient mana")
	}
}

func TestLoadCardDatabaseFromFile(t *testing.T) {
	// Backup existing cardDB if present
	backup := CardDBFile + ".testbackup"
	existing := false
	if _, err := os.Stat(CardDBFile); err == nil {
		if err := os.Rename(CardDBFile, backup); err != nil {
			t.Fatalf("failed to backup existing cardDB: %v", err)
		}
		existing = true
	}

	// Ensure cleanup and restore
	defer func() {
		_ = os.Remove(CardDBFile)
		if existing {
			_ = os.Rename(backup, CardDBFile)
		}
	}()

	// Create cache dir and write minimal cardDB
	if err := os.MkdirAll(filepath.Dir(CardDBFile), 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}
	data := []Card{
		{Name: "Test Card", TypeLine: "Creature", CMC: 1},
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test cards: %v", err)
	}
	if err := os.WriteFile(CardDBFile, jsonData, 0644); err != nil {
		t.Fatalf("failed to write test cardDB: %v", err)
	}

	db, err := LoadCardDatabase()
	if err != nil {
		t.Fatalf("LoadCardDatabase failed: %v", err)
	}
	if db == nil {
		t.Fatal("LoadCardDatabase returned nil")
	}
	if db.Size() != 1 {
		t.Errorf("expected db size 1, got %d", db.Size())
	}
	card, ok := db.GetCardByName("Test Card")
	if !ok {
		t.Fatal("expected Test Card to exist in loaded db")
	}
	if card.Name != "Test Card" {
		t.Errorf("expected Test Card, got %s", card.Name)
	}
}
