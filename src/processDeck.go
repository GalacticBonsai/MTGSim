package main

import (
    "bufio"
    "fmt"
    "math/rand"
    "os"
    "strings"
    "time"
)

type Card struct {
    Name              string
    ManaValue         int
    ActivatedAbilities []string
    CardType          string
}

type Deck struct {
    Cards []Card
}

func importDeck(filename string) (Deck, error) {
    file, err := os.Open(filename)
	if err != nil {
		return Deck{}, err
    }
    defer file.Close()

    var cards []Card
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        card, err := parseCard(line)
        if err != nil {
            return Deck{}, err
        }
        cards = append(cards, card)
    }

    if err := scanner.Err(); err != nil {
        return Deck{}, err
    }

    return Deck{Cards: cards}, nil
}

func parseCard(line string) (Card, error) {
    switch line {
    case "Mountain":
        return Card{Name: "Mountain", ManaValue: 0, ActivatedAbilities: nil, CardType: "Land"}, nil
    case "Fireball":
        return Card{Name: "Fireball", ManaValue: 1, ActivatedAbilities: []string{"Deal damage"}, CardType: "Sorcery"}, nil
    default:
        return Card{}, fmt.Errorf("invalid card name: %s", line)
    }
}

func shuffleDeck(deck Deck) {
    rand.Seed(time.Now().UnixNano())
    rand.Shuffle(len(deck.Cards), func(i, j int) {
        deck.Cards[i], deck.Cards[j] = deck.Cards[j], deck.Cards[i]
    })
}

func drawCards(deck Deck, count int) []Card {
    if count > len(deck.Cards) {
        count = len(deck.Cards)
    }
    hand := deck.Cards[:count]
    deck.Cards = deck.Cards[count:]
    return hand
}

func drawCard(deck Deck) Card {
    if len(deck.Cards) == 0 {
        return Card{}
    }
    card := deck.Cards[0]
    deck.Cards = deck.Cards[1:]
    return card
}