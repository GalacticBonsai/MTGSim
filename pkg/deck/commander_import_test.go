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
		"Thrasios, Triton Hero": {
			Name: "Thrasios, Triton Hero", CMC: 2, ManaCost: "{G}{U}",
			TypeLine: "Legendary Creature — Merfolk Wizard",
			Colors:   []string{"G", "U"}, ColorIdentity: []string{"G", "U"},
		},
		"Tymna the Weaver": {
			Name: "Tymna the Weaver", CMC: 3, ManaCost: "{1}{W}{B}",
			TypeLine: "Legendary Creature — Human Cleric",
			Colors:   []string{"W", "B"}, ColorIdentity: []string{"W", "B"},
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

func TestImportCommanderDeckfileWithSideboard_ValidatesSideboard(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `Commander
1 Krenko, Mob Boss

Deck
1 Goblin Guide
1 Mountain

Sideboard
1 Sol Ring
`)
	cmd, main, side, err := ImportCommanderDeckfileWithSideboard(path, db)
	if err != nil {
		t.Fatalf("expected valid commander deck with sideboard, got %v", err)
	}
	if cmd.Name != "Krenko, Mob Boss" || main.Size() != 2 || side.Size() != 1 {
		t.Fatalf("unexpected import cmd=%s main=%d side=%d", cmd.Name, main.Size(), side.Size())
	}

	offColor := writeTempDeck(t, `Commander
1 Krenko, Mob Boss

Deck
1 Goblin Guide
1 Mountain

Sideboard
1 Llanowar Elves
`)
	_, _, _, err = ImportCommanderDeckfileWithSideboard(offColor, db)
	if err == nil || !strings.Contains(err.Error(), "color identity") {
		t.Fatalf("expected sideboard color identity error, got %v", err)
	}
}

func TestImportCommanderDeckfile_CockatriceDCK(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `NAME:krenko test
1 [FDN:204] Goblin Guide
2 [KTK:262] Mountain
SB: 1 [FDN:204] Krenko, Mob Boss
LAYOUT MAIN:(1,1)(NONE,false,50)|([FDN:204])
`)
	cmd, main, side, err := ImportCommanderDeckfileWithSideboard(path, db)
	if err != nil {
		t.Fatalf("expected cockatrice import, got %v", err)
	}
	if cmd.Name != "Krenko, Mob Boss" || main.Name != "krenko test" || main.Size() != 3 || side.Size() != 0 {
		t.Fatalf("unexpected import cmd=%s name=%s main=%d side=%d", cmd.Name, main.Name, main.Size(), side.Size())
	}
}

func TestImportCommanderDeckfile_MoxfieldTXT(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `1 Goblin Guide
1 Mountain

SIDEBOARD:
1 Sol Ring

1 Krenko, Mob Boss
`)
	cmd, main, side, err := ImportCommanderDeckfileWithSideboard(path, db)
	if err != nil {
		t.Fatalf("expected moxfield import, got %v", err)
	}
	if cmd.Name != "Krenko, Mob Boss" || main.Size() != 2 || side.Size() != 1 {
		t.Fatalf("unexpected import cmd=%s main=%d side=%d", cmd.Name, main.Size(), side.Size())
	}
}

func TestImportCommanderDeckfile_UnknownCommanderStillImported(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `1 [FDN:204] Goblin Guide
1 [KTK:262] Mountain
SB: 1 [NEW:1] New Preview Commander
`)
	cmd, main, side, err := ImportCommanderDeckfileWithSideboard(path, db)
	if err != nil {
		t.Fatalf("expected unknown commander name to import, got %v", err)
	}
	if cmd.Name != "New Preview Commander" || main.Size() != 2 || side.Size() != 0 {
		t.Fatalf("unexpected import cmd=%s main=%d side=%d", cmd.Name, main.Size(), side.Size())
	}
}

func TestImportCommanderDeckfileWithCommanders_PartnersUseCombinedIdentity(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `1 Llanowar Elves
1 Sol Ring

1 Thrasios, Triton Hero
1 Tymna the Weaver
`)
	commanders, main, side, err := ImportCommanderDeckfileWithCommanders(path, db)
	if err != nil {
		t.Fatalf("expected partner import, got %v", err)
	}
	if len(commanders) != 2 || commanders[0].Name != "Thrasios, Triton Hero" || commanders[1].Name != "Tymna the Weaver" {
		t.Fatalf("unexpected commanders: %+v", commanders)
	}
	if main.Size() != 2 || side.Size() != 0 {
		t.Fatalf("unexpected main=%d side=%d", main.Size(), side.Size())
	}
}

func TestImportDeckfile_KeepsSBPrefixAsSideboard(t *testing.T) {
	db := newCommanderMockDB()
	path := writeTempDeck(t, `1 [FDN:204] Goblin Guide
SB: 1 [FDN:204] Krenko, Mob Boss
`)
	main, side, err := ImportDeckfile(path, db)
	if err != nil {
		t.Fatalf("unexpected import error: %v", err)
	}
	if main.Size() != 1 || side.Size() != 1 || side.Cards[0].Name != "Krenko, Mob Boss" {
		t.Fatalf("unexpected main=%d side=%d", main.Size(), side.Size())
	}
}
