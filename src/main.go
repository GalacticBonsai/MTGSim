package main

import (
    "fmt"
)

func main() {
    deck, err := importDeck("fireball.deck")
    if err != nil {
        fmt.Println("Error importing deck:", err)
        return
    }

    fmt.Println("Deck imported successfully:")
    for _, card := range deck.Cards {
        fmt.Printf("Name: %s, Mana Value: %d, Activated Abilities: %v, Card Type: %s\n", card.Name, card.ManaValue, card.ActivatedAbilities, card.CardType)
    }

    // Shuffle the deck
    shuffleDeck(deck)

    // Draw initial hand of 7 cards
    hand := drawCards(deck, 7)
    fmt.Println("Initial hand drawn:")
    for _, card := range hand {
        fmt.Printf("Name: %s, Mana Value: %d, Activated Abilities: %v, Card Type: %s\n", card.Name, card.ManaValue, card.ActivatedAbilities, card.CardType)
    }

    // Simulate turns
    for turn := 1; turn <= 10; turn++ {
        fmt.Printf("Turn %d:\n", turn)

        // Draw a card at the beginning of the turn
        drawnCard := drawCard(deck)
        fmt.Printf("Drew card: Name: %s, Mana Value: %d, Activated Abilities: %v, Card Type: %s\n", drawnCard.Name, drawnCard.ManaValue, drawnCard.ActivatedAbilities, drawnCard.CardType)
        hand = append(hand, drawnCard)

        // Play a land if available
        landPlayed := false
        for i, card := range hand {
            if card.CardType == "Land" {
                fmt.Printf("Playing land: %s\n", card.Name)
                hand = append(hand[:i], hand[i+1:]...)
                landPlayed = true
                break
            }
        }

        if !landPlayed {
            fmt.Println("No land to play this turn.")
        }
    }
}