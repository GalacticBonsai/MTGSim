package main

import "fmt"

type Game struct {
	Players    []*Player
	turnNumber int
	won        bool
}

func newGame() *Game {
	return &Game{
		turnNumber: 0,
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
		p.Opponents = append(g.Players[:i], g.Players[i+1:]...)
		p.Hand = append(p.Hand, p.Deck.DrawCards(7)...)
	}
	// add shuffle players to pick start

	//play game
	for !g.won {
		g.Players[currentPlayer].PlayTurn()

		if g.Players[currentPlayer].Opponents[0].LifeTotal <= 0 {
			g.won = true
		}

		fmt.Printf("player A life total:%d\nplayer B life total:%d\n", g.Players[0].LifeTotal, g.Players[1].LifeTotal)

		if currentPlayer == 0 {
			currentPlayer = 1
		} else {
			currentPlayer = 0
			g.turnNumber++
		}
	}

}
