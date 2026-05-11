package combo

// Index maps cards to the combo variants they participate in for a specific deck.
type Index struct {
	// card name -> list of variant IDs this card is part of.
	CardToVariants map[string][]string
	// variant ID -> distilled info.
	Variants map[string]VariantInfo
	// Deck card names for quick membership tests.
	DeckCards map[string]bool
}

// VariantInfo holds distilled combo information for AI use.
type VariantInfo struct {
	ID           string
	CardNames    []string
	MissingCards []string // empty if fully included
	IsIncluded   bool
	Description  string
}

// NewIndex builds a combo index from a Commander Spellbook result and a decklist.
func NewIndex(result *FindMyCombosResult, deckCards []string) *Index {
	idx := &Index{
		CardToVariants: make(map[string][]string),
		Variants:       make(map[string]VariantInfo),
		DeckCards:      make(map[string]bool),
	}
	for _, c := range deckCards {
		idx.DeckCards[c] = true
	}

	for _, v := range result.Included {
		idx.addVariant(v, true)
	}
	for _, v := range result.AlmostIncluded {
		idx.addVariant(v, false)
	}
	return idx
}

func (idx *Index) addVariant(v Variant, included bool) {
	names := make([]string, 0, len(v.Uses))
	for _, u := range v.Uses {
		names = append(names, u.Card.Name)
		idx.CardToVariants[u.Card.Name] = append(idx.CardToVariants[u.Card.Name], v.ID)
	}

	missing := []string{}
	if !included {
		for _, u := range v.Uses {
			if !idx.DeckCards[u.Card.Name] {
				missing = append(missing, u.Card.Name)
			}
		}
	}

	idx.Variants[v.ID] = VariantInfo{
		ID:           v.ID,
		CardNames:    names,
		MissingCards: missing,
		IsIncluded:   included,
		Description:  v.Description,
	}
}

// IsComboPiece returns true if the card is part of any known combo variant.
func (idx *Index) IsComboPiece(cardName string) bool {
	_, ok := idx.CardToVariants[cardName]
	return ok
}

// IsMissingPiece returns true if the given card would complete an almost-included combo.
func (idx *Index) IsMissingPiece(cardName string) bool {
	for _, v := range idx.Variants {
		if !v.IsIncluded {
			for _, m := range v.MissingCards {
				if m == cardName {
					return true
				}
			}
		}
	}
	return false
}

// MissingPieces returns all card names that would complete almost-included combos.
func (idx *Index) MissingPieces() []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range idx.Variants {
		if !v.IsIncluded {
			for _, m := range v.MissingCards {
				if !seen[m] {
					seen[m] = true
					out = append(out, m)
				}
			}
		}
	}
	return out
}

// AlmostCompleteVariants returns variants that are missing the fewest cards.
func (idx *Index) AlmostCompleteVariants() []VariantInfo {
	var out []VariantInfo
	for _, v := range idx.Variants {
		if !v.IsIncluded && len(v.MissingCards) > 0 {
			out = append(out, v)
		}
	}
	return out
}

// ComboPiecesInHand returns the subset of hand cards that are combo pieces.
func (idx *Index) ComboPiecesInHand(hand []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, c := range hand {
		if idx.IsComboPiece(c) && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// MissingPiecesForHand returns missing combo pieces that could be tutored given the current hand.
func (idx *Index) MissingPiecesForHand(hand []string) []string {
	handSet := make(map[string]bool)
	for _, c := range hand {
		handSet[c] = true
	}
	seen := make(map[string]bool)
	var out []string
	for _, v := range idx.Variants {
		if v.IsIncluded {
			continue
		}
		// A variant is "relevant" if at least one of its cards is in hand.
		hasPiece := false
		for _, c := range v.CardNames {
			if handSet[c] {
				hasPiece = true
				break
			}
		}
		if !hasPiece {
			continue
		}
		for _, m := range v.MissingCards {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	return out
}
