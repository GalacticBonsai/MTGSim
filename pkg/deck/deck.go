// Package deck provides deck-related functionality for MTG simulation.
package deck

import (
	"bufio"
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
	file, err := os.Open(filename)
	if err != nil {
		return Deck{}, Deck{}, err
	}

	defer func() {
		if err := file.Close(); err != nil {
			logger.LogDeck("Error closing file: %v", err)
		}
	}()

	var cards []card.Card
	var sideboardCards []card.Card
	var deckName = filename
	inSideboard := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Handle "About" section for deck name
		if strings.HasPrefix(line, "About") {
			scanner.Scan()
			nameLine := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(nameLine, "Name ") {
				deckName = strings.TrimPrefix(nameLine, "Name ")
			}
			continue
		}

		// Detect the start of the sideboard section
		if strings.EqualFold(line, "Sideboard") {
			inSideboard = true
			continue
		}

		// Handle multiple formats: "4x Elvish Mystic (CMM) 284", "4 Elvish Mystic", or just "Elvish Mystic"
		var count int
		var name string

		if strings.Contains(line, "x ") {
			// Format: "4x Elvish Mystic (CMM) 284"
			parts := strings.SplitN(line, "x ", 2)
			if len(parts) != 2 {
				continue
			}
			count, err = strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				continue
			}
			name = strings.TrimSpace(parts[1])
			// Remove set and collector number if present
			if idx := strings.Index(name, " ("); idx != -1 {
				name = name[:idx]
			}
		} else {
			// Try format: "4 Elvish Mystic"
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				count, err = strconv.Atoi(strings.TrimSpace(parts[0]))
				if err == nil {
					// Successfully parsed count, use the rest as name
					name = strings.TrimSpace(parts[1])
					// Remove set and collector number if present
					if idx := strings.Index(name, " ("); idx != -1 {
						name = name[:idx]
					}
				} else {
					// Failed to parse count, treat entire line as card name with count 1
					count = 1
					name = strings.TrimSpace(line)
				}
			} else {
				// Single word or no spaces, treat as card name with count 1
				count = 1
				name = strings.TrimSpace(line)
			}
		}

		// Lookup the card in the card database
		cardData, exists := cardDB.GetCardByName(name)
		if !exists {
			logger.LogDeck("Card not found: %s", name)
			continue
		}

		// Add the card to the appropriate section
		for i := 0; i < count; i++ {
			if inSideboard {
				sideboardCards = append(sideboardCards, cardData)
			} else {
				cards = append(cards, cardData)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Deck{}, Deck{}, err
	}

	return Deck{Cards: cards, Name: deckName}, Deck{Cards: sideboardCards}, nil
}

// CardDatabase interface for card lookup functionality.
type CardDatabase interface {
	GetCardByName(name string) (card.Card, bool)
}
