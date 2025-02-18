package main

import (
	"fmt"
	"sort"
)

// winner represents a deck with its name and number of wins
type winner struct {
	Name   string
	Wins   int
	Losses int
}

// Global list of winning decks
var results = []winner{}

// AddWin adds a win to the specified deck
func AddWin(deckName string) {
	for i, deck := range results {
		if deck.Name == deckName {
			results[i].Wins++
			return
		}
	}
	results = append(results, winner{Name: deckName, Wins: 1})
}

// AddWin adds a win to the specified deck
func AddLoss(deckName string) {
	for i, deck := range results {
		if deck.Name == deckName {
			results[i].Losses++
			return
		}
	}
	results = append(results, winner{Name: deckName, Losses: 1})
}

// SortWinners sorts the results list based on the number of wins in descending order
func SortWinners() {
	sort.Slice(results, func(i, j int) bool {
		winPercentageI := float64(results[i].Wins) / float64(results[i].Wins+results[i].Losses)
		winPercentageJ := float64(results[j].Wins) / float64(results[j].Wins+results[j].Losses)
		return winPercentageI > winPercentageJ
	})
}

func PrintTopWinners() {
	SortWinners()
	for _, player := range results {
		winPercent := float64(player.Wins) / float64(player.Wins+player.Losses) * 100
		fmt.Printf("%s won %d/%d games(%.2fPercent)\n", player.Name, player.Wins, player.Wins+player.Losses, winPercent)
	}
}
