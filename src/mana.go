package main

import (
	"regexp"
	"strconv"
	"strings"
)

type ManaType string

const (
	White     ManaType = "W"
	Blue      ManaType = "U"
	Black     ManaType = "B"
	Red       ManaType = "R"
	Green     ManaType = "G"
	Colorless ManaType = "C"
	Any       ManaType = "A"
	Phyrexian ManaType = "P"
	Snow      ManaType = "S"
	X         ManaType = "X"
)

func (mt ManaType) String() string {
	return string(mt)
}

type mana struct {
	pool map[ManaType]int
}

func newMana() mana {
	return mana{pool: make(map[ManaType]int)}
}

func (m *mana) total() int {
	total := 0
	for _, count := range m.pool {
		total += count
	}
	return total
}

func ParseManaCost(cost string) mana {
	pool := newMana()
	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(cost, -1)

	for _, match := range matches {
		value := match[1]
		if num, err := strconv.Atoi(value); err == nil {
			pool.pool[Any] += num
		} else {
			pool.pool[ManaType(value)]++
		}
	}
	return pool
}

// func (m *mana) adjust(cost mana, add bool) {
// 	for mt, count := range cost.pool {
// 		if add {
// 			m.pool[mt] += count
// 		} else {
// 			m.pool[mt] -= count
// 		}
// 	}
// }

// func (m *mana) pay(cost mana) error {
// 	if m.total() < cost.total() {
// 		return fmt.Errorf("not enough Mana")
// 	}

// 	m.adjust(cost, false)

// 	// if no any color, early return
// 	if cost.pool[Any] == 0 {
// 		return nil
// 	}

// 	// Try to pay it from same manapool
// 	for mt, count := range m.pool {
// 		if count >= cost.pool[Any] {
// 			m.pool[mt] -= cost.pool[Any]
// 			return nil
// 		}
// 	}

// 	// pay from residual mana
// 	for cost.pool[Any] > 0 {
// 		for mt, count := range m.pool {
// 			if count > 0 {
// 				m.pool[mt]--
// 				cost.pool[Any]--
// 				break
// 			}
// 		}
// 		if cost.pool[Any] > 0 {
// 			return fmt.Errorf("not enough Mana")
// 		}
// 	}
// 	return nil
// }

func CheckManaProducer(oracleText string) (bool, []ManaType) {
	parts := strings.Split(oracleText, ":")
	if len(parts) < 2 {
		return false, []ManaType{}
	}

	effect := parts[1]
	manaRegex := regexp.MustCompile(`\{([WUBRGC])\}|any color`)
	matches := manaRegex.FindAllStringSubmatch(effect, -1)

	if len(matches) == 0 {
		return false, []ManaType{}
	}

	var manaTypes []ManaType
	for _, match := range matches {
		if match[1] == "" {
			manaTypes = append(manaTypes, Any)
		} else {
			manaTypes = append(manaTypes, ManaType(match[1]))
		}
	}

	return true, manaTypes
}
