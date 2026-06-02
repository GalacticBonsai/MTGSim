package dashboard

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/simulation"
	"github.com/mtgsim/mtgsim/pkg/stats"
)

// CardRecommendation represents a suggested card change for a deck.
type CardRecommendation struct {
	Action      string  `json:"action"` // "remove", "add", "swap", "test"
	CardName    string  `json:"card_name"`
	Reason      string  `json:"reason"`
	Impact      string  `json:"impact"` // estimated impact description
	WinRate     float64 `json:"win_rate"`
	Casts       int     `json:"casts"`
	Priority    int     `json:"priority"` // 1=high, 2=medium, 3=low
}

// DeckRecommendations contains suggested improvements for a deck.
type DeckRecommendations struct {
	DeckName        string                 `json:"deck_name"`
	CommanderName   string                 `json:"commander_name"`
	Cards           []CardRecommendation   `json:"cards"`
	RemoveCandidates []CardRecommendation   `json:"remove_candidates"`
	AddCandidates   []CardRecommendation   `json:"add_candidates"`
	SideboardSuggs  []SideboardSuggestion  `json:"sideboard_suggestions"`
}

// SideboardSuggestion represents a sideboard swap recommendation.
type SideboardSuggestion struct {
	Remove string  `json:"remove"`
	Add    string  `json:"add"`
	Reason string  `json:"reason"`
	Impact string  `json:"impact"`
	Priority int   `json:"priority"`
}

// MatchupResult represents a deck matchup performance.
type MatchupResult struct {
	Deck1      string  `json:"deck1"`
	Deck2      string  `json:"deck2"`
	Deck1Wins  int     `json:"deck1_wins"`
	Deck2Wins  int     `json:"deck2_wins"`
	WinRate1   float64 `json:"win_rate1"`
	WinRate2   float64 `json:"win_rate2"`
	Games      int     `json:"games"`
}

// getDeckColorIdentity returns the combined color identity of a deck's commander(s).
// It parses the CommanderName field (which may contain multiple partners separated by " / ")
// and looks each up in the card database. Returns nil when the DB is unavailable or no
// commander is found, which signals the caller to skip color-based filtering.
func getDeckColorIdentity(deckStats *simulation.EDHDeckStats, cardDB *card.CardDB) []string {
	if cardDB == nil || deckStats.CommanderName == "" {
		return nil
	}
	identitySet := make(map[string]bool)
	for _, name := range strings.Split(deckStats.CommanderName, " / ") {
		name = strings.TrimSpace(name)
		if c, ok := cardDB.GetCardByName(name); ok {
			for _, col := range c.ColorIdentity {
				identitySet[col] = true
			}
		}
	}
	if len(identitySet) == 0 {
		return nil
	}
	identity := make([]string, 0, len(identitySet))
	for col := range identitySet {
		identity = append(identity, col)
	}
	return identity
}

// isColorIdentitySubset reports whether a card's colors are legal inside the
// given commander color identity. An empty card identity (colorless) is always
// legal; an empty commander identity (e.g. unknown commander) causes the
// function to return true so we don't over-filter.
func isColorIdentitySubset(cardColors, commanderColors []string) bool {
	if len(commanderColors) == 0 {
		return true // don't know the commander, allow everything
	}
	if len(cardColors) == 0 {
		return true // colorless cards fit everywhere
	}
	cmdrSet := make(map[string]bool, len(commanderColors))
	for _, c := range commanderColors {
		cmdrSet[c] = true
	}
	for _, c := range cardColors {
		if !cmdrSet[c] {
			return false
		}
	}
	return true
}

// GenerateDeckRecommendations analyzes a deck and provides improvement suggestions.
func GenerateDeckRecommendations(deckStats *simulation.EDHDeckStats, cardLib map[string]stats.GlobalCardStats, cardDB *card.CardDB) DeckRecommendations {
	recs := DeckRecommendations{
		DeckName:         deckStats.DeckName,
		CommanderName:    deckStats.CommanderName,
		Cards:            []CardRecommendation{},
		RemoveCandidates: []CardRecommendation{},
		AddCandidates:    []CardRecommendation{},
	}

	if deckStats.CardStats == nil {
		return recs
	}

	cmdrIdentity := getDeckColorIdentity(deckStats, cardDB)
	deckWinRate := deckStats.WinRate

	// Analyze each card in the deck — compare performance to the deck's own average
	for cardName, perf := range deckStats.CardStats {
		winRate := 0.0
		if perf.Casts > 0 {
			winRate = (float64(perf.Wins) / float64(perf.Casts)) * 100
		}

		// Remove candidates: cards that drag the deck down (significantly below deck average)
		if perf.Casts >= 5 && winRate < deckWinRate-10 {
			delta := deckWinRate - winRate
			priority := 2
			if delta > 20 {
				priority = 1
			}
			recs.RemoveCandidates = append(recs.RemoveCandidates, CardRecommendation{
				Action:   "remove",
				CardName: cardName,
				Reason:   fmt.Sprintf("%.1f%% win rate is %.1f%% below deck average (%.1f%%)", winRate, delta, deckWinRate),
				Impact:   "Removing could bring the deck closer to its average performance",
				WinRate:  winRate,
				Casts:    perf.Casts,
				Priority: priority,
			})
		}

		// Keep candidates: cards that lift the deck up (significantly above deck average)
		if perf.Casts >= 3 && winRate > deckWinRate+5 {
			delta := winRate - deckWinRate
			priority := 2
			if delta > 15 {
				priority = 1
			}
			recs.Cards = append(recs.Cards, CardRecommendation{
				Action:   "keep",
				CardName: cardName,
				Reason:   fmt.Sprintf("%.1f%% win rate is %.1f%% above deck average (%.1f%%)", winRate, delta, deckWinRate),
				Impact:   "Core card that lifts the deck above its baseline",
				WinRate:  winRate,
				Casts:    perf.Casts,
				Priority: priority,
			})
		}
	}

	// Find high-performing cards globally that aren't in this deck
	for cardName, globalStats := range cardLib {
		if _, inDeck := deckStats.CardStats[cardName]; !inDeck {
			globalWinRate := 0.0
			if globalStats.Casts > 0 {
				globalWinRate = (float64(globalStats.Wins) / float64(globalStats.Casts)) * 100
			}

			// Suggest cards that outperform this deck's average (with a floor so we don't suggest garbage)
			if globalStats.Casts >= 10 && globalWinRate > deckWinRate+5 && globalWinRate >= 35 {
				// Filter out cards that violate the commander's color identity
				if cardDB != nil {
					c, ok := cardDB.GetCardByName(cardName)
					if !ok {
						continue // skip cards we can't verify color identity for
					}
					if !isColorIdentitySubset(c.ColorIdentity, cmdrIdentity) {
						continue
					}
				}
				delta := globalWinRate - deckWinRate
				recs.AddCandidates = append(recs.AddCandidates, CardRecommendation{
					Action:   "test",
					CardName: cardName,
					Reason:   fmt.Sprintf("Global win rate %.1f%% is %.1f%% above this deck's %.1f%%", globalWinRate, delta, deckWinRate),
					Impact:   "Testing could improve deck power level",
					WinRate:  globalWinRate,
					Casts:    globalStats.Casts,
					Priority: 2,
				})
			}
		}
	}

	// Sort by priority and impact
	sort.Slice(recs.RemoveCandidates, func(i, j int) bool {
		if recs.RemoveCandidates[i].Priority != recs.RemoveCandidates[j].Priority {
			return recs.RemoveCandidates[i].Priority < recs.RemoveCandidates[j].Priority
		}
		return recs.RemoveCandidates[i].WinRate < recs.RemoveCandidates[j].WinRate
	})

	sort.Slice(recs.AddCandidates, func(i, j int) bool {
		if recs.AddCandidates[i].Priority != recs.AddCandidates[j].Priority {
			return recs.AddCandidates[i].Priority < recs.AddCandidates[j].Priority
		}
		return recs.AddCandidates[i].WinRate > recs.AddCandidates[j].WinRate
	})

	return recs
}

// GenerateSideboardSuggestions analyzes matchup performance and suggests sideboard swaps.
func GenerateSideboardSuggestions(deckStats *simulation.EDHDeckStats, allDecks []simulation.EDHDeckStats, cardLib map[string]stats.GlobalCardStats, cardDB *card.CardDB) []SideboardSuggestion {
	suggs := []SideboardSuggestion{}

	if deckStats.CardStats == nil || deckStats.Wins == 0 {
		return suggs
	}

	cmdrIdentity := getDeckColorIdentity(deckStats, cardDB)

	// Calculate win rate against specific archetypes/decks
	// For now, we'll suggest swaps based on global card performance variations
	poorPerformers := []string{}
	for cardName, perf := range deckStats.CardStats {
		if perf.Casts >= 3 {
			winRate := (float64(perf.Wins) / float64(perf.Casts)) * 100
			if winRate < 35 {
				poorPerformers = append(poorPerformers, cardName)
			}
		}
	}

	// Suggest replacements for poor performers
	for _, cardName := range poorPerformers {
		for replName, replStats := range cardLib {
			if replStats.Casts >= 15 {
				replWR := (float64(replStats.Wins) / float64(replStats.Casts)) * 100
				if replWR > 50 {
					// Filter out replacements that violate the commander's color identity
					if cardDB != nil {
						c, ok := cardDB.GetCardByName(replName)
						if !ok {
							continue // skip cards we can't verify color identity for
						}
						if !isColorIdentitySubset(c.ColorIdentity, cmdrIdentity) {
							continue
						}
					}
					suggs = append(suggs, SideboardSuggestion{
						Remove:   cardName,
						Add:      replName,
						Reason:   "Replace underperformer with proven card",
						Impact:   "Could gain +5-10% win rate",
						Priority: 1,
					})
					break // Only suggest one replacement per poor performer
				}
			}
		}
	}

	return suggs
}
