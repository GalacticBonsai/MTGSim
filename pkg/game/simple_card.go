package game

import "strings"

// SimpleCard is a minimal in-engine card representation to avoid package cycles.
type SimpleCard struct {
	Name       string
	TypeLine   string
	Power      string
	Toughness  string
	OracleText string
	Colors     []string
	// ColorIdentity is used by Commander/EDH import and sideboard variant
	// automation to keep generated lists legal under CR 903.4.
	ColorIdentity []string
	ManaCost      string

}

func (c SimpleCard) IsLand() bool         { return contains(c.TypeLine, "Land") }
func (c SimpleCard) IsCreature() bool     { return contains(c.TypeLine, "Creature") }
func (c SimpleCard) IsAura() bool         { return contains(c.TypeLine, "Aura") }
func (c SimpleCard) IsInstant() bool      { return contains(c.TypeLine, "Instant") }
func (c SimpleCard) IsSorcery() bool      { return contains(c.TypeLine, "Sorcery") }
func (c SimpleCard) IsArtifact() bool     { return contains(c.TypeLine, "Artifact") }
func (c SimpleCard) IsEnchantment() bool  { return contains(c.TypeLine, "Enchantment") }
func (c SimpleCard) IsPlaneswalker() bool { return contains(c.TypeLine, "Planeswalker") }
func (c SimpleCard) IsLegendary() bool    { return contains(c.TypeLine, "Legendary") }
func (c SimpleCard) IsBattle() bool       { return contains(c.TypeLine, "Battle") }

// GetManaCost parses the mana cost string into a Mana map.
func (c SimpleCard) GetManaCost() Mana {
	return parseManaCost(c.ManaCost)
}

// IsCounterspell returns true if this card is a counterspell (instant that counters).
// Matches standard patterns: "counter target spell", "counter target creature spell",
// "counter target noncreature spell", and similar variants.
func (c SimpleCard) IsCounterspell() bool {
	if !c.IsInstant() {
		return false
	}
	lower := strings.ToLower(c.OracleText)
	if strings.Contains(lower, "counter target") && strings.Contains(lower, "spell") {
		return true
	}
	// Also match "counter <qualifier> spell" patterns like "counter target activated or triggered ability"
	if strings.HasPrefix(lower, "counter") && strings.Contains(lower, "spell") {
		return true
	}
	return false
}

// HasAlternateCosts returns true if the card has alternate casting costs (e.g., "{U}{B} or {3}{B}").
func (c SimpleCard) HasAlternateCosts() bool {
	return strings.Contains(c.ManaCost, " or ")
}

// GetAlternateCosts returns a slice of all available mana cost options for this card.
// If the card has alternate costs, returns each option; otherwise returns just the main cost.
func (c SimpleCard) GetAlternateCosts() []Mana {
	if !c.HasAlternateCosts() {
		return []Mana{c.GetManaCost()}
	}
	// Split by " or " and parse each cost option
	costParts := strings.Split(c.ManaCost, " or ")
	var costs []Mana
	for _, part := range costParts {
		part = strings.TrimSpace(part)
		costs = append(costs, parseManaCost(part))
	}
	return costs
}

// GetMinManaCost returns the mana cost with the lowest total value.
// Useful for prioritizing the cheapest casting option.
func (c SimpleCard) GetMinManaCost() Mana {
	costs := c.GetAlternateCosts()
	if len(costs) == 0 {
		return c.GetManaCost()
	}
	minCost := costs[0]
	minTotal := minCost.Total()
	for _, cost := range costs[1:] {
		if cost.Total() < minTotal {
			minCost = cost
			minTotal = cost.Total()
		}
	}
	return minCost
}

// contains is a simple substring checker (ASCII)
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// NewClueToken creates a colorless Clue artifact token (CR 111.1).
// {2}, Sacrifice this artifact: Draw a card.
func NewClueToken() SimpleCard {
	return SimpleCard{
		Name:     "Clue",
		TypeLine: "Artifact — Clue",
		OracleText: "{2}, Sacrifice this artifact: Draw a card.",
	}
}

// NewTreasureToken creates a colorless Treasure artifact token (CR 111.1).
// {T}, Sacrifice this artifact: Add one mana of any color.
func NewTreasureToken() SimpleCard {
	return SimpleCard{
		Name:     "Treasure",
		TypeLine: "Artifact — Treasure",
		OracleText: "{T}, Sacrifice this artifact: Add one mana of any color.",
	}
}

// NewFoodToken creates a colorless Food artifact token (CR 111.1).
// {2}, {T}, Sacrifice this artifact: You gain 3 life.
func NewFoodToken() SimpleCard {
	return SimpleCard{
		Name:     "Food",
		TypeLine: "Artifact — Food",
		OracleText: "{2}, {T}, Sacrifice this artifact: You gain 3 life.",
	}
}
