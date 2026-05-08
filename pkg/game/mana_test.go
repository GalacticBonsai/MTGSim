package game

import "testing"

func TestManaPool_CanPayAndPay(t *testing.T) {
	mp := NewManaPool()
	mp.Add(White, 1)
	mp.Add(Blue, 1)
	mp.Add(Colorless, 2)

	cost := Mana{White: 1, Any: 2}
	if !mp.CanPay(cost) {
		t.Fatalf("expected CanPay to be true")
	}
	if !mp.Pay(cost) {
		t.Fatalf("expected Pay to succeed")
	}
	if mp.Get(White) != 0 {
		t.Fatalf("expected white to be 0, got %d", mp.Get(White))
	}
	// Any should have consumed 2 from remaining pool (prefer colorless in our greedy order)
	// Our Pay prefers specific order W,U,B,R,G,C. After spending White, we deduct Any from W,U,B,R,G,C order.
	// Remaining were Blue=1, Colorless=2, so Any drains Blue then Colorless.
	if mp.Get(Blue) != 0 || mp.Get(Colorless) != 1 {
		t.Fatalf("unexpected pool after pay: U=%d C=%d", mp.Get(Blue), mp.Get(Colorless))
	}
}

func TestManaPool_CannotPay(t *testing.T) {
	mp := NewManaPool()
	mp.Add(Red, 1)

	cost := Mana{Green: 1}
	if mp.CanPay(cost) {
		t.Fatalf("expected CanPay to be false")
	}
	if mp.Pay(cost) {
		t.Fatalf("expected Pay to fail")
	}
}

func TestSimpleCard_GetManaCost_ParsesScryfallCost(t *testing.T) {
	c := SimpleCard{Name: "Charm", ManaCost: "{2}{W}{U}{C}{X}"}
	cost := c.GetManaCost()
	if cost[Any] != 2 || cost[White] != 1 || cost[Blue] != 1 || cost[Colorless] != 1 || cost[X] != 1 {
		t.Fatalf("unexpected parsed cost: %+v", cost)
	}
}

func TestAdvancePhase_ClearsManaPools(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	p1.AddManaToPool(Green, 2)
	p2.AddManaToPool(Red, 1)
	g.AdvancePhase()
	if p1.GetManaPool()[Green] != 0 || p2.GetManaPool()[Red] != 0 {
		t.Fatalf("expected phase advance to clear mana pools, got p1=%v p2=%v", p1.GetManaPool(), p2.GetManaPool())
	}
}

func TestPlayer_CanPayCommanderIncludesTax(t *testing.T) {
	p := NewPlayer("P", 40)
	c := SimpleCard{Name: "Commander", ManaCost: "{G}"}
	p.RegisterCommander(c)
	p.IncrementCommanderCast(c.Name) // next cast owes {2} tax
	p.AddManaToPool(Green, 1)
	p.AddManaToPool(Colorless, 1)
	if p.CanPayForCommander(c) {
		t.Fatalf("expected {G}+{2} to be unaffordable with only two mana")
	}
	p.AddManaToPool(Colorless, 1)
	if !p.CanPayForCommander(c) || !p.PayForCommander(c) {
		t.Fatalf("expected {G}+{2} to be payable with three mana")
	}
}
