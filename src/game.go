package main

import "fmt"

type Game struct {
	Players    []*Player
	turnNumber int
	won        bool
}

func newGame() *Game {
	return &Game{
		turnNumber: 1,
	}
}

func (g *Game) AddPlayer(decklist string) {
	player := NewPlayer(decklist)
	g.Players = append(g.Players, player)
}

func (g *Game) Start() {
	g.won = false
	currentPlayer := 0
	for i, p := range g.Players {
		p.Deck.Shuffle()
		p.Opponents = append([]*Player{}, g.Players[:i]...)
		p.Opponents = append(p.Opponents, g.Players[i+1:]...)
		p.Hand = append(p.Hand, p.Deck.DrawCards(7)...)
	}
	// add shuffle players to pick start

	//play game
	fmt.Printf("turn %d\n", g.turnNumber)
	for !g.won {
		g.Players[currentPlayer].PlayTurn()

		if g.Players[currentPlayer].Opponents[0].LifeTotal <= 0 {
			g.won = true
			break
		}

		// fmt.Printf("player A life total:%d\nplayer B life total:%d\n", g.Players[0].LifeTotal, g.Players[1].LifeTotal)

		if currentPlayer == 0 {
			currentPlayer = 1
		} else {
			currentPlayer = 0
			g.turnNumber++
			fmt.Printf("turn %d\n", g.turnNumber)
		}
	}
	fmt.Printf("Game Over\nPlayer %d wins\n", currentPlayer)
	fmt.Printf("Game lasted %d turns\n", g.turnNumber)
	g.Players[0].Display()
	g.Players[1].Display()
}
