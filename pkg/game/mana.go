package game

import (
	"strconv"
	"strings"
)

// Mana is a simple map of mana amounts by type.
type Mana map[ManaType]int

// Add adds amount of a mana type to the cost/collection.
func (m Mana) Add(t ManaType, n int) {
	if n <= 0 {
		return
	}
	if m == nil {
		return
	}
	m[t] = m[t] + n
}

// Get returns the amount of a mana type.
func (m Mana) Get(t ManaType) int { return m[t] }

// Total returns the sum of all mana symbols (including Any/X as stored).
func (m Mana) Total() int {
	sum := 0
	for _, v := range m {
		sum += v
	}
	return sum
}

// ManaPool represents a player's available mana for payment.
type ManaPool struct {
	pool map[ManaType]int
}

func NewManaPool() *ManaPool { return &ManaPool{pool: map[ManaType]int{}} }

// Add adds to the player's mana pool.
func (mp *ManaPool) Add(t ManaType, n int) {
	if n <= 0 {
		return
	}
	if mp.pool == nil {
		mp.pool = map[ManaType]int{}
	}
	mp.pool[t] = mp.pool[t] + n
}

// Clear empties the mana pool at the end of a step or phase (CR 500.4).
func (mp *ManaPool) Clear() {
	if mp == nil {
		return
	}
	for k := range mp.pool {
		delete(mp.pool, k)
	}
}

// Get returns amount for a mana type.
func (mp *ManaPool) Get(t ManaType) int { return mp.pool[t] }

// CanPay checks if the pool has enough to cover the required mana exactly by type.
// For now, "Any" in the cost can be paid by any colored or colorless mana.
func (mp *ManaPool) CanPay(cost Mana) bool {
	if cost == nil {
		return true
	}
	// First satisfy specific colored/colorless requirements (not Any/X)
	temp := make(map[ManaType]int, len(mp.pool))
	for k, v := range mp.pool {
		temp[k] = v
	}

	// Specific symbols (W,U,B,R,G,C)
	specific := []ManaType{White, Blue, Black, Red, Green, Colorless}
	for _, t := range specific {
		need := cost[t]
		if need == 0 {
			continue
		}
		if temp[t] < need {
			return false
		}
		temp[t] -= need
	}

	// Any can be covered by remaining total of all mana types
	anyNeed := cost[Any]
	if anyNeed > 0 {
		remain := 0
		for _, v := range temp {
			remain += v
		}
		if remain < anyNeed {
			return false
		}
		// not mutating temp further as this is just a check
	}
	return true
}

// Pay removes the specified cost from the pool if possible.
func (mp *ManaPool) Pay(cost Mana) bool {
	if !mp.CanPay(cost) {
		return false
	}
	// Deduct specific first
	specific := []ManaType{White, Blue, Black, Red, Green, Colorless}
	for _, t := range specific {
		need := cost[t]
		if need == 0 {
			continue
		}
		mp.pool[t] -= need
	}
	// Deduct Any from whatever remains (greedy).
	// We iterate over every entry in the pool, not just the specific slice,
	// so that Snow/Any/Phyrexian/etc. can also be spent for generic costs.
	anyNeed := cost[Any]
	for anyNeed > 0 {
		found := false
		for t, v := range mp.pool {
			if anyNeed == 0 {
				break
			}
			if v > 0 {
				mp.pool[t]--
				anyNeed--
				found = true
			}
		}
		if !found {
			break
		}
	}
	return true
}

// parseManaCost parses Scryfall-style mana costs such as "{2}{W}{U}" into
// the engine's Mana representation. Generic numeric symbols become Any;
// colored and colorless symbols remain specific requirements. X is tracked
// separately so callers can decide what value X should have in context.
func parseManaCost(cost string) Mana {
	out := Mana{}
	for _, sym := range manaSymbols(cost) {
		if sym == "" {
			continue
		}
		if n, err := strconv.Atoi(sym); err == nil {
			out.Add(Any, n)
			continue
		}
		switch strings.ToUpper(sym) {
		case "W":
			out.Add(White, 1)
		case "U":
			out.Add(Blue, 1)
		case "B":
			out.Add(Black, 1)
		case "R":
			out.Add(Red, 1)
		case "G":
			out.Add(Green, 1)
		case "C":
			out.Add(Colorless, 1)
		case "X":
			out.Add(X, 1)
		case "S":
			out.Add(Snow, 1)
		}
	}
	return out
}

func manaSymbols(cost string) []string {
	cost = strings.TrimSpace(cost)
	if cost == "" {
		return nil
	}
	var out []string
	for i := 0; i < len(cost); i++ {
		if cost[i] != '{' {
			continue
		}
		end := strings.IndexByte(cost[i+1:], '}')
		if end < 0 {
			break
		}
		out = append(out, cost[i+1:i+1+end])
		i += end + 1
	}
	if len(out) > 0 {
		return out
	}
	// Fallback for compact test inputs like "2WU".
	for i := 0; i < len(cost); i++ {
		ch := cost[i]
		if ch >= '0' && ch <= '9' {
			j := i + 1
			for j < len(cost) && cost[j] >= '0' && cost[j] <= '9' {
				j++
			}
			out = append(out, cost[i:j])
			i = j - 1
			continue
		}
		out = append(out, string(ch))
	}
	return out
}
