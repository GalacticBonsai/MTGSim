package game

import "testing"

func TestEvents_ETB_LTB_ZoneChange(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	// prepare a simple creature in hand
	c := SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
	p1.Hand = append(p1.Hand, c)

	var seen []Event
	g.AddListener(func(e Event) { seen = append(seen, e) })

	perm, err := g.SummonCreature(p1, "Bear")
	if err != nil {
		t.Fatalf("summon creature: %v", err)
	}

	// ETB fired
	if len(seen) == 0 || seen[len(seen)-1].Type != EventEntersBattlefield {
		t.Fatalf("expected ETB event, got %+v", seen)
	}

	// destroy the permanent
	ok := g.DestroyPermanent(perm)
	if !ok {
		t.Fatalf("expected destroy to succeed")
	}

	// Expect LTB followed by ZoneChange to Graveyard
	if len(seen) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(seen))
	}
	if seen[len(seen)-2].Type != EventLeavesBattlefield {
		t.Fatalf("expected LTB as second last event, got %v", seen[len(seen)-2].Type)
	}
	last := seen[len(seen)-1]
	if last.Type != EventZoneChange || last.ZoneChange.To != Graveyard {
		t.Fatalf("expected ZoneChange to Graveyard, got %+v", last)
	}
	if last.ZoneChange.LKI == nil || last.ZoneChange.LKI.Toughness != 2 {
		t.Fatalf("expected LKI snapshot with toughness 2, got %+v", last.ZoneChange.LKI)
	}
}

func TestEvents_GraveyardToExile(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	// put a card in graveyard
	card := SimpleCard{Name: "Spell", TypeLine: "Instant"}
	p1.Graveyard = append(p1.Graveyard, card)

	var last Event
	g.AddListener(func(e Event) { last = e })

	ok := g.ExileFromGraveyard(p1, "Spell")
	if !ok {
		t.Fatalf("expected exile to succeed")
	}
	if last.Type != EventZoneChange || last.ZoneChange.From != Graveyard || last.ZoneChange.To != Exile {
		t.Fatalf("expected ZoneChange Graveyard->Exile, got %+v", last)
	}
}
