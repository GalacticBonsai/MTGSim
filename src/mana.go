package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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

type ManaPool struct {
	pool map[ManaType]int
}

func NewManaPool() *ManaPool {
	return &ManaPool{pool: make(map[ManaType]int)}
}

func (mp *ManaPool) Add(manaType ManaType, amount int) {
	mp.pool[manaType] += amount
}

func (mp *ManaPool) CanPay(cost mana) bool {
	tempPool := make(map[ManaType]int)
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

func (mp *ManaPool) Pay(cost mana) error {
	if !mp.CanPay(cost) {
		return fmt.Errorf("not enough mana to pay the cost")
	}

	for manaType, amount := range cost.pool {
		mp.pool[manaType] -= amount
	}

	return nil
}

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
