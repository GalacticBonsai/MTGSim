package deck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/simulation"
)

// MockCardDB implements the CardDatabase interface for testing
type MockCardDB struct {
	cards map[string]card.Card
}

func (db *MockCardDB) GetCardByName(name string) (card.Card, bool) {
	c, exists := db.cards[name]
	return c, exists
}

func createMockCardDB() *MockCardDB {
	cards := map[string]card.Card{
		"Lightning Bolt": {
			Name:     "Lightning Bolt",
			CMC:      1,
			ManaCost: "{R}",
			TypeLine: "Instant",
		},
		"Mountain": {
			Name:     "Mountain",
			CMC:      0,
			TypeLine: "Basic Land — Mountain",
		},
		"Forest": {
			Name:     "Forest",
			CMC:      0,
			TypeLine: "Basic Land — Forest",
		},
		"Llanowar Elves": {
			Name:      "Llanowar Elves",
			CMC:       1,
			ManaCost:  "{G}",
			TypeLine:  "Creature — Elf Druid",
			Power:     "1",
			Toughness: "1",
		},
	}
	return &MockCardDB{cards: cards}
}

func createComprehensiveMockCardDB() *MockCardDB {
	cards := map[string]card.Card{
		// Basic Lands
		"Plains":   {Name: "Plains", CMC: 0, TypeLine: "Basic Land — Plains"},
		"Island":   {Name: "Island", CMC: 0, TypeLine: "Basic Land — Island"},
		"Swamp":    {Name: "Swamp", CMC: 0, TypeLine: "Basic Land — Swamp"},
		"Mountain": {Name: "Mountain", CMC: 0, TypeLine: "Basic Land — Mountain"},
		"Forest":   {Name: "Forest", CMC: 0, TypeLine: "Basic Land — Forest"},

		// Common Spells
		"Lightning Bolt":     {Name: "Lightning Bolt", CMC: 1, ManaCost: "{R}", TypeLine: "Instant"},
		"Fireball":           {Name: "Fireball", CMC: 1, ManaCost: "{X}{R}", TypeLine: "Sorcery"},
		"Counterspell":       {Name: "Counterspell", CMC: 2, ManaCost: "{U}{U}", TypeLine: "Instant"},
		"Dark Ritual":        {Name: "Dark Ritual", CMC: 1, ManaCost: "{B}", TypeLine: "Instant"},
		"Giant Growth":       {Name: "Giant Growth", CMC: 1, ManaCost: "{G}", TypeLine: "Instant"},
		"Swords to Plowshares": {Name: "Swords to Plowshares", CMC: 1, ManaCost: "{W}", TypeLine: "Instant"},

		// Common Creatures
		"Llanowar Elves":     {Name: "Llanowar Elves", CMC: 1, ManaCost: "{G}", TypeLine: "Creature — Elf Druid", Power: "1", Toughness: "1"},
		"Birds of Paradise":  {Name: "Birds of Paradise", CMC: 1, ManaCost: "{G}", TypeLine: "Creature — Bird", Power: "0", Toughness: "1", Keywords: []string{"Flying"}},
		"Serra Angel":        {Name: "Serra Angel", CMC: 5, ManaCost: "{3}{W}{W}", TypeLine: "Creature — Angel", Power: "4", Toughness: "4", Keywords: []string{"Flying", "Vigilance"}},
		"Shivan Dragon":      {Name: "Shivan Dragon", CMC: 6, ManaCost: "{4}{R}{R}", TypeLine: "Creature — Dragon", Power: "5", Toughness: "5", Keywords: []string{"Flying"}},
		"Lord of Atlantis":   {Name: "Lord of Atlantis", CMC: 2, ManaCost: "{U}{U}", TypeLine: "Creature — Merfolk", Power: "2", Toughness: "2"},

		// Artifacts
		"Sol Ring":           {Name: "Sol Ring", CMC: 1, ManaCost: "{1}", TypeLine: "Artifact"},
		"Mox Pearl":          {Name: "Mox Pearl", CMC: 0, ManaCost: "{0}", TypeLine: "Artifact"},
		"Black Lotus":        {Name: "Black Lotus", CMC: 0, ManaCost: "{0}", TypeLine: "Artifact"},
		"Nevinyrral's Disk":  {Name: "Nevinyrral's Disk", CMC: 4, ManaCost: "{4}", TypeLine: "Artifact"},

		// Dual Lands
		"Tundra":             {Name: "Tundra", CMC: 0, TypeLine: "Land — Plains Island"},
		"Underground Sea":    {Name: "Underground Sea", CMC: 0, TypeLine: "Land — Island Swamp"},
		"Badlands":           {Name: "Badlands", CMC: 0, TypeLine: "Land — Swamp Mountain"},
		"Taiga":              {Name: "Taiga", CMC: 0, TypeLine: "Land — Mountain Forest"},
		"Savannah":           {Name: "Savannah", CMC: 0, TypeLine: "Land — Forest Plains"},
		"Scrubland":          {Name: "Scrubland", CMC: 0, TypeLine: "Land — Plains Swamp"},
		"Volcanic Island":    {Name: "Volcanic Island", CMC: 0, TypeLine: "Land — Island Mountain"},
		"Bayou":              {Name: "Bayou", CMC: 0, TypeLine: "Land — Swamp Forest"},
		"Plateau":            {Name: "Plateau", CMC: 0, TypeLine: "Land — Mountain Plains"},
		"Tropical Island":    {Name: "Tropical Island", CMC: 0, TypeLine: "Land — Forest Island"},

		// Shock Lands
		"Sacred Foundry":     {Name: "Sacred Foundry", CMC: 0, TypeLine: "Land — Mountain Plains"},
		"Steam Vents":        {Name: "Steam Vents", CMC: 0, TypeLine: "Land — Island Mountain"},
		"Overgrown Tomb":     {Name: "Overgrown Tomb", CMC: 0, TypeLine: "Land — Swamp Forest"},
		"Temple Garden":      {Name: "Temple Garden", CMC: 0, TypeLine: "Land — Forest Plains"},
		"Hallowed Fountain":  {Name: "Hallowed Fountain", CMC: 0, TypeLine: "Land — Plains Island"},
		"Watery Grave":       {Name: "Watery Grave", CMC: 0, TypeLine: "Land — Island Swamp"},
		"Blood Crypt":        {Name: "Blood Crypt", CMC: 0, TypeLine: "Land — Swamp Mountain"},
		"Stomping Ground":    {Name: "Stomping Ground", CMC: 0, TypeLine: "Land — Mountain Forest"},
		"Breeding Pool":      {Name: "Breeding Pool", CMC: 0, TypeLine: "Land — Forest Island"},
		"Godless Shrine":     {Name: "Godless Shrine", CMC: 0, TypeLine: "Land — Plains Swamp"},

		// Check Lands
		"Clifftop Retreat":   {Name: "Clifftop Retreat", CMC: 0, TypeLine: "Land"},
		"Dragonskull Summit": {Name: "Dragonskull Summit", CMC: 0, TypeLine: "Land"},
		"Isolated Chapel":    {Name: "Isolated Chapel", CMC: 0, TypeLine: "Land"},
		"Sulfur Falls":       {Name: "Sulfur Falls", CMC: 0, TypeLine: "Land"},
		"Woodland Cemetery":  {Name: "Woodland Cemetery", CMC: 0, TypeLine: "Land"},

		// Popular Modern/Standard Cards
		"Tarmogoyf":          {Name: "Tarmogoyf", CMC: 2, ManaCost: "{1}{G}", TypeLine: "Creature — Lhurgoyf", Power: "*", Toughness: "*+1"},
		"Dark Confidant":     {Name: "Dark Confidant", CMC: 2, ManaCost: "{1}{B}", TypeLine: "Creature — Human Wizard", Power: "2", Toughness: "1"},
		"Snapcaster Mage":    {Name: "Snapcaster Mage", CMC: 2, ManaCost: "{1}{U}", TypeLine: "Creature — Human Wizard", Power: "2", Toughness: "1"},
		"Stoneforge Mystic":  {Name: "Stoneforge Mystic", CMC: 2, ManaCost: "{1}{W}", TypeLine: "Creature — Kor Artificer", Power: "1", Toughness: "2"},

		// Planeswalkers
		"Jace, the Mind Sculptor": {Name: "Jace, the Mind Sculptor", CMC: 4, ManaCost: "{2}{U}{U}", TypeLine: "Legendary Planeswalker — Jace"},
		"Liliana of the Veil":     {Name: "Liliana of the Veil", CMC: 3, ManaCost: "{1}{B}{B}", TypeLine: "Legendary Planeswalker — Liliana"},
		"Chandra, Torch of Defiance": {Name: "Chandra, Torch of Defiance", CMC: 4, ManaCost: "{2}{R}{R}", TypeLine: "Legendary Planeswalker — Chandra"},

		// Common deck archetype cards
		"Wrath of God":       {Name: "Wrath of God", CMC: 4, ManaCost: "{2}{W}{W}", TypeLine: "Sorcery"},
		"Damnation":          {Name: "Damnation", CMC: 4, ManaCost: "{2}{B}{B}", TypeLine: "Sorcery"},
		"Pyroclasm":          {Name: "Pyroclasm", CMC: 2, ManaCost: "{1}{R}", TypeLine: "Sorcery"},
		"Brainstorm":         {Name: "Brainstorm", CMC: 1, ManaCost: "{U}", TypeLine: "Instant"},
		"Ponder":             {Name: "Ponder", CMC: 1, ManaCost: "{U}", TypeLine: "Sorcery"},
		"Preordain":          {Name: "Preordain", CMC: 1, ManaCost: "{U}", TypeLine: "Sorcery"},

		// Equipment
		"Sword of Fire and Ice": {Name: "Sword of Fire and Ice", CMC: 3, ManaCost: "{3}", TypeLine: "Artifact — Equipment"},
		"Umezawa's Jitte":       {Name: "Umezawa's Jitte", CMC: 2, ManaCost: "{2}", TypeLine: "Legendary Artifact — Equipment"},

		// Enchantments
		"Necropotence":       {Name: "Necropotence", CMC: 3, ManaCost: "{B}{B}{B}", TypeLine: "Enchantment"},
		"Sylvan Library":     {Name: "Sylvan Library", CMC: 2, ManaCost: "{1}{G}", TypeLine: "Enchantment"},
		"Rhystic Study":      {Name: "Rhystic Study", CMC: 3, ManaCost: "{2}{U}", TypeLine: "Enchantment"},
	}

	return &MockCardDB{cards: cards}
}

func TestDeckOperations(t *testing.T) {
	// Create a test deck
	testCards := []card.Card{
		{Name: "Lightning Bolt"},
		{Name: "Mountain"},
		{Name: "Forest"},
		{Name: "Llanowar Elves"},
	}
	
	deck := Deck{
		Cards: testCards,
		Name:  "Test Deck",
	}

	// Test initial size
	if deck.Size() != 4 {
		t.Errorf("Expected deck size 4, got %d", deck.Size())
	}

	// Test drawing a card
	drawnCard := deck.DrawCard()
	if drawnCard.Name != "Lightning Bolt" {
		t.Errorf("Expected to draw Lightning Bolt, got %s", drawnCard.Name)
	}
	if deck.Size() != 3 {
		t.Errorf("Expected deck size 3 after drawing, got %d", deck.Size())
	}

	// Test drawing multiple cards
	drawnCards := deck.DrawCards(2)
	if len(drawnCards) != 2 {
		t.Errorf("Expected to draw 2 cards, got %d", len(drawnCards))
	}
	if deck.Size() != 1 {
		t.Errorf("Expected deck size 1 after drawing 2 more, got %d", deck.Size())
	}

	// Test drawing more cards than available
	drawnCards = deck.DrawCards(5)
	if len(drawnCards) != 1 {
		t.Errorf("Expected to draw only 1 card (remaining), got %d", len(drawnCards))
	}
	if !deck.IsEmpty() {
		t.Errorf("Expected deck to be empty")
	}

	// Test drawing from empty deck
	emptyCard := deck.DrawCard()
	if emptyCard.Name != "" {
		t.Errorf("Expected empty card from empty deck, got %s", emptyCard.Name)
	}
}

func TestDeckShuffle(t *testing.T) {
	// Create a deck with many cards to test shuffling
	var testCards []card.Card
	for i := 0; i < 20; i++ {
		testCards = append(testCards, card.Card{Name: "Card " + string(rune('A'+i))})
	}
	
	deck := Deck{Cards: testCards}
	originalOrder := make([]string, len(deck.Cards))
	for i, c := range deck.Cards {
		originalOrder[i] = c.Name
	}

	// Shuffle the deck
	deck.Shuffle()

	// Check if order changed (this might occasionally fail due to randomness, but very unlikely)
	changed := false
	for i, c := range deck.Cards {
		if c.Name != originalOrder[i] {
			changed = true
			break
		}
	}

	if !changed {
		t.Errorf("Deck order should have changed after shuffling")
	}

	// Verify all cards are still present
	if len(deck.Cards) != 20 {
		t.Errorf("Expected 20 cards after shuffle, got %d", len(deck.Cards))
	}
}

func TestImportDeckfile(t *testing.T) {
	// Create a temporary deck file
	tempDir := t.TempDir()
	deckFile := filepath.Join(tempDir, "test.deck")
	
	deckContent := `About
Name Test Deck

Deck
4 Lightning Bolt
20 Mountain
4 Llanowar Elves
12 Forest

Sideboard
2 Lightning Bolt
`

	err := os.WriteFile(deckFile, []byte(deckContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test deck file: %v", err)
	}

	// Import the deck
	mockDB := createMockCardDB()
	mainDeck, sideboard, err := ImportDeckfile(deckFile, mockDB)
	if err != nil {
		t.Fatalf("Failed to import deck: %v", err)
	}

	// Test main deck
	if mainDeck.Name != "Test Deck" {
		t.Errorf("Expected deck name 'Test Deck', got '%s'", mainDeck.Name)
	}

	expectedMainSize := 4 + 20 + 4 + 12 // Lightning Bolt + Mountain + Llanowar Elves + Forest
	if mainDeck.Size() != expectedMainSize {
		t.Errorf("Expected main deck size %d, got %d", expectedMainSize, mainDeck.Size())
	}

	// Test sideboard
	if sideboard.Size() != 2 {
		t.Errorf("Expected sideboard size 2, got %d", sideboard.Size())
	}

	// Count specific cards in main deck
	lightningBoltCount := 0
	mountainCount := 0
	for _, c := range mainDeck.Cards {
		switch c.Name {
		case "Lightning Bolt":
			lightningBoltCount++
		case "Mountain":
			mountainCount++
		}
	}

	if lightningBoltCount != 4 {
		t.Errorf("Expected 4 Lightning Bolts in main deck, got %d", lightningBoltCount)
	}
	if mountainCount != 20 {
		t.Errorf("Expected 20 Mountains in main deck, got %d", mountainCount)
	}
}

func TestImportSimpleDeckFormat(t *testing.T) {
	// Create a temporary deck file with simple format
	tempDir := t.TempDir()
	deckFile := filepath.Join(tempDir, "simple.deck")
	
	deckContent := `4 Lightning Bolt
20 Mountain
4 Llanowar Elves
12 Forest
`

	err := os.WriteFile(deckFile, []byte(deckContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test deck file: %v", err)
	}

	// Import the deck
	mockDB := createMockCardDB()
	mainDeck, sideboard, err := ImportDeckfile(deckFile, mockDB)
	if err != nil {
		t.Fatalf("Failed to import deck: %v", err)
	}

	// Test main deck
	expectedMainSize := 4 + 20 + 4 + 12
	if mainDeck.Size() != expectedMainSize {
		t.Errorf("Expected main deck size %d, got %d", expectedMainSize, mainDeck.Size())
	}

	// Test sideboard (should be empty)
	if sideboard.Size() != 0 {
		t.Errorf("Expected empty sideboard, got %d cards", sideboard.Size())
	}
}

// TestParseAllDecksInRepository tests parsing every deck file in the decks directory
func TestParseAllDecksInRepository(t *testing.T) {
	// Get the project root directory (go up from pkg/deck to project root)
	projectRoot := filepath.Join("..", "..", "decks")

	// Check if decks directory exists
	if _, err := os.Stat(projectRoot); os.IsNotExist(err) {
		t.Skip("Decks directory not found, skipping repository deck parsing test")
		return
	}

	// Create a comprehensive mock card database with common MTG cards
	mockDB := createComprehensiveMockCardDB()

	// Get all deck files recursively
	deckFiles, err := simulation.GetDecks(projectRoot)
	if err != nil {
		t.Fatalf("Failed to get deck files: %v", err)
	}

	if len(deckFiles) == 0 {
		t.Skip("No deck files found in repository")
		return
	}

	t.Logf("Found %d deck files to test", len(deckFiles))

	// Track parsing results
	var successCount, failCount int
	var failedDecks []string

	// Test each deck file
	for _, deckFile := range deckFiles {
		// Skip non-deck files
		if !strings.HasSuffix(deckFile, ".deck") && !strings.HasSuffix(deckFile, ".txt") {
			continue
		}

		t.Run(filepath.Base(deckFile), func(t *testing.T) {
			mainDeck, sideboard, err := ImportDeckfile(deckFile, mockDB)
			if err != nil {
				t.Errorf("Failed to parse deck %s: %v", deckFile, err)
				failCount++
				failedDecks = append(failedDecks, deckFile)
				return
			}

			// Basic validation
			if mainDeck.Size() == 0 {
				t.Errorf("Deck %s has no cards in main deck", deckFile)
				return
			}

			// Log deck information
			t.Logf("Deck: %s, Main: %d cards, Sideboard: %d cards",
				mainDeck.Name, mainDeck.Size(), sideboard.Size())

			// Validate deck name is set
			if mainDeck.Name == "" {
				t.Errorf("Deck %s has empty name", deckFile)
			}

			// Check for reasonable deck size (between 40 and 100 cards for main deck)
			if mainDeck.Size() < 40 || mainDeck.Size() > 100 {
				t.Logf("Warning: Deck %s has unusual size: %d cards", deckFile, mainDeck.Size())
			}

			// Check sideboard size (should be 0-15 cards)
			if sideboard.Size() > 15 {
				t.Logf("Warning: Deck %s has large sideboard: %d cards", deckFile, sideboard.Size())
			}

			successCount++
		})
	}

	// Summary
	t.Logf("Deck parsing summary: %d successful, %d failed", successCount, failCount)
	if failCount > 0 {
		t.Logf("Failed decks: %v", failedDecks)
	}

	// Fail the test if more than 10% of decks failed to parse
	if failCount > 0 && float64(failCount)/float64(successCount+failCount) > 0.1 {
		t.Errorf("Too many deck parsing failures: %d/%d (%.1f%%)",
			failCount, successCount+failCount,
			float64(failCount)/float64(successCount+failCount)*100)
	}
}
