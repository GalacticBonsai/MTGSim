// Package deck provides welcome deck management functionality.
package deck

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
)

// WelcomeDeckManager manages welcome deck loading and randomization.
type WelcomeDeckManager struct {
	welcomeDecks []WelcomeDeckInfo
	cardDB       *card.CardDB
	rng          *rand.Rand
}

// WelcomeDeckInfo contains information about a welcome deck.
type WelcomeDeckInfo struct {
	Name      string
	Path      string
	MainDeck  Deck
	Sideboard Deck
}

// SideboardIntegrationMode defines how sideboard cards are integrated.
type SideboardIntegrationMode int

const (
	// SideboardIgnore ignores sideboard cards
	SideboardIgnore SideboardIntegrationMode = iota
	// SideboardAdd adds sideboard cards to main deck
	SideboardAdd
	// SideboardReplace replaces random main deck cards with sideboard cards
	SideboardReplace
)

// NewWelcomeDeckManager creates a new welcome deck manager.
func NewWelcomeDeckManager(cardDB *card.CardDB) *WelcomeDeckManager {
	return &WelcomeDeckManager{
		welcomeDecks: make([]WelcomeDeckInfo, 0),
		cardDB:       cardDB,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// LoadWelcomeDecks loads all welcome decks from the specified directory.
func (wdm *WelcomeDeckManager) LoadWelcomeDecks(welcomeDir string) error {
	logger.LogMeta("Loading welcome decks from: %s", welcomeDir)

	// Check if directory exists
	if _, err := os.Stat(welcomeDir); os.IsNotExist(err) {
		return fmt.Errorf("welcome deck directory does not exist: %s", welcomeDir)
	}

	// Read directory contents
	files, err := os.ReadDir(welcomeDir)
	if err != nil {
		return fmt.Errorf("failed to read welcome deck directory: %v", err)
	}

	// Load each .deck file
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".deck" {
			continue
		}

		deckPath := filepath.Join(welcomeDir, file.Name())
		mainDeck, sideboard, err := ImportDeckfile(deckPath, wdm.cardDB)
		if err != nil {
			logger.LogDeck("Failed to load welcome deck %s: %v", file.Name(), err)
			continue
		}

		deckInfo := WelcomeDeckInfo{
			Name:      mainDeck.Name,
			Path:      deckPath,
			MainDeck:  mainDeck,
			Sideboard: sideboard,
		}

		wdm.welcomeDecks = append(wdm.welcomeDecks, deckInfo)
		logger.LogDeck("Loaded welcome deck: %s (%d main, %d sideboard)", 
			deckInfo.Name, len(deckInfo.MainDeck.Cards), len(deckInfo.Sideboard.Cards))
	}

	if len(wdm.welcomeDecks) == 0 {
		return fmt.Errorf("no welcome decks found in directory: %s", welcomeDir)
	}

	logger.LogMeta("Successfully loaded %d welcome decks", len(wdm.welcomeDecks))
	return nil
}

// GetRandomDeck returns a random welcome deck with optional sideboard integration.
func (wdm *WelcomeDeckManager) GetRandomDeck(mode SideboardIntegrationMode) (Deck, error) {
	if len(wdm.welcomeDecks) == 0 {
		return Deck{}, fmt.Errorf("no welcome decks loaded")
	}

	// Select random deck
	deckInfo := wdm.welcomeDecks[wdm.rng.Intn(len(wdm.welcomeDecks))]
	
	// Create a copy of the main deck
	resultDeck := Deck{
		Name:  deckInfo.Name,
		Cards: make([]card.Card, len(deckInfo.MainDeck.Cards)),
	}
	copy(resultDeck.Cards, deckInfo.MainDeck.Cards)

	// Apply sideboard integration
	switch mode {
	case SideboardIgnore:
		// Do nothing
	case SideboardAdd:
		// Add all sideboard cards to the deck
		resultDeck.Cards = append(resultDeck.Cards, deckInfo.Sideboard.Cards...)
		logger.LogDeck("Added %d sideboard cards to %s", len(deckInfo.Sideboard.Cards), deckInfo.Name)
	case SideboardReplace:
		// Replace random main deck cards with sideboard cards
		if len(deckInfo.Sideboard.Cards) > 0 {
			numToReplace := wdm.rng.Intn(len(deckInfo.Sideboard.Cards)) + 1
			if numToReplace > len(resultDeck.Cards) {
				numToReplace = len(resultDeck.Cards)
			}

			// Replace random cards
			for i := 0; i < numToReplace && i < len(deckInfo.Sideboard.Cards); i++ {
				replaceIndex := wdm.rng.Intn(len(resultDeck.Cards))
				resultDeck.Cards[replaceIndex] = deckInfo.Sideboard.Cards[i]
			}
			logger.LogDeck("Replaced %d cards with sideboard cards in %s", numToReplace, deckInfo.Name)
		}
	}

	// Shuffle the deck
	resultDeck.Shuffle()

	return resultDeck, nil
}

// GetRandomDeckPair returns two different random welcome decks.
func (wdm *WelcomeDeckManager) GetRandomDeckPair(mode SideboardIntegrationMode) (Deck, Deck, error) {
	if len(wdm.welcomeDecks) < 2 {
		return Deck{}, Deck{}, fmt.Errorf("need at least 2 welcome decks for pair selection")
	}

	// Get first deck
	deck1, err := wdm.GetRandomDeck(mode)
	if err != nil {
		return Deck{}, Deck{}, err
	}

	// Get second deck (ensure it's different)
	var deck2 Deck
	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		deck2, err = wdm.GetRandomDeck(mode)
		if err != nil {
			return Deck{}, Deck{}, err
		}
		
		// Check if decks are different
		if deck1.Name != deck2.Name {
			break
		}
	}

	return deck1, deck2, nil
}

// GetAllDeckNames returns the names of all loaded welcome decks.
func (wdm *WelcomeDeckManager) GetAllDeckNames() []string {
	names := make([]string, len(wdm.welcomeDecks))
	for i, deck := range wdm.welcomeDecks {
		names[i] = deck.Name
	}
	return names
}

// GetDeckCount returns the number of loaded welcome decks.
func (wdm *WelcomeDeckManager) GetDeckCount() int {
	return len(wdm.welcomeDecks)
}

// GetDeckByName returns a specific deck by name.
func (wdm *WelcomeDeckManager) GetDeckByName(name string, mode SideboardIntegrationMode) (Deck, error) {
	for _, deckInfo := range wdm.welcomeDecks {
		if deckInfo.Name == name {
			// Create a copy of the main deck
			resultDeck := Deck{
				Name:  deckInfo.Name,
				Cards: make([]card.Card, len(deckInfo.MainDeck.Cards)),
			}
			copy(resultDeck.Cards, deckInfo.MainDeck.Cards)

			// Apply sideboard integration (same logic as GetRandomDeck)
			switch mode {
			case SideboardAdd:
				resultDeck.Cards = append(resultDeck.Cards, deckInfo.Sideboard.Cards...)
			case SideboardReplace:
				if len(deckInfo.Sideboard.Cards) > 0 {
					numToReplace := wdm.rng.Intn(len(deckInfo.Sideboard.Cards)) + 1
					if numToReplace > len(resultDeck.Cards) {
						numToReplace = len(resultDeck.Cards)
					}
					for i := 0; i < numToReplace && i < len(deckInfo.Sideboard.Cards); i++ {
						replaceIndex := wdm.rng.Intn(len(resultDeck.Cards))
						resultDeck.Cards[replaceIndex] = deckInfo.Sideboard.Cards[i]
					}
				}
			}

			resultDeck.Shuffle()
			return resultDeck, nil
		}
	}
	
	return Deck{}, fmt.Errorf("deck not found: %s", name)
}

// GetDeckInfo returns detailed information about all loaded decks.
func (wdm *WelcomeDeckManager) GetDeckInfo() []WelcomeDeckInfo {
	// Return a copy to prevent external modification
	info := make([]WelcomeDeckInfo, len(wdm.welcomeDecks))
	copy(info, wdm.welcomeDecks)
	return info
}
