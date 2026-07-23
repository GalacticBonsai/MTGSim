package game

import (
	"testing"

	"github.com/google/uuid"
)

// --- Event System Tests ---

func TestEventBatchCreation(t *testing.T) {
	events := []*Event{
		{ID: uuid.New(), Type: EventDamagePlayer, Amount: 5},
		{ID: uuid.New(), Type: EventDamagePlayer, Amount: 3},
	}
	batch := BatchEvent(EventDamageBatchForPlayers, events)
	if !batch.IsBatch {
		t.Fatal("expected batch event")
	}
	if len(batch.BatchEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(batch.BatchEvents))
	}
}

func TestEventAppliedEffectTracking(t *testing.T) {
	e := Event{ID: uuid.New(), Type: EventDamagePlayer}
	effectID := uuid.New()
	if e.HasAppliedEffect(effectID) {
		t.Fatal("should not have applied effect yet")
	}
	e.AddAppliedEffect(effectID)
	if !e.HasAppliedEffect(effectID) {
		t.Fatal("should have applied effect")
	}
}

// --- Replacement Effects Tests ---

func TestReplacementEffectRegistration(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	re := &ReplacementEffect{
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, e Event) bool {
			return e.Type == EventDamagePlayer
		},
		Apply: func(g *Game, e *Event) *Event {
			e.Amount = 0
			return e
		},
	}
	g.RegisterReplacementEffect(re)
	if g.replacementEngine == nil || len(g.replacementEngine.effects) != 1 {
		t.Fatal("expected 1 replacement effect")
	}
}

func TestReplacementEffectApplication(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	g.RegisterReplacementEffect(&ReplacementEffect{
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, e Event) bool {
			return e.Type == EventDamagePlayer
		},
		Apply: func(g *Game, e *Event) *Event {
			e.Amount = 0
			return e
		},
	})
	e := Event{Type: EventDamagePlayer, Amount: 10}
	result := g.applyReplacementEffects(e)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Amount != 0 {
		t.Fatalf("expected 0 damage, got %d", result.Amount)
	}
}

func TestReplacementEffectInfiniteLoopPrevention(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	effectID := uuid.New()
	g.RegisterReplacementEffect(&ReplacementEffect{
		ID:       effectID,
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, e Event) bool {
			return e.Type == EventDamagePlayer
		},
		Apply: func(g *Game, e *Event) *Event {
			e.Amount = 0
			return e
		},
	})
	// Mark effect as already applied
	e := Event{Type: EventDamagePlayer, Amount: 10, AppliedEffects: []uuid.UUID{effectID}}
	result := g.applyReplacementEffects(e)
	// Should pass through since the effect is already applied
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Amount != 10 {
		t.Fatalf("expected 10 damage (effect skipped), got %d", result.Amount)
	}
}

func TestWouldDieExileUntilEOT(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	perm := NewPermanent(SimpleCard{Name: "Test"}, g.players[0], g.players[0])
	g.players[0].Battlefield = append(g.players[0].Battlefield, perm)

	g.AddWouldDieExileUntilEOT(perm)

	// Verify the replacement effect was registered
	if g.replacementEngine == nil || len(g.replacementEngine.effects) != 1 {
		t.Fatal("expected 1 replacement effect")
	}
}

// --- Delayed Triggered Abilities Tests ---

func TestDelayedTriggerRegistration(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	dt := &DelayedTriggeredAbility{
		Controller:  g.players[0],
		TriggerType: EventDamagePlayer,
		Once:        true,
		Action:      func(g *Game, e Event) {},
	}
	g.RegisterDelayedTrigger(dt)
	if len(g.delayedTriggers) != 1 {
		t.Fatal("expected 1 delayed trigger")
	}
}

func TestDelayedTriggerMatching(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	fired := false
	dt := &DelayedTriggeredAbility{
		Controller:  g.players[0],
		TriggerType: EventDamagePlayer,
		Once:        true,
		Action: func(g *Game, e Event) {
			fired = true
		},
	}
	g.RegisterDelayedTrigger(dt)

	// Emit a matching event
	g.handleDelayedTriggers(Event{Type: EventDamagePlayer, Amount: 5})

	// Should have created a pending trigger
	if !g.HasPendingTriggers() {
		t.Fatal("expected pending triggers")
	}

	// Process pending triggers
	g.ProcessPendingTriggers()
	if !fired {
		t.Fatal("expected delayed trigger to fire")
	}

	// Should be removed (Once = true)
	if len(g.delayedTriggers) != 0 {
		t.Fatal("expected delayed trigger to be removed")
	}
}

func TestDelayedTriggerNonMatching(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	fired := false
	dt := &DelayedTriggeredAbility{
		Controller:  g.players[0],
		TriggerType: EventDamagePlayer,
		Once:        true,
		Action: func(g *Game, e Event) {
			fired = true
		},
	}
	g.RegisterDelayedTrigger(dt)

	// Emit a non-matching event
	g.handleDelayedTriggers(Event{Type: EventGainLife, Amount: 5})

	if g.HasPendingTriggers() {
		t.Fatal("expected no pending triggers")
	}
	if fired {
		t.Fatal("trigger should not have fired")
	}
}

// --- Reflexive Triggered Abilities Tests ---

func TestReflexiveTriggerRegistration(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	rt := &ReflexiveTriggeredAbility{
		Controller:  g.players[0],
		TriggerType: EventEntersBattlefield,
		Once:        true,
		Action:      func(g *Game, e Event) {},
	}
	g.RegisterReflexiveTrigger(rt)
	if len(g.reflexiveTriggers) != 1 {
		t.Fatal("expected 1 reflexive trigger")
	}
}

func TestReflexiveTriggerFiring(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	fired := false
	rt := &ReflexiveTriggeredAbility{
		Controller:  g.players[0],
		TriggerType: EventEntersBattlefield,
		Once:        true,
		Action: func(g *Game, e Event) {
			fired = true
		},
	}
	g.RegisterReflexiveTrigger(rt)

	// Emit a matching event
	g.handleReflexiveTriggers(Event{Type: EventEntersBattlefield})

	if !fired {
		t.Fatal("expected reflexive trigger to fire")
	}
	if len(g.reflexiveTriggers) != 0 {
		t.Fatal("expected reflexive trigger to be removed (Once=true)")
	}
}

// --- As-Though Effects Tests ---

func TestAsThoughEffectRegistration(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	ate := &AsThoughEffect{
		Type:     AsThoughTypeCanTarget,
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, sourceID, targetID uuid.UUID) bool {
			return true
		},
	}
	g.RegisterAsThoughEffect(ate)
	if g.asThoughEngine == nil || len(g.asThoughEngine.effects) != 1 {
		t.Fatal("expected 1 as-though effect")
	}
}

func TestCanTargetWithAsThough(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	sourceID := uuid.New()
	targetID := uuid.New()

	// No effect yet
	if g.CanTargetWithAsThough(sourceID, targetID) {
		t.Fatal("should not be able to target without effect")
	}

	// Add effect
	g.RegisterAsThoughEffect(&AsThoughEffect{
		Type:     AsThoughTypeCanTarget,
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, sID, tID uuid.UUID) bool {
			return sID == sourceID && tID == targetID
		},
	})

	if !g.CanTargetWithAsThough(sourceID, targetID) {
		t.Fatal("should be able to target with effect")
	}
}

func TestCanCastAsThough(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	cardID := uuid.New()

	if g.CanCastAsThrough(cardID) {
		t.Fatal("should not be able to cast without effect")
	}

	g.RegisterAsThoughEffect(&AsThoughEffect{
		Type:     AsThoughTypeCardCanCast,
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, cID, _ uuid.UUID) bool {
			return cID == cardID
		},
	})

	if !g.CanCastAsThrough(cardID) {
		t.Fatal("should be able to cast with effect")
	}
}

// --- Copy Effects Tests ---

func TestCopyPermanent(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	src := NewPermanent(SimpleCard{
		Name:       "Grizzly Bears",
		TypeLine:   "Creature — Bear",
		Power:      "2",
		Toughness:  "2",
		OracleText: "Trample",
	}, g.players[0], g.players[0])
	src.printedKeywords[KWTrample] = true
	src.effPower = 2
	src.effToughness = 2

	copy := g.copyPermanent(src, g.players[1])
	if copy == nil {
		t.Fatal("expected non-nil copy")
	}
	if copy.GetName() != "Grizzly Bears" {
		t.Fatalf("expected name 'Grizzly Bears', got '%s'", copy.GetName())
	}
	if copy.GetPower() != 2 {
		t.Fatalf("expected power 2, got %d", copy.GetPower())
	}
	if copy.GetToughness() != 2 {
		t.Fatalf("expected toughness 2, got %d", copy.GetToughness())
	}
	if !copy.HasKeyword(KWTrample) {
		t.Fatal("expected trample keyword")
	}
	if copy.GetController() != g.players[1] {
		t.Fatal("expected copy to be controlled by player B")
	}
}

func TestCopySpell(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	src := &GameStackItem{
		ID:          uuid.New(),
		Type:        GameStackSpell,
		Name:        "Lightning Bolt",
		Controller:  g.players[0],
		Description: "Lightning Bolt (spell)",
	}

	copy := g.CopySpell(src)
	if copy == nil {
		t.Fatal("expected non-nil copy")
	}
	if copy.Name != "Lightning Bolt" {
		t.Fatalf("expected name 'Lightning Bolt', got '%s'", copy.Name)
	}
	if copy.ID == src.ID {
		t.Fatal("expected copy to have different ID")
	}
}

// --- Keyword Tests ---

func TestNonEvergreenKeywords(t *testing.T) {
	// Test that all new keywords are recognized by the parser
	keywords := []struct {
		name   string
		keyword Keyword
	}{
		{"kicker", KWKicker},
		{"storm", KWStorm},
		{"cascade", KWCascade},
		{"suspend", KWSuspend},
		{"flashback", KWFlashback},
		{"madness", KWMadness},
		{"dredge", KWDredge},
		{"escape", KWEscape},
		{"echo", KWEcho},
		{"affinity", KWAffinity},
		{"delve", KWDelve},
		{"improvise", KWImprovise},
		{"embalm", KWEmbalm},
		{"eternalize", KWEternalize},
		{"prototype", KWPrototype},
	}

	for _, kw := range keywords {
		parsed, ok := keywordFromToken(kw.name)
		if !ok {
			t.Fatalf("keyword '%s' should be recognized", kw.name)
		}
		if parsed != kw.keyword {
			t.Fatalf("expected keyword %v, got %v", kw.keyword, parsed)
		}
	}
}

func TestKeywordStringRoundtrip(t *testing.T) {
	// Test that String() and keywordFromToken() are consistent
	// Skip protection because it has a "from X" suffix that doesn't roundtrip
	for k := KWFlying; k <= KWPrototype; k++ {
		name := keywordName(k)
		if name == "unknown" || name == "protection" {
			continue
		}
		parsed, ok := keywordFromToken(name)
		if !ok {
			t.Fatalf("keyword '%s' should be parsed back", name)
		}
		if parsed != k {
			t.Fatalf("roundtrip failed for %s: expected %v, got %v", name, k, parsed)
		}
	}
}

func TestParseKeywordsFromOracleNonEvergreen(t *testing.T) {
	oracle := "kicker\nstorm"
	keywords := parseKeywordsFromOracle(oracle)
	if !keywords[KWKicker] {
		t.Fatal("expected kicker keyword")
	}
	if !keywords[KWStorm] {
		t.Fatal("expected storm keyword")
	}
}

// --- Game Loop Tests ---

func TestGameLoopBasic(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	g.SetGameStack(&simpleGameStack{})

	// Play one phase (untap → upkeep)
	g.currentPhase = PhaseUntap
	g.PlayPhase()

	if g.currentPhase != PhaseUpkeep {
		t.Fatalf("expected PhaseUpkeep, got %d", g.currentPhase)
	}
}

func TestHasWinner(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	if g.HasWinner() {
		t.Fatal("game should not have winner yet")
	}

	g.players[0].Lose("test")
	if !g.HasWinner() {
		t.Fatal("game should have winner")
	}

	winner := g.GetWinner()
	if winner != g.players[1] {
		t.Fatal("expected player B to be winner")
	}
}

func TestPlayerActionCallback(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	g.SetGameStack(&simpleGameStack{})

	// Set up a player action that always passes
	passCount := 0
	g.playerAction = func(g *Game, p *Player) bool {
		passCount++
		return false // pass
	}

	g.currentPhase = PhaseUpkeep
	g.PlayPhase()

	// Both players should have been asked
	if passCount != 2 {
		t.Fatalf("expected 2 passes, got %d", passCount)
	}
}

func TestPhaseActionCallback(t *testing.T) {
	g := NewGame(NewPlayer("A", 40), NewPlayer("B", 40))
	g.SetGameStack(&simpleGameStack{})

	g.playerAction = func(g *Game, p *Player) bool {
		return false
	}

	g.currentPhase = PhaseDraw
	g.PlayPhase()

	// The draw step should have been called
	if g.turnNumber != 1 {
		t.Fatal("expected turn 1")
	}
}

// --- Zone Change Tracking Tests ---

func TestZoneChangeEvent(t *testing.T) {
	zc := ZoneChangeEvent{
		CardName: "Lightning Bolt",
		From:     Hand,
		To:       Graveyard,
		Player:   NewPlayer("A", 40),
		Turn:     1,
	}
	if zc.CardName != "Lightning Bolt" {
		t.Fatal("expected card name")
	}
	if zc.From != Hand {
		t.Fatal("expected from Hand")
	}
	if zc.To != Graveyard {
		t.Fatal("expected to Graveyard")
	}
}
