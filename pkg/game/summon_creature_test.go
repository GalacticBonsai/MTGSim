package game

import "testing"

// payFromPoolMap deducts cost from a player's manaPool map with the same rules
// as ManaPool.Pay: specific first, then Any from W,U,B,R,G,C in order.
func payFromPoolMap(pool map[ManaType]int, cost Mana) bool {
	// check specific
	specific := []ManaType{White, Blue, Black, Red, Green, Colorless}
	for _, t := range specific {
		need := cost.Get(t)
		if need == 0 {
			continue
		}
		if pool[t] < need {
			return false
		}
	}
	// deduct specific
	for _, t := range specific {
		need := cost.Get(t)
		if need == 0 {
			continue
		}
		pool[t] -= need
	}
	// deduct Any greedily
	anyNeed := cost.Get(Any)
	for anyNeed > 0 {
		progress := false
		for _, t := range specific {
			if anyNeed == 0 {
				break
			}
			if pool[t] > 0 {
				pool[t]--
				anyNeed--
				progress = true
			}
		}
		if !progress {
			return false
		}
	}
	return true
}

func TestSummonCreatureWorkflow(t *testing.T) {
	// 1) Game state with creature in hand and sufficient mana
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)

	bear := SimpleCard{
		Name:       "Grizzly Bears",
		TypeLine:   "Creature — Bear",
		Power:      "2",
		Toughness:  "2",
		OracleText: "",
	}
	p1.AddCardToHand(bear)
	if len(p1.Hand) != 1 {
		t.Fatalf("expected hand size 1, got %d", len(p1.Hand))
	}
	// pool: {G}{1}
	p1.AddManaToPool(Green, 1)
	p1.AddManaToPool(Colorless, 1)

	// 2) Pay cost {1}{G} (engine summoning itself doesn't handle costs)
	cost := Mana{Green: 1, Any: 1}
	pool := p1.GetManaPool()
	if ok := payFromPoolMap(pool, cost); !ok {
		t.Fatalf("expected to be able to pay {1}{G} from pool, pool=%v", pool)
	}

	// 3) Call the game-level summon wrapper
	perm, err := g.SummonCreature(p1, "Grizzly Bears")
	if err != nil {
		t.Fatalf("SummonCreature returned error: %v", err)
	}
	if perm == nil {
		t.Fatalf("expected non-nil permanent")
	}

	// 4) Assert card removed from hand
	if len(p1.Hand) != 0 {
		t.Fatalf("expected hand size 0 after summoning, got %d", len(p1.Hand))
	}
	if p1.FindCardInHand("Grizzly Bears") >= 0 {
		t.Fatalf("card should be removed from hand")
	}

	// 5) Assert permanent on battlefield with correct attributes
	if len(p1.Battlefield) != 1 {
		t.Fatalf("expected 1 permanent on battlefield, got %d", len(p1.Battlefield))
	}
	bf := p1.Battlefield[0]
	if bf.GetName() != "Grizzly Bears" {
		t.Fatalf("permanent name mismatch: got %q", bf.GetName())
	}
	if bf.GetPower() != 2 || bf.GetToughness() != 2 {
		t.Fatalf("expected 2/2, got %d/%d", bf.GetPower(), bf.GetToughness())
	}
	if bf.GetOwner() != p1 || bf.GetController() != p1 {
		t.Fatalf("owner/controller mismatch")
	}
	if bf.IsTapped() {
		t.Fatalf("newly summoned creature should enter untapped in this simplified engine")
	}

	// 6) Assert mana cost was deducted from player's mana pool
	if pool[Green] != 0 {
		t.Fatalf("expected G to be 0 after payment, got %d", pool[Green])
	}
	if pool[Colorless] != 0 {
		t.Fatalf("expected colorless to be 0 after paying generic, got %d", pool[Colorless])
	}
}
