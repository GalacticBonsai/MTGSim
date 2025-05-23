package main

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
)

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

// DisplayCardSingleLine prints the details of a Card instance in a single line
func (c *Card) Display() {
	if strings.Contains(c.TypeLine, "Land") {
		LogCard("%s", c.Name)
	} else if strings.Contains(c.TypeLine, "Creature") {
		LogCard("Name: %s, Mana Value: %.2f, Power: %s, Toughness: %s", c.Name, c.CMC, c.Power, c.Toughness)
	} else {
		LogCard("Name: %s, Mana Value: %.2f, Type: %s", c.Name, c.CMC, c.TypeLine)
	}
}

// DisplayCard prints the details of a Card instance
func DisplayCard(card Card) {
	card.Display()
}

func DisplayCards(cards []Card) {
	for _, card := range cards {
		card.Display()
	}
}

func (c *Card) Cast(target *Permanant, p *Player) {
	if strings.Contains(c.TypeLine, "Creature") {
		power, _ := strconv.Atoi(c.Power)
		toughness, _ := strconv.Atoi(c.Toughness)
		p.Creatures = append(p.Creatures, &Permanant{
			id:                uuid.New(),
			source:            *c,
			owner:             p,
			tokenType:         Creature,
			summoningSickness: true,
			power:             power,
			toughness:         toughness,
		})
		return
	}

	// Hardcoding bolt to face
	// p.Opponents[0].LifeTotal -= 3
	p.Graveyard = append(p.Graveyard, *c)

}
