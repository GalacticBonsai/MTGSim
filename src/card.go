package main

import (
	"fmt"
)

type Card struct {
    ID                string
    OracleID          string
    MultiverseIDs     []int
    Lang              string
    ReleasedAt        string
    URI               string
    ScryfallURI       string
    Layout            string
    CMC               float64
    ColorIdentity     []string
    Keywords          []string
    CardFaces         []CardFace
    Legalities        map[string]string
    Games             []string
    Reserved          bool
    Foil              bool
    NonFoil           bool
    Finishes          []string
    Oversized         bool
    Promo             bool
    Reprint           bool
    Variation         bool
    SetID             string
    Set               string
    SetName           string
    SetType           string
    SetURI            string
    SetSearchURI      string
    ScryfallSetURI    string
    RulingsURI        string
    PrintsSearchURI   string
    Name           string
    ManaCost       string
    TypeLine       string
    OracleText     string
    Colors         []string
    CollectorNumber   string
    Digital           bool
    Rarity            string
    Artist            string
    ArtistIDs         []string
    BorderColor       string
    Frame             string
    FullArt           bool
    Textless          bool
    Booster           bool
    StorySpotlight    bool
    PromoTypes        []string
    Prices            map[string]string
    RelatedURIs       map[string]string
    PurchaseURIs      map[string]string
}

type CardFace struct {

    ArtistID       string
    IllustrationID string
    ImageURIs      map[string]string
}

// DisplayCardSingleLine prints the details of a Card instance in a single line
func (c *Card) Display() {
    fmt.Printf("Name: %s, Mana Value: %f, Type: %s\n", c.Name, c.CMC, c.TypeLine)
}

// DisplayCard prints the details of a Card instance
func DisplayCard(card Card) {
    fmt.Printf("Name: %s\n", card.Name)
    fmt.Printf("Mana Value: %f\n", card.CMC)
    fmt.Printf("Type: %s\n", card.TypeLine)
    // fmt.Printf("Set: %s\n", card.SetName)
    // fmt.Printf("Scryfall URI: %s\n", card.ScryfallURI)
}

func DisplayCards(cards []Card) {
	for _, card := range cards {
		DisplayCard(card)
	}
}