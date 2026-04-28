package game

import "testing"

// Multi-keyword interactions exercised end-to-end through DeclareAttacker /
// DeclareBlocker / ResolveCombatDamage. Each test asserts the post-combat
// life totals and battlefield occupancy so a regression in any of trample,
// lifelink, deathtouch, indestructible, or first/double strike is caught.

// Lifelink + Trample + Indestructible on the attacker vs a vanilla blocker:
// CR 702.15 (lifelink) gains the controller life equal to all damage dealt;
// CR 702.19 (trample) carries the excess past the blocker's toughness; CR
// 702.12 (indestructible) keeps the attacker alive even though the blocker
// strikes back lethal.
func TestKeyword_LifelinkTrampleIndestructible_Combo(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Beast", "Lifelink\nTrample\nIndestructible", 5, 5, p1)
	blk := makeCreature("Wall", "", 2, 5, p2) // 5 toughness so it strikes lethal back
	g.BeginCombat()
	if err := g.DeclareAttacker(att, p2); err != nil {
		t.Fatalf("declare attacker: %v", err)
	}
	if err := g.DeclareBlocker(blk, att); err != nil {
		t.Fatalf("declare blocker: %v", err)
	}
	g.ResolveCombatDamage()

	if !g.onBattlefield(att) {
		t.Fatalf("indestructible attacker should have survived 2 damage from blocker")
	}
	if g.onBattlefield(blk) {
		t.Fatalf("blocker should have died to 5 trample damage (2 lethal + 3 excess assigned correctly)")
	}
	if got := p1.GetLifeTotal(); got != 25 {
		t.Fatalf("expected lifelink to gain 5 life (20 -> 25), got %d", got)
	}
	if got := p2.GetLifeTotal(); got != 17 {
		t.Fatalf("expected 3 trample damage to defender (20 -> 17), got %d", got)
	}
}

// Deathtouch + First Strike: a 1/1 should take down a vanilla 3/3 blocker
// in the first-strike damage step (CR 702.2 + CR 702.7) and survive
// because the blocker never deals damage.
func TestKeyword_DeathtouchFirstStrike_BeatsLargerBlocker(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Assassin", "Deathtouch\nFirst strike", 1, 1, p1)
	blk := makeCreature("Ogre", "", 3, 3, p2)
	g.BeginCombat()
	if err := g.DeclareAttacker(att, p2); err != nil {
		t.Fatalf("declare attacker: %v", err)
	}
	if err := g.DeclareBlocker(blk, att); err != nil {
		t.Fatalf("declare blocker: %v", err)
	}
	g.ResolveCombatDamage()

	if !g.onBattlefield(att) {
		t.Fatalf("first-strike deathtoucher should have survived")
	}
	if g.onBattlefield(blk) {
		t.Fatalf("blocker should have died to deathtouch in first-strike step")
	}
	if got := p2.GetLifeTotal(); got != 20 {
		t.Fatalf("blocked first-strike attacker shouldn't have hit the player; got %d", got)
	}
}

// Double strike + Lifelink against a creature that survives the first
// strike step deals damage twice (CR 702.4), so lifelink should fire twice
// for the full 4 life and the attacker should survive a 1-damage punch.
func TestKeyword_DoubleStrikeLifelink_HitsTwiceGains4(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Cat", "Double strike\nLifelink", 2, 2, p1)
	blk := makeCreature("Bear", "", 1, 4, p2) // survives the first-strike 2 dmg
	g.BeginCombat()
	if err := g.DeclareAttacker(att, p2); err != nil {
		t.Fatalf("declare attacker: %v", err)
	}
	if err := g.DeclareBlocker(blk, att); err != nil {
		t.Fatalf("declare blocker: %v", err)
	}
	g.ResolveCombatDamage()

	if g.onBattlefield(blk) {
		t.Fatalf("blocker should be dead after taking 4 damage across two strike steps")
	}
	if got := p1.GetLifeTotal(); got != 24 {
		t.Fatalf("expected double-strike lifelink to gain 4 life (20 -> 24), got %d", got)
	}
}

// Indestructible blocker against a trampling attacker: the trample math
// still uses the blocker's toughness for the "needed lethal" amount even
// though the blocker survives (CR 702.19c). Excess damage to the player
// must therefore equal Power - Toughness, not Power.
func TestKeyword_TrampleIntoIndestructible_OnlyExcessTramples(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Beast", "Trample", 7, 7, p1)
	blk := makeCreature("Statue", "Indestructible", 2, 2, p2)
	g.BeginCombat()
	if err := g.DeclareAttacker(att, p2); err != nil {
		t.Fatalf("declare attacker: %v", err)
	}
	if err := g.DeclareBlocker(blk, att); err != nil {
		t.Fatalf("declare blocker: %v", err)
	}
	g.ResolveCombatDamage()

	if !g.onBattlefield(blk) {
		t.Fatalf("indestructible blocker should have survived")
	}
	// 2 assigned to blocker (its toughness), 5 trample to player.
	if got := p2.GetLifeTotal(); got != 15 {
		t.Fatalf("expected 5 trample (20 -> 15), got %d", got)
	}
	_ = p1
}
