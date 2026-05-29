package dashboard

import (
	"sort"

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

// GenerateDeckRecommendations analyzes a deck and provides improvement suggestions.
func GenerateDeckRecommendations(deckStats *simulation.EDHDeckStats, cardLib map[string]stats.GlobalCardStats) DeckRecommendations {
	recs := DeckRecommendations{
		DeckName:      deckStats.DeckName,
		CommanderName: deckStats.CommanderName,
		Cards:         []CardRecommendation{},
		RemoveCandidates: []CardRecommendation{},
		AddCandidates: []CardRecommendation{},
	}

	if deckStats.CardStats == nil {
		return recs
	}

	// Analyze each card in the deck
	for cardName, perf := range deckStats.CardStats {
		winRate := 0.0
		if perf.Casts > 0 {
			winRate = (float64(perf.Wins) / float64(perf.Casts)) * 100
		}

		// Low-performance cards (below 40% win rate with 5+ casts)
		if perf.Casts >= 5 && winRate < 40 {
			recs.RemoveCandidates = append(recs.RemoveCandidates, CardRecommendation{
				Action:   "remove",
				CardName: cardName,
				Reason:   "Low win rate in this deck",
				Impact:   "Removing could improve deck consistency",
				WinRate:  winRate,
				Casts:    perf.Casts,
				Priority: 1,
			})
		}

		// Good performers to keep (60%+ win rate)
		if perf.Casts >= 3 && winRate >= 60 {
			recs.Cards = append(recs.Cards, CardRecommendation{
				Action:   "keep",
				CardName: cardName,
				Reason:   "High win rate performer",
				Impact:   "Core deck card",
				WinRate:  winRate,
				Casts:    perf.Casts,
				Priority: 1,
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

			// Suggest high performers that aren't in deck
			if globalStats.Casts >= 10 && globalWinRate >= 55 {
				recs.AddCandidates = append(recs.AddCandidates, CardRecommendation{
					Action:   "test",
					CardName: cardName,
					Reason:   "Consistently performs well globally",
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
func GenerateSideboardSuggestions(deckStats *simulation.EDHDeckStats, allDecks []simulation.EDHDeckStats, cardLib map[string]stats.GlobalCardStats) []SideboardSuggestion {
	suggs := []SideboardSuggestion{}

	if deckStats.CardStats == nil || deckStats.Wins == 0 {
		return suggs
	}

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
