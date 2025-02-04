package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

var cardDB *CardDB

type CardDB struct {
	cards map[string]Card
}

func NewCardDB(cards []Card) *CardDB {
	cardMap := make(map[string]Card)
	for _, card := range cards {
		cardMap[card.CardFaces[0].Name] = card
	}
	return &CardDB{cards: cardMap}
}

func (db *CardDB) GetCardByName(name string) (Card, bool) {
	card, exists := db.cards[name]
	return card, exists
}

func downloadAndParseJSON(url string) ([]Card, error) {
	resp, err := http.Get(url)
	if (err != nil) {
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
	if _, err := ioutil.ReadFile("cardDB.json"); err == nil {
		file, err := ioutil.ReadFile("cardDB.json")
		if err != nil {
			fmt.Printf("Error reading cardDB.json: %v\n", err)
			return
		}

		var cards []Card
		err = json.Unmarshal(file, &cards)
		if err != nil {
			fmt.Printf("Error parsing cardDB.json: %v\n", err)
			return
		}

		cardDB = NewCardDB(cards)
		return
	}

	url := "https://data.scryfall.io/oracle-cards/oracle-cards-20250204100217.json"
	cards, err := downloadAndParseJSON(url)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	cardDB = NewCardDB(cards)

	file, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		fmt.Printf("Error marshalling JSON: %v\n", err)
		return
	}
	
	err = ioutil.WriteFile("cardDB.json", file, 0644)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}

	// Print the names of the cards as a test
	for i, card := range cards {
		fmt.Println(card.CardFaces[0].Name)
		if i > 10 {
			break
		}
	}
}