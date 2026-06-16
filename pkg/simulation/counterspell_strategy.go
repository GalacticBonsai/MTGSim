package simulation

import (
	"strings"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// CounterspellStrategy provides decision logic for when to cast counterspells.
// This enables intelligent opponent responses when using a PriorityHandler.
type CounterspellStrategy struct {
	player *game.Player
}

// NewCounterspellStrategy creates a new strategy for the given player.
func NewCounterspellStrategy(player *game.Player) *CounterspellStrategy {
	return &CounterspellStrategy{
		player: player,
	}
}

// isHighPriorityCounterTarget returns true for card names that represent
// combo pieces, game-winning threats, or otherwise high-priority counterspell
// targets regardless of CMC.
func isHighPriorityCounterTarget(name string) bool {
	lower := strings.ToLower(name)
	highPriority := []string{
		"torment of hailfire", "expropriate", "craterhoof behemoth",
		"approach of the second sun", "insurrection", "debt to the deathless",
		"jace, wielder of mysteries", "thassa's oracle", "laboratory maniac",
		"ad nauseam", "necropotence", "doomsday", "tendrils of agony",
		"brain freeze", "demonic consultation", "tainted pact",
		"hermit druid", "food chain", "protean hulk", "flash",
		"omniscience", "enter the infinite", "dark ritual",
		"cabal ritual", "rite of flames", "seething song",
		"finale of devastation", "green sun's zenith",
		"tooth and nail", "time warp", "temporal manipulation",
		"walk the aeons", "nexus of fate", "ugin, the spirit dragon",
		"cyclonic rift", "toxic deluge", "farewell",
	}
	for _, hp := range highPriority {
		if strings.Contains(lower, hp) {
			return true
		}
	}
	return false
}

// isTutorEffect returns true if the card name suggests a tutor or search effect.
func isTutorEffect(name string) bool {
	lower := strings.ToLower(name)
	tutors := []string{
		"tutor", "demonic", "vampiric", "enlightened", "mystical",
		"worldly", "survival", "entomb", "buried alive",
		"gamble", "imperial seal", "grim tutor", "diabolic",
		"increasing ambition", "beseech the queen",
	}
	for _, t := range tutors {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// ShouldCounterSpell determines if the player should cast a counterspell in response
// to an opponent's spell cast. This checks:
// 1. Does the player have a counterspell in hand?
// 2. Can the player afford to cast it?
// 3. Is the opponent's spell worth countering (higher casting cost than our counterspell,
//    or a known high-priority threat like a combo piece / tutor)?
//
// This returns true if counter action is recommended, along with the counterspell card to use.
func (cs *CounterspellStrategy) ShouldCounterSpell(opponentSpellName string, opponentSpellCMC int) (bool, game.SimpleCard) {
	// Find available counterspells in hand
	var counterspells []game.SimpleCard
	for _, c := range cs.player.Hand {
		if c.IsCounterspell() {
			counterspells = append(counterspells, c)
		}
	}

	if len(counterspells) == 0 {
		return false, game.SimpleCard{}
	}

	// Find the most efficient counterspell we can afford
	var bestCounter game.SimpleCard
	bestCost := 999
	canAffordAny := false

	for _, counter := range counterspells {
		if cs.player.CanPayForCard(counter) {
			canAffordAny = true
			costTotal := counter.GetMinManaCost().Total()
			if costTotal < bestCost {
				bestCost = costTotal
				bestCounter = counter
			}
		}
	}

	if !canAffordAny {
		return false, game.SimpleCard{}
	}

	// Always counter known high-priority targets (combo pieces, game-winners)
	if isHighPriorityCounterTarget(opponentSpellName) {
		return true, bestCounter
	}

	// Always counter tutors — they represent hidden card advantage / combo assembly
	if isTutorEffect(opponentSpellName) {
		return true, bestCounter
	}

	// Counter is worth casting if opponent's spell costs more than our counter.
	// This represents smart threat assessment: counter bigger threats with cheap counters.
	// Use > not >= to avoid counterspell wars over marginal threats.
	shouldCounter := opponentSpellCMC > bestCounter.GetMinManaCost().Total()

	return shouldCounter, bestCounter
}

// GetCounterspellsInHand returns all counterspells currently in the player's hand.
func (cs *CounterspellStrategy) GetCounterspellsInHand() []game.SimpleCard {
	var counterspells []game.SimpleCard
	for _, c := range cs.player.Hand {
		if c.IsCounterspell() {
			counterspells = append(counterspells, c)
		}
	}
	return counterspells
}

// HasCounterableMana checks if the player has enough mana to cast at least one counterspell.
func (cs *CounterspellStrategy) HasCounterableMana() bool {
	for _, c := range cs.player.Hand {
		if c.IsCounterspell() && cs.player.CanPayForCard(c) {
			return true
		}
	}
	return false
}

// GetCheapestCounterspell returns the counterspell with the lowest mana cost
// that the player can currently afford.
func (cs *CounterspellStrategy) GetCheapestCounterspell() (game.SimpleCard, bool) {
	var cheapest game.SimpleCard
	minCost := 999
	found := false

	for _, c := range cs.player.Hand {
		if c.IsCounterspell() && cs.player.CanPayForCard(c) {
			cost := c.GetMinManaCost().Total()
			if cost < minCost {
				minCost = cost
				cheapest = c
				found = true
			}
		}
	}

	return cheapest, found
}

// CounterspellPriorityHandler is a priority handler that responds to opponent spells
// with counterspells. This demonstrates how the CounterspellStrategy can be integrated
// into the simulation's priority system.
//
// To use this, pass it to EDHRunOptions.Priority:
//   opts := EDHRunOptions{
//       Seats: [...],
//       Priority: &CounterspellPriorityHandler{},
//   }
//
// Note: The current simplified simulation runner doesn't track spells on the stack,
// so this handler is primarily useful for future implementations that integrate
// the full ability/stack system from pkg/ability.
type CounterspellPriorityHandler struct{}

// OnOpponentPriority is called when an opponent receives priority.
// In a full implementation with the stack, this would detect spells and respond with counters.
func (h *CounterspellPriorityHandler) OnOpponentPriority(g *game.Game, active *game.Player, opp *game.Player, phase game.Phase) {
	// This is a placeholder for future integration with the full stack system.
	// When the simulation supports casting spells at instant speed with the stack,
	// this method can be enhanced to:
	// 1. Inspect the stack for spells cast by the opponent
	// 2. Use CounterspellStrategy to decide if we should counter
	// 3. Actually cast the counterspell via the ability/stack system
	//
	// Currently, the simplified runner doesn't use this capability.
}
