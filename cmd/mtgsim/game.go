// Game logic for MTGSim command-line application
package main

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/deck"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// PermanentType represents different types of permanents on the battlefield.
type PermanentType int

const (
	Creature PermanentType = iota
	Artifact
	Enchantment
	Land
	Planeswalker
)

// Permanent represents a permanent on the battlefield.
type Permanent struct {
	source            card.Card
	owner             *Player
	id                uuid.UUID
	tokenType         PermanentType
	tapped            bool
	summoningSickness bool
	manaProducer      bool
	manaTypes         []game.ManaType
	attacking         *Player
	blocking          *Permanent
	blockedBy         []*Permanent
	power             int
	toughness         int
	damage_counters   int
	goaded            bool
}

// tap taps the permanent
func (p *Permanent) tap() {
	p.tapped = true
}

// untap untaps the permanent
func (p *Permanent) untap() {
	p.tapped = false
}

// Player represents a Magic: The Gathering player and their board state.
type Player struct {
	Name          string
	LifeTotal     int
	Deck          deck.Deck
	Hand          []card.Card
	Graveyard     []card.Card
	Exile         []card.Card
	Creatures     []*Permanent
	Enchantments  []*Permanent
	Artifacts     []*Permanent
	Planeswalkers []*Permanent
	Lands         []*Permanent
	Opponents     []*Player
}

// Game represents a Magic: The Gathering game.
type Game struct {
	Players    []*Player
	turnNumber int
	cardDB     *card.CardDB
	winner     *Player
	loser      *Player
}

// NewGame creates a new game instance.
func NewGame(cardDB *card.CardDB) *Game {
	return &Game{
		turnNumber: 1,
		cardDB:     cardDB,
	}
}

// AddPlayer adds a player to the game by loading their deck.
func (g *Game) AddPlayer(decklistPath string) error {
	mainDeck, _, err := deck.ImportDeckfile(decklistPath, g.cardDB)
	if err != nil {
		return err
	}

	player := &Player{
		Name:          mainDeck.Name,
		LifeTotal:     20,
		Deck:          mainDeck,
		Hand:          make([]card.Card, 0),
		Graveyard:     make([]card.Card, 0),
		Exile:         make([]card.Card, 0),
		Creatures:     make([]*Permanent, 0),
		Enchantments:  make([]*Permanent, 0),
		Artifacts:     make([]*Permanent, 0),
		Planeswalkers: make([]*Permanent, 0),
		Lands:         make([]*Permanent, 0),
	}

	g.Players = append(g.Players, player)
	return nil
}

// Start begins the game and returns the winner and loser.
func (g *Game) Start() (*Player, *Player) {
	if len(g.Players) < 2 {
		logger.LogGame("Not enough players to start game")
		return nil, nil
	}

	// Setup game
	for i, p := range g.Players {
		p.Deck.Shuffle()
		p.Name = p.Deck.Name
		// Set opponents
		p.Opponents = make([]*Player, 0)
		for j, opponent := range g.Players {
			if i != j {
				p.Opponents = append(p.Opponents, opponent)
			}
		}
		// Draw opening hand
		p.Hand = append(p.Hand, p.Deck.DrawCards(7)...)
	}

	logger.LogGame("Starting game between %s and %s", g.Players[0].Name, g.Players[1].Name)

	// Main game loop with proper turn structure
	maxTurns := 100 // Prevent infinite games
	currentPlayerIndex := 0

	for g.turnNumber <= maxTurns {
		currentPlayer := g.Players[currentPlayerIndex]
		logger.LogPlayer("Turn %d: %s's turn", g.turnNumber, currentPlayer.Name)

		// Execute the turn
		g.executeTurn(currentPlayer)

		// Check for game end conditions
		if g.checkGameEnd() {
			return g.winner, g.loser
		}

		// Next player's turn
		currentPlayerIndex = (currentPlayerIndex + 1) % len(g.Players)
		if currentPlayerIndex == 0 {
			g.turnNumber++
		}
	}

	// Game went too long, declare first player winner
	logger.LogMeta("Game reached maximum turns, %s wins by default", g.Players[0].Name)
	return g.Players[0], g.Players[1]
}

// executeTurn executes a complete turn for the given player
func (g *Game) executeTurn(player *Player) {
	// Untap step
	g.untapStep(player)

	// Upkeep step
	g.upkeepStep(player)

	// Draw step
	g.drawStep(player)

	// Main phase 1
	g.mainPhase(player)

	// Combat phase
	g.combatPhase(player)

	// Main phase 2
	g.mainPhase(player)

	// End step
	g.endStep(player)
}

// checkGameEnd checks if the game has ended and sets winner/loser
func (g *Game) checkGameEnd() bool {
	for _, player := range g.Players {
		if player.LifeTotal <= 0 {
			g.loser = player
			// Find the winner (first opponent with positive life)
			for _, opponent := range player.Opponents {
				if opponent.LifeTotal > 0 {
					g.winner = opponent
					logger.LogMeta("Game Over: %s wins! %s loses with %d life",
						g.winner.Name, g.loser.Name, g.loser.LifeTotal)
					return true
				}
			}
		}

		// Check for deck out
		if player.Deck.IsEmpty() {
			g.loser = player
			for _, opponent := range player.Opponents {
				if !opponent.Deck.IsEmpty() {
					g.winner = opponent
					logger.LogMeta("Game Over: %s wins by decking! %s ran out of cards",
						g.winner.Name, g.loser.Name)
					return true
				}
			}
		}
	}
	return false
}

// untapStep untaps all permanents controlled by the player
func (g *Game) untapStep(player *Player) {
	logger.LogPlayer("%s: Untap step", player.Name)

	// Untap all permanents
	allPermanents := [][]*Permanent{
		player.Creatures, player.Artifacts, player.Enchantments,
		player.Planeswalkers, player.Lands,
	}

	for _, permanentList := range allPermanents {
		for _, permanent := range permanentList {
			permanent.tapped = false
			permanent.summoningSickness = false
		}
	}
}

// upkeepStep handles upkeep triggers
func (g *Game) upkeepStep(player *Player) {
	logger.LogPlayer("%s: Upkeep step", player.Name)
	// Placeholder for upkeep triggers
}

// drawStep draws a card for the active player
func (g *Game) drawStep(player *Player) {
	logger.LogPlayer("%s: Draw step", player.Name)

	if player.Deck.IsEmpty() {
		logger.LogPlayer("%s attempts to draw from an empty deck and loses the game!", player.Name)
		player.LifeTotal = 0
		return
	}

	drawnCard := player.Deck.DrawCard()
	player.Hand = append(player.Hand, drawnCard)
	logger.LogCard("%s draws %s", player.Name, drawnCard.Name)
}

// endStep handles end of turn cleanup
func (g *Game) endStep(player *Player) {
	logger.LogPlayer("%s: End step", player.Name)

	// Remove damage from creatures
	for _, creature := range player.Creatures {
		creature.damage_counters = 0
	}

	// Discard to hand size (7)
	handSize := len(player.Hand)
	if handSize > 7 {
		discardCount := handSize - 7
		logger.LogPlayer("%s discards %d cards to hand size", player.Name, discardCount)

		// Simple discard - remove from end of hand
		for i := 0; i < discardCount; i++ {
			if len(player.Hand) > 0 {
				discarded := player.Hand[len(player.Hand)-1]
				player.Hand = player.Hand[:len(player.Hand)-1]
				player.Graveyard = append(player.Graveyard, discarded)
				logger.LogCard("%s discards %s", player.Name, discarded.Name)
			}
		}
	}
}

// mainPhase handles the main phase where players can cast spells and play lands
func (g *Game) mainPhase(player *Player) {
	logger.LogPlayer("%s: Main phase", player.Name)

	// Simple AI: play a land if possible
	g.playLand(player)

	// Simple AI: cast creatures if possible
	g.castCreatures(player)
}

// playLand attempts to play a land from hand
func (g *Game) playLand(player *Player) {
	for i, cardInHand := range player.Hand {
		if cardInHand.IsLand() {
			// Remove from hand
			player.Hand = append(player.Hand[:i], player.Hand[i+1:]...)

			// Create permanent
			permanent := g.createPermanent(cardInHand, player, Land)
			player.Lands = append(player.Lands, permanent)

			logger.LogCard("%s plays %s", player.Name, cardInHand.Name)
			return // Only play one land per turn
		}
	}
}

// castCreatures attempts to cast creature spells
func (g *Game) castCreatures(player *Player) {
	for i := len(player.Hand) - 1; i >= 0; i-- {
		cardInHand := player.Hand[i]
		if cardInHand.IsCreature() {
			// Simple mana check - just check if we have enough lands
			availableMana := len(player.Lands)
			if availableMana >= int(cardInHand.CMC) {
				// Remove from hand
				player.Hand = append(player.Hand[:i], player.Hand[i+1:]...)

				// Create permanent
				permanent := g.createPermanent(cardInHand, player, Creature)
				permanent.summoningSickness = true
				player.Creatures = append(player.Creatures, permanent)

				logger.LogCard("%s casts %s", player.Name, cardInHand.Name)
			}
		}
	}
}

// createPermanent creates a permanent from a card
func (g *Game) createPermanent(source card.Card, owner *Player, pType PermanentType) *Permanent {
	permanent := &Permanent{
		source:            source,
		owner:             owner,
		id:                uuid.New(),
		tokenType:         pType,
		tapped:            false,
		summoningSickness: false,
		manaProducer:      false,
		manaTypes:         []game.ManaType{},
		attacking:         nil,
		blocking:          nil,
		blockedBy:         []*Permanent{},
		power:             0,
		toughness:         0,
		damage_counters:   0,
		goaded:            false,
	}

	// Set power and toughness for creatures
	if pType == Creature && source.IsCreature() {
		if power, err := strconv.Atoi(source.Power); err == nil {
			permanent.power = power
		}
		if toughness, err := strconv.Atoi(source.Toughness); err == nil {
			permanent.toughness = toughness
		}
	}

	// Check if it's a mana producer
	if isProducer, manaTypes := card.CheckManaProducer(source.OracleText); isProducer {
		permanent.manaProducer = true
		permanent.manaTypes = manaTypes
	}

	return permanent
}

// hasEvergreenAbility checks if a permanent has a specific evergreen ability
func (p *Permanent) hasEvergreenAbility(abilityName string) bool {
	for _, keyword := range p.source.Keywords {
		if strings.EqualFold(keyword, abilityName) {
			return true
		}
	}
	// Also check oracle text for abilities not in keywords
	return strings.Contains(strings.ToLower(p.source.OracleText), strings.ToLower(abilityName))
}

// canBlock checks if a blocker can legally block an attacker based on evasion abilities
func (blocker *Permanent) canBlock(attacker *Permanent) bool {
	// Flying: can only be blocked by creatures with flying or reach
	if attacker.hasEvergreenAbility("Flying") {
		if !blocker.hasEvergreenAbility("Flying") && !blocker.hasEvergreenAbility("Reach") {
			return false
		}
	}

	// Fear: can only be blocked by artifact creatures or black creatures
	if attacker.hasEvergreenAbility("Fear") {
		isArtifact := strings.Contains(blocker.source.TypeLine, "Artifact")
		isBlack := false
		for _, color := range blocker.source.Colors {
			if color == "B" {
				isBlack = true
				break
			}
		}
		if !isArtifact && !isBlack {
			return false
		}
	}

	// Intimidate: can only be blocked by artifact creatures or creatures that share a color
	if attacker.hasEvergreenAbility("Intimidate") {
		isArtifact := strings.Contains(blocker.source.TypeLine, "Artifact")
		if !isArtifact {
			// Check if they share a color
			sharesColor := false
			for _, attackerColor := range attacker.source.Colors {
				for _, blockerColor := range blocker.source.Colors {
					if attackerColor == blockerColor {
						sharesColor = true
						break
					}
				}
				if sharesColor {
					break
				}
			}
			if !sharesColor {
				return false
			}
		}
	}

	// Shadow: can only be blocked by creatures with shadow
	if attacker.hasEvergreenAbility("Shadow") {
		if !blocker.hasEvergreenAbility("Shadow") {
			return false
		}
	}

	// Horsemanship: can only be blocked by creatures with horsemanship
	if attacker.hasEvergreenAbility("Horsemanship") {
		if !blocker.hasEvergreenAbility("Horsemanship") {
			return false
		}
	}

	// Unblockable: cannot be blocked at all
	if attacker.hasEvergreenAbility("Unblockable") ||
	   strings.Contains(strings.ToLower(attacker.source.OracleText), "can't be blocked") ||
	   strings.Contains(strings.ToLower(attacker.source.OracleText), "unblockable") {
		return false
	}

	// Protection: check if blocker has qualities the attacker has protection from
	if strings.Contains(strings.ToLower(attacker.source.OracleText), "protection from") {
		// This would need more sophisticated parsing, but for now we'll skip it
		// as it's complex to parse "protection from red" vs "protection from artifacts" etc.
	}

	return true
}

// combatPhase handles the combat phase
func (g *Game) combatPhase(player *Player) {
	logger.LogPlayer("%s: Combat phase", player.Name)

	// Declare attackers
	attackers := g.declareAttackers(player)
	if len(attackers) == 0 {
		logger.LogPlayer("%s: No attackers declared", player.Name)
		return
	}

	// Declare blockers (for each opponent)
	for _, opponent := range player.Opponents {
		g.declareBlockers(opponent, attackers)
	}

	// Resolve combat damage
	g.resolveCombatDamage(attackers)
}

// declareAttackers chooses which creatures attack
func (g *Game) declareAttackers(player *Player) []*Permanent {
	var attackers []*Permanent

	for _, creature := range player.Creatures {
		// Can attack if not tapped and no summoning sickness and doesn't have defender
		if !creature.tapped && !creature.summoningSickness && creature.power > 0 {
			// Check if creature has defender (can't attack)
			if creature.hasEvergreenAbility("Defender") {
				continue
			}

			// Simple AI: attack with all available creatures
			if len(player.Opponents) > 0 {
				creature.attacking = player.Opponents[0] // Attack first opponent

				// Attacking taps the creature unless it has vigilance
				if !creature.hasEvergreenAbility("Vigilance") {
					creature.tapped = true
				}

				attackers = append(attackers, creature)
				logger.LogCard("%s attacks with %s (%d/%d)",
					player.Name, creature.source.Name, creature.power, creature.toughness)
			}
		}
	}

	return attackers
}

// declareBlockers chooses which creatures block
func (g *Game) declareBlockers(defender *Player, attackers []*Permanent) {
	for _, attacker := range attackers {
		if attacker.attacking == defender {
			// Simple AI: block with first available creature that can legally block and survive or trade
			for _, blocker := range defender.Creatures {
				if !blocker.tapped && blocker.power > 0 {
					// Check if this blocker can legally block the attacker (evasion abilities)
					if !blocker.canBlock(attacker) {
						logger.LogCard("%s cannot block %s (evasion)",
							blocker.source.Name, attacker.source.Name)
						continue
					}

					// Block if we can kill the attacker or if we're going to die anyway
					if blocker.power >= attacker.toughness || blocker.toughness <= attacker.power {
						blocker.blocking = attacker
						attacker.blockedBy = append(attacker.blockedBy, blocker)
						logger.LogCard("%s blocks %s with %s",
							defender.Name, attacker.source.Name, blocker.source.Name)
						break // Only one blocker per attacker for simplicity
					}
				}
			}
		}
	}
}

// resolveCombatDamage resolves all combat damage with first strike and double strike
func (g *Game) resolveCombatDamage(attackers []*Permanent) {
	// Collect all creatures in combat
	allCombatants := make(map[*Permanent]bool)
	for _, attacker := range attackers {
		allCombatants[attacker] = true
		for _, blocker := range attacker.blockedBy {
			allCombatants[blocker] = true
		}
	}

	// Track which creatures have already dealt damage
	hasDealtDamage := make(map[*Permanent]bool)

	// First Strike Damage Step
	logger.LogPlayer("First Strike Damage Step")
	for creature := range allCombatants {
		if creature.hasEvergreenAbility("First Strike") || creature.hasEvergreenAbility("Double Strike") {
			logger.LogCard("First strike: %s dealing damage", creature.source.Name)
			g.dealCombatDamageForCreature(creature)
			hasDealtDamage[creature] = true
		}
	}

	g.checkStateBasedActions()

	// Regular Damage Step
	logger.LogPlayer("Regular Damage Step")
	for creature := range allCombatants {
		// Only deal damage if the creature hasn't already dealt damage (unless it has double strike)
		if !hasDealtDamage[creature] || creature.hasEvergreenAbility("Double Strike") {
			logger.LogCard("Regular damage: %s dealing damage (hasDealtDamage: %v, doubleStrike: %v)",
				creature.source.Name, hasDealtDamage[creature], creature.hasEvergreenAbility("Double Strike"))
			g.dealCombatDamageForCreature(creature)
		}
	}

	// Reset combat state
	for _, attacker := range attackers {
		attacker.attacking = nil
		attacker.blockedBy = []*Permanent{}
	}
	for creature := range allCombatants {
		creature.blocking = nil
	}

	// Check for destroyed creatures and remove them
	g.checkStateBasedActions()
}

// dealCombatDamageForCreature deals combat damage for a single creature
func (g *Game) dealCombatDamageForCreature(creature *Permanent) {
	if creature.attacking != nil {
		// This creature is attacking
		if len(creature.blockedBy) > 0 {
			// Blocked combat
			for _, blocker := range creature.blockedBy {
				g.dealDamageBetweenCreatures(creature, blocker)
			}
		} else {
			// Unblocked - damage goes to defending player
			damage := creature.power

			// Handle trample (excess damage goes to player even if blocked)
			if creature.hasEvergreenAbility("Trample") && len(creature.blockedBy) > 0 {
				totalBlockerToughness := 0
				for _, blocker := range creature.blockedBy {
					totalBlockerToughness += blocker.toughness
				}
				if damage > totalBlockerToughness {
					damage = damage - totalBlockerToughness
				} else {
					damage = 0
				}
			}

			if damage > 0 && creature.attacking != nil {
				creature.attacking.LifeTotal -= damage
				logger.LogCard("%s deals %d damage to %s (Life: %d)",
					creature.source.Name, damage,
					creature.attacking.Name, creature.attacking.LifeTotal)

				// Handle lifelink
				if creature.hasEvergreenAbility("Lifelink") {
					creature.owner.LifeTotal += damage
					logger.LogCard("%s gains %d life from lifelink (Life: %d)",
						creature.owner.Name, damage, creature.owner.LifeTotal)
				}
			}
		}
	} else if creature.blocking != nil {
		// This creature is blocking - damage is already dealt when the attacker deals damage
		// No need to deal damage again here
	}
}

// dealDamageBetweenCreatures deals damage between two creatures in combat
func (g *Game) dealDamageBetweenCreatures(creature1, creature2 *Permanent) {
	// Calculate damage amounts
	damage1to2 := creature1.power
	damage2to1 := creature2.power

	// Double strike deals damage twice (but this is handled in the combat damage step)
	// For this function, we just deal the base damage

	// Deal damage to each other
	creature1.damage_counters += damage2to1
	creature2.damage_counters += damage1to2

	logger.LogCard("Combat: %s deals %d damage to %s, %s deals %d damage to %s",
		creature1.source.Name, damage1to2, creature2.source.Name,
		creature2.source.Name, damage2to1, creature1.source.Name)

	// Handle lifelink
	if creature1.hasEvergreenAbility("Lifelink") {
		creature1.owner.LifeTotal += damage1to2
		logger.LogCard("%s gains %d life from lifelink", creature1.owner.Name, damage1to2)
	}
	if creature2.hasEvergreenAbility("Lifelink") {
		creature2.owner.LifeTotal += damage2to1
		logger.LogCard("%s gains %d life from lifelink", creature2.owner.Name, damage2to1)
	}
}

// checkStateBasedActions removes destroyed creatures
func (g *Game) checkStateBasedActions() {
	for _, player := range g.Players {
		// Check creatures for lethal damage
		var survivingCreatures []*Permanent
		for _, creature := range player.Creatures {
			shouldDie := false

			// Check for lethal damage
			if creature.damage_counters >= creature.toughness {
				shouldDie = true
			}

			// Check for deathtouch damage
			if creature.damage_counters > 0 {
				// Check if any damage source had deathtouch
				// For simplicity, we'll check if the creature was in combat with a deathtouch creature
				for _, attacker := range g.getAllAttackers() {
					if attacker.hasEvergreenAbility("Deathtouch") &&
					   g.wasInCombatWith(creature, attacker) {
						shouldDie = true
						logger.LogCard("%s dies to deathtouch from %s",
							creature.source.Name, attacker.source.Name)
						break
					}
				}
				for _, blocker := range g.getAllBlockers() {
					if blocker.hasEvergreenAbility("Deathtouch") &&
					   g.wasInCombatWith(creature, blocker) {
						shouldDie = true
						logger.LogCard("%s dies to deathtouch from %s",
							creature.source.Name, blocker.source.Name)
						break
					}
				}
			}

			// Indestructible prevents destruction
			if shouldDie && creature.hasEvergreenAbility("Indestructible") {
				shouldDie = false
				logger.LogCard("%s is indestructible and survives", creature.source.Name)
			}

			if shouldDie {
				// Creature dies
				player.Graveyard = append(player.Graveyard, creature.source)
				logger.LogCard("%s's %s dies", player.Name, creature.source.Name)
			} else {
				// Reset blocking state
				creature.blocking = nil
				survivingCreatures = append(survivingCreatures, creature)
			}
		}
		player.Creatures = survivingCreatures
	}
}

// Helper functions for deathtouch checking
func (g *Game) getAllAttackers() []*Permanent {
	var attackers []*Permanent
	for _, player := range g.Players {
		for _, creature := range player.Creatures {
			if creature.attacking != nil {
				attackers = append(attackers, creature)
			}
		}
	}
	return attackers
}

func (g *Game) getAllBlockers() []*Permanent {
	var blockers []*Permanent
	for _, player := range g.Players {
		for _, creature := range player.Creatures {
			if creature.blocking != nil {
				blockers = append(blockers, creature)
			}
		}
	}
	return blockers
}

func (g *Game) wasInCombatWith(creature1, creature2 *Permanent) bool {
	// Check if creature1 was blocking creature2 or vice versa
	if creature1.blocking == creature2 || creature2.blocking == creature1 {
		return true
	}
	// Check if they were in the same combat (one attacking, one blocking)
	for _, blocker := range creature1.blockedBy {
		if blocker == creature2 {
			return true
		}
	}
	for _, blocker := range creature2.blockedBy {
		if blocker == creature1 {
			return true
		}
	}
	return false
}
