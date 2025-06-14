// Package card provides card database functionality for MTG simulation.
package card

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mtgsim/mtgsim/internal/logger"
)

const (
	CardDBFile = "cardDB.json"
	CardDBURL  = "https://data.scryfall.io/oracle-cards/oracle-cards-20250204100217.json"
)

// CardDB represents a database of Magic: The Gathering cards.
type CardDB struct {
	cards map[string]Card
}

// NewCardDB creates a new card database from a slice of cards.
func NewCardDB(cards []Card) *CardDB {
	if len(cards) == 0 {
		return nil
	}

	cardMap := make(map[string]Card)
	for _, card := range cards {
		cardMap[card.Name] = card
	}
	return &CardDB{cards: cardMap}
}

// GetCardByName retrieves a card by its name from the database.
func (db *CardDB) GetCardByName(name string) (Card, bool) {
	card, exists := db.cards[name]
	return card, exists
}

// Size returns the number of cards in the database.
func (db *CardDB) Size() int {
	return len(db.cards)
}

// LoadCardDatabase loads the card database from file or downloads it if not present.
func LoadCardDatabase() (*CardDB, error) {
	var cards []Card
	
	// Try to load from file first
	if file, err := os.ReadFile(CardDBFile); err == nil {
		err = json.Unmarshal(file, &cards)
		if err != nil {
			logger.LogGame("Error parsing cardDB.json: %v", err)
			return nil, err
		}
		logger.LogMeta("Loaded %d cards from local database", len(cards))
	} else {
		// Download from URL if file doesn't exist
		logger.LogMeta("Local card database not found, downloading...")
		cards, err = downloadAndParseJSON(CardDBURL)
		if err != nil {
			logger.LogGame("Error downloading card database: %v", err)
			return nil, err
		}

		// Save to file for future use
		content, err := json.MarshalIndent(cards, "", "  ")
		if err != nil {
			logger.LogGame("Error marshalling JSON: %v", err)
			return nil, err
		}

		err = os.WriteFile(CardDBFile, content, 0644)
		if err != nil {
			logger.LogGame("Error writing to file: %v", err)
			return nil, err
		}
		logger.LogMeta("Downloaded and saved %d cards to local database", len(cards))
	}

	cardDB := NewCardDB(cards)
	if cardDB == nil {
		return nil, fmt.Errorf("failed to create card database")
	}

	return cardDB, nil
}

// downloadAndParseJSON downloads card data from the given URL and parses it.
func downloadAndParseJSON(url string) ([]Card, error) {
	logger.LogMeta("Downloading JSON from %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download JSON: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.LogMeta("Error closing response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var cards []Card
	err = json.Unmarshal(body, &cards)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return cards, nil
}
