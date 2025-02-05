package main

func main() {
	g := newGame()
	g.AddPlayer("../decks/lightningbolt.deck")
	g.AddPlayer("../decks/lightningbolt.deck")
	g.Start()
}
