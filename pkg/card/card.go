// Package card provides card-related types and functionality for MTG simulation.
package card

import (
	"strconv"
	"strings"

	"github.com/mtgsim/mtgsim/internal/logger"
)

// Card represents a Magic: The Gathering card with all its properties.
type Card struct {
	Name            string            `json:"name,omitempty"`
	CMC             float32           `json:"cmc,omitempty"`
	ManaCost        string            `json:"mana_cost,omitempty"`
	TypeLine        string            `json:"type_line,omitempty"`
	Power           string            `json:"power,omitempty"`
	Toughness       string            `json:"toughness,omitempty"`
	Keywords        []string          `json:"keywords,omitempty"`
	OracleText      string            `json:"oracle_text,omitempty"`
	ID              string            `json:"id,omitempty"`
	OracleID        string            `json:"oracle_id,omitempty"`
	MultiverseIDs   []int             `json:"multiverse_i_ds,omitempty"`
	Lang            string            `json:"lang,omitempty"`
	ReleasedAt      string            `json:"released_at,omitempty"`
	URI             string            `json:"uri,omitempty"`
	ScryfallURI     string            `json:"scryfall_uri,omitempty"`
	Layout          string            `json:"layout,omitempty"`
	ColorIdentity   []string          `json:"color_identity,omitempty"`
	Colors          []string          `json:"colors,omitempty"`
	Legalities      map[string]string `json:"legalities,omitempty"`
	Variation       bool              `json:"variation,omitempty"`
	Set             string            `json:"set,omitempty"`
	SetName         string            `json:"set_name,omitempty"`
	SetType         string            `json:"set_type,omitempty"`
	CollectorNumber string            `json:"collector_number,omitempty"`
	Rarity          string            `json:"rarity,omitempty"`
	Artist          string            `json:"artist,omitempty"`
}

// Display prints the details of a Card instance in a single line.
func (c *Card) Display() {
	if strings.Contains(c.TypeLine, "Land") {
		logger.LogCard("%s", c.Name)
	} else if strings.Contains(c.TypeLine, "Creature") {
		logger.LogCard("Name: %s, Mana Value: %.2f, Power: %s, Toughness: %s", c.Name, c.CMC, c.Power, c.Toughness)
	} else {
		logger.LogCard("Name: %s, Mana Value: %.2f, Type: %s", c.Name, c.CMC, c.TypeLine)
	}
}

// DisplayCards prints the details of multiple cards.
func DisplayCards(cards []Card) {
	for _, card := range cards {
		card.Display()
	}
}

// Cast handles casting a card, creating permanents or resolving spells.
func (c *Card) Cast(target interface{}, player interface{}) {
	if strings.Contains(c.TypeLine, "Creature") {
		power, _ := strconv.Atoi(c.Power)
		toughness, _ := strconv.Atoi(c.Toughness)

		// This would need to be implemented with proper player interface
		logger.LogCard("Casting creature: %s (%d/%d)", c.Name, power, toughness)
		return
	}

	// For non-creature spells, they typically go to graveyard after resolving
	logger.LogCard("Casting spell: %s", c.Name)
}

// IsLand returns true if the card is a land.
func (c *Card) IsLand() bool {
	return strings.Contains(c.TypeLine, "Land")
}

// IsCreature returns true if the card is a creature.
func (c *Card) IsCreature() bool {
	return strings.Contains(c.TypeLine, "Creature")
}

// IsInstant returns true if the card is an instant.
func (c *Card) IsInstant() bool {
	return strings.Contains(c.TypeLine, "Instant")
}

// IsSorcery returns true if the card is a sorcery.
func (c *Card) IsSorcery() bool {
	return strings.Contains(c.TypeLine, "Sorcery")
}

// IsArtifact returns true if the card is an artifact.
func (c *Card) IsArtifact() bool {
	return strings.Contains(c.TypeLine, "Artifact")
}

// IsEnchantment returns true if the card is an enchantment.
func (c *Card) IsEnchantment() bool {
	return strings.Contains(c.TypeLine, "Enchantment")
}

// IsPlaneswalker returns true if the card is a planeswalker.
func (c *Card) IsPlaneswalker() bool {
	return strings.Contains(c.TypeLine, "Planeswalker")
}

// HasKeyword returns true if the card has the specified keyword.
func (c *Card) HasKeyword(keyword string) bool {
	for _, k := range c.Keywords {
		if strings.EqualFold(k, keyword) {
			return true
		}
	}
	return false
}
