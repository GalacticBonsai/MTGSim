package ability

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// GameSimulationTest runs multiple games to ensure different outcomes
func TestGameSimulation_MultipleGames(t *testing.T) {
	// Set up random generator for varied results
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	const numGames = 10
	outcomes := make(map[string]int)
	gameResults := make([]GameResult, 0, numGames)
	
	for i := 0; i < numGames; i++ {
		result := runSingleGame(t, i+1, rng)
		gameResults = append(gameResults, result)
		outcomes[result.Winner]++
		
		t.Logf("Game %d: Winner=%s, Turns=%d, FinalLife=[%d,%d], AbilitiesUsed=%d", 
			i+1, result.Winner, result.TurnsPlayed, 
			result.Player1FinalLife, result.Player2FinalLife, result.TotalAbilitiesUsed)
	}
	
	// Verify we have different outcomes
	if len(outcomes) < 2 {
		t.Errorf("Expected different winners across games, but only got: %v", outcomes)
	}
	
	// Verify reasonable distribution (no player should win 100% of games)
	for winner, wins := range outcomes {
		winRate := float64(wins) / float64(numGames)
		if winRate > 0.9 {
			t.Errorf("Player %s won %.1f%% of games, which seems too dominant", winner, winRate*100)
		}
		t.Logf("Player %s won %d/%d games (%.1f%%)", winner, wins, numGames, winRate*100)
	}
	
	// Verify games had different lengths
	turnCounts := make(map[int]int)
	for _, result := range gameResults {
		turnCounts[result.TurnsPlayed]++
	}
	
	if len(turnCounts) < 3 {
		t.Errorf("Expected games of different lengths, but got turn counts: %v", turnCounts)
	}
	
	// Verify abilities were actually used
	totalAbilities := 0
	for _, result := range gameResults {
		totalAbilities += result.TotalAbilitiesUsed
	}
	
	if totalAbilities == 0 {
		t.Error("No abilities were used across all games")
	}
	
	avgAbilitiesPerGame := float64(totalAbilities) / float64(numGames)
	t.Logf("Average abilities used per game: %.1f", avgAbilitiesPerGame)
}

type GameResult struct {
	Winner              string
	TurnsPlayed         int
	Player1FinalLife    int
	Player2FinalLife    int
	TotalAbilitiesUsed  int
	GameEndReason       string
}

func runSingleGame(t *testing.T, gameNum int, rng *rand.Rand) GameResult {
	// Create players with different strategies and randomized starting life
	player1Life := 12 + rng.Intn(6) // 12-17 life (lower for faster games)
	player2Life := 12 + rng.Intn(6) // 12-17 life

	// Randomly assign strategies to add variation
	strategies := []string{"aggressive", "defensive"}
	if rng.Float32() < 0.5 {
		strategies[0], strategies[1] = strategies[1], strategies[0]
	}

	player1 := &gamePlayer{
		name:     "Player1",
		life:     player1Life,
		strategy: strategies[0],
		hand:     make([]gameCard, 0),
		lands:    make([]*gamePermanent, 0),
		creatures: make([]*gamePermanent, 0),
		manaPool: make(map[game.ManaType]int),
	}

	player2 := &gamePlayer{
		name:     "Player2",
		life:     player2Life,
		strategy: strategies[1],
		hand:     make([]gameCard, 0),
		lands:    make([]*gamePermanent, 0),
		creatures: make([]*gamePermanent, 0),
		manaPool: make(map[game.ManaType]int),
	}
	
	// Create game state
	gameState := &gameSimulationState{
		players:       []AbilityPlayer{player1, player2},
		currentPlayer: player1,
		turn:          1,
		phase:         "main",
		isMainPhase:   true,
		random:        rng,
	}
	
	// Create execution engine
	engine := NewExecutionEngine(gameState)
	
	// Give players some cards with abilities
	setupPlayerCards(player1, engine)
	setupPlayerCards(player2, engine)
	
	// Simulate game
	maxTurns := 15 // Reduced for faster games
	abilitiesUsed := 0
	
	for turn := 1; turn <= maxTurns; turn++ {
		gameState.turn = turn
		
		// Alternate current player
		if turn%2 == 1 {
			gameState.currentPlayer = player1
		} else {
			gameState.currentPlayer = player2
		}
		
		// Play turn
		turnAbilities := playTurn(t, gameState, engine)
		abilitiesUsed += turnAbilities
		
		// Check win conditions
		if player1.GetLifeTotal() <= 0 {
			return GameResult{
				Winner:              player2.GetName(),
				TurnsPlayed:         turn,
				Player1FinalLife:    player1.GetLifeTotal(),
				Player2FinalLife:    player2.GetLifeTotal(),
				TotalAbilitiesUsed:  abilitiesUsed,
				GameEndReason:       "Player 1 life reached 0",
			}
		}
		
		if player2.GetLifeTotal() <= 0 {
			return GameResult{
				Winner:              player1.GetName(),
				TurnsPlayed:         turn,
				Player1FinalLife:    player1.GetLifeTotal(),
				Player2FinalLife:    player2.GetLifeTotal(),
				TotalAbilitiesUsed:  abilitiesUsed,
				GameEndReason:       "Player 2 life reached 0",
			}
		}
	}
	
	// Game went to max turns, determine winner by life total
	winner := player1.GetName()
	if player2.GetLifeTotal() > player1.GetLifeTotal() {
		winner = player2.GetName()
	}
	
	return GameResult{
		Winner:              winner,
		TurnsPlayed:         maxTurns,
		Player1FinalLife:    player1.GetLifeTotal(),
		Player2FinalLife:    player2.GetLifeTotal(),
		TotalAbilitiesUsed:  abilitiesUsed,
		GameEndReason:       "Max turns reached",
	}
}

func setupPlayerCards(player *gamePlayer, engine *ExecutionEngine) {
	var decklist []CardData

	// Use different real decklists based on strategy
	if player.strategy == "aggressive" {
		decklist = getRedDeckWinsDecklist()
	} else {
		decklist = getControlDecklist()
	}

	// Add lands from decklist
	for _, cardData := range decklist {
		if cardData.CardType == "Land" {
			for i := 0; i < cardData.Count; i++ {
				land := &gamePermanent{
					id:         uuid.New(),
					name:       cardData.Name,
					owner:      player,
					controller: player,
					tapped:     false,
					abilities:  make([]*Ability, 0),
				}

				// Parse abilities from oracle text
				if cardData.OracleText != "" {
					abilities, err := engine.parser.ParseAbilities(cardData.OracleText, land)
					if err == nil {
						land.abilities = abilities
					}
				}

				player.lands = append(player.lands, land)

				// Limit to reasonable number for testing
				if len(player.lands) >= 6 {
					break
				}
			}
		}
	}

	// Add creatures from decklist
	for _, cardData := range decklist {
		if cardData.CardType == "Creature" {
			for i := 0; i < cardData.Count; i++ {
				creature := &gamePermanent{
					id:         uuid.New(),
					name:       cardData.Name,
					owner:      player,
					controller: player,
					tapped:     false,
					abilities:  make([]*Ability, 0),
					power:      cardData.Power,
					toughness:  cardData.Toughness,
				}

				// Parse abilities from oracle text
				if cardData.OracleText != "" {
					abilities, err := engine.parser.ParseAbilities(cardData.OracleText, creature)
					if err == nil {
						creature.abilities = abilities
					}
				}

				player.creatures = append(player.creatures, creature)

				// Limit to reasonable number for testing
				if len(player.creatures) >= 4 {
					break
				}
			}
		}
	}

	// Add some spells to hand
	for _, cardData := range decklist {
		if cardData.CardType == "Instant" || cardData.CardType == "Sorcery" {
			for i := 0; i < cardData.Count && len(player.hand) < 3; i++ {
				spell := gameCard{
					name:       cardData.Name,
					cardType:   cardData.CardType,
					oracleText: cardData.OracleText,
					manaCost:   cardData.ManaCost,
				}
				player.hand = append(player.hand, spell)
			}
		}
	}
}

func playTurn(t *testing.T, gameState *gameSimulationState, engine *ExecutionEngine) int {
	abilitiesUsed := 0
	currentPlayer := gameState.currentPlayer.(*gamePlayer)
	
	// Reset mana pool
	currentPlayer.manaPool = make(map[game.ManaType]int)
	
	// Untap permanents
	for _, land := range currentPlayer.lands {
		land.tapped = false
	}
	for _, creature := range currentPlayer.creatures {
		creature.tapped = false
	}
	
	// Activate mana abilities (tap lands for mana)
	for _, land := range currentPlayer.lands {
		if !land.tapped && len(land.abilities) > 0 {
			for _, ability := range land.abilities {
				if ability.Type == Mana && engine.canActivateAbility(ability, currentPlayer) {
					err := engine.ExecuteAbility(ability, currentPlayer, []interface{}{})
					if err == nil {
						abilitiesUsed++
						// Add mana to pool
						currentPlayer.manaPool[game.Green]++
					}
					break
				}
			}
		}
	}
	
	// Try to activate other abilities based on strategy
	for _, creature := range currentPlayer.creatures {
		for _, ability := range creature.abilities {
			if ability.Type != Mana && engine.canActivateAbility(ability, currentPlayer) {
				// Decide whether to activate based on strategy and randomness
				shouldActivate := false
				
				switch currentPlayer.strategy {
				case "aggressive":
					// Aggressive players use damage abilities more often
					if len(ability.Effects) > 0 && ability.Effects[0].Type == DealDamage {
						shouldActivate = gameState.random.Float32() < 0.9 // Higher chance for damage
					} else {
						shouldActivate = gameState.random.Float32() < 0.3
					}
				case "defensive":
					// Defensive players use card draw and life gain more often
					if len(ability.Effects) > 0 && (ability.Effects[0].Type == DrawCards || ability.Effects[0].Type == GainLife) {
						shouldActivate = gameState.random.Float32() < 0.8
					} else if len(ability.Effects) > 0 && ability.Effects[0].Type == DealDamage {
						shouldActivate = gameState.random.Float32() < 0.4 // Still use damage sometimes
					} else {
						shouldActivate = gameState.random.Float32() < 0.2
					}
				}
				
				if shouldActivate {
					// Choose targets
					targets := chooseTargetsForAbility(ability, gameState)
					if len(targets) > 0 { // Only execute if we have valid targets
						err := engine.ExecuteAbility(ability, currentPlayer, targets)
						if err == nil {
							abilitiesUsed++
							// Log damage abilities for debugging
							if len(ability.Effects) > 0 && ability.Effects[0].Type == DealDamage {
								t.Logf("Turn %d: %s used %s dealing %d damage to %v",
									gameState.turn, currentPlayer.GetName(), ability.Name,
									ability.Effects[0].Value, targets[0])
							}
						}
					}
				}
			}
		}
	}
	
	return abilitiesUsed
}

func chooseTargetsForAbility(ability *Ability, gameState *gameSimulationState) []interface{} {
	var targets []interface{}
	
	for _, effect := range ability.Effects {
		for _, targetReq := range effect.Targets {
			if targetReq.Required {
				switch targetReq.Type {
				case PlayerTarget:
					// Choose opponent for damage, self for beneficial effects
					if effect.Type == DealDamage {
						// Choose opponent
						for _, player := range gameState.players {
							if player != gameState.currentPlayer {
								targets = append(targets, player)
								break
							}
						}
					} else {
						// Choose self
						targets = append(targets, gameState.currentPlayer)
					}
				case AnyTarget:
					// For damage, prefer opponent
					if effect.Type == DealDamage {
						for _, player := range gameState.players {
							if player != gameState.currentPlayer {
								targets = append(targets, player)
								break
							}
						}
					}
				}
			}
		}
	}
	
	return targets
}

// Real Magic: The Gathering decklists

func getRedDeckWinsDecklist() []CardData {
	return []CardData{
		// Lands
		{
			Name:       "Mountain",
			CardType:   "Land",
			OracleText: "{T}: Add {R}.",
			Count:      20,
		},

		// Creatures
		{
			Name:       "Goblin Guide",
			ManaCost:   map[game.ManaType]int{game.Red: 1},
			CardType:   "Creature",
			OracleText: "Haste. Whenever Goblin Guide attacks, defending player reveals the top card of their library. If it's a land card, that player puts it into their hand.",
			Power:      2,
			Toughness:  2,
			Count:      4,
		},
		{
			Name:       "Monastery Swiftspear",
			ManaCost:   map[game.ManaType]int{game.Red: 1},
			CardType:   "Creature",
			OracleText: "Haste. Prowess",
			Power:      1,
			Toughness:  2,
			Count:      4,
		},
		{
			Name:       "Prodigal Pyromancer",
			ManaCost:   map[game.ManaType]int{game.Red: 3},
			CardType:   "Creature",
			OracleText: "{T}: Prodigal Pyromancer deals 3 damage to any target.",
			Power:      1,
			Toughness:  1,
			Count:      4,
		},
		{
			Name:       "Fire Elemental",
			ManaCost:   map[game.ManaType]int{game.Red: 2},
			CardType:   "Creature",
			OracleText: "{1}, {T}: Fire Elemental deals 2 damage to any target.",
			Power:      2,
			Toughness:  2,
			Count:      3,
		},

		// Burn spells
		{
			Name:       "Lightning Bolt",
			ManaCost:   map[game.ManaType]int{game.Red: 1},
			CardType:   "Instant",
			OracleText: "Lightning Bolt deals 3 damage to any target.",
			Count:      4,
		},
		{
			Name:       "Lava Spike",
			ManaCost:   map[game.ManaType]int{game.Red: 1},
			CardType:   "Sorcery",
			OracleText: "Lava Spike deals 3 damage to target player or planeswalker.",
			Count:      4,
		},
		{
			Name:       "Rift Bolt",
			ManaCost:   map[game.ManaType]int{game.Red: 3},
			CardType:   "Sorcery",
			OracleText: "Suspend 1—{R}. Rift Bolt deals 3 damage to any target.",
			Count:      4,
		},
	}
}

func getControlDecklist() []CardData {
	return []CardData{
		// Lands
		{
			Name:       "Island",
			CardType:   "Land",
			OracleText: "{T}: Add {U}.",
			Count:      12,
		},
		{
			Name:       "Plains",
			CardType:   "Land",
			OracleText: "{T}: Add {W}.",
			Count:      8,
		},

		// Creatures
		{
			Name:       "Snapcaster Mage",
			ManaCost:   map[game.ManaType]int{game.Blue: 1, game.Any: 1},
			CardType:   "Creature",
			OracleText: "Flash. When Snapcaster Mage enters the battlefield, target instant or sorcery card in your graveyard gains flashback until end of turn.",
			Power:      2,
			Toughness:  1,
			Count:      4,
		},
		{
			Name:       "Wall of Omens",
			ManaCost:   map[game.ManaType]int{game.White: 1, game.Any: 1},
			CardType:   "Creature",
			OracleText: "When Wall of Omens enters the battlefield, draw a card.",
			Power:      0,
			Toughness:  4,
			Count:      4,
		},

		// Spells
		{
			Name:       "Counterspell",
			ManaCost:   map[game.ManaType]int{game.Blue: 2},
			CardType:   "Instant",
			OracleText: "Counter target spell.",
			Count:      4,
		},
		{
			Name:       "Wrath of God",
			ManaCost:   map[game.ManaType]int{game.White: 2, game.Any: 2},
			CardType:   "Sorcery",
			OracleText: "Destroy all creatures. They can't be regenerated.",
			Count:      4,
		},
		{
			Name:       "Fact or Fiction",
			ManaCost:   map[game.ManaType]int{game.Blue: 1, game.Any: 3},
			CardType:   "Instant",
			OracleText: "Reveal the top five cards of your library. An opponent separates those cards into two piles. Put one pile into your hand and the other into your graveyard.",
			Count:      4,
		},
	}
}

// Game simulation types

type CardData struct {
	Name       string
	ManaCost   map[game.ManaType]int
	CardType   string
	OracleText string
	Power      int
	Toughness  int
	Count      int // Number of copies in deck
}

type gamePlayer struct {
	name      string
	life      int
	strategy  string
	hand      []gameCard
	lands     []*gamePermanent
	creatures []*gamePermanent
	manaPool  map[game.ManaType]int
}

func (p *gamePlayer) GetName() string {
	return p.name
}

func (p *gamePlayer) GetLifeTotal() int {
	return p.life
}

func (p *gamePlayer) SetLifeTotal(life int) {
	p.life = life
}

func (p *gamePlayer) GetHand() []interface{} {
	result := make([]interface{}, len(p.hand))
	for i, card := range p.hand {
		result[i] = card
	}
	return result
}

func (p *gamePlayer) AddCardToHand(card interface{}) {
	if gameCard, ok := card.(gameCard); ok {
		p.hand = append(p.hand, gameCard)
	}
}

func (p *gamePlayer) GetCreatures() []interface{} {
	result := make([]interface{}, len(p.creatures))
	for i, creature := range p.creatures {
		result[i] = creature
	}
	return result
}

func (p *gamePlayer) GetLands() []interface{} {
	result := make([]interface{}, len(p.lands))
	for i, land := range p.lands {
		result[i] = land
	}
	return result
}

func (p *gamePlayer) CanPayCost(cost Cost) bool {
	// Check mana cost
	totalMana := 0
	for _, amount := range p.manaPool {
		totalMana += amount
	}

	requiredMana := 0
	for _, amount := range cost.ManaCost {
		requiredMana += amount
	}

	return totalMana >= requiredMana
}

func (p *gamePlayer) PayCost(cost Cost) error {
	// Pay mana cost
	requiredMana := 0
	for _, amount := range cost.ManaCost {
		requiredMana += amount
	}

	// Simple implementation: pay from any available mana
	totalMana := 0
	for _, amount := range p.manaPool {
		totalMana += amount
	}

	if totalMana < requiredMana {
		return fmt.Errorf("cannot pay cost: need %d mana, have %d", requiredMana, totalMana)
	}

	// Deduct mana (simplified)
	remaining := requiredMana
	for manaType, amount := range p.manaPool {
		if remaining <= 0 {
			break
		}

		deduct := amount
		if deduct > remaining {
			deduct = remaining
		}

		p.manaPool[manaType] -= deduct
		remaining -= deduct
	}

	return nil
}

func (p *gamePlayer) GetManaPool() map[game.ManaType]int {
	return p.manaPool
}

type gameCard struct {
	name       string
	manaCost   map[game.ManaType]int
	cardType   string
	oracleText string
}

type gamePermanent struct {
	id         uuid.UUID
	name       string
	owner      AbilityPlayer
	controller AbilityPlayer
	tapped     bool
	abilities  []*Ability
	power      int
	toughness  int
}

func (p *gamePermanent) GetID() uuid.UUID {
	return p.id
}

func (p *gamePermanent) GetName() string {
	return p.name
}

func (p *gamePermanent) GetOwner() AbilityPlayer {
	return p.owner
}

func (p *gamePermanent) GetController() AbilityPlayer {
	return p.controller
}

func (p *gamePermanent) IsTapped() bool {
	return p.tapped
}

func (p *gamePermanent) Tap() {
	p.tapped = true
}

func (p *gamePermanent) Untap() {
	p.tapped = false
}

func (p *gamePermanent) GetAbilities() []*Ability {
	return p.abilities
}

func (p *gamePermanent) AddAbility(ability *Ability) {
	p.abilities = append(p.abilities, ability)
}

func (p *gamePermanent) RemoveAbility(abilityID uuid.UUID) {
	for i, ability := range p.abilities {
		if ability.ID == abilityID {
			p.abilities = append(p.abilities[:i], p.abilities[i+1:]...)
			break
		}
	}
}

// Implement CreatureStats interface
func (p *gamePermanent) GetPower() int {
	return p.power
}

func (p *gamePermanent) GetToughness() int {
	return p.toughness
}

func (p *gamePermanent) GetCMC() int {
	// Simplified CMC calculation
	return 3 // Default for testing
}

// Implement TypeChecker interface
func (p *gamePermanent) IsCreature() bool {
	return strings.Contains(strings.ToLower(p.name), "creature") ||
		   p.power > 0 || p.toughness > 0
}

func (p *gamePermanent) IsArtifact() bool {
	return strings.Contains(strings.ToLower(p.name), "artifact")
}

func (p *gamePermanent) IsEnchantment() bool {
	return strings.Contains(strings.ToLower(p.name), "enchantment")
}

func (p *gamePermanent) IsLand() bool {
	return strings.Contains(strings.ToLower(p.name), "forest") ||
		   strings.Contains(strings.ToLower(p.name), "mountain") ||
		   strings.Contains(strings.ToLower(p.name), "island") ||
		   strings.Contains(strings.ToLower(p.name), "plains") ||
		   strings.Contains(strings.ToLower(p.name), "swamp")
}

func (p *gamePermanent) IsPlaneswalker() bool {
	return strings.Contains(strings.ToLower(p.name), "planeswalker")
}

type gameSimulationState struct {
	players       []AbilityPlayer
	currentPlayer AbilityPlayer
	turn          int
	phase         string
	isMainPhase   bool
	random        *rand.Rand
}

func (g *gameSimulationState) GetCurrentPlayer() AbilityPlayer {
	return g.currentPlayer
}

func (g *gameSimulationState) GetActivePlayer() AbilityPlayer {
	return g.currentPlayer
}

func (g *gameSimulationState) GetPlayer(name string) AbilityPlayer {
	for _, player := range g.players {
		if player.GetName() == name {
			return player
		}
	}
	return nil
}

func (g *gameSimulationState) GetAllPlayers() []AbilityPlayer {
	return g.players
}

func (g *gameSimulationState) IsMainPhase() bool {
	return g.isMainPhase
}

func (g *gameSimulationState) IsCombatPhase() bool {
	return g.phase == "combat"
}

func (g *gameSimulationState) CanActivateAbilities() bool {
	return g.isMainPhase
}

func (g *gameSimulationState) AddManaToPool(player AbilityPlayer, manaType game.ManaType, amount int) {
	if gamePlayer, ok := player.(*gamePlayer); ok {
		gamePlayer.manaPool[manaType] += amount
	}
}

func (g *gameSimulationState) DealDamage(source interface{}, target interface{}, amount int) {
	if player, ok := target.(*gamePlayer); ok {
		player.life -= amount
		if player.life < 0 {
			player.life = 0
		}
	}
}

func (g *gameSimulationState) DrawCard(player AbilityPlayer, count int) {
	// Simplified: just add generic cards to hand
	if gamePlayer, ok := player.(*gamePlayer); ok {
		for i := 0; i < count; i++ {
			card := gameCard{
				name:     fmt.Sprintf("Card %d", len(gamePlayer.hand)+1),
				cardType: "Instant",
			}
			gamePlayer.hand = append(gamePlayer.hand, card)
		}
	}
}

func (g *gameSimulationState) DrawCards(player AbilityPlayer, count int) {
	g.DrawCard(player, count)
}

func (g *gameSimulationState) GainLife(player AbilityPlayer, amount int) {
	if gamePlayer, ok := player.(*gamePlayer); ok {
		gamePlayer.life += amount
	}
}

func (g *gameSimulationState) LoseLife(player AbilityPlayer, amount int) {
	if gamePlayer, ok := player.(*gamePlayer); ok {
		gamePlayer.life -= amount
		if gamePlayer.life < 0 {
			gamePlayer.life = 0
		}
	}
}
