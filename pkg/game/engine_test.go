package game

import (
	"testing"
)

// --- Keyword tests ---

func TestFlashKeyword(t *testing.T) {
	c := SimpleCard{Name: "Flash Threat", TypeLine: "Creature — Beast", OracleText: "Flash"}
	perm := NewPermanent(c, nil, nil)
	if !perm.HasKeywordByName("flash") {
		t.Fatal("expected creature with Flash keyword")
	}
	if perm.HasKeywordByName("flying") {
		t.Fatal("should not have flying")
	}
}

func TestShroudKeyword(t *testing.T) {
	c := SimpleCard{Name: "Shroud Creature", TypeLine: "Creature — Elf", OracleText: "Shroud"}
	perm := NewPermanent(c, nil, nil)
	if !perm.HasKeywordByName("shroud") {
		t.Fatal("expected Shroud keyword")
	}
}

func TestWardKeyword(t *testing.T) {
	c := SimpleCard{Name: "Warded", TypeLine: "Creature — Spirit", OracleText: "Ward {2}"}
	perm := NewPermanent(c, nil, nil)
	if !perm.HasKeywordByName("ward") {
		t.Fatal("expected Ward keyword")
	}
	if perm.HasWard() {
		t.Fatal("HasWard should be false without SetWardCost")
	}
	perm.SetWardCost("{2}")
	if !perm.HasWard() {
		t.Fatal("HasWard should be true after SetWardCost")
	}
}

func TestProtectionKeyword(t *testing.T) {
	p := NewPlayer("TestPlayer", 20)
	perm := NewPermanent(SimpleCard{}, p, p)
	perm.SetProtectionFrom("white", "red")
	if !perm.HasProtectionFrom("white") {
		t.Fatal("expected protection from white")
	}
	if !perm.HasProtectionFrom("red") {
		t.Fatal("expected protection from red")
	}
	if perm.HasProtectionFrom("blue") {
		t.Fatal("should not have protection from blue")
	}
}

func TestGrantKeyword(t *testing.T) {
	c := SimpleCard{Name: "Boring", TypeLine: "Creature — Human"}
	perm := NewPermanent(c, nil, nil)
	if perm.HasKeywordByName("haste") {
		t.Fatal("should not have haste initially")
	}
	perm.GrantKeyword(KWHaste)
	if !perm.HasKeywordByName("haste") {
		t.Fatal("expected haste after grant")
	}
}

// --- Counter cancellation tests ---

func TestCounterCancellation_PlusMinus(t *testing.T) {
	p := NewPlayer("P", 20)
	c := SimpleCard{Name: "TestCreature", TypeLine: "Creature — Beast", Power: "3", Toughness: "3"}
	perm := NewPermanent(c, p, p)
	perm.AddCounters("+1/+1", 2)
	perm.AddCounters("-1/-1", 3)
	changed := perm.removeCancelledCounters()
	if !changed {
		t.Fatal("expected counters to cancel")
	}
	if perm.GetCounters("+1/+1") != 0 {
		t.Fatalf("expected 0 +1/+1 counters, got %d", perm.GetCounters("+1/+1"))
	}
	if perm.GetCounters("-1/-1") != 1 {
		t.Fatalf("expected 1 -1/-1 counter, got %d", perm.GetCounters("-1/-1"))
	}
}

func TestCounterCancellation_EqualAmounts(t *testing.T) {
	p := NewPlayer("P", 20)
	c := SimpleCard{Name: "TestCreature", TypeLine: "Creature — Beast", Power: "2", Toughness: "2"}
	perm := NewPermanent(c, p, p)
	perm.AddCounters("+1/+1", 2)
	perm.AddCounters("-1/-1", 2)
	changed := perm.removeCancelledCounters()
	if !changed {
		t.Fatal("expected counters to cancel")
	}
	if perm.GetCounters("+1/+1") != 0 {
		t.Fatalf("expected 0 +1/+1 counters, got %d", perm.GetCounters("+1/+1"))
	}
	if perm.GetCounters("-1/-1") != 0 {
		t.Fatalf("expected 0 -1/-1 counters, got %d", perm.GetCounters("-1/-1"))
	}
}

func TestCounterCancellation_NoCounters(t *testing.T) {
	p := NewPlayer("P", 20)
	c := SimpleCard{Name: "TestCreature", TypeLine: "Creature — Beast", Power: "1", Toughness: "1"}
	perm := NewPermanent(c, p, p)
	changed := perm.removeCancelledCounters()
	if changed {
		t.Fatal("no counters should not change anything")
	}
}

// --- Poison counter tests ---

func TestPoisonCounters(t *testing.T) {
	p := NewPlayer("P", 20)
	if p.PoisonCounters() != 0 {
		t.Fatal("should start with 0 poison")
	}
	p.AddPoison(3)
	if p.PoisonCounters() != 3 {
		t.Fatalf("expected 3 poison, got %d", p.PoisonCounters())
	}
}

func TestPoisonCounters_LoseAtTen(t *testing.T) {
	g := NewGame(NewPlayer("P1", 20), NewPlayer("P2", 20))
	p := g.GetPlayersRaw()[0]
	p.AddPoison(9)
	g.ApplyStateBasedActions()
	if p.HasLost() {
		t.Fatal("should not lose at 9 poison")
	}
	p.AddPoison(1)
	g.ApplyStateBasedActions()
	if !p.HasLost() {
		t.Fatal("should lose at 10 poison")
	}
}

// --- Phase automation tests ---

func TestUntapStep(t *testing.T) {
	p1 := &Player{name: "P1"}
	p2 := &Player{name: "P2"}
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Tapper", TypeLine: "Creature — Elf", Power: "1", Toughness: "1"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = []*Permanent{perm}
	perm.Tap()
	if !perm.IsTapped() {
		t.Fatal("expected tapped before untap step")
	}
	g.AdvancePhase() // Untap -> Upkeep
	if perm.IsTapped() {
		t.Fatal("expected untapped after untap step")
	}
}

func TestDrawStep(t *testing.T) {
	p1 := &Player{name: "P1", Library: []SimpleCard{{Name: "Card1"}, {Name: "Card2"}}}
	p2 := &Player{name: "P2", Library: []SimpleCard{{Name: "Card3"}}}
	g := NewGame(p1, p2)
	// Advance through P1's turn to P2's turn
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	// Now on P2's turn, advance through draw step
	g.AdvancePhase() // Untap -> Upkeep
	g.AdvancePhase() // Upkeep -> Draw
	g.AdvancePhase() // Draw -> Main1 (draw happens here)
	if len(p2.Hand) != 1 {
		t.Fatalf("expected 1 card drawn, got %d", len(p2.Hand))
	}
}

func TestDrawStepFirstPlayerTurnOneSkip(t *testing.T) {
	p1 := &Player{name: "P1", Library: []SimpleCard{{Name: "Card1"}}}
	p2 := &Player{name: "P2"}
	g := NewGame(p1, p2)
	g.AdvancePhase() // Untap -> Upkeep
	g.AdvancePhase() // Upkeep -> Draw
	// First player skips draw on turn 1 (CR 103.4a)
	if len(p1.Hand) != 0 {
		t.Fatalf("expected no card drawn on turn 1 for first player, got %d", len(p1.Hand))
	}
}

func TestCleanupStep_DiscardToMax(t *testing.T) {
	p1 := &Player{name: "P1"}
	p2 := &Player{name: "P2"}
	g := NewGame(p1, p2)
	// Give P1 10 cards in hand
	for i := 0; i < 10; i++ {
		p1.Hand = append(p1.Hand, SimpleCard{Name: "Card"})
	}
	// Advance through a full turn to cleanup
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	// P1 should have discarded to max hand size (7)
	if len(p1.Hand) > 7 {
		t.Fatalf("expected hand size <= 7 after cleanup, got %d", len(p1.Hand))
	}
}

func TestCleanupStep_RemovesDamage(t *testing.T) {
	p1 := &Player{name: "P1"}
	p2 := &Player{name: "P2"}
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "ToughCreature", TypeLine: "Creature — Beast", Power: "5", Toughness: "5"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = []*Permanent{perm}
	perm.AddDamage(3)
	if perm.GetDamageCounters() != 3 {
		t.Fatal("expected damage before cleanup")
	}
	// Advance to cleanup
	for i := 0; i < 8; i++ {
		g.AdvancePhase()
	}
	if perm.GetDamageCounters() != 0 {
		t.Fatalf("expected 0 damage after cleanup, got %d", perm.GetDamageCounters())
	}
}

// --- SBA tests ---

func TestSBA_LethalDamage(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Vulnerable", TypeLine: "Creature — Beast", Power: "2", Toughness: "2"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = []*Permanent{perm}
	perm.AddDamage(2)
	g.ApplyStateBasedActions()
	if len(p1.Battlefield) != 0 {
		t.Fatal("expected creature to die from lethal damage")
	}
}

func TestSBA_ZeroToughness(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Zero", TypeLine: "Creature — Beast", Power: "1", Toughness: "0"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = []*Permanent{perm}
	g.ApplyStateBasedActions()
	if len(p1.Battlefield) != 0 {
		t.Fatal("expected creature with 0 toughness to die")
	}
}

func TestSBA_IndestructibleSurvivesLethalDamage(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Indestructible", TypeLine: "Creature — Angel", Power: "5", Toughness: "5"}
	perm := NewPermanent(c, p1, p1)
	perm.GrantKeyword(KWIndestructible)
	p1.Battlefield = []*Permanent{perm}
	perm.AddDamage(5)
	g.ApplyStateBasedActions()
	if len(p1.Battlefield) != 1 {
		t.Fatal("indestructible creature should survive lethal damage")
	}
}

func TestSBA_LifeLoss(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	p1.SetLifeTotal(0)
	g.ApplyStateBasedActions()
	if !p1.HasLost() {
		t.Fatal("player with 0 life should lose")
	}
}

func TestSBA_CommanderDamage(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	p1.AddCommanderDamage(p2, "General", 21)
	g.ApplyStateBasedActions()
	if !p1.HasLost() {
		t.Fatal("player with 21 commander damage should lose")
	}
}

func TestSBA_PlaneswalkerDeath(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Jace", TypeLine: "Planeswalker — Jace"}
	perm := NewPermanent(c, p1, p1)
	perm.AddCounters("loyalty", 3)
	p1.Battlefield = []*Permanent{perm}
	// Remove all loyalty
	perm.AddCounters("loyalty", -3)
	g.ApplyStateBasedActions()
	if len(p1.Battlefield) != 0 {
		t.Fatal("planeswalker with 0 loyalty should die")
	}
}

func TestSBA_LegendRule(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c1 := SimpleCard{Name: "Jace", TypeLine: "Legendary Planeswalker — Jace"}
	c2 := SimpleCard{Name: "Jace", TypeLine: "Legendary Planeswalker — Jace"}
	perm1 := NewPermanent(c1, p1, p1)
	perm2 := NewPermanent(c2, p1, p1)
	perm1.AddCounters("loyalty", 3)
	perm2.AddCounters("loyalty", 3)
	p1.Battlefield = []*Permanent{perm1, perm2}
	g.ApplyStateBasedActions()
	if len(p1.Battlefield) != 1 {
		t.Fatalf("legend rule should leave 1 permanent, got %d", len(p1.Battlefield))
	}
}

func TestSBA_LoopStabilizes(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Creature", TypeLine: "Creature — Beast", Power: "2", Toughness: "2"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = []*Permanent{perm}
	perm.AddDamage(2)
	g.ApplyStateBasedActions()
	// Should stabilize after one pass
	if len(p1.Battlefield) != 0 {
		t.Fatal("expected creature to die")
	}
}

// --- Sacrifice tests ---

func TestSacrifice(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Doomed", TypeLine: "Creature — Goblin", Power: "1", Toughness: "1"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = []*Permanent{perm}
	ok := g.Sacrifice(perm)
	if !ok {
		t.Fatal("sacrifice should return true")
	}
	if len(p1.Battlefield) != 0 {
		t.Fatal("sacrificed permanent should be removed from battlefield")
	}
	if len(p1.Graveyard) != 1 {
		t.Fatal("sacrificed permanent should go to graveyard")
	}
}

func TestSacrificeNil(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	ok := g.Sacrifice(nil)
	if ok {
		t.Fatal("sacrificing nil should return false")
	}
}

// --- Controller change tests ---

func TestChangeController(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	p2 := NewPlayer("P2", 20)
	g := NewGame(p1, p2)
	c := SimpleCard{Name: "Stolen", TypeLine: "Creature — Beast", Power: "2", Toughness: "2"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = []*Permanent{perm}
	ok := g.ChangeController(perm, p2)
	if !ok {
		t.Fatal("change controller should return true")
	}
	if len(p1.Battlefield) != 0 {
		t.Fatal("old controller should not have the permanent")
	}
	if len(p2.Battlefield) != 1 {
		t.Fatal("new controller should have the permanent")
	}
	if perm.GetController() != p2 {
		t.Fatal("permanent controller should be p2")
	}
}

func TestChangeControllerSamePlayer(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	_ = NewPlayer("P2", 20)
	g := NewGame(p1, NewPlayer("P2", 20))
	c := SimpleCard{Name: "Same", TypeLine: "Creature — Beast", Power: "1", Toughness: "1"}
	perm := NewPermanent(c, p1, p1)
	p1.Battlefield = []*Permanent{perm}
	ok := g.ChangeController(perm, p1)
	if !ok {
		t.Fatal("change to same controller should return true")
	}
	if len(p1.Battlefield) != 1 {
		t.Fatal("permanent should remain with same controller")
	}
}

// --- Token factory tests ---

func TestNewClueToken(t *testing.T) {
	clue := NewClueToken()
	if clue.Name != "Clue" {
		t.Fatal("expected Clue token")
	}
	if !clue.IsArtifact() {
		t.Fatal("Clue should be an artifact")
	}
}

func TestNewTreasureToken(t *testing.T) {
	treasure := NewTreasureToken()
	if treasure.Name != "Treasure" {
		t.Fatal("expected Treasure token")
	}
	if !treasure.IsArtifact() {
		t.Fatal("Treasure should be an artifact")
	}
}

func TestNewFoodToken(t *testing.T) {
	food := NewFoodToken()
	if food.Name != "Food" {
		t.Fatal("expected Food token")
	}
	if !food.IsArtifact() {
		t.Fatal("Food should be an artifact")
	}
}

// --- Proliferate tests ---

func TestProliferate_Counters(t *testing.T) {
	p := NewPlayer("P", 20)
	c := SimpleCard{Name: "Creature", TypeLine: "Creature — Beast", Power: "2", Toughness: "2"}
	perm := NewPermanent(c, p, p)
	perm.AddCounters("+1/+1", 2)
	perm.AddCounters("charge", 1)
	added := p.Proliferate([]*Permanent{perm}, nil)
	if added != 2 {
		t.Fatalf("expected 2 counters added, got %d", added)
	}
	if perm.GetCounters("+1/+1") != 3 {
		t.Fatalf("expected 3 +1/+1 counters, got %d", perm.GetCounters("+1/+1"))
	}
	if perm.GetCounters("charge") != 2 {
		t.Fatalf("expected 2 charge counters, got %d", perm.GetCounters("charge"))
	}
}

func TestProliferate_Poison(t *testing.T) {
	p1 := NewPlayer("P1", 20)
	_ = NewPlayer("P2", 20)
	p1.AddPoison(5)
	p1.Proliferate(nil, []*Player{p1})
	if p1.PoisonCounters() != 6 {
		t.Fatalf("expected 6 poison after proliferate, got %d", p1.PoisonCounters())
	}
}

func TestProliferate_NoCountersNoop(t *testing.T) {
	p := NewPlayer("P", 20)
	c := SimpleCard{Name: "Clean", TypeLine: "Creature — Beast", Power: "1", Toughness: "1"}
	perm := NewPermanent(c, p, p)
	added := p.Proliferate([]*Permanent{perm}, nil)
	if added != 0 {
		t.Fatalf("expected 0 counters added for permanent with no counters, got %d", added)
	}
}

// --- Keyword edge cases ---

func TestHasKeywordByName_CaseInsensitive(t *testing.T) {
	c := SimpleCard{Name: "Test", TypeLine: "Creature — Beast", OracleText: "Flying, First Strike"}
	perm := NewPermanent(c, nil, nil)
	if !perm.HasKeywordByName("Flying") {
		t.Fatal("should match case-insensitively")
	}
	if !perm.HasKeywordByName("FIRST STRIKE") {
		t.Fatal("should match case-insensitively")
	}
}

func TestHasKeywordByName_NilPermanent(t *testing.T) {
	var perm *Permanent
	if perm.HasKeywordByName("flying") {
		t.Fatal("nil permanent should not have any keywords")
	}
}

// --- Permanent type checks ---

func TestIsBattle(t *testing.T) {
	c := SimpleCard{Name: "Siege", TypeLine: "Battle — Siege"}
	if !c.IsBattle() {
		t.Fatal("expected IsBattle to return true")
	}
	c2 := SimpleCard{Name: "Creature", TypeLine: "Creature — Beast"}
	if c2.IsBattle() {
		t.Fatal("expected IsBattle to return false")
	}
}

// --- Discard to max hand size ---
