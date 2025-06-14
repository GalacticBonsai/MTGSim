// Package card provides mana-related functionality for MTG simulation.
package card

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// Mana represents a collection of mana with different types.
type Mana struct {
	pool map[game.ManaType]int
}

// NewMana creates a new empty mana pool.
func NewMana() Mana {
	return Mana{pool: make(map[game.ManaType]int)}
}

// Total returns the total amount of mana in the pool.
func (m *Mana) Total() int {
	total := 0
	for _, count := range m.pool {
		total += count
	}
	return total
}

// Add adds mana of a specific type to the pool.
func (m *Mana) Add(manaType game.ManaType, amount int) {
	if m.pool == nil {
		m.pool = make(map[game.ManaType]int)
	}
	m.pool[manaType] += amount
}

// Get returns the amount of mana of a specific type.
func (m *Mana) Get(manaType game.ManaType) int {
	return m.pool[manaType]
}

// ParseManaCost parses a mana cost string like "{2}{R}{G}" into a Mana object.
func ParseManaCost(cost string) Mana {
	pool := NewMana()
	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(cost, -1)

	for _, match := range matches {
		value := match[1]
		if num, err := strconv.Atoi(value); err == nil {
			pool.Add(game.Any, num)
		} else {
			pool.Add(game.ManaType(value), 1)
		}
	}
	return pool
}

// ManaPool represents a player's available mana.
type ManaPool struct {
	pool map[game.ManaType]int
}

// NewManaPool creates a new empty mana pool.
func NewManaPool() *ManaPool {
	return &ManaPool{pool: make(map[game.ManaType]int)}
}

// Add adds mana of a specific type to the pool.
func (mp *ManaPool) Add(manaType game.ManaType, amount int) {
	mp.pool[manaType] += amount
}

// CanPay checks if the pool has enough mana to pay the given cost.
func (mp *ManaPool) CanPay(cost Mana) bool {
	tempPool := make(map[game.ManaType]int)
	for k, v := range mp.pool {
		tempPool[k] = v
	}

	for manaType, amount := range cost.pool {
		if tempPool[manaType] < amount {
			return false
		}
		tempPool[manaType] -= amount
	}

	return true
}

// Pay removes the specified mana cost from the pool.
func (mp *ManaPool) Pay(cost Mana) error {
	if !mp.CanPay(cost) {
		return fmt.Errorf("not enough mana to pay the cost")
	}

	for manaType, amount := range cost.pool {
		mp.pool[manaType] -= amount
	}

	return nil
}

// CheckManaProducer analyzes oracle text to determine if a card produces mana.
// Returns true if it's a mana producer and the types of mana it produces.
func CheckManaProducer(oracleText string) (bool, []game.ManaType) {
	// Check if the text contains "Add" which indicates mana production
	if !strings.Contains(oracleText, "Add") {
		return false, []game.ManaType{}
	}

	var manaTypes []game.ManaType

	// Look for specific mana symbols in the text
	manaRegex := regexp.MustCompile(`\{([WUBRGC])\}`)
	matches := manaRegex.FindAllStringSubmatch(oracleText, -1)

	for _, match := range matches {
		switch match[1] {
		case "W":
			manaTypes = append(manaTypes, game.White)
		case "U":
			manaTypes = append(manaTypes, game.Blue)
		case "B":
			manaTypes = append(manaTypes, game.Black)
		case "R":
			manaTypes = append(manaTypes, game.Red)
		case "G":
			manaTypes = append(manaTypes, game.Green)
		case "C":
			manaTypes = append(manaTypes, game.Colorless)
		}
	}

	// Check for "any color" or "one mana of any color"
	if strings.Contains(strings.ToLower(oracleText), "any color") ||
	   strings.Contains(strings.ToLower(oracleText), "one mana of any color") {
		manaTypes = append(manaTypes, game.Any)
	}

	return len(manaTypes) > 0, manaTypes
}
