package main

import "fmt"

type Game struct {
	Players    []*Player
	turnNumber int
	winner     *Player
	loser      *Player
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
	for g.winner == nil {
		g.Players[currentPlayer].PlayTurn()

		if g.Players[currentPlayer].Opponents[0].LifeTotal <= 0 {
			g.winner = g.Players[currentPlayer]
			g.loser = g.winner.Opponents[0]
			break
		}

		if currentPlayer == 0 {
			currentPlayer = 1
		} else {
			currentPlayer = 0
			g.turnNumber++
			fmt.Printf("turn %d\n%d to %d\n", g.turnNumber, g.Players[currentPlayer].LifeTotal, g.Players[currentPlayer].Opponents[0].LifeTotal)
		}
	}
	fmt.Printf("Game Over\nPlayer %s wins\n", g.winner.Name)

	fmt.Printf("Game lasted %d turns\n", g.turnNumber)
	g.Players[0].Display()
	g.Players[1].Display()
}
