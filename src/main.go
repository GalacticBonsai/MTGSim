package main

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

func main() {
	g := newGame()
	g.AddPlayer("../decks/lightningbolt.deck")
	g.AddPlayer("../decks/lightningbolt.deck")
	// g.AddPlayer("../decks/vanilla.txt")
	g.Start()
}
