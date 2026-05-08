package simulation

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// SideboardVariantOptions controls automatic sideboard variant generation.
type SideboardVariantOptions struct {
	VariantsPerDeck int
	SwapsPerVariant int
	RNG             *rand.Rand
}

// ExpandSideboardVariants returns the original seats plus generated variants
// that preserve library size and commander assignment while swapping cards
// from each deck's sideboard into the main library. DeckName is annotated so
// aggregate stats compare the original list against each variant.
func ExpandSideboardVariants(seats []EDHSeat, opts SideboardVariantOptions) []EDHSeat {
	out := cloneSeats(seats)
	if opts.VariantsPerDeck <= 0 || opts.SwapsPerVariant <= 0 {
		return out
	}
	rng := opts.RNG
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	for _, seat := range seats {
		if len(seat.Sideboard) == 0 || len(seat.Library) == 0 {
			continue
		}
		for i := 1; i <= opts.VariantsPerDeck; i++ {
			v := cloneSeat(seat)
			ins, outs := applySideboardSwaps(&v, opts.SwapsPerVariant, rng)
			if len(ins) == 0 {
				continue
			}
			v.DeckName = fmt.Sprintf("%s [SB v%d: +%s / -%s]", seat.DeckName, i, strings.Join(ins, ","), strings.Join(outs, ","))
			out = append(out, v)
		}
	}
	return out
}

func applySideboardSwaps(seat *EDHSeat, maxSwaps int, rng *rand.Rand) (ins []string, outs []string) {
	usedMain := map[int]bool{}
	for _, sbIdx := range rng.Perm(len(seat.Sideboard)) {
		if len(ins) >= maxSwaps {
			break
		}
		sb := seat.Sideboard[sbIdx]
		mainIdx := chooseMainSwapIndex(seat.Library, sb, usedMain, rng)
		if mainIdx < 0 {
			continue
		}
		usedMain[mainIdx] = true
		outs = append(outs, seat.Library[mainIdx].Name)
		ins = append(ins, sb.Name)
		seat.Library[mainIdx] = sb
	}
	return ins, outs
}

func chooseMainSwapIndex(library []game.SimpleCard, side game.SimpleCard, used map[int]bool, rng *rand.Rand) int {
	preferred := candidateIndexes(library, used, func(c game.SimpleCard) bool { return c.IsLand() == side.IsLand() })
	if len(preferred) == 0 {
		preferred = candidateIndexes(library, used, func(game.SimpleCard) bool { return true })
	}
	if len(preferred) == 0 {
		return -1
	}
	return preferred[rng.Intn(len(preferred))]
}

func candidateIndexes(library []game.SimpleCard, used map[int]bool, keep func(game.SimpleCard) bool) []int {
	out := []int{}
	for i, c := range library {
		if used[i] || !keep(c) {
			continue
		}
		out = append(out, i)
	}
	return out
}

func cloneSeats(seats []EDHSeat) []EDHSeat {
	out := make([]EDHSeat, len(seats))
	for i := range seats {
		out[i] = cloneSeat(seats[i])
	}
	return out
}

func cloneSeat(s EDHSeat) EDHSeat {
	out := s
	out.Library = append([]game.SimpleCard(nil), s.Library...)
	out.Sideboard = append([]game.SimpleCard(nil), s.Sideboard...)
	if s.Commander != nil {
		c := *s.Commander
		out.Commander = &c
	}
	return out
}
