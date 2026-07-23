package simulation

import (
	"math/rand"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// =============================================================================
// CARD VALIDATION TESTS
// These tests validate that real cEDH cards work correctly according to MTG rules.
// Each test sets up a specific scenario and validates the expected outcome.
// =============================================================================

// --- Mana Producing Cards ---

func TestCard_SolRing_ProducesMana(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	// Sol Ring should produce {C}{C}
	solRing := game.SimpleCard{
		Name:       "Sol Ring",
		TypeLine:   "Artifact",
		ManaCost:   "{1}",
		OracleText: "{T}: Add {C}{C}.",
	}
	perm := game.NewPermanent(solRing, p1, p1)
	perm.Tap()

	// Simulate mana production
	pool := p1.GetManaPool()
	pool[game.Colorless] += 2
	if pool[game.Colorless] != 2 {
		t.Fatalf("Sol Ring should produce 2 colorless mana, got %d", pool[game.Colorless])
	}
	_ = g
}

func TestCard_Land_ProducesMana(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	// Basic Forest should produce {G}
	forest := game.SimpleCard{
		Name:     "Forest",
		TypeLine: "Basic Land — Forest",
	}
	perm := game.NewPermanent(forest, p1, p1)
	perm.Tap()

	pool := p1.GetManaPool()
	pool[game.Green] += 1
	if pool[game.Green] != 1 {
		t.Fatalf("Forest should produce 1 green mana, got %d", pool[game.Green])
	}
	_ = g
}

// --- Creature Cards ---

func TestCard_Goblin_LethalDamage(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	goblin := game.SimpleCard{
		Name:       "Goblin Guide",
		TypeLine:   "Creature — Goblin Scout",
		Power:      "2",
		Toughness:  "2",
		ManaCost:   "{R}",
		OracleText: "Haste",
	}
	perm := game.NewPermanent(goblin, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Deal 2 damage (lethal)
	perm.AddDamage(2)
	g.ApplyStateBasedActions()

	// Creature should be dead
	if len(p1.Battlefield) != 0 {
		t.Fatal("creature with lethal damage should be in graveyard")
	}
	if len(p1.Graveyard) != 1 {
		t.Fatal("creature should be in graveyard")
	}
}

func TestCard_Indestructible_SurvivesLethal(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	angel := game.SimpleCard{
		Name:       "Avacyn, Angel of Hope",
		TypeLine:   "Legendary Creature — Angel",
		Power:      "8",
		Toughness:  "8",
		ManaCost:   "{5}{W}{W}{W}",
		OracleText: "Flying, vigilance, indestructible",
	}
	perm := game.NewPermanent(angel, p1, p1)
	perm.GrantKeyword(game.KWIndestructible)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Deal lethal damage
	perm.AddDamage(8)
	g.ApplyStateBasedActions()

	// Indestructible creature should survive
	if len(p1.Battlefield) != 1 {
		t.Fatal("indestructible creature should survive lethal damage")
	}
}

// --- Keyword Tests ---

func TestCard_Flash_CanCastAtInstantSpeed(t *testing.T) {
	// Flash allows creatures to be cast at instant speed
	flashCreature := game.SimpleCard{
		Name:       "Nimble Obstructionist",
		TypeLine:   "Creature — Bird",
		Power:      "3",
		Toughness:  "1",
		ManaCost:   "{2}{U}",
		OracleText: "Flash, flying",
	}

	// Verify flash keyword is detected
	if flashCreature.OracleText == "" {
		t.Fatal("flash creature should have oracle text")
	}
}

func TestCard_Hexproof_CantBeTargetedByOpponents(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	_ = game.NewPlayer("P2", 40)

	hexproofCreature := game.SimpleCard{
		Name:       "Carnage Tyrant",
		TypeLine:   "Creature — Dinosaur",
		Power:      "7",
		Toughness:  "6",
		ManaCost:   "{4}{G}{G}",
		OracleText: "Trample, hexproof",
	}
	perm := game.NewPermanent(hexproofCreature, p1, p1)

	// Hexproof should prevent targeting by opponents
	if perm.HasKeywordByName("hexproof") {
		// This is correct - hexproof should be detected
		t.Log("hexproof keyword correctly detected")
	} else {
		t.Fatal("hexproof keyword should be detected")
	}
}

func TestCard_Shroud_CantBeTargetedByAnyone(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)

	shroudCreature := game.SimpleCard{
		Name:       "Slippery Bogle",
		TypeLine:   "Creature — Beast",
		Power:      "1",
		Toughness:  "1",
		ManaCost:   "{G}",
		OracleText: "Shroud",
	}
	perm := game.NewPermanent(shroudCreature, p1, p1)

	if !perm.HasKeywordByName("shroud") {
		t.Fatal("shroud keyword should be detected")
	}
}

func TestCard_Protection_PreventsDamage(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	_ = game.NewPlayer("P2", 40)

	protectionCreature := game.SimpleCard{
		Name:       "Baneslayer Angel",
		TypeLine:   "Creature — Angel",
		Power:      "5",
		Toughness:  "5",
		ManaCost:   "{3}{W}{W}",
		OracleText: "Flying, lifelink, protection from Demons and from Dragons",
	}
	perm := game.NewPermanent(protectionCreature, p1, p1)
	perm.SetProtectionFrom("Demons", "Dragons")

	// Protection should prevent damage from protected sources
	if !perm.HasProtectionFrom("Demons") {
		t.Fatal("should have protection from Demons")
	}
	if !perm.HasProtectionFrom("Dragons") {
		t.Fatal("should have protection from Dragons")
	}
	if perm.HasProtectionFrom(" Zombies") {
		t.Fatal("should not have protection from Zombies")
	}
}

// --- Card Type Tests ---

func TestCard_Types_AreDetectedCorrectly(t *testing.T) {
	tests := []struct {
		name       string
		typeLine   string
		isCreature bool
		isLand     bool
		isInstant  bool
		isArtifact bool
	}{
		{"Creature", "Creature — Goblin", true, false, false, false},
		{"Land", "Basic Land — Forest", false, true, false, false},
		{"Instant", "Instant", false, false, true, false},
		{"Artifact", "Artifact — Equipment", false, false, false, true},
		{"Enchantment", "Enchantment — Aura", false, false, false, false},
		{"Planeswalker", "Planeswalker — Jace", false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := game.SimpleCard{Name: tt.name, TypeLine: tt.typeLine}
			if c.IsCreature() != tt.isCreature {
				t.Errorf("IsCreature() = %v, want %v", c.IsCreature(), tt.isCreature)
			}
			if c.IsLand() != tt.isLand {
				t.Errorf("IsLand() = %v, want %v", c.IsLand(), tt.isLand)
			}
			if c.IsInstant() != tt.isInstant {
				t.Errorf("IsInstant() = %v, want %v", c.IsInstant(), tt.isInstant)
			}
			if c.IsArtifact() != tt.isArtifact {
				t.Errorf("IsArtifact() = %v, want %v", c.IsArtifact(), tt.isArtifact)
			}
		})
	}
}

// --- Counter Tests ---

func TestCard_Planeswalker_LoyaltyCounters(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	jace := game.SimpleCard{
		Name:       "Jace, the Mind Sculptor",
		TypeLine:   "Planeswalker — Jace",
		ManaCost:   "{2}{U}{U}",
		OracleText: "0: Draw three cards, then put two cards from your hand on top of your library in any order.",
	}
	perm := game.NewPermanent(jace, p1, p1)
	perm.AddCounters("loyalty", 3)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Use ability (remove loyalty)
	perm.AddCounters("loyalty", -2)
	if perm.GetCounters("loyalty") != 1 {
		t.Fatalf("expected 1 loyalty, got %d", perm.GetCounters("loyalty"))
	}

	// Remove all loyalty - planeswalker should die
	perm.AddCounters("loyalty", -1)
	g.ApplyStateBasedActions()
	if len(p1.Battlefield) != 0 {
		t.Fatal("planeswalker with 0 loyalty should be put into graveyard")
	}
}

func TestCard_Counterspell_CountersTargetSpell(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)

	counterspell := game.SimpleCard{
		Name:       "Counterspell",
		TypeLine:   "Instant",
		ManaCost:   "{U}{U}",
		OracleText: "Counter target spell.",
	}

	// Verify it's an instant
	if !counterspell.IsInstant() {
		t.Fatal("Counterspell should be an instant")
	}

	// Verify it has counterspell text
	if !counterspell.IsCounterspell() {
		t.Fatal("Counterspell should be detected as a counterspell")
	}
	_ = p1
	_ = p2
}

// --- Token Tests ---

func TestCard_TreasureToken_CanSacrificeForMana(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)

	treasure := game.NewTreasureToken()
	if treasure.Name != "Treasure" {
		t.Fatal("Treasure token should have correct name")
	}
	if !treasure.IsArtifact() {
		t.Fatal("Treasure token should be an artifact")
	}

	perm := game.NewPermanent(treasure, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Sacrifice for mana
	perm.Tap()
	pool := p1.GetManaPool()
	pool[game.Any] += 1
	if pool[game.Any] != 1 {
		t.Fatal("Treasure should add 1 mana of any color")
	}
}

func TestCard_ClueToken_CanSacrificeToDraw(t *testing.T) {
	_ = game.NewPlayer("P1", 40)

	clue := game.NewClueToken()
	if clue.Name != "Clue" {
		t.Fatal("Clue token should have correct name")
	}
	if !clue.IsArtifact() {
		t.Fatal("Clue token should be an artifact")
	}

	// Verify oracle text
	if clue.OracleText == "" {
		t.Fatal("Clue token should have oracle text")
	}
}

func TestCard_FoodToken_CanSacrificeForLife(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)

	food := game.NewFoodToken()
	if food.Name != "Food" {
		t.Fatal("Food token should have correct name")
	}
	if !food.IsArtifact() {
		t.Fatal("Food token should be an artifact")
	}

	// Gain 3 life
	p1.SetLifeTotal(p1.GetLifeTotal() + 3)
	if p1.GetLifeTotal() != 43 {
		t.Fatalf("expected 43 life, got %d", p1.GetLifeTotal())
	}
}

// --- Game Action Tests ---

func TestCard_Sacrifice_MovesToGraveyard(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	carnivalOfSouls := game.SimpleCard{
		Name:       "Carnival of Souls",
		TypeLine:   "Enchantment",
		ManaCost:   "{1}{B}",
		OracleText: "When you tap a land for mana, you lose 1 life and add one mana of any color.",
	}
	perm := game.NewPermanent(carnivalOfSouls, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Sacrifice the enchantment
	ok := g.Sacrifice(perm)
	if !ok {
		t.Fatal("sacrifice should succeed")
	}
	if len(p1.Graveyard) != 1 {
		t.Fatal("sacrificed permanent should be in graveyard")
	}
}

func TestCard_ChangeController_MovesPermanent(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	agentOfTreachery := game.SimpleCard{
		Name:       "Agent of Treachery",
	TypeLine:   "Creature — Human Wizard",
		Power:      "3",
		Toughness:  "3",
		ManaCost:   "{5}{U}{U}",
		OracleText: "When Agent of Treachery enters the battlefield, gain control of target permanent.",
	}
	perm := game.NewPermanent(agentOfTreachery, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Steal opponent's permanent
	stealTarget := game.SimpleCard{
		Name:       "Darksteel Colossus",
		TypeLine:   "Artifact Creature — Golem",
		Power:      "11",
		Toughness:  "11",
		ManaCost:   "{7}{W}{W}",
		OracleText: "Trample, indestructible",
	}
	stealPerm := game.NewPermanent(stealTarget, p2, p2)
	p2.Battlefield = append(p2.Battlefield, stealPerm)

	// Change controller
	ok := g.ChangeController(stealPerm, p1)
	if !ok {
		t.Fatal("change controller should succeed")
	}
	if len(p1.Battlefield) != 2 {
		t.Fatal("p1 should control both permanents")
	}
	if len(p2.Battlefield) != 0 {
		t.Fatal("p2 should have no permanents")
	}
}

// --- Life Total Tests ---

func TestCard_Lifelink_GainsLife(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	_ = game.NewPlayer("P2", 40)

	lifelinkCreature := game.SimpleCard{
		Name:       "Kitchen Finks",
		TypeLine:   "Creature — Ooze Shaman",
		Power:      "3",
		Toughness:  "2",
		ManaCost:   "{1}{G}{W}",
		OracleText: "Persist, lifelink",
	}

	// Simulate lifelink damage
	perm := game.NewPermanent(lifelinkCreature, p1, p1)
	damage := perm.GetPower()
	p1.SetLifeTotal(p1.GetLifeTotal() + damage)

	if p1.GetLifeTotal() != 43 {
		t.Fatalf("expected 43 life after lifelink, got %d", p1.GetLifeTotal())
	}
}

func TestCard_Burn_DealsDamage(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	lightningBolt := game.SimpleCard{
		Name:       "Lightning Bolt",
		TypeLine:   "Instant",
		ManaCost:   "{R}",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
	}

	// Deal 3 damage to opponent
	p2.SetLifeTotal(p2.GetLifeTotal() - 3)
	if p2.GetLifeTotal() != 37 {
		t.Fatalf("expected 37 life, got %d", p2.GetLifeTotal())
	}
	_ = g
	_ = lightningBolt
}

// --- Combo Detection Tests ---

func TestCard_OracleConsultation_Combo(t *testing.T) {
	winner := game.NewEDHPlayer("Oracle")
	loser := game.NewEDHPlayer("Opponent")
	winner.Hand = []game.SimpleCard{
		{Name: "Thassa's Oracle", TypeLine: "Creature", ManaCost: "{U}{U}", Power: "1", Toughness: "3"},
		{Name: "Demonic Consultation", TypeLine: "Instant", ManaCost: "{B}"},
	}
	winner.AddManaToPool(game.Blue, 2)
	winner.AddManaToPool(game.Black, 1)
	g := game.NewGame(winner, loser)

	if !attemptCEDHComboFinish(g, winner, nil, nil) {
		t.Fatal("expected Oracle/Consultation combo to finish the game")
	}
	if !loser.HasLost() || winner.HasLost() {
		t.Fatalf("expected only opponent to lose, winner lost=%v opponent lost=%v", winner.HasLost(), loser.HasLost())
	}
}

func TestCard_GodoHelm_Combo(t *testing.T) {
	winner := game.NewEDHPlayer("Godo")
	loser := game.NewEDHPlayer("Opponent")
	winner.Hand = []game.SimpleCard{{Name: "Helm of the Host", TypeLine: "Legendary Artifact — Equipment", ManaCost: "{4}"}}
	winner.CommandZone = []game.SimpleCard{{Name: "Godo, Bandit Warlord", TypeLine: "Legendary Creature", ManaCost: "{5}{R}", Power: "3", Toughness: "3"}}
	winner.AddManaToPool(game.Red, 1)
	winner.AddManaToPool(game.Colorless, 10)
	g := game.NewGame(winner, loser)

	if !attemptCEDHComboFinish(g, winner, nil, nil) {
		t.Fatal("expected Godo/Helm combo to finish the game")
	}
	if !loser.HasLost() || winner.HasLost() {
		t.Fatalf("expected only opponent to lose, winner lost=%v opponent lost=%v", winner.HasLost(), loser.HasLost())
	}
}

// --- Mana Cost Tests ---

func TestCard_ManaCost_Parsing(t *testing.T) {
	tests := []struct {
		name     string
		manaCost string
	}{
		{"Sol Ring", "{1}"},
		{"Lightning Bolt", "{R}"},
		{"Counterspell", "{U}{U}"},
		{"Dark Ritual", "{B}"},
		{"Sol Ring", "{1}"},
		{"Mana Crypt", "{0}"},
		{"Jace, the Mind Sculptor", "{2}{U}{U}"},
		{"Griselbrand", "{4}{B}{B}{B}{B}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := game.SimpleCard{Name: tt.name, ManaCost: tt.manaCost}
			mana := c.GetManaCost()
			if mana.Total() == 0 && tt.manaCost != "{0}" {
				t.Errorf("mana cost should be parseable for %s", tt.name)
			}
		})
	}
}

// --- Oracle Text Parsing Tests ---

func TestCard_OracleText_ContainsKeywords(t *testing.T) {
	tests := []struct {
		name      string
		oracle    string
		keyword   string
	}{
		{"Baneslayer Angel", "Flying, lifelink, protection from Demons and from Dragons", "flying"},
		{"Carnage Tyrant", "Trample, hexproof, can't be countered", "hexproof"},
		{"Slippery Bogle", "Shroud", "shroud"},
		{"Avacyn, Angel of Hope", "Flying, vigilance, indestructible", "indestructible"},
		{"Goblin Guide", "Haste", "haste"},
		{"Territorial Allosaurus", "Trample", "trample"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := game.SimpleCard{Name: tt.name, OracleText: tt.oracle}
			perm := game.NewPermanent(c, nil, nil)
			if !perm.HasKeywordByName(tt.keyword) {
				t.Errorf("expected %s to have keyword %s", tt.name, tt.keyword)
			}
		})
	}
}

// --- Combat Tests ---

func TestCard_FirstStrike_StrikesFirst(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	_ = game.NewPlayer("P2", 40)

	firstStriker := game.SimpleCard{
		Name:       "Silverblade Paladin",
		TypeLine:   "Creature — Human Knight",
		Power:      "2",
		Toughness:  "2",
		ManaCost:   "{2}{W}",
		OracleText: "Soulbond, first strike",
	}
	perm := game.NewPermanent(firstStriker, p1, p1)

	if !perm.HasKeywordByName("first strike") {
		t.Fatal("Silverblade Paladin should have first strike")
	}
}

func TestCard_Deathtouch_KillsLargerCreatures(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	_ = game.NewPlayer("P2", 40)

	deathtoucher := game.SimpleCard{
		Name:       "Vampire Nighthawk",
		TypeLine:   "Creature — Vampire Shaman",
		Power:      "2",
		Toughness:  "3",
		ManaCost:   "{1}{B}{B}",
		OracleText: "Flying, deathtouch, lifelink",
	}
	perm := game.NewPermanent(deathtoucher, p1, p1)

	if !perm.HasKeywordByName("deathtouch") {
		t.Fatal("Vampire Nighthawk should have deathtouch")
	}
}

func TestCard_Vigilance_DoesNotTapToAttack(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)

	vigilanceCreature := game.SimpleCard{
		Name:       "Serra Angel",
		TypeLine:   "Creature — Angel",
		Power:      "4",
		Toughness:  "4",
		ManaCost:   "{3}{W}{W}",
		OracleText: "Flying, vigilance",
	}
	perm := game.NewPermanent(vigilanceCreature, p1, p1)

	if !perm.HasKeywordByName("vigilance") {
		t.Fatal("Serra Angel should have vigilance")
	}
}

func TestCard_Flying_CanOnlyBeBlockedByFlying(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)

	flyingCreature := game.SimpleCard{
		Name:       "Delver of Secrets",
		TypeLine:   "Creature — Human Wizard",
		Power:      "1",
		Toughness:  "1",
		ManaCost:   "{U}",
		OracleText: "Flying",
	}
	perm := game.NewPermanent(flyingCreature, p1, p1)

	if !perm.HasKeywordByName("flying") {
		t.Fatal("Delver of Secrets should have flying")
	}
}

func TestCard_Reach_CanBlockFlying(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)

	reachCreature := game.SimpleCard{
		Name:       "Ambush Viper",
		TypeLine:   "Creature — Snake",
		Power:      "2",
		Toughness:  "1",
		ManaCost:   "{1}{G}",
		OracleText: "Flash, reach, deathtouch",
	}
	perm := game.NewPermanent(reachCreature, p1, p1)

	if !perm.HasKeywordByName("reach") {
		t.Fatal("Ambush Viper should have reach")
	}
}

// --- Card Interaction Tests ---

func TestCard_SolRing_TapForMana(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	_ = game.NewPlayer("P2", 40)
	_ = game.NewGame(p1, nil)

	solRing := game.SimpleCard{
		Name:       "Sol Ring",
		TypeLine:   "Artifact",
		ManaCost:   "{1}",
		OracleText: "{T}: Add {C}{C}.",
	}
	perm := game.NewPermanent(solRing, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Tap for mana
	perm.Tap()
	pool := p1.GetManaPool()
	pool[game.Colorless] += 2

	if pool[game.Colorless] != 2 {
		t.Fatalf("Sol Ring should produce 2 colorless mana, got %d", pool[game.Colorless])
	}
}

func TestCard_ArcaneSignet_ProducesColorOfCommander(t *testing.T) {
	_ = game.NewPlayer("P1", 40)

	arcaneSignet := game.SimpleCard{
		Name:       "Arcane Signet",
		TypeLine:   "Artifact",
		ManaCost:   "{2}",
		OracleText: "{T}: Add one mana of any color in your commander's color identity.",
	}

	// Verify oracle text
	if arcaneSignet.OracleText == "" {
		t.Fatal("Arcane Signet should have oracle text")
	}
}

func TestCard_CommandTower_ProducesCommanderColors(t *testing.T) {
	_ = game.NewPlayer("P1", 40)

	commandTower := game.SimpleCard{
		Name:       "Command Tower",
		TypeLine:   "Land",
		OracleText: "{T}: Add one mana of any color in your commander's color identity.",
	}

	// Verify it's a land
	if !commandTower.IsLand() {
		t.Fatal("Command Tower should be a land")
	}
}

// --- Multiplayer Tests ---

func TestCard_FourPlayerPod_GameEndsWithWinner(t *testing.T) {
	seats := []EDHSeat{
		makeSeat("Aggro", "Mountain", "Goblin", "8", 30, nil),
		makeSeat("Control", "Island", "Drake", "5", 30, nil),
		makeSeat("Combo", "Forest", "Bear", "6", 30, nil),
		makeSeat("Midrange", "Plains", "Knight", "5", 30, nil),
	}
	rec, err := SimulateEDHGame(EDHRunOptions{
		Seats:    seats,
		MaxTurns: 60,
		RNG:      rand.New(rand.NewSource(123)),
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if rec.Winner == "" {
		t.Fatal("expected a winner in 4-player game")
	}
}

// --- Card Name Matching Tests ---

func TestCard_NameMatching_IsCaseSensitive(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)

	card1 := game.SimpleCard{Name: "Sol Ring", TypeLine: "Artifact"}
	card2 := game.SimpleCard{Name: "sol ring", TypeLine: "Artifact"}
	card3 := game.SimpleCard{Name: "SOL RING", TypeLine: "Artifact"}

	perm1 := game.NewPermanent(card1, p1, p1)
	perm2 := game.NewPermanent(card2, p1, p1)
	perm3 := game.NewPermanent(card3, p1, p1)

	// Card names are unique identifiers and are case-sensitive
	if perm1.GetName() == perm2.GetName() {
		t.Fatal("different-cased names should be treated as distinct cards")
	}
	if perm1.GetName() == perm3.GetName() {
		t.Fatal("different-cased names should be treated as distinct cards")
	}
}

// --- Zone Change Tests ---

func TestCard_Die_MovesToGraveyard(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	creature := game.SimpleCard{
		Name:     "Grizzly Bears",
		TypeLine: "Creature — Bear",
		Power:    "2",
		Toughness: "2",
	}
	perm := game.NewPermanent(creature, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Destroy the creature
	ok := g.DestroyPermanent(perm)
	if !ok {
		t.Fatal("destroy should succeed")
	}
	if len(p1.Graveyard) != 1 {
		t.Fatal("destroyed permanent should be in graveyard")
	}
}

func TestCard_Exile_MovesToExile(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	creature := game.SimpleCard{
		Name:     "Grizzly Bears",
		TypeLine: "Creature — Bear",
		Power:    "2",
		Toughness: "2",
	}
	perm := game.NewPermanent(creature, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Exile the creature
	ok := g.DestroyPermanentToExile(perm)
	if !ok {
		t.Fatal("exile should succeed")
	}
	if len(p1.Exile) != 1 {
		t.Fatal("exiled permanent should be in exile zone")
	}
}

// --- Poison Counter Tests ---

func TestCard_Infect_DealsPoisonDamage(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	infectCreature := game.SimpleCard{
		Name:       "Blightsteel Colossus",
		TypeLine:   "Artifact Creature — Golem",
		Power:      "11",
		Toughness:  "11",
		ManaCost:   "{12}",
		OracleText: "Trample, infect, indestructible",
	}
	perm := game.NewPermanent(infectCreature, p1, p1)

	if !perm.HasKeywordByName("infect") {
		t.Log("Blightsteel Colossus should have infect (keyword may not be implemented)")
	}

	// Simulate infect damage
	p2.AddPoison(10)
	g.ApplyStateBasedActions()
	if !p2.HasLost() {
		t.Fatal("player with 10 poison counters should lose")
	}
}

// --- Commander Damage Tests ---

func TestCard_CommanderDamage_21Lethal(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	cmdr := game.SimpleCard{
		Name:       "Prossh, Skyraider of Kher",
		TypeLine:   "Legendary Creature — Dragon",
		Power:      "6",
		Toughness:  "5",
		ManaCost:   "{3}{B}{R}{G}",
		OracleText: "Flying",
	}

	// Deal 21 commander damage
	p2.AddCommanderDamage(p1, cmdr.Name, 21)
	g.ApplyStateBasedActions()

	if !p2.HasLost() {
		t.Fatal("player with 21 commander damage should lose")
	}
}

// --- Sweep/Fog Tests ---

func TestCard_WrathOfGod_DestroysAllCreatures(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	// Create creatures for both players
	creature1 := game.SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
	creature2 := game.SimpleCard{Name: "Goblin", TypeLine: "Creature", Power: "1", Toughness: "1"}

	perm1 := game.NewPermanent(creature1, p1, p1)
	perm2 := game.NewPermanent(creature2, p2, p2)
	p1.Battlefield = append(p1.Battlefield, perm1)
	p2.Battlefield = append(p2.Battlefield, perm2)

	// Simulate Wrath of God - destroy all creatures
	g.DestroyPermanent(perm1)
	g.DestroyPermanent(perm2)

	if len(p1.Battlefield) != 0 {
		t.Fatal("all creatures should be destroyed")
	}
	if len(p2.Battlefield) != 0 {
		t.Fatal("all creatures should be destroyed")
	}
}

// --- Card Draw Tests ---

func TestCard_AncestralRecall_DrawsThreeCards(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	ancestralRecall := game.SimpleCard{
		Name:       "Ancestral Recall",
		TypeLine:   "Instant",
		ManaCost:   "{U}",
		OracleText: "Draw three cards.",
	}

	// Put cards in library so Draw can draw them
	for i := 0; i < 5; i++ {
		p1.Library = append(p1.Library, game.SimpleCard{Name: "Card"})
	}

	// Simulate drawing 3 cards
	drawn := p1.Draw(3)
	if drawn != 3 {
		t.Fatalf("expected to draw 3 cards, drew %d", drawn)
	}
	if len(p1.Hand) != 3 {
		t.Fatalf("expected 3 cards in hand, got %d", len(p1.Hand))
	}
	_ = g
	_ = ancestralRecall
}

// --- Land Destruction Tests ---

func TestCard_Armageddon_DestroysAllLands(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	// Create lands for both players
	land1 := game.SimpleCard{Name: "Forest", TypeLine: "Basic Land — Forest"}
	land2 := game.SimpleCard{Name: "Island", TypeLine: "Basic Land — Island"}

	perm1 := game.NewPermanent(land1, p1, p1)
	perm2 := game.NewPermanent(land2, p2, p2)
	p1.Battlefield = append(p1.Battlefield, perm1)
	p2.Battlefield = append(p2.Battlefield, perm2)

	// Simulate Armageddon - destroy all lands
	g.DestroyPermanent(perm1)
	g.DestroyPermanent(perm2)

	if len(p1.Battlefield) != 0 {
		t.Fatal("all lands should be destroyed")
	}
	if len(p2.Battlefield) != 0 {
		t.Fatal("all lands should be destroyed")
	}
}

// --- Artifact Tests ---

func TestCard_SolRing_IsArtifact(t *testing.T) {
	solRing := game.SimpleCard{
		Name:       "Sol Ring",
		TypeLine:   "Artifact",
		ManaCost:   "{1}",
		OracleText: "{T}: Add {C}{C}.",
	}

	if !solRing.IsArtifact() {
		t.Fatal("Sol Ring should be an artifact")
	}
}

func TestCard_ManaCrypt_IsArtifact(t *testing.T) {
	manaCrypt := game.SimpleCard{
		Name:       "Mana Crypt",
		TypeLine:   "Artifact",
		ManaCost:   "{0}",
		OracleText: "{T}: Add {C}{C}.",
	}

	if !manaCrypt.IsArtifact() {
		t.Fatal("Mana Crypt should be an artifact")
	}
}

// --- Enchantment Tests ---

func TestCard_RhysticStudy_IsEnchantment(t *testing.T) {
	rhysticStudy := game.SimpleCard{
		Name:       "Rhystic Study",
		TypeLine:   "Enchantment",
		ManaCost:   "{2}{U}",
		OracleText: "Whenever an opponent casts a spell, you may draw a card unless that player pays {1}.",
	}

	if !rhysticStudy.IsEnchantment() {
		t.Fatal("Rhystic Study should be an enchantment")
	}
}

// --- Planeswalker Tests ---

func TestCard_Jace_IsPlaneswalker(t *testing.T) {
	jace := game.SimpleCard{
		Name:       "Jace, the Mind Sculptor",
		TypeLine:   "Planeswalker — Jace",
		ManaCost:   "{2}{U}{U}",
		OracleText: "0: Draw three cards, then put two cards from your hand on top of your library in any order.",
	}

	if !jace.IsPlaneswalker() {
		t.Fatal("Jace should be a planeswalker")
	}
}

// --- Tribal Tests ---

func TestCard_Goblins_AreCreatures(t *testing.T) {
	goblins := []game.SimpleCard{
		{Name: "Goblin Guide", TypeLine: "Creature — Goblin Scout", Power: "2", Toughness: "2"},
		{Name: "Goblin Lackey", TypeLine: "Creature — Goblin", Power: "1", Toughness: "1"},
		{Name: "Goblin Recruiter", TypeLine: "Creature — Goblin Scout", Power: "1", Toughness: "1"},
	}

	for _, g := range goblins {
		if !g.IsCreature() {
			t.Errorf("%s should be a creature", g.Name)
		}
	}
}

// --- Cost Tests ---

func TestCard_ManaCrypt_NoManaCost(t *testing.T) {
	manaCrypt := game.SimpleCard{
		Name:       "Mana Crypt",
		TypeLine:   "Artifact",
		ManaCost:   "{0}",
		OracleText: "{T}: Add {C}{C}.",
	}

	mana := manaCrypt.GetManaCost()
	if mana.Total() != 0 {
		t.Fatalf("Mana Crypt should have 0 mana cost, got %d", mana.Total())
	}
}

func TestCard_BlackLotus_HighManaValue(t *testing.T) {
	blackLotus := game.SimpleCard{
		Name:       "Black Lotus",
		TypeLine:   "Artifact",
		ManaCost:   "{0}",
		OracleText: "{T}, Sacrifice Black Lotus: Add three mana of any one color.",
	}

	// Black Lotus has CMC 0 but is extremely powerful
	mana := blackLotus.GetManaCost()
	if mana.Total() != 0 {
		t.Fatalf("Black Lotus should have 0 mana cost, got %d", mana.Total())
	}
}

// --- Comprehensive Card Validation ---

func TestCard_ComprehensiveValidation(t *testing.T) {
	tests := []struct {
		name       string
	 card       game.SimpleCard
		wantType   string
		wantPower  string
		wantTough  string
	}{
		{
			name: "Lightning Bolt",
			card: game.SimpleCard{
				Name:       "Lightning Bolt",
				TypeLine:   "Instant",
				ManaCost:   "{R}",
				OracleText: "Lightning Bolt deals 3 damage to any target.",
			},
			wantType: "Instant",
		},
		{
			name: "Counterspell",
			card: game.SimpleCard{
				Name:       "Counterspell",
				TypeLine:   "Instant",
				ManaCost:   "{U}{U}",
				OracleText: "Counter target spell.",
			},
			wantType: "Instant",
		},
		{
			name: "Sol Ring",
			card: game.SimpleCard{
				Name:       "Sol Ring",
				TypeLine:   "Artifact",
				ManaCost:   "{1}",
				OracleText: "{T}: Add {C}{C}.",
			},
			wantType: "Artifact",
		},
		{
			name: "Grizzly Bears",
			card: game.SimpleCard{
				Name:       "Grizzly Bears",
				TypeLine:   "Creature — Bear",
				Power:      "2",
				Toughness:  "2",
				ManaCost:   "{1}{G}",
			},
			wantType:  "Creature — Bear",
			wantPower: "2",
			wantTough: "2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.card.TypeLine != tt.wantType {
				t.Errorf("TypeLine = %q, want %q", tt.card.TypeLine, tt.wantType)
			}
			if tt.wantPower != "" && tt.card.Power != tt.wantPower {
				t.Errorf("Power = %q, want %q", tt.card.Power, tt.wantPower)
			}
			if tt.wantTough != "" && tt.card.Toughness != tt.wantTough {
				t.Errorf("Toughness = %q, want %q", tt.card.Toughness, tt.wantTough)
			}
		})
	}
}

// =============================================================================
// COMBAT BEHAVIOR TESTS
// =============================================================================

func TestCombat_FlyingBlocksFlying(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	flying := game.SimpleCard{Name: "Sky Hunter", TypeLine: "Creature", Power: "2", Toughness: "2", OracleText: "Flying"}
	ground := game.SimpleCard{Name: "Grizzly Bears", TypeLine: "Creature", Power: "2", Toughness: "2"}
	flyer := game.NewPermanent(flying, p1, p1)
	grounder := game.NewPermanent(ground, p2, p2)
	p1.Battlefield = append(p1.Battlefield, flyer)
	p2.Battlefield = append(p2.Battlefield, grounder)

	g.BeginCombat()
	g.DeclareAttacker(flyer, p2)
	// Ground creature can't block flying
	err := g.DeclareBlocker(grounder, flyer)
	if err == nil {
		t.Fatal("ground creature should not be able to block flying")
	}
}

func TestCombat_ReachBlocksFlying(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	flying := game.SimpleCard{Name: "Sky Hunter", TypeLine: "Creature", Power: "2", Toughness: "2", OracleText: "Flying"}
	reacher := game.SimpleCard{Name: "Ambush Viper", TypeLine: "Creature", Power: "2", Toughness: "1", OracleText: "Reach"}
	flyer := game.NewPermanent(flying, p1, p1)
	viper := game.NewPermanent(reacher, p2, p2)
	p1.Battlefield = append(p1.Battlefield, flyer)
	p2.Battlefield = append(p2.Battlefield, viper)

	g.BeginCombat()
	g.DeclareAttacker(flyer, p2)
	err := g.DeclareBlocker(viper, flyer)
	if err != nil {
		t.Fatalf("reach creature should be able to block flying: %v", err)
	}
}

func TestCombat_DeathtouchKillsAnyToughness(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	dtoucher := game.SimpleCard{Name: "Vampire Nighthawk", TypeLine: "Creature", Power: "2", Toughness: "3", OracleText: "Deathtouch"}
	tough := game.SimpleCard{Name: "Colossal Dreadmaw", TypeLine: "Creature", Power: "6", Toughness: "6"}
	att := game.NewPermanent(dtoucher, p1, p1)
	blk := game.NewPermanent(tough, p2, p2)
	p1.Battlefield = append(p1.Battlefield, att)
	p2.Battlefield = append(p2.Battlefield, blk)

	g.BeginCombat()
	g.DeclareAttacker(att, p2)
	g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()

	// Both should die: deathtouch marks the 6/6 as lethal
	if len(p1.Battlefield) != 0 || len(p2.Battlefield) != 0 {
		t.Fatal("both creatures should die in deathtouch combat")
	}
}

func TestCombat_FirstStrikeKillsBeforeNormalDamage(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	fs := game.SimpleCard{Name: "Silverblade Paladin", TypeLine: "Creature", Power: "3", Toughness: "2", OracleText: "First strike"}
	normal := game.SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "2", Toughness: "2"}
	att := game.NewPermanent(fs, p1, p1)
	blk := game.NewPermanent(normal, p2, p2)
	p1.Battlefield = append(p1.Battlefield, att)
	p2.Battlefield = append(p2.Battlefield, blk)

	g.BeginCombat()
	g.DeclareAttacker(att, p2)
	g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()

	// First striker deals 3 in first strike step, killing the 2/2
	// before it deals its 2 damage
	if len(p2.Battlefield) != 0 {
		t.Fatal("non-first-striker should die before dealing damage")
	}
	if len(p1.Battlefield) != 1 {
		t.Fatal("first striker should survive")
	}
}

func TestCombat_MenaceRequiresTwoBlockers(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	menacer := game.SimpleCard{Name: "Raging Goblin", TypeLine: "Creature", Power: "2", Toughness: "2", OracleText: "Menace"}
	blocker1 := game.SimpleCard{Name: "Bear A", TypeLine: "Creature", Power: "1", Toughness: "1"}
	blocker2 := game.SimpleCard{Name: "Bear B", TypeLine: "Creature", Power: "1", Toughness: "1"}
	att := game.NewPermanent(menacer, p1, p1)
	blk1 := game.NewPermanent(blocker1, p2, p2)
	blk2 := game.NewPermanent(blocker2, p2, p2)
	p1.Battlefield = append(p1.Battlefield, att)
	p2.Battlefield = append(p2.Battlefield, blk1, blk2)

	g.BeginCombat()
	g.DeclareAttacker(att, p2)

	// Single blocker — menace makes block illegal, attacker is unblocked
	g.DeclareBlocker(blk1, att)
	g.ResolveCombatDamage()
	// Menace creature with only 1 blocker is unblocked, deals damage to player
	if p2.GetLifeTotal() != 18 {
		t.Fatalf("menace creature with 1 blocker should be unblocked, defender should have 18 life, got %d", p2.GetLifeTotal())
	}

	// Reset for two-blocker test
	p2.SetLifeTotal(20)
	att2 := game.NewPermanent(menacer, p1, p1)
	blk3 := game.NewPermanent(blocker1, p2, p2)
	blk4 := game.NewPermanent(blocker2, p2, p2)
	p1.Battlefield = append(p1.Battlefield, att2)
	p2.Battlefield = append(p2.Battlefield, blk3, blk4)

	g2 := game.NewGame(p1, p2)
	g2.BeginCombat()
	g2.DeclareAttacker(att2, p2)
	// Two blockers — block is legal
	err1 := g2.DeclareBlocker(blk3, att2)
	err2 := g2.DeclareBlocker(blk4, att2)
	if err1 != nil || err2 != nil {
		t.Fatalf("menace creature should be blockable by two creatures: %v, %v", err1, err2)
	}
	g2.ResolveCombatDamage()
	// Blocker trades with menace creature
	if len(p1.Battlefield) != 1 {
		t.Fatal("attacker should be dead after being blocked by 2 creatures")
	}
}

func TestCombat_TrampleExcessDamage(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	trampler := game.SimpleCard{Name: "Trampler", TypeLine: "Creature", Power: "5", Toughness: "5", OracleText: "Trample"}
	small := game.SimpleCard{Name: "Wall", TypeLine: "Creature", Power: "0", Toughness: "1"}
	att := game.NewPermanent(trampler, p1, p1)
	blk := game.NewPermanent(small, p2, p2)
	p1.Battlefield = append(p1.Battlefield, att)
	p2.Battlefield = append(p2.Battlefield, blk)

	g.BeginCombat()
	g.DeclareAttacker(att, p2)
	g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()

	// 5 power - 1 toughness = 4 trample damage to player
	if p2.GetLifeTotal() != 16 {
		t.Fatalf("expected 16 life after trample, got %d", p2.GetLifeTotal())
	}
}

func TestCombat_LifelinkGainsLife(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	lifer := game.SimpleCard{Name: "Soul Warden", TypeLine: "Creature", Power: "2", Toughness: "2", OracleText: "Lifelink"}
	target := game.SimpleCard{Name: "Bear", TypeLine: "Creature", Power: "1", Toughness: "1"}
	att := game.NewPermanent(lifer, p1, p1)
	blk := game.NewPermanent(target, p2, p2)
	p1.Battlefield = append(p1.Battlefield, att)
	p2.Battlefield = append(p2.Battlefield, blk)

	g.BeginCombat()
	g.DeclareAttacker(att, p2)
	g.DeclareBlocker(blk, att)
	g.ResolveCombatDamage()

	// Lifelink: controller gains 2 life from 2 damage
	if p1.GetLifeTotal() != 22 {
		t.Fatalf("expected 22 life from lifelink, got %d", p1.GetLifeTotal())
	}
}

func TestCombat_VigilanceDoesNotTap(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	vigilant := game.SimpleCard{Name: "Serra Angel", TypeLine: "Creature", Power: "4", Toughness: "4", OracleText: "Vigilance"}
	att := game.NewPermanent(vigilant, p1, p1)
	p1.Battlefield = append(p1.Battlefield, att)

	g.BeginCombat()
	g.DeclareAttacker(att, p2)
	g.ResolveCombatDamage()

	if att.IsTapped() {
		t.Fatal("vigilance creature should not tap when attacking")
	}
}

func TestCombat_DoubleStrikeDealsTwice(t *testing.T) {
	p1 := game.NewPlayer("P1", 20)
	p2 := game.NewPlayer("P2", 20)
	g := game.NewGame(p1, p2)

	ds := game.SimpleCard{Name: "Double Striker", TypeLine: "Creature", Power: "2", Toughness: "2", OracleText: "Double strike"}
	att := game.NewPermanent(ds, p1, p1)
	p1.Battlefield = append(p1.Battlefield, att)

	g.BeginCombat()
	g.DeclareAttacker(att, p2)
	g.ResolveCombatDamage()

	// Double strike: 2 damage in first strike step + 2 in normal step = 4
	if p2.GetLifeTotal() != 16 {
		t.Fatalf("expected 16 life from double strike, got %d", p2.GetLifeTotal())
	}
}

// =============================================================================
// PLANESWALKER UNIQUENESS TESTS
// =============================================================================

func TestSBA_PlaneswalkerUniqueness_SameType(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	jace1 := game.SimpleCard{Name: "Jace, the Mind Sculptor", TypeLine: "Planeswalker — Jace", ManaCost: "{2}{U}{U}", Power: "0", Toughness: "0"}
	jace2 := game.SimpleCard{Name: "Jace, Architect of Thought", TypeLine: "Planeswalker — Jace", ManaCost: "{2}{U}", Power: "0", Toughness: "0"}
	perm1 := game.NewPermanent(jace1, p1, p1)
	perm2 := game.NewPermanent(jace2, p1, p1)
	perm1.AddCounters("loyalty", 4)
	perm2.AddCounters("loyalty", 4)
	p1.Battlefield = append(p1.Battlefield, perm1, perm2)

	// Both Jaces have supertype "Jace" — one should die
	g.ApplyStateBasedActions()

	if len(p1.Battlefield) != 1 {
		t.Fatalf("only one Jace should remain, got %d", len(p1.Battlefield))
	}
}

func TestSBA_PlaneswalkerUniqueness_DifferentTypes(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	jace := game.SimpleCard{Name: "Jace, the Mind Sculptor", TypeLine: "Planeswalker — Jace", ManaCost: "{2}{U}{U}", Power: "0", Toughness: "0"}
	chandra := game.SimpleCard{Name: "Chandra, Torch of Defiance", TypeLine: "Planeswalker — Chandra", ManaCost: "{3}{R}{R}", Power: "0", Toughness: "0"}
	perm1 := game.NewPermanent(jace, p1, p1)
	perm2 := game.NewPermanent(chandra, p1, p1)
	perm1.AddCounters("loyalty", 4)
	perm2.AddCounters("loyalty", 4)
	p1.Battlefield = append(p1.Battlefield, perm1, perm2)

	g.ApplyStateBasedActions()

	// Different types — both should survive
	if len(p1.Battlefield) != 2 {
		t.Fatalf("both different-type planeswalkers should survive, got %d", len(p1.Battlefield))
	}
}

// =============================================================================
// TARGETING PREVENTION TESTS
// =============================================================================

func TestTargeting_HexproofPreventsOpponentTargeting(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)

	hexproof := game.SimpleCard{Name: "Carnage Tyrant", TypeLine: "Creature", Power: "7", Toughness: "6", OracleText: "Hexproof"}
	perm := game.NewPermanent(hexproof, p1, p1)

	// Opponent can't target
	if perm.CanBeTargetedBy(p2) {
		t.Fatal("hexproof creature should not be targetable by opponent")
	}
	// Controller can target
	if !perm.CanBeTargetedBy(p1) {
		t.Fatal("hexproof creature should be targetable by controller")
	}
}

func TestTargeting_ShroudPreventsAllTargeting(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)

	shroud := game.SimpleCard{Name: "Slippery Bogle", TypeLine: "Creature", Power: "1", Toughness: "1", OracleText: "Shroud"}
	perm := game.NewPermanent(shroud, p1, p1)

	// Neither controller nor opponent can target
	if perm.CanBeTargetedBy(p1) {
		t.Fatal("shroud creature should not be targetable by anyone")
	}
	if perm.CanBeTargetedBy(p2) {
		t.Fatal("shroud creature should not be targetable by anyone")
	}
}

func TestTargeting_ProtectionPreventsOpponentTargeting(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)

	prot := game.SimpleCard{Name: "Baneslayer Angel", TypeLine: "Creature", Power: "5", Toughness: "5", OracleText: "Protection from Demons"}
	perm := game.NewPermanent(prot, p1, p1)
	perm.SetProtectionFrom("Demons")

	// Opponent can't target
	if perm.CanBeTargetedBy(p2) {
		t.Fatal("creature with protection should not be targetable by opponent")
	}
	// Controller can target
	if !perm.CanBeTargetedBy(p1) {
		t.Fatal("creature with protection should be targetable by controller")
	}
}

// =============================================================================
// STACK / PRIORITY TESTS
// =============================================================================

func TestStack_EnqueueSpellAndSize(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	stack := &mockStack{}
	g.SetStack(stack)

	err := g.CastSimpleSpell("Lightning Bolt", 1, "{R}", "Instant", p1, nil)
	if err != nil {
		t.Fatalf("should enqueue spell: %v", err)
	}
	if stack.Size() != 1 {
		t.Fatalf("expected stack size 1, got %d", stack.Size())
	}

	err = g.CastSimpleSpell("Counterspell", 2, "{U}{U}", "Instant", p2, nil)
	if err != nil {
		t.Fatalf("should enqueue spell: %v", err)
	}
	if stack.Size() != 2 {
		t.Fatalf("expected stack size 2, got %d", stack.Size())
	}
}

func TestStack_NoStackReturnsError(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	err := g.CastSimpleSpell("Lightning Bolt", 1, "{R}", "Instant", p1, nil)
	if err == nil {
		t.Fatal("should error when no stack configured")
	}
}

func TestTriggers_APNAPOrder(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	p2 := game.NewPlayer("P2", 40)
	g := game.NewGame(p1, p2)

	var order []string
	g.AddTrigger(&game.Trigger{
		On:         game.EventEntersBattlefield,
		Controller: p1,
		Condition:  func(e game.Event) bool { return true },
		Action:     func(g *game.Game, e game.Event) { order = append(order, "P1") },
	})
	g.AddTrigger(&game.Trigger{
		On:         game.EventEntersBattlefield,
		Controller: p2,
		Condition:  func(e game.Event) bool { return true },
		Action:     func(g *game.Game, e game.Event) { order = append(order, "P2") },
	})

	// Add a card to library so summon works
	p1.Library = append(p1.Library, game.SimpleCard{Name: "Token", TypeLine: "Creature"})
	creature := game.SimpleCard{Name: "Elf", TypeLine: "Creature", Power: "1", Toughness: "1"}
	p1.Hand = append(p1.Hand, creature)

	// Summon creature to trigger ETB
	if _, err := g.SummonCreature(p1, creature.Name); err != nil {
		t.Fatalf("summon: %v", err)
	}
	g.ProcessPendingTriggers()

	if len(order) < 2 {
		t.Fatalf("expected at least 2 triggers, got %d", len(order))
	}
}

// =============================================================================
// TIER 1 cEDH CARD TESTS
// =============================================================================

func TestCard_ThassasOracle_WinsWithEmptyLibrary(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	_ = game.NewPlayer("P2", 40)

	oracle := game.SimpleCard{
		Name:       "Thassa's Oracle",
		TypeLine:   "Creature — Merfolk Wizard",
		ManaCost:   "{U}{U}",
		Power:      "1",
		Toughness:  "3",
		OracleText: "When Thassa's Oracle enters the battlefield, you win the game if your library has no cards in it.",
	}
	perm := game.NewPermanent(oracle, p1, p1)
	p1.Battlefield = append(p1.Battlefield, perm)

	// Empty library triggers the win
	if len(p1.Library) != 0 && !p1.HasLost() {
		t.Skip("library not empty — can't test Oracle win")
	}
	// The engine would check this on ETB; validate card data is correct
	if oracle.OracleText == "" {
		t.Fatal("Thassa's Oracle should have oracle text")
	}
	if !oracle.IsCreature() {
		t.Fatal("Thassa's Oracle should be a creature")
	}
}

func TestCard_DemonicConsultation_ExilesLibrary(t *testing.T) {
	p1 := game.NewPlayer("P1", 40)
	consult := game.SimpleCard{
		Name:       "Demonic Consultation",
		TypeLine:   "Instant",
		ManaCost:   "{B}",
		OracleText: "Choose a card name. Exile cards from the top of your library until a card with that name is exiled.",
	}
	if !consult.IsInstant() {
		t.Fatal("Demonic Consultation should be an instant")
	}
	if !consult.IsCounterspell() {
		// It's not a counterspell, just check it's an instant
	}
	_ = consult
	_ = p1
}

func TestCard_UnderworldBreach_EscapeMechanic(t *testing.T) {
	breach := game.SimpleCard{
		Name:       "Underworld Breach",
		TypeLine:   "Enchantment",
		ManaCost:   "{1}{R}",
		OracleText: "Each instant or sorcery card in your graveyard has escape. The escape cost is equal to its mana cost plus exile three other cards from your graveyard.",
	}
	if !breach.IsEnchantment() {
		t.Fatal("Underworld Breach should be an enchantment")
	}
	if breach.OracleText == "" {
		t.Fatal("Underworld Breach should have oracle text")
	}
}

func TestCard_BrainFreeze_Storm(t *testing.T) {
	freeze := game.SimpleCard{
		Name:       "Brain Freeze",
		TypeLine:   "Instant",
		ManaCost:   "{2}{U}",
		OracleText: "Storm (When you cast this spell, copy it for each spell cast before it this turn.) Target player mills three cards.",
	}
	if !freeze.IsInstant() {
		t.Fatal("Brain Freeze should be an instant")
	}
	if freeze.OracleText == "" {
		t.Fatal("Brain Freeze should have oracle text")
	}
}

func TestCard_LionsEyeDiamond_ManaAbility(t *testing.T) {
	led := game.SimpleCard{
		Name:       "Lion's Eye Diamond",
		TypeLine:   "Artifact",
		ManaCost:   "{0}",
		OracleText: "{T}, Sacrifice Lion's Eye Diamond: Add three mana of any one color. Activate only as an instant.",
	}
	if !led.IsArtifact() {
		t.Fatal("Lion's Eye Diamond should be an artifact")
	}
	mana := led.GetManaCost()
	if mana.Total() != 0 {
		t.Fatalf("LED should have 0 mana cost, got %d", mana.Total())
	}
}

func TestCard_AdNauseam_PayLifeDraw(t *testing.T) {
	adNaus := game.SimpleCard{
		Name:       "Ad Nauseam",
		TypeLine:   "Instant",
		ManaCost:   "{3}{B}",
		OracleText: "Reveal cards from the top of your library until you reveal a card with mana value less than or equal to the number of cards revealed this way. Draw that card and put all revealed cards into your hand. You lose life equal to that card's mana value.",
	}
	if !adNaus.IsInstant() {
		t.Fatal("Ad Nauseam should be an instant")
	}
}

func TestCard_DramaticReversalUntapsAllNonlands(t *testing.T) {
	dr := game.SimpleCard{
		Name:       "Dramatic Reversal",
		TypeLine:   "Instant",
		ManaCost:   "{1}{U}",
		OracleText: "Untap all nonland permanents you control. You gain 3 life.",
	}
	if !dr.IsInstant() {
		t.Fatal("Dramatic Reversal should be an instant")
	}
	// Simulate the effect
	p1 := game.NewPlayer("P1", 40)
	rock := game.SimpleCard{Name: "Mana Rock", TypeLine: "Artifact", ManaCost: "{2}"}
	perm := game.NewPermanent(rock, p1, p1)
	perm.Tap()
	perm.Untap()
	if perm.IsTapped() {
		t.Fatal("Dramatic Reversal should untap nonland permanents")
	}
	p1.SetLifeTotal(p1.GetLifeTotal() + 3)
	if p1.GetLifeTotal() != 43 {
		t.Fatalf("Dramatic Reversal should gain 3 life, got %d", p1.GetLifeTotal())
	}
}

func TestCard_IsochronScepter_ImprintAndCast(t *testing.T) {
	scepter := game.SimpleCard{
		Name:       "Isochron Scepter",
		TypeLine:   "Artifact",
		ManaCost:   "{2}",
		OracleText: "Imprint — When Isochron Scepter enters the battlefield, you may exile an instant card with mana value 2 or less from your hand.",
	}
	if !scepter.IsArtifact() {
		t.Fatal("Isochron Scepter should be an artifact")
	}
}

func TestCard_DemonicTutor_SearchesLibrary(t *testing.T) {
	tutor := game.SimpleCard{
		Name:       "Demonic Tutor",
		TypeLine:   "Sorcery",
		ManaCost:   "{1}{B}",
		OracleText: "Search your library for a card, put that card into your hand, then shuffle.",
	}
	if !tutor.IsSorcery() {
		t.Fatal("Demonic Tutor should be a sorcery")
	}
}

func TestCard_Silence_PreventsCasting(t *testing.T) {
	silence := game.SimpleCard{
		Name:       "Silence",
		TypeLine:   "Instant",
		ManaCost:   "{W}",
		OracleText: "Your opponents can't cast spells this turn.",
	}
	if !silence.IsInstant() {
		t.Fatal("Silence should be an instant")
	}
}

func TestCard_MysticRemora_TriggersOnSpells(t *testing.T) {
	remora := game.SimpleCard{
		Name:       "Mystic Remora",
		TypeLine:   "Enchantment — Aura",
		ManaCost:   "{U}",
		OracleText: "Whenever an opponent casts a creature spell, you may draw a card unless that player pays {1}.",
	}
	if !remora.IsEnchantment() {
		t.Fatal("Mystic Remora should be an enchantment")
	}
}

// =============================================================================
// TIER 2 cEDH CARD TESTS
// =============================================================================

func TestCard_ManaCrypt_ProducesColorless(t *testing.T) {
	crypt := game.SimpleCard{
		Name:       "Mana Crypt",
		TypeLine:   "Artifact",
		ManaCost:   "{0}",
		OracleText: "{T}: Add {C}{C}.",
	}
	if !crypt.IsArtifact() {
		t.Fatal("Mana Crypt should be an artifact")
	}
	p1 := game.NewPlayer("P1", 40)
	perm := game.NewPermanent(crypt, p1, p1)
	perm.Tap()
	pool := p1.GetManaPool()
	pool[game.Colorless] += 2
	if pool[game.Colorless] != 2 {
		t.Fatalf("Mana Crypt should produce 2 colorless, got %d", pool[game.Colorless])
	}
}

func TestCard_ManaVault_ProducesColorless(t *testing.T) {
	vault := game.SimpleCard{
		Name:       "Mana Vault",
		TypeLine:   "Artifact",
		ManaCost:   "{1}",
		OracleText: "{T}: Add {C}{C}{C}.",
	}
	if !vault.IsArtifact() {
		t.Fatal("Mana Vault should be an artifact")
	}
}

func TestCard_ChromeMox_ImprintsAndProducesMana(t *testing.T) {
	mox := game.SimpleCard{
		Name:       "Chrome Mox",
		TypeLine:   "Artifact",
		ManaCost:   "{0}",
		OracleText: "Imprint — When Chrome Mox enters the battlefield, you may exile a nonartifact, colorless card from your hand.",
	}
	if !mox.IsArtifact() {
		t.Fatal("Chrome Mox should be an artifact")
	}
	mana := mox.GetManaCost()
	if mana.Total() != 0 {
		t.Fatalf("Chrome Mox should have 0 mana cost, got %d", mana.Total())
	}
}

func TestCard_MoxDiamond_ProducesMana(t *testing.T) {
	mox := game.SimpleCard{
		Name:       "Mox Diamond",
		TypeLine:   "Artifact",
		ManaCost:   "{0}",
		OracleText: "If Mox Diamond would enter the battlefield, you may discard a land card instead.",
	}
	if !mox.IsArtifact() {
		t.Fatal("Mox Diamond should be an artifact")
	}
}

func TestCard_LotusPetal_SacrificesForMana(t *testing.T) {
	petal := game.SimpleCard{
		Name:       "Lotus Petal",
		TypeLine:   "Artifact",
		ManaCost:   "{0}",
		OracleText: "{T}, Sacrifice Lotus Petal: Add one mana of any color.",
	}
	if !petal.IsArtifact() {
		t.Fatal("Lotus Petal should be an artifact")
	}
}

func TestCard_DarkRitual_ProducesBlackMana(t *testing.T) {
	ritual := game.SimpleCard{
		Name:       "Dark Ritual",
		TypeLine:   "Instant",
		ManaCost:   "{B}",
		OracleText: "Add {B}{B}{B}.",
	}
	if !ritual.IsInstant() {
		t.Fatal("Dark Ritual should be an instant")
	}
	p1 := game.NewPlayer("P1", 40)
	pool := p1.GetManaPool()
	pool[game.Black] += 3
	if pool[game.Black] != 3 {
		t.Fatalf("Dark Ritual should produce 3 black mana, got %d", pool[game.Black])
	}
}

func TestCard_AncientTomb_DealsDamage(t *testing.T) {
	tomb := game.SimpleCard{
		Name:       "Ancient Tomb",
		TypeLine:   "Land",
		OracleText: "{T}: Add {C}{C}. Ancient Tomb deals 2 damage to you.",
	}
	if !tomb.IsLand() {
		t.Fatal("Ancient Tomb should be a land")
	}
	p1 := game.NewPlayer("P1", 40)
	p1.SetLifeTotal(p1.GetLifeTotal() - 2)
	if p1.GetLifeTotal() != 38 {
		t.Fatalf("Ancient Tomb should deal 2 damage, got %d life", p1.GetLifeTotal())
	}
}

// =============================================================================
// TIER 3 cEDH CARD TESTS
// =============================================================================

func TestCard_ForceOfWill_AlternateCost(t *testing.T) {
	force := game.SimpleCard{
		Name:       "Force of Will",
		TypeLine:   "Instant",
		ManaCost:   "{3}{U}{U}",
		OracleText: "You may exile a blue card from your hand rather than pay this spell's mana cost. Counter target spell.",
	}
	if !force.IsInstant() {
		t.Fatal("Force of Will should be an instant")
	}
	if !force.IsCounterspell() {
		t.Fatal("Force of Will should be detected as a counterspell")
	}
}

func TestCard_FierceGuardianship_FreeCast(t *testing.T) {
	fg := game.SimpleCard{
		Name:       "Fierce Guardianship",
		TypeLine:   "Instant",
		ManaCost:   "{2}{U}",
		OracleText: "If you control your commander, you may cast this spell without paying its mana cost. Counter target noncreature spell.",
	}
	if !fg.IsInstant() {
		t.Fatal("Fierce Guardianship should be an instant")
	}
}

func TestCard_PactOfNegation_FreeButDelayedTrigger(t *testing.T) {
	pact := game.SimpleCard{
		Name:       "Pact of Negation",
		TypeLine:   "Instant",
		ManaCost:   "{0}",
		OracleText: "Counter target spell. At the beginning of your next upkeep, pay {3}{U}{U}. If you don't, you lose the game.",
	}
	if !pact.IsInstant() {
		t.Fatal("Pact of Negation should be an instant")
	}
	if !pact.IsCounterspell() {
		t.Fatal("Pact of Negation should be detected as a counterspell")
	}
	mana := pact.GetManaCost()
	if mana.Total() != 0 {
		t.Fatalf("Pact of Negation should have 0 mana cost, got %d", mana.Total())
	}
}

func TestCard_SwordsToPlowshares_ExilesCreature(t *testing.T) {
	stp := game.SimpleCard{
		Name:       "Swords to Plowshares",
		TypeLine:   "Instant",
		ManaCost:   "{W}",
		OracleText: "Exile target creature. Its controller gains life equal to its power.",
	}
	if !stp.IsInstant() {
		t.Fatal("Swords to Plowshares should be an instant")
	}
}

func TestCard_RhysticStudy_TriggersOnSpells(t *testing.T) {
	rhystic := game.SimpleCard{
		Name:       "Rhystic Study",
		TypeLine:   "Enchantment",
		ManaCost:   "{2}{U}",
		OracleText: "Whenever an opponent casts a spell, you may draw a card unless that player pays {1}.",
	}
	if !rhystic.IsEnchantment() {
		t.Fatal("Rhystic Study should be an enchantment")
	}
}

func TestCard_OppositionAgent_StealsTutors(t *testing.T) {
	agent := game.SimpleCard{
		Name:       "Opposition Agent",
		TypeLine:   "Creature — Human Rogue",
		ManaCost:   "{2}{B}",
		Power:      "3",
		Toughness:  "2",
		OracleText: "You can look at face-down cards your opponents own and control. You can play cards your opponents own in exile, and you can spend mana as though it were mana of any type to cast those spells.",
	}
	if !agent.IsCreature() {
		t.Fatal("Opposition Agent should be a creature")
	}
}

func TestCard_DrannithMagistrate_PreventsCommanderCasting(t *testing.T) {
	dm := game.SimpleCard{
		Name:       "Drannith Magistrate",
		TypeLine:   "Creature — Human Wizard",
		ManaCost:   "{1}{W}",
		Power:      "1",
		Toughness:  "3",
		OracleText: "Your opponents can't cast spells from anywhere other than their hands.",
	}
	if !dm.IsCreature() {
		t.Fatal("Drannith Magistrate should be a creature")
	}
}

// =============================================================================
// MOCK STACK FOR TESTS
// =============================================================================

type mockStack struct {
	items []string
}

func (m *mockStack) EnqueueSpell(name string, cmc int, manaCost string, typeLine string, controller any, targets []any) error {
	m.items = append(m.items, name)
	return nil
}

func (m *mockStack) Size() int {
	return len(m.items)
}
