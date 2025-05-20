package main

import (
	"math/rand"
	"os"
	"time"
)

func sliceGet[T any](slice []T, index int) (T, []T) {
	var out T
	if index < 0 || index >= len(slice) {
		return out, slice
	}
	if len(slice) == 0 {
		return out, slice
	}

	out = slice[index]
	if index == len(slice)-1 {
		slice = slice[:index]
		return out, slice
	}
	slice = append(slice[:index], slice[index+1:]...)
	return out, slice
}

func getDecks(dir string) ([]string, error) {
	var fileList []string
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			subDirFiles, err := getDecks(dir + "/" + file.Name())
			if err != nil {
				return nil, err
			}
			fileList = append(fileList, subDirFiles...)
		} else {
			fileList = append(fileList, dir+"/"+file.Name())
		}
	}
	return fileList, nil
}

func getRandom[T any](slice []T) T {
	rand.Seed(time.Now().UnixNano())
	return slice[rand.Intn(len(slice))]
}

func main() {
	decks, err := getDecks("../decks/vanilla")
	if err != nil || len(decks) == 0 {
		return
	}

	for i := 0; i < 10; i++ {
		g := newGame()
		g.AddPlayer(getRandom(decks))
		g.AddPlayer(getRandom(decks))
		g.Start()
		AddWin(g.winner.Name)
		AddLoss(g.loser.Name)
	}

	PrintTopWinners()
}
