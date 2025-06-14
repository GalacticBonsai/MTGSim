// Package main provides deck generation utilities for MTGSim.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func generateDecks(cardNames []string, deckSize int, outputDir string) {
	combinations := generateCombinations(cardNames, deckSize)
	for i, deck := range combinations {
		fileName := fmt.Sprintf("deck_%d.deck", i+1)
		filePath := filepath.Join(outputDir, fileName)
		writeDeckToFile(deck, filePath)
	}
}

func generateCombinations(cardNames []string, deckSize int) [][]string {
	var combinations [][]string
	var generate func([]string, int, []string)
	generate = func(currentDeck []string, remaining int, remainingCards []string) {
		if remaining == 0 {
			combinations = append(combinations, append([]string{}, currentDeck...))
			return
		}
		for i, card := range remainingCards {
			generate(append(currentDeck, card), remaining-1, remainingCards[i:])
		}
	}
	generate([]string{}, deckSize, cardNames)
	return combinations
}

func writeDeckToFile(deck []string, filePath string) {
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	cardCount := make(map[string]int)
	for _, card := range deck {
		cardCount[card]++
	}

	for card, count := range cardCount {
		file.WriteString(fmt.Sprintf("%d %s\n", count, card))
	}
}

func main() {
	cardNames := []string{"Mountain", "Lightning Bolt"}
	deckSize := 30
	outputDir := "../decks/generated"

	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		fmt.Println("Error creating directory:", err)
		return
	}

	files, err := os.ReadDir(outputDir)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return
	}

	for _, file := range files {
		err := os.Remove(filepath.Join(outputDir, file.Name()))
		if err != nil {
			fmt.Println("Error deleting file:", err)
		}
	}

	generateDecks(cardNames, deckSize, outputDir)
}
