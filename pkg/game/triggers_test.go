package game

import "testing"

func TestTrigger_ETBDraw(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	// Load one card in library and a creature in hand
	drawCard := SimpleCard{Name: "Cantrip", TypeLine: "Sorcery"}
	p1.Library = append(p1.Library, drawCard)
	creature := SimpleCard{Name: "Elf", TypeLine: "Creature", Power: "1", Toughness: "1"}
	p1.Hand = append(p1.Hand, creature)

	// Register ETB trigger: controller draws 1
	g.AddTrigger(&Trigger{On: EventEntersBattlefield, Action: func(g *Game, e Event) {
		if e.ZoneChange != nil && e.ZoneChange.Permanent != nil {
			e.ZoneChange.Permanent.GetController().Draw(1)
		}
	}})

	// Summon creature via Game wrapper to emit ETB
	beforeHand := len(p1.Hand)
	if _, err := g.SummonCreature(p1, creature.Name); err != nil {
		t.Fatalf("summon creature: %v", err)
	}

	// Hand should be unchanged overall (spent 1, drew 1)
	if len(p1.Hand) != beforeHand {
		t.Fatalf("expected hand size %d, got %d", beforeHand, len(p1.Hand))
	}
}
