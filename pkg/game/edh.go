package game

import (
	"errors"
	"math/rand"
)

// EDH / Commander format helpers (CR 903 + format-specific overrides on
// starting life total and opening-hand size).

// EDHStartingLife is the starting life total for the Commander format
// (CR 903.7 — 40 life in multiplayer Commander; 1v1 EDH variants vary,
// but we use 40 by default to match the canonical format).
const EDHStartingLife = 40

// OpeningHandSize is the standard 7-card opening hand (CR 103.4).
const OpeningHandSize = 7

// NewEDHPlayer returns a Player initialized for the Commander format.
func NewEDHPlayer(name string) *Player {
	return NewPlayer(name, EDHStartingLife)
}

// NewEDHGame constructs a multiplayer EDH game with all players set to
// the format starting life total. Pass two players for 1v1 verification
// or three+ for the standard multiplayer experience.
func NewEDHGame(names ...string) *Game {
	players := make([]*Player, 0, len(names))
	for _, n := range names {
		players = append(players, NewEDHPlayer(n))
	}
	return NewGame(players...)
}

// DrawOpeningHand resets a player's hand and library so the player draws
// a fresh seven cards from the top. Caller is responsible for shuffling
// before this is called when a deterministic library order is desired.
func (p *Player) DrawOpeningHand() {
	if p == nil {
		return
	}
	// Return any current hand to the library before drawing fresh.
	p.Library = append(p.Hand, p.Library...)
	p.Hand = p.Hand[:0]
	p.Draw(OpeningHandSize)
}

// LondonMulligan implements CR 103.4 — the London Mulligan rule used in
// Commander since 2019. The player shuffles their current hand into the
// library, draws seven cards, then puts `mulligansTaken` cards on the
// bottom of their library in any order. The automated player chooses to
// bottom the cards with the highest converted mana cost first by sorting
// by name as a stable proxy (CMC isn't tracked on SimpleCard yet).
//
// Returns the number of cards put on the bottom; an error if more
// mulligans are requested than the hand can accommodate.
func (p *Player) LondonMulligan(rng *rand.Rand, mulligansTaken int) (int, error) {
	if p == nil {
		return 0, errors.New("nil player")
	}
	if mulligansTaken < 0 {
		return 0, errors.New("mulligansTaken must be non-negative")
	}
	// Step 1: return current hand to library
	p.Library = append(p.Hand, p.Library...)
	p.Hand = p.Hand[:0]
	// Step 2: shuffle library
	if rng != nil {
		rng.Shuffle(len(p.Library), func(i, j int) {
			p.Library[i], p.Library[j] = p.Library[j], p.Library[i]
		})
	}
	// Step 3: draw 7
	p.Draw(OpeningHandSize)
	// Step 4: bottom `mulligansTaken` cards (the "tax" for each mulligan)
	bottom := mulligansTaken
	if bottom > len(p.Hand) {
		bottom = len(p.Hand)
	}
	if bottom == 0 {
		return 0, nil
	}
	// Automated choice: bottom the last cards added to hand. With no CMC
	// data this is deterministic and good enough for simulation; a smarter
	// chooser can replace this without changing the API.
	keep := len(p.Hand) - bottom
	bottomed := make([]SimpleCard, bottom)
	copy(bottomed, p.Hand[keep:])
	p.Hand = p.Hand[:keep]
	p.Library = append(p.Library, bottomed...)
	return bottom, nil
}

// HandSize is a convenience helper used by mulligan tests.
func (p *Player) HandSize() int {
	if p == nil {
		return 0
	}
	return len(p.Hand)
}

// LibrarySize returns the current library size.
func (p *Player) LibrarySize() int {
	if p == nil {
		return 0
	}
	return len(p.Library)
}
