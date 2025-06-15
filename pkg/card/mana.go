// Package card provides mana-related functionality for MTG simulation.
package card

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mtgsim/mtgsim/pkg/types"
)

// Mana represents a collection of mana with different types.
type Mana struct {
	pool map[types.ManaType]int
}

// NewMana creates a new empty mana pool.
func NewMana() Mana {
	return Mana{pool: make(map[types.ManaType]int)}
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
func (m *Mana) Add(manaType types.ManaType, amount int) {
	if m.pool == nil {
		m.pool = make(map[types.ManaType]int)
	}
	m.pool[manaType] += amount
}

// Get returns the amount of mana of a specific type.
func (m *Mana) Get(manaType types.ManaType) int {
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
			pool.Add(types.Any, num)
		} else {
			pool.Add(types.ManaType(value), 1)
		}
	}
	return pool
}

// ManaPool represents a player's available mana.
type ManaPool struct {
	pool map[types.ManaType]int
}

// NewManaPool creates a new empty mana pool.
func NewManaPool() *ManaPool {
	return &ManaPool{pool: make(map[types.ManaType]int)}
}

// Add adds mana of a specific type to the pool.
func (mp *ManaPool) Add(manaType types.ManaType, amount int) {
	mp.pool[manaType] += amount
}

// Total returns the total amount of mana in the pool.
func (mp *ManaPool) Total() int {
	total := 0
	for _, amount := range mp.pool {
		total += amount
	}
	return total
}

// Get returns the amount of mana of a specific type.
func (mp *ManaPool) Get(manaType types.ManaType) int {
	return mp.pool[manaType]
}

// String returns a string representation of the mana pool for debugging.
func (mp *ManaPool) String() string {
	if mp == nil || mp.pool == nil || mp.Total() == 0 {
		return "empty"
	}

	var parts []string
	for manaType, amount := range mp.pool {
		if amount > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", manaType, amount))
		}
	}
	return strings.Join(parts, ", ")
}

// CanPay checks if the pool has enough mana to pay the given cost.
func (mp *ManaPool) CanPay(cost Mana) bool {
	tempPool := make(map[types.ManaType]int)
	for k, v := range mp.pool {
		tempPool[k] = v
	}

	// First, pay for specific colored mana requirements
	for manaType, amount := range cost.pool {
		if manaType == types.Any {
			continue // Handle generic mana later
		}

		if tempPool[manaType] >= amount {
			tempPool[manaType] -= amount
		} else {
			return false // Not enough of this specific color
		}
	}

	// Then, pay for generic mana with any remaining mana
	if genericCost, hasGeneric := cost.pool[types.Any]; hasGeneric {
		totalAvailable := 0
		for _, amount := range tempPool {
			totalAvailable += amount
		}
		if totalAvailable < genericCost {
			return false
		}
	}

	return true
}

// Pay removes the specified mana cost from the pool.
func (mp *ManaPool) Pay(cost Mana) error {
	if !mp.CanPay(cost) {
		return fmt.Errorf("not enough mana to pay the cost")
	}

	// First, pay for specific colored mana requirements
	for manaType, amount := range cost.pool {
		if manaType == types.Any {
			continue // Handle generic mana later
		}
		mp.pool[manaType] -= amount
	}

	// Then, pay for generic mana with any remaining mana
	if genericCost, hasGeneric := cost.pool[types.Any]; hasGeneric {
		remaining := genericCost

		// First try to use any colorless mana
		if mp.pool[types.Colorless] > 0 {
			used := min(mp.pool[types.Colorless], remaining)
			mp.pool[types.Colorless] -= used
			remaining -= used
		}

		// Then use any other mana types
		for manaType := range mp.pool {
			if manaType == types.Any || manaType == types.Colorless {
				continue
			}
			if remaining <= 0 {
				break
			}
			available := mp.pool[manaType]
			used := min(available, remaining)
			mp.pool[manaType] -= used
			remaining -= used
		}
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CheckManaProducer analyzes oracle text to determine if a card produces mana.
// Returns true if it's a mana producer and the types of mana it produces.
func CheckManaProducer(oracleText string) (bool, []types.ManaType) {
	// Check if the text contains "Add" which indicates mana production
	if !strings.Contains(oracleText, "Add") {
		return false, []types.ManaType{}
	}

	var manaTypes []types.ManaType

	// Look for specific mana symbols in the text
	manaRegex := regexp.MustCompile(`\{([WUBRGC])\}`)
	matches := manaRegex.FindAllStringSubmatch(oracleText, -1)

	for _, match := range matches {
		switch match[1] {
		case "W":
			manaTypes = append(manaTypes, types.White)
		case "U":
			manaTypes = append(manaTypes, types.Blue)
		case "B":
			manaTypes = append(manaTypes, types.Black)
		case "R":
			manaTypes = append(manaTypes, types.Red)
		case "G":
			manaTypes = append(manaTypes, types.Green)
		case "C":
			manaTypes = append(manaTypes, types.Colorless)
		}
	}

	// Check for "any color" or "one mana of any color"
	if strings.Contains(strings.ToLower(oracleText), "any color") ||
	   strings.Contains(strings.ToLower(oracleText), "one mana of any color") {
		manaTypes = append(manaTypes, types.Any)
	}

	return len(manaTypes) > 0, manaTypes
}
