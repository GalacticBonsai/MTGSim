// Package deck provides deck-related functionality for MTG simulation.
package deck

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
)

// Deck represents a Magic: The Gathering deck.
type Deck struct {
	Cards []card.Card
	Name  string
}

// Shuffle randomizes the order of cards in the deck.
func (d *Deck) Shuffle() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(d.Cards), func(i, j int) {
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i]
	})
}

// DrawCards draws the specified number of cards from the top of the deck.
func (d *Deck) DrawCards(count int) []card.Card {
	if count > len(d.Cards) {
		count = len(d.Cards)
	}
	hand := d.Cards[:count]
	d.Cards = d.Cards[count:]
	return hand
}

// DrawCard draws a single card from the top of the deck.
func (d *Deck) DrawCard() card.Card {
	if len(d.Cards) == 0 {
		return card.Card{}
	}
	drawnCard := d.Cards[0]
	d.Cards = d.Cards[1:]
	return drawnCard
}

// Display prints all cards in the deck.
func (d *Deck) Display() {
	for _, c := range d.Cards {
		c.Display()
	}
}

// Size returns the number of cards in the deck.
func (d *Deck) Size() int {
	return len(d.Cards)
}

// IsEmpty returns true if the deck has no cards.
func (d *Deck) IsEmpty() bool {
	return len(d.Cards) == 0
}

// ImportDeckfile imports a deck from a file, supporting multiple formats.
// Returns the main deck and sideboard as separate Deck objects.
func ImportDeckfile(filename string, cardDB CardDatabase) (Deck, Deck, error) {
	main, side, _, err := importDeckfileWithCommanders(filename, cardDB, false)
	return main, side, err
}

// ImportCommanderDeckfile imports a Commander deck. The file may declare a
// commander explicitly or use common EDH export conventions such as Cockatrice
// SB: command-zone lines or a final Moxfield command-zone group. The returned
// Deck is validated against the commander's color identity (CR 903.4). Returns
// the first commander and the main deck.
func ImportCommanderDeckfile(filename string, cardDB CardDatabase) (card.Card, Deck, error) {
	commanderCard, main, _, err := ImportCommanderDeckfileWithSideboard(filename, cardDB)
	return commanderCard, main, err
}

// ImportCommanderDeckfileWithSideboard imports a Commander deck and returns
// the commander, main deck, and sideboard. Both main and sideboard cards are
// validated against the commander's color identity because sideboard variants
// may swap those cards into the main deck.
func ImportCommanderDeckfileWithSideboard(filename string, cardDB CardDatabase) (card.Card, Deck, Deck, error) {
	commanders, main, side, err := ImportCommanderDeckfileWithCommanders(filename, cardDB)
	if err != nil {
		return card.Card{}, Deck{}, Deck{}, err
	}
	return commanders[0], main, side, nil
}

// ImportCommanderDeckfileWithCommanders imports a Commander deck and returns
// all command-zone cards. This covers partner/background-style exports while
// the older single-commander APIs continue returning the first commander.
func ImportCommanderDeckfileWithCommanders(filename string, cardDB CardDatabase) ([]card.Card, Deck, Deck, error) {
	main, side, commanders, err := importDeckfileWithCommanders(filename, cardDB, true)
	if err != nil {
		return nil, Deck{}, Deck{}, err
	}
	if len(commanders) == 0 {
		return nil, Deck{}, Deck{}, errMissingCommander
	}
	legalCards := append([]card.Card{}, main.Cards...)
	legalCards = append(legalCards, side.Cards...)
	if err := validateColorIdentityForCommanders(commanders, legalCards); err != nil {
		return commanders, main, side, err
	}
	return commanders, main, side, nil
}

type deckSection int

const (
	sectionMain deckSection = iota
	sectionSideboard
	sectionCommander
	sectionIgnored
)

type parsedDeckEntry struct {
	count    int
	card     card.Card
	known    bool
	section  deckSection
	group    int
	inlineSB bool
}

func importDeckfileWithCommanders(filename string, cardDB CardDatabase, inferCommander bool) (Deck, Deck, []card.Card, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Deck{}, Deck{}, nil, err
	}

	defer func() {
		if err := file.Close(); err != nil {
			logger.LogDeck("Error closing file: %v", err)
		}
	}()

	var entries []parsedDeckEntry
	var deckName = filename
	section := sectionMain
	group := 0

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			if line == "" {
				group++
				if section == sectionSideboard || section == sectionIgnored {
					section = sectionMain
				}
			}
			continue
		}

		// Handle "About" section for deck name
		if strings.HasPrefix(line, "About") {
			scanner.Scan()
			nameLine := strings.TrimSpace(scanner.Text())
			if name, ok := strings.CutPrefix(nameLine, "Name "); ok {
				deckName = name
			}
			continue
		}
		if name, ok := parseDeckName(line); ok {
			deckName = name
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "LAYOUT ") {
			continue
		}

		if nextSection, handled := parseSectionHeading(line); handled {
			section = nextSection
			continue
		}

		entrySection := section
		inlineSB := false
		if rest, ok := trimInlinePrefix(line, "SB:"); ok {
			line = rest
			entrySection = sectionSideboard
			inlineSB = true
		} else if rest, ok := trimInlinePrefix(line, "COMMANDER:"); ok {
			line = rest
			entrySection = sectionCommander
		}

		if entrySection == sectionIgnored {
			continue
		}

		count, name, ok := parseDeckCardLine(line)
		if !ok {
			continue
		}

		// Lookup the card in the card database
		cardData, exists := cardDB.GetCardByName(name)
		if !exists {
			logger.LogDeck("Card not found: %s", name)
			if !inferCommander {
				continue
			}
			cardData = card.Card{Name: name}
		}

		entries = append(entries, parsedDeckEntry{
			count: count, card: cardData, known: exists, section: entrySection, group: group, inlineSB: inlineSB,
		})
	}

	if err := scanner.Err(); err != nil {
		return Deck{}, Deck{}, nil, err
	}

	if inferCommander {
		inferCommanderEntries(entries)
	}

	cards, sideboardCards, commanders := materializeDeckEntries(entries)
	return Deck{Cards: cards, Name: deckName}, Deck{Cards: sideboardCards}, commanders, nil
}

func parseDeckName(line string) (string, bool) {
	if rest, ok := trimInlinePrefix(line, "NAME:"); ok {
		return rest, rest != ""
	}
	return "", false
}

func parseSectionHeading(line string) (deckSection, bool) {
	normalized := strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(line)), ":")
	switch normalized {
	case "COMMANDER", "COMMANDERS", "COMMAND ZONE":
		return sectionCommander, true
	case "SIDEBOARD", "SIDEBOARDS":
		return sectionSideboard, true
	case "DECK", "MAIN", "MAINBOARD":
		return sectionMain, true
	case "STICKERS", "ATTRACTIONS":
		return sectionIgnored, true
	default:
		return sectionMain, false
	}
}

func trimInlinePrefix(line, prefix string) (string, bool) {
	if !strings.HasPrefix(strings.ToUpper(line), prefix) {
		return "", false
	}
	return strings.TrimSpace(line[len(prefix):]), true
}

func parseDeckCardLine(line string) (int, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, "", false
	}
	count := 1
	name := line
	parts := strings.Fields(line)
	if len(parts) > 0 {
		countToken := strings.TrimSuffix(strings.ToLower(parts[0]), "x")
		if parsed, err := strconv.Atoi(countToken); err == nil {
			count = parsed
			name = strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
		}
	}
	name = normalizeDeckCardName(name)
	return count, name, count > 0 && name != ""
}

func normalizeDeckCardName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "[") {
		if end := strings.Index(name, "]"); end >= 0 && end+1 < len(name) {
			name = strings.TrimSpace(name[end+1:])
		}
	}
	if idx := strings.Index(name, " ("); idx != -1 {
		name = strings.TrimSpace(name[:idx])
	}
	return name
}

func inferCommanderEntries(entries []parsedDeckEntry) {
	if hasCommanders(entries) {
		return
	}
	if markInlineSBCommanders(entries) {
		return
	}
	markFinalGroupCommanders(entries)
}

func hasCommanders(entries []parsedDeckEntry) bool {
	for _, e := range entries {
		if e.section == sectionCommander {
			return true
		}
	}
	return false
}

func markInlineSBCommanders(entries []parsedDeckEntry) bool {
	marked := false
	for i := range entries {
		if entries[i].inlineSB && entries[i].count == 1 {
			entries[i].section = sectionCommander
			marked = true
		}
	}
	return marked
}

func markFinalGroupCommanders(entries []parsedDeckEntry) {
	if len(entries) == 0 {
		return
	}
	lastGroup := entries[len(entries)-1].group
	var idxs []int
	for i := range entries {
		if entries[i].group == lastGroup {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) == 0 || len(idxs) > 2 || lastGroup == entries[0].group {
		return
	}
	for _, idx := range idxs {
		if entries[idx].section != sectionMain || entries[idx].count != 1 {
			return
		}
	}
	for _, idx := range idxs {
		entries[idx].section = sectionCommander
	}
}

func materializeDeckEntries(entries []parsedDeckEntry) ([]card.Card, []card.Card, []card.Card) {
	var cards []card.Card
	var sideboardCards []card.Card
	var commanders []card.Card
	for _, e := range entries {
		for i := 0; i < e.count; i++ {
			switch e.section {
			case sectionCommander:
				commanders = append(commanders, e.card)
			case sectionSideboard:
				if !e.known {
					continue
				}
				sideboardCards = append(sideboardCards, e.card)
			case sectionMain:
				if !e.known {
					continue
				}
				cards = append(cards, e.card)
			}
		}
	}
	return cards, sideboardCards, commanders
}

// CardDatabase interface for card lookup functionality.
type CardDatabase interface {
	GetCardByName(name string) (card.Card, bool)
}

// errMissingCommander is returned when a Commander deckfile lacks a commander.
var errMissingCommander = errors.New("commander deck has no commander declared")

func validateColorIdentityForCommanders(commanders []card.Card, main []card.Card) error {
	if hasUnknownCommander(commanders) {
		return nil
	}
	allowed := map[string]bool{}
	var commanderNames []string
	var commanderIdentities []string
	for _, commander := range commanders {
		commanderNames = append(commanderNames, commander.Name)
		for _, c := range commander.ColorIdentity {
			upper := strings.ToUpper(c)
			allowed[upper] = true
			commanderIdentities = append(commanderIdentities, upper)
		}
	}
	for _, c := range main {
		for _, ci := range c.ColorIdentity {
			if !allowed[strings.ToUpper(ci)] {
				return fmt.Errorf("card %q color identity %v not within commander(s) %q identity %v",
					c.Name, c.ColorIdentity, strings.Join(commanderNames, " / "), commanderIdentities)
			}
		}
	}
	return nil
}

func hasUnknownCommander(commanders []card.Card) bool {
	for _, commander := range commanders {
		if commander.TypeLine == "" && commander.ManaCost == "" && len(commander.ColorIdentity) == 0 {
			return true
		}
	}
	return false
}
