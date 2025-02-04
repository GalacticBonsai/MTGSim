package main

import (
	"fmt"
)

func main() {
    deck, err := importDeckfile("fireball.deck")
    if err != nil {
        fmt.Println("Error importing deck:", err)
        return
    }

    fmt.Println("Deck imported successfully:")
    deck.Display()

    // Shuffle the deck
    deck.Shuffle()

    // Draw initial hand of 7 cards
    hand := deck.DrawCards(7)
    fmt.Println("Initial hand drawn:")
    DisplayCards(hand)

    // Simulate turns
    for turn := 1; turn <= 10; turn++ {
        fmt.Printf("Turn %d:\n", turn)

        // Draw a card at the beginning of the turn
        drawnCard := deck.DrawCard()
        drawnCard.Display()
        hand = append(hand, drawnCard)

        // Play a land if available
        landPlayed := false
        for i, card := range hand {
            if card.CardFaces[0].TypeLine == "Land" {
                fmt.Printf("Playing land: ")
                card.Display()
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