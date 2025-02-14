package main

import (
	"fmt"
	"strings"
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
	fmt.Printf("Name: %s, Mana Value: %.0f, Type: %s\n", c.Name, c.CMC, c.TypeLine)
}

// DisplayCard prints the details of a Card instance
func DisplayCard(card Card) {
	fmt.Printf("Name: %s\n", card.Name)
	fmt.Printf("Mana Value: %.0f\n", card.CMC)
	fmt.Printf("Type: %s\n", card.TypeLine)
	// fmt.Printf("Set: %s\n", card.SetName)
	// fmt.Printf("Scryfall URI: %s\n", card.ScryfallURI)
}

func DisplayCards(cards []Card) {
	for _, card := range cards {
		card.Display()
	}
}

func (c *Card) Cast(target *Permanant, p *Player) {
	if strings.Contains(c.TypeLine, "Creature") {
		p.Board = append(p.Board, Permanant{
			source:    *c,
			owner:     p,
			tokenType: Creature,
		})
		return
	}

	// Hardcoding bolt to face
	p.Opponents[0].LifeTotal -= 3
	p.Graveyard = append(p.Graveyard, *c)

}
