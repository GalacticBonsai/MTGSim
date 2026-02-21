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
