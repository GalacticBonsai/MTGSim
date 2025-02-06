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
	Name  string
}

func (d *Deck) Shuffle() {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(d.Cards), func(i, j int) {
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i]
	})
}

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
		parts := strings.Split(strings.TrimSpace(scanner.Text()), " ")
		count, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		
		name := ""
		if strings.Contains(scanner.Text(), "("){
			name = strings.Join(parts[1:len(parts)-2], " ")
			// set_code := strings.Trim(parts[len(parts)-2], "()")
			// set_number := parts[len(parts)-1]
		} else {
			name = strings.Join(parts[1:], " ")
		}
		
		card, exists := cardDB.GetCardByName(name)
		if !exists {
			fmt.Println(fmt.Errorf("card not found: %s", parts))
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
