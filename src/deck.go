package main

import (
	"bufio"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

type Deck struct {
	Cards []Card
	Name  string
}

func (d *Deck) Shuffle() {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(d.Cards), func(i, j int) {
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i]
	})
}

func (d *Deck) DrawCards(count int) []Card {
	if count > len(d.Cards) {
		count = len(d.Cards)
	}
	hand := d.Cards[:count]
	d.Cards = d.Cards[count:]
	return hand
}

func (d *Deck) DrawCard() Card {
	if len(d.Cards) == 0 {
		return Card{}
	}
	card := d.Cards[0]
	d.Cards = d.Cards[1:]
	return card
}

func importDeckfile(filename string) (Deck, Deck, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Deck{}, Deck{}, err
	}

	defer func() {
		if err := file.Close(); err != nil {
			LogDeck("Error closing file: %v", err)
		}
	}()

	var cards []Card
	var sideboardCards []Card
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

		// Handle both formats: "4x Elvish Mystic (CMM) 284" and "4 Elvish Mystic"
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
			// Format: "4 Elvish Mystic"
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue
			}
			count, err = strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				continue
			}
			name = strings.TrimSpace(parts[1])
		}

		// Lookup the card in the card database
		card, exists := cardDB.GetCardByName(name)
		if !exists {
			LogDeck("Card not found: %s", name)
			continue
		}

		// Add the card to the appropriate section
		for i := 0; i < count; i++ {
			if inSideboard {
				sideboardCards = append(sideboardCards, card)
			} else {
				cards = append(cards, card)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Deck{}, Deck{}, err
	}

	return Deck{Cards: cards, Name: deckName}, Deck{Cards: sideboardCards}, nil
}

func (d *Deck) Display() {
	for _, card := range d.Cards {
		card.Display()
	}
}
