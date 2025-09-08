package game

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
    // Deduct Any from whatever remains (greedy)
    anyNeed := cost[Any]
    for anyNeed > 0 {
        for _, t := range specific {
            if anyNeed == 0 {
                break
            }
            if mp.pool[t] > 0 {
                mp.pool[t]--
                anyNeed--
            }
        }
        if anyNeed == 0 {
            break
        }
        // If still needed and we have no specific left, break to avoid infinite loop
        if mp.total() == 0 {
            break
        }
    }
    return true
}

func (mp *ManaPool) total() int {
    sum := 0
    for _, v := range mp.pool {
        sum += v
    }
    return sum
}

