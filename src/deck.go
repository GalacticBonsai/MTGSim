package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

type Deck struct {
    Cards []Card
    Name string
}

func (d *Deck) Shuffle() {
    rand.Seed(time.Now().UnixNano())
    rand.Shuffle(len(d.Cards), func(i, j int) {
        d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i]
    })}

func (d *Deck) DrawCards(count int) []Card {
    if count > len(d.Cards) {
        count = len(d.Cards)
    }
    hand := d.Cards[:count]
    d.Cards = d.Cards[count:]
    return hand
}

func (d *Deck) DrawCard() Card {
    if len(d.Cards) == 0 {
        return Card{}
    }
    card := d.Cards[0]
    d.Cards = d.Cards[1:]
    return card
}

func importDeckfile(filename string) (Deck, error) {
    file, err := os.Open(filename)
	if err != nil {
		return Deck{}, err
    }
    defer file.Close()

    var cards []Card
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        name := strings.Split(line, "x")[0]
        count := 1
        if strings.Contains(line, "x") {
            count, err = strconv.Atoi(strings.Split(line, "x")[1])
            if err != nil {
                count = 1
            }
        }
        card, exists := cardDB.GetCardByName(name)
        if !exists {
            fmt.Println(fmt.Errorf("card not found: %s", line))
        }
        for i := 0; i < count; i++ {
            cards = append(cards, card)
        }
    }

    if err := scanner.Err(); err != nil {
        return Deck{}, err
    }

    return Deck{Cards: cards}, nil
}

func (d *Deck) Display() {
    for _, card := range d.Cards {
        card.Display()
    }
}