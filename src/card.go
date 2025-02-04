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
    HighresImage      bool
    ImageStatus       string
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
    Name           string
    ManaCost       string
    TypeLine       string
    OracleText     string
    Colors         []string
    Artist         string
    ArtistID       string
    IllustrationID string
    ImageURIs      map[string]string
}

// NewCard creates a new Card instance
func NewCard(id, oracleID, name, lang, releasedAt, uri, scryfallURI, layout, imageStatus, setID, set, setName, setType, setURI, setSearchURI, scryfallSetURI, rulingsURI, printsSearchURI, collectorNumber, rarity, artist, borderColor, frame string, multiverseIDs []int, colorIdentity, keywords, games, finishes, artistIDs, promoTypes []string, cardFaces []CardFace, legalities, prices, relatedURIs, purchaseURIs map[string]string, highresImage, reserved, foil, nonFoil, oversized, promo, reprint, variation, digital, fullArt, textless, booster, storySpotlight bool, cmc float64) *Card {
    return &Card{
        ID:                id,
        OracleID:          oracleID,
        MultiverseIDs:     multiverseIDs,
        Lang:              lang,
        ReleasedAt:        releasedAt,
        URI:               uri,
        ScryfallURI:       scryfallURI,
        Layout:            layout,
        HighresImage:      highresImage,
        ImageStatus:       imageStatus,
        CMC:               cmc,
        ColorIdentity:     colorIdentity,
        Keywords:          keywords,
        CardFaces:         cardFaces,
        Legalities:        legalities,
        Games:             games,
        Reserved:          reserved,
        Foil:              foil,
        NonFoil:           nonFoil,
        Finishes:          finishes,
        Oversized:         oversized,
        Promo:             promo,
        Reprint:           reprint,
        Variation:         variation,
        SetID:             setID,
        Set:               set,
        SetName:           setName,
        SetType:           setType,
        SetURI:            setURI,
        SetSearchURI:      setSearchURI,
        ScryfallSetURI:    scryfallSetURI,
        RulingsURI:        rulingsURI,
        PrintsSearchURI:   printsSearchURI,
        CollectorNumber:   collectorNumber,
        Digital:           digital,
        Rarity:            rarity,
        Artist:            artist,
        ArtistIDs:         artistIDs,
        BorderColor:       borderColor,
        Frame:             frame,
        FullArt:           fullArt,
        Textless:          textless,
        Booster:           booster,
        StorySpotlight:    storySpotlight,
        PromoTypes:        promoTypes,
        Prices:            prices,
        RelatedURIs:       relatedURIs,
        PurchaseURIs:      purchaseURIs,
    }
}

// UpdateCard updates the details of an existing Card instance
func (c *Card) UpdateCard(name, lang, releasedAt, uri, scryfallURI, layout, imageStatus, setID, set, setName, setType, setURI, setSearchURI, scryfallSetURI, rulingsURI, printsSearchURI, collectorNumber, rarity, artist, borderColor, frame string, multiverseIDs, colorIdentity, keywords, games, finishes, artistIDs, promoTypes []string, cardFaces []CardFace, legalities, prices, relatedURIs, purchaseURIs map[string]string, highresImage, reserved, foil, nonFoil, oversized, promo, reprint, variation, digital, fullArt, textless, booster, storySpotlight bool, cmc float64) {
    c.CardFaces[0].Name = name
    c.Lang = lang
    c.ReleasedAt = releasedAt
    c.URI = uri
    c.ScryfallURI = scryfallURI
    c.Layout = layout
    c.HighresImage = highresImage
    c.ImageStatus = imageStatus
    c.CMC = cmc
    c.ColorIdentity = colorIdentity
    c.Keywords = keywords
    c.CardFaces = cardFaces
    c.Legalities = legalities
    c.Games = games
    c.Reserved = reserved
    c.Foil = foil
    c.NonFoil = nonFoil
    c.Finishes = finishes
    c.Oversized = oversized
    c.Promo = promo
    c.Reprint = reprint
    c.Variation = variation
    c.SetID = setID
    c.Set = set
    c.SetName = setName
    c.SetType = setType
    c.SetURI = setURI
    c.SetSearchURI = setSearchURI
    c.ScryfallSetURI = scryfallSetURI
    c.RulingsURI = rulingsURI
    c.PrintsSearchURI = printsSearchURI
    c.CollectorNumber = collectorNumber
    c.Digital = digital
    c.Rarity = rarity
    c.Artist = artist
    c.ArtistIDs = artistIDs
    c.BorderColor = borderColor
    c.Frame = frame
    c.FullArt = fullArt
    c.Textless = textless
    c.Booster = booster
    c.StorySpotlight = storySpotlight
    c.PromoTypes = promoTypes
    c.Prices = prices
    c.RelatedURIs = relatedURIs
    c.PurchaseURIs = purchaseURIs
}

// DisplayCardSingleLine prints the details of a Card instance in a single line
func (c *Card) Display() {
    fmt.Printf("Name: %s, Mana Value: %f, Type: %s\n", c.CardFaces[0].Name, c.CMC, c.CardFaces[0].TypeLine)
}

// DisplayCard prints the details of a Card instance
func DisplayCard(card Card) {
    fmt.Printf("Name: %s\n", card.CardFaces[0].Name)
    fmt.Printf("Mana Value: %f\n", card.CMC)
    fmt.Printf("Type: %s\n", card.CardFaces[0].TypeLine)
    // fmt.Printf("Set: %s\n", card.SetName)
    // fmt.Printf("Scryfall URI: %s\n", card.ScryfallURI)
}

func DisplayCards(cards []Card) {
	for _, card := range cards {
		DisplayCard(card)
	}
}