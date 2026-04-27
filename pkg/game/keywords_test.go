package game

import "testing"

// 1v1 verification of CR 702 keyword behaviour wired through combat / SBA.

func twoPlayer(t *testing.T) (*Game, *Player, *Player) {
	t.Helper()
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	return NewGame(p1, p2), p1, p2
}

func makeCreature(name, oracle string, pow, tough int, owner *Player) *Permanent {
	c := SimpleCard{Name: name, TypeLine: "Creature", Power: itoa(pow), Toughness: itoa(tough), OracleText: oracle}
	p := NewPermanent(c, owner, owner)
	owner.Battlefield = append(owner.Battlefield, p)
	// Ensure not summoning-sick from the active player's perspective.
	p.SetEnteredTurn(0)
	return p
}

func TestKeyword_ParseFromOracleText(t *testing.T) {
	c := SimpleCard{Name: "X", TypeLine: "Creature", Power: "2", Toughness: "2", OracleText: "Flying, vigilance"}
	p := NewPermanent(c, NewPlayer("P", 20), nil)
	if !p.HasKeyword(KWFlying) || !p.HasKeyword(KWVigilance) {
		t.Fatalf("expected flying+vigilance parsed, got flying=%v vigilance=%v",
			p.HasKeyword(KWFlying), p.HasKeyword(KWVigilance))
	}
	if p.HasKeyword(KWTrample) {
		t.Fatalf("did not expect trample")
	}
}

func TestKeyword_VigilanceDoesNotTapOnAttack(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Knight", "Vigilance", 2, 2, p1)
	g.BeginCombat()
	if err := g.DeclareAttacker(att, p2); err != nil {
		t.Fatalf("declare: %v", err)
	}
	if att.IsTapped() {
		t.Fatalf("vigilance attacker should not be tapped")
	}
}

func TestKeyword_HasteAllowsAttackOnETB(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Goblin", "Haste", 2, 1, p1)
	att.SetEnteredTurn(g.GetTurnNumber())
	g.BeginCombat()
	if err := g.DeclareAttacker(att, p2); err != nil {
		t.Fatalf("haste should permit attacking same turn: %v", err)
	}
	// Without haste the same configuration must fail.
	att2 := makeCreature("Vanilla", "", 2, 1, p1)
	att2.SetEnteredTurn(g.GetTurnNumber())
	if err := g.DeclareAttacker(att2, p2); err == nil {
		t.Fatalf("expected summoning sickness error")
	}
}

func TestKeyword_FlyingBlockedOnlyByFlyingOrReach(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	flier := makeCreature("Drake", "Flying", 2, 2, p1)
	ground := makeCreature("Bear", "", 2, 2, p2)
	reacher := makeCreature("Spider", "Reach", 1, 3, p2)
	g.BeginCombat()
	if err := g.DeclareAttacker(flier, p2); err != nil {
		t.Fatalf("declare: %v", err)
	}
	if err := g.DeclareBlocker(ground, flier); err == nil {
		t.Fatalf("non-flying/reach should not be able to block flier")
	}
	if err := g.DeclareBlocker(reacher, flier); err != nil {
		t.Fatalf("reach should block flier: %v", err)
	}
}

func TestKeyword_TrampleExcessToDefender(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Trampler", "Trample", 5, 5, p1)
	blk := makeCreature("Wall", "", 0, 2, p2)
	g.BeginCombat()
	_ = g.DeclareAttacker(att, p2)
	_ = g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()
	// Blocker takes 2 (lethal), defender takes 3 trample.
	if p2.GetLifeTotal() != 17 {
		t.Fatalf("expected 17 life after 3 trample, got %d", p2.GetLifeTotal())
	}
}

func TestKeyword_LifelinkGainsLife(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Vamp", "Lifelink", 3, 3, p1)
	g.BeginCombat()
	_ = g.DeclareAttacker(att, p2)
	g.ResolveCombatDamage()
	if p1.GetLifeTotal() != 23 {
		t.Fatalf("expected lifelink to bring P1 to 23, got %d", p1.GetLifeTotal())
	}
	if p2.GetLifeTotal() != 17 {
		t.Fatalf("expected P2 to be 17, got %d", p2.GetLifeTotal())
	}
}

func TestKeyword_DeathtouchKillsLargerCreature(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Snake", "Deathtouch", 1, 1, p1)
	blk := makeCreature("Giant", "", 5, 5, p2)
	g.BeginCombat()
	_ = g.DeclareAttacker(att, p2)
	_ = g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()
	if g.onBattlefield(blk) {
		t.Fatalf("deathtouch should have killed the 5/5")
	}
	if g.onBattlefield(att) {
		t.Fatalf("attacker should be dead from 5 damage")
	}
}

func TestKeyword_IndestructibleSurvivesLethal(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Big", "", 5, 5, p1)
	blk := makeCreature("Hero", "Indestructible", 2, 2, p2)
	g.BeginCombat()
	_ = g.DeclareAttacker(att, p2)
	_ = g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()
	if !g.onBattlefield(blk) {
		t.Fatalf("indestructible blocker should have survived 5 damage")
	}
}

func TestKeyword_IndestructibleVsDeathtouch(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Snake", "Deathtouch", 1, 1, p1)
	blk := makeCreature("Hero", "Indestructible", 2, 2, p2)
	g.BeginCombat()
	_ = g.DeclareAttacker(att, p2)
	_ = g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()
	// Deathtouch marks lethal but indestructible (CR 702.12) survives it.
	if !g.onBattlefield(blk) {
		t.Fatalf("indestructible should beat deathtouch")
	}
}

func TestKeyword_TrampleWithDeathtouchAssignsOneToBlocker(t *testing.T) {
	g, p1, p2 := twoPlayer(t)
	att := makeCreature("Hydra", "Trample\nDeathtouch", 5, 5, p1)
	blk := makeCreature("Bear", "", 2, 2, p2)
	g.BeginCombat()
	_ = g.DeclareAttacker(att, p2)
	_ = g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()
	// CR 702.2c: any 1 damage is lethal with deathtouch, so 4 trample.
	if p2.GetLifeTotal() != 16 {
		t.Fatalf("expected 16 life (4 trample), got %d", p2.GetLifeTotal())
	}
}
