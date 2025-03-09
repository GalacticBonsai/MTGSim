package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const cardDBfile = "../cardDB.json"
const carddburl = "https://data.scryfall.io/oracle-cards/oracle-cards-20250204100217.json"

var cardDB *CardDB

type CardDB struct {
	cards map[string]Card
}

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

func (db *CardDB) GetCardByName(name string) (Card, bool) {
	card, exists := db.cards[name]
	return card, exists
}

func downloadAndParseJSON(url string) ([]Card, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download JSON: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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

func init() {
	var cards []Card

	if _, err := ioutil.ReadFile(cardDBfile); err == nil {
		file, err := ioutil.ReadFile(cardDBfile)
		if err != nil {
			Error("Error reading cardDB.json:", err)
			return
		}

		err = json.Unmarshal(file, &cards)
		if err != nil {
			Error("Error parsing cardDB.json:", err)
			return
		}
	} else {
		url := carddburl
		cards, err := downloadAndParseJSON(url)
		if err != nil {
			Error("Error:", err)
			return
		}

		content, err := json.MarshalIndent(cards, "", "  ")
		if err != nil {
			Error("Error marshalling JSON:", err)
			return
		}

		err = ioutil.WriteFile(cardDBfile, content, 0644)
		if err != nil {
			Error("Error writing to file:", err)
			return
		}
	}

	cardDB = NewCardDB(cards)
	if cardDB == nil {
		Error("Error creating cardDB")
		return
	}
}
