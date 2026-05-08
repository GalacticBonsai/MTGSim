package simulation

import (
	"math/rand"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestExpandSideboardVariants_AddsOriginalPlusVariants(t *testing.T) {
	seat := EDHSeat{
		DeckName: "Deck",
		Library: []game.SimpleCard{
			{Name: "Forest", TypeLine: "Basic Land — Forest"},
			{Name: "Bear", TypeLine: "Creature", ManaCost: "{G}"},
			{Name: "Elf", TypeLine: "Creature", ManaCost: "{G}"},
		},
		Sideboard: []game.SimpleCard{
			{Name: "Naturalize", TypeLine: "Instant", ManaCost: "{1}{G}"},
			{Name: "Plains", TypeLine: "Basic Land — Plains"},
		},
	}
	got := ExpandSideboardVariants([]EDHSeat{seat}, SideboardVariantOptions{
		VariantsPerDeck: 2,
		SwapsPerVariant: 1,
		RNG:             rand.New(rand.NewSource(1)),
	})
	if len(got) != 3 {
		t.Fatalf("expected original + 2 variants, got %d", len(got))
	}
	if got[0].DeckName != "Deck" || got[0].Library[1].Name != "Bear" {
		t.Fatalf("original seat should be preserved, got %+v", got[0])
	}
	for _, v := range got[1:] {
		if len(v.Library) != len(seat.Library) {
			t.Fatalf("variant changed library size: %+v", v)
		}
		if v.DeckName == seat.DeckName {
			t.Fatalf("variant deck name should be annotated")
		}
	}
}

func TestExpandSideboardVariants_DisabledReturnsCopy(t *testing.T) {
	seat := EDHSeat{DeckName: "Deck", Library: []game.SimpleCard{{Name: "A"}}, Sideboard: []game.SimpleCard{{Name: "B"}}}
	got := ExpandSideboardVariants([]EDHSeat{seat}, SideboardVariantOptions{})
	if len(got) != 1 || got[0].DeckName != "Deck" {
		t.Fatalf("unexpected disabled variants: %+v", got)
	}
	got[0].Library[0].Name = "mutated"
	if seat.Library[0].Name != "A" {
		t.Fatalf("ExpandSideboardVariants should return cloned seats")
	}
}
