//go:build !ci
// +build !ci

package main

// func TestMain(m *testing.M) {
// 	SetLogLevel(WARN)
// 	m.Run()
// }

// func TestSingleGame(t *testing.T) {
// 	decks, err := getDecks("../decks/welcome")
// 	if err != nil || len(decks) == 0 {
// 		t.Fatalf("Failed to get decks: %v", err)
// 	}

// 	g := newGame()
// 	g.AddPlayer(getRandom(decks))
// 	g.AddPlayer(getRandom(decks))
// 	g.Start()

// 	if g.winner == nil {
// 		t.Errorf("Expected a winner, but got nil")
// 	}
// 	if g.loser == nil {
// 		t.Errorf("Expected a loser, but got nil")
// 	}
// 	if g.turnNumber >= 15 {
// 		t.Errorf("Expected game to last less than 15 turns, but got %d", g.turnNumber)
// 	}
// }
