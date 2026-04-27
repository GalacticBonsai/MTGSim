package deck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
)

func newCommanderMockDB() *MockCardDB {
	cards := map[string]card.Card{
		"Krenko, Mob Boss": {
			Name: "Krenko, Mob Boss", CMC: 4, ManaCost: "{2}{R}{R}",
			TypeLine: "Legendary Creature — Goblin Warrior",
			Colors:   []string{"R"}, ColorIdentity: []string{"R"},
		},
		"Goblin Guide": {
			Name: "Goblin Guide", CMC: 1, ManaCost: "{R}",
			TypeLine: "Creature — Goblin Scout",
			Colors:   []string{"R"}, ColorIdentity: []string{"R"},
		},
		"Mountain": {
			Name: "Mountain", CMC: 0, TypeLine: "Basic Land — Mountain",
			ColorIdentity: []string{"R"},
		},
		"Llanowar Elves": {
			Name: "Llanowar Elves", CMC: 1, ManaCost: "{G}",
			TypeLine: "Creature — Elf Druid",
			Colors:   []string{"G"}, ColorIdentity: []string{"G"},
		},
		"Sol Ring": {
			Name: "Sol Ring", CMC: 1, ManaCost: "{1}",
			TypeLine: "Artifact", ColorIdentity: []string{},
		},
	}
	return &MockCardDB{cards: cards}
}

func writeTempDeck(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.deck")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp deck: %v", err)
	}
	return path
}

func TestImportCommanderDeckfile_Valid(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `Commander
1 Krenko, Mob Boss

Deck
1 Goblin Guide
2 Mountain
1 Sol Ring
`)
	cmd, main, err := ImportCommanderDeckfile(path, db)
	if err != nil {
		t.Fatalf("expected valid commander import, got %v", err)
	}
	if cmd.Name != "Krenko, Mob Boss" {
		t.Fatalf("expected Krenko commander, got %q", cmd.Name)
	}
	if main.Size() != 4 {
		t.Fatalf("expected 4 cards in main, got %d", main.Size())
	}
}

func TestImportCommanderDeckfile_RejectsOffColor(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `Commander
1 Krenko, Mob Boss

Deck
1 Llanowar Elves
1 Mountain
`)
	_, _, err := ImportCommanderDeckfile(path, db)
	if err == nil {
		t.Fatalf("expected color identity violation error")
	}
	if !strings.Contains(err.Error(), "color identity") {
		t.Fatalf("expected color identity error, got %v", err)
	}
}

func TestImportCommanderDeckfile_MissingCommander(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `Deck
1 Goblin Guide
1 Mountain
`)
	_, _, err := ImportCommanderDeckfile(path, db)
	if err == nil {
		t.Fatalf("expected missing commander error")
	}
}

func TestImportDeckfile_StillIgnoresCommanderSection(t *testing.T) {
	// Plain ImportDeckfile should still parse OK even with a Commander
	// heading present (for backwards compat). The commander goes into a
	// separate slot internal to the importer and is dropped at this API.
	db := newCommanderMockDB()
	path := writeTempDeck(t, `Commander
1 Krenko, Mob Boss

Deck
1 Goblin Guide
1 Mountain
`)
	main, side, err := ImportDeckfile(path, db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if main.Size() != 2 {
		t.Fatalf("expected 2 main cards, got %d", main.Size())
	}
	if side.Size() != 0 {
		t.Fatalf("expected empty sideboard, got %d", side.Size())
	}
}
