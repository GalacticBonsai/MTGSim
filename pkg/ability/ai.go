// Package ability provides AI decision-making for ability activation.
package ability

import (
	"math/rand"
	"sort"
	"time"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/combo"
)

// AbilityPriority represents the priority level for different ability types.
type AbilityPriority int

const (
	PriorityLow AbilityPriority = iota
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

// AIDecisionMaker makes decisions about when and how to activate abilities.
type AIDecisionMaker struct {
	engine       *ExecutionEngine
	priorities   map[EffectType]AbilityPriority
	rng          *rand.Rand
	comboIndices map[string]*combo.Index // keyed by player/deck name
}

// NewAIDecisionMaker creates a new AI decision maker.
func NewAIDecisionMaker(engine *ExecutionEngine) *AIDecisionMaker {
	ai := &AIDecisionMaker{
		engine:       engine,
		priorities:   make(map[EffectType]AbilityPriority),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
		comboIndices: make(map[string]*combo.Index),
	}
	ai.initializePriorities()
	return ai
}

// SetComboIndex attaches a combo index for a specific player so the AI can
// prioritize that player's combo pieces.
func (ai *AIDecisionMaker) SetComboIndex(playerName string, ci *combo.Index) {
	ai.comboIndices[playerName] = ci
}

// initializePriorities sets up the default priority levels for different effect types.
func (ai *AIDecisionMaker) initializePriorities() {
	// Mana abilities are highest priority (needed for everything else)
	ai.priorities[AddMana] = PriorityCritical

	// Card advantage is very important
	ai.priorities[DrawCards] = PriorityHigh

	// Direct damage and removal are important
	ai.priorities[DealDamage] = PriorityHigh
	ai.priorities[DestroyPermanent] = PriorityHigh

	// Life gain/loss is medium priority
	ai.priorities[GainLife] = PriorityMedium
	ai.priorities[LoseLife] = PriorityMedium

	// Creature pumping is situational
	ai.priorities[PumpCreature] = PriorityMedium

	// Utility effects are lower priority
	ai.priorities[TapUntap] = PriorityLow
	ai.priorities[SearchLibrary] = PriorityLow
}

// DecisionContext provides context for AI decision making.
type DecisionContext struct {
	Player              AbilityPlayer
	Opponents           []AbilityPlayer
	Phase               string
	AvailableMana       int
	HandSize            int
	BoardState          BoardState
	ThreatLevel         int
	CanCastMoreSpells   bool
	ComboPiecesInHand   []string // card names in hand that are combo pieces
	MissingComboPieces  []string // card names that would complete a partial combo
}

// BoardState represents the current state of the battlefield.
type BoardState struct {
	MyCreatures       int
	MyCreaturePower   int
	OpponentCreatures int
	OpponentPower     int
	MyLands           int
	OpponentLands     int
	MyArtifacts       int
	MyEnchantments    int
	OpponentArtifacts int
	OpponentEnchantments int
	MyUtilityPerms    int // high-utility non-creature permanents (combo, lock, stax)
	OpponentLockPerms int // opponent permanents that disrupt our strategy
}

// ShouldActivateAbilities determines if the AI should look for abilities to activate.
func (ai *AIDecisionMaker) ShouldActivateAbilities(context DecisionContext) bool {
	// Always activate mana abilities when needed
	if context.CanCastMoreSpells && context.AvailableMana < 3 {
		return true
	}

	// Activate abilities when we have excess mana
	if context.AvailableMana > 5 {
		return true
	}

	// Activate abilities when under threat
	if context.ThreatLevel > 3 {
		return true
	}

	// Activate abilities when we have few cards in hand
	if context.HandSize < 3 {
		return true
	}

	// Random chance to activate abilities for variety
	return ai.rng.Float32() < 0.3
}

// ChooseAbilitiesToActivate selects which abilities to activate based on the current context.
func (ai *AIDecisionMaker) ChooseAbilitiesToActivate(abilities []*Ability, context DecisionContext) []*Ability {
	if len(abilities) == 0 {
		return nil
	}

	// Score each ability based on current context
	scoredAbilities := ai.scoreAbilities(abilities, context)

	// Sort by score (highest first)
	sort.Slice(scoredAbilities, func(i, j int) bool {
		return scoredAbilities[i].Score > scoredAbilities[j].Score
	})

	// Select abilities to activate based on available mana and priorities
	var toActivate []*Ability
	remainingMana := context.AvailableMana

	for _, scored := range scoredAbilities {
		ability := scored.Ability
		cost := ai.calculateManaCost(ability)

		// Check if we can afford this ability
		if cost <= remainingMana {
			// Check if we should activate this ability
			if ai.shouldActivateSpecificAbility(ability, context, scored.Score) {
				toActivate = append(toActivate, ability)
				remainingMana -= cost

				// Don't activate too many abilities at once
				if len(toActivate) >= 3 {
					break
				}
			}
		}
	}

	return toActivate
}

// ScoredAbility represents an ability with its calculated score.
type ScoredAbility struct {
	Ability *Ability
	Score   float64
}

// scoreAbilities calculates scores for abilities based on the current context.
func (ai *AIDecisionMaker) scoreAbilities(abilities []*Ability, context DecisionContext) []ScoredAbility {
	var scored []ScoredAbility

	for _, ability := range abilities {
		score := ai.scoreAbility(ability, context)
		scored = append(scored, ScoredAbility{
			Ability: ability,
			Score:   score,
		})
	}

	return scored
}

// scoreAbility calculates a score for a single ability.
func (ai *AIDecisionMaker) scoreAbility(ability *Ability, context DecisionContext) float64 {
	baseScore := 0.0

	// Base score from priority
	for _, effect := range ability.Effects {
		priority := ai.priorities[effect.Type]
		switch priority {
		case PriorityCritical:
			baseScore += 10.0
		case PriorityHigh:
			baseScore += 7.0
		case PriorityMedium:
			baseScore += 4.0
		case PriorityLow:
			baseScore += 1.0
		}

		// Adjust score based on effect type and context
		baseScore += ai.scoreEffectInContext(effect, context)
	}

	// Adjust for mana cost efficiency
	manaCost := ai.calculateManaCost(ability)
	if manaCost > 0 {
		baseScore = baseScore / float64(manaCost) * 2.0 // Favor cheaper abilities
	}

	// Adjust for timing
	baseScore += ai.scoreTimingContext(ability, context)

	// Add some randomness for variety
	baseScore += ai.rng.Float64() * 0.5

	return baseScore
}

// scoreEffectInContext adjusts the score based on how useful the effect is in the current context.
func (ai *AIDecisionMaker) scoreEffectInContext(effect Effect, context DecisionContext) float64 {
	switch effect.Type {
	case AddMana:
		// Mana is more valuable when we can cast more spells
		if context.CanCastMoreSpells {
			return 3.0
		}
		return 0.5

	case DrawCards:
		// Card draw is more valuable when we have few cards
		if context.HandSize < 3 {
			return 4.0
		} else if context.HandSize < 5 {
			return 2.0
		}
		return 1.0

	case DealDamage:
		// Damage is more valuable when opponent has creatures or low life
		score := 2.0
		if context.BoardState.OpponentCreatures > 0 {
			score += 2.0
		}
		// Assume we can target opponent's life total
		score += 1.0
		return score

	case GainLife:
		// Life gain is more valuable when we're low on life
		// We don't have access to current life, so use a base value
		return 1.5

	case PumpCreature:
		// Creature pumping is more valuable when we have creatures and are attacking
		if context.BoardState.MyCreatures > 0 {
			if context.Phase == "Combat" {
				return 3.0
			}
			return 1.5
		}
		return 0.5

	case DestroyPermanent:
		// Removal is more valuable when opponent has threats
		if context.BoardState.OpponentCreatures > 0 {
			score := 4.0
			// Extra value for destroying lock pieces
			if context.BoardState.OpponentLockPerms > 0 {
				score += 3.0
			}
			return score
		}
		return 1.0

	case SearchLibrary:
		// Tutoring is high priority when we have few cards or need combo pieces
		if context.HandSize < 3 {
			return 5.0
		}
		if context.BoardState.MyUtilityPerms == 0 {
			return 3.5 // likely need to find ramp/combo engine
		}
		// Boost tutoring significantly when we hold combo pieces and are missing a key card.
		if len(context.MissingComboPieces) > 0 && len(context.ComboPiecesInHand) > 0 {
			return 6.0
		}
		return 2.0

	case TapUntap:
		// Tapping opponent lock pieces is valuable
		if context.BoardState.OpponentLockPerms > 0 {
			return 3.0
		}
		return 0.5

	default:
		return 1.0
	}
}

// scoreTimingContext adjusts score based on when the ability can be activated.
func (ai *AIDecisionMaker) scoreTimingContext(ability *Ability, context DecisionContext) float64 {
	switch ability.TimingRestriction {
	case AnyTime:
		return 1.0 // No penalty for flexible timing
	case SorcerySpeed:
		if context.Phase == "Main" {
			return 0.5 // Slight penalty for timing restriction
		}
		return -5.0 // Heavy penalty if we can't activate now
	case OnlyDuringCombat:
		if context.Phase == "Combat" {
			return 1.0
		}
		return -5.0
	case OnlyOnYourTurn:
		return 0.8 // Slight penalty for turn restriction
	default:
		return 0.0
	}
}

// shouldActivateSpecificAbility determines if a specific ability should be activated.
func (ai *AIDecisionMaker) shouldActivateSpecificAbility(ability *Ability, context DecisionContext, score float64) bool {
	// Always activate high-scoring abilities
	if score > 8.0 {
		return true
	}

	// Activate medium-scoring abilities based on context
	if score > 5.0 {
		// More likely to activate if we have excess mana
		if context.AvailableMana > 4 {
			return true
		}
		// More likely if we're under threat
		if context.ThreatLevel > 2 {
			return true
		}
		// Random chance
		return ai.rng.Float32() < 0.7
	}

	// Activate low-scoring abilities only if we have lots of excess mana
	if score > 2.0 && context.AvailableMana > 6 {
		return ai.rng.Float32() < 0.4
	}

	return false
}

// calculateManaCost calculates the total mana cost of an ability.
func (ai *AIDecisionMaker) calculateManaCost(ability *Ability) int {
	totalCost := 0
	for _, amount := range ability.Cost.ManaCost {
		totalCost += amount
	}
	return totalCost
}

// ActivateAbilitiesForPlayer activates abilities for a player based on AI decisions.
func (ai *AIDecisionMaker) ActivateAbilitiesForPlayer(player AbilityPlayer, phase string) {
	// Build decision context
	context := ai.buildDecisionContext(player, phase)

	// Check if we should activate abilities at all
	if !ai.ShouldActivateAbilities(context) {
		return
	}

	// First, activate mana abilities if needed
	if context.CanCastMoreSpells || context.AvailableMana < 2 {
		manaAdded := ai.engine.ActivateManaAbilities(player)
		if manaAdded > 0 {
			logger.LogPlayer("%s activates mana abilities, adding %d mana", player.GetName(), manaAdded)
			context.AvailableMana += manaAdded
		}
	}

	// Get all activatable abilities
	abilities := ai.engine.GetActivatableAbilities(player)
	if len(abilities) == 0 {
		return
	}

	// Choose which abilities to activate
	toActivate := ai.ChooseAbilitiesToActivate(abilities, context)

	// Activate chosen abilities
	for _, ability := range toActivate {
		targets := ai.chooseTargets(ability, context)
		err := ai.engine.ExecuteAbility(ability, player, targets)
		if err != nil {
			logger.LogPlayer("Failed to activate %s: %v", ability.Name, err)
		} else {
			logger.LogPlayer("%s activates %s", player.GetName(), ability.Name)
		}
	}
}

// Exported helpers for spell casting integration

// BuildDecisionContext exposes the decision context builder for external callers (e.g., spell AI).
func (ai *AIDecisionMaker) BuildDecisionContext(player AbilityPlayer, opponents []AbilityPlayer, phase string) DecisionContext {
	ctx := ai.buildDecisionContext(player, phase)
	ctx.Opponents = opponents

	// Populate opponent board state for synergy-aware targeting
	for _, opp := range opponents {
		ctx.BoardState.OpponentCreatures += len(opp.GetCreatures())
		ctx.BoardState.OpponentLands += len(opp.GetLands())
		for range opp.GetCreatures() {
			ctx.BoardState.OpponentPower += 2
		}
		ctx.BoardState.OpponentLockPerms += estimateLockPermanents(opp)
	}
	ctx.BoardState.MyUtilityPerms = estimateUtilityPermanents(player)

	if ctx.BoardState.OpponentLockPerms > 0 {
		ctx.ThreatLevel += 3
	}

	// Populate combo awareness if a combo index is attached.
	if ci := ai.comboIndices[player.GetName()]; ci != nil {
		handNames := handCardNames(player)
		ctx.ComboPiecesInHand = ci.ComboPiecesInHand(handNames)
		ctx.MissingComboPieces = ci.MissingPiecesForHand(handNames)
	}

	return ctx
}

// handCardNames extracts card names from a player's hand.
func handCardNames(player AbilityPlayer) []string {
	hand := player.GetHand()
	names := make([]string, 0, len(hand))
	for _, c := range hand {
		if s, ok := c.(interface{ GetName() string }); ok {
			names = append(names, s.GetName())
		}
	}
	return names
}

// ChooseTargetsFor exposes target selection for a given (parsed) ability using the AI logic.
func (ai *AIDecisionMaker) ChooseTargetsFor(ability *Ability, context DecisionContext) []interface{} {
	return ai.chooseTargets(ability, context)
}

// buildDecisionContext builds a decision context for the given player.
func (ai *AIDecisionMaker) buildDecisionContext(player AbilityPlayer, phase string) DecisionContext {
	// Calculate available mana
	availableMana := 0
	for _, amount := range player.GetManaPool() {
		availableMana += amount
	}

	// Add untapped lands (simplified)
	for _, land := range player.GetLands() {
		if permanent, ok := land.(AbilityPermanent); ok {
			if !permanent.IsTapped() {
				availableMana++
			}
		}
	}

	// Build board state
	boardState := BoardState{
		MyCreatures: len(player.GetCreatures()),
		MyLands:     len(player.GetLands()),
	}

	// Calculate creature power (simplified)
	for range player.GetCreatures() {
		boardState.MyCreaturePower += 2 // Assume average power of 2
	}

	// Calculate threat level (simplified)
	threatLevel := 0
	if boardState.OpponentCreatures > boardState.MyCreatures {
		threatLevel += 2
	}
	if boardState.OpponentPower > boardState.MyCreaturePower {
		threatLevel += 2
	}

	return DecisionContext{
		Player:            player,
		Phase:             phase,
		AvailableMana:     availableMana,
		HandSize:          len(player.GetHand()),
		BoardState:        boardState,
		ThreatLevel:       threatLevel,
		CanCastMoreSpells: availableMana >= 2 && len(player.GetHand()) > 0,
	}
}

// estimateUtilityPermanents returns a rough count of high-utility non-creature
// permanents controlled by the player (artifacts/enchantments).
func estimateUtilityPermanents(player AbilityPlayer) int {
	utility := 0
	// Lands are not utility for this heuristic; artifacts/enchantments are.
	for _, perm := range player.GetLands() {
		if p, ok := perm.(AbilityPermanent); ok {
			name := p.GetName()
			if isHighUtilityPermanent(name) {
				utility++
			}
		}
	}
	return utility
}

// estimateLockPermanents counts opponent permanents that disrupt our strategy.
func estimateLockPermanents(opponent AbilityPlayer) int {
	lock := 0
	for _, perm := range opponent.GetLands() {
		if p, ok := perm.(AbilityPermanent); ok {
			name := p.GetName()
			if isLockPiece(name) {
				lock++
			}
		}
	}
	return lock
}

// isHighUtilityPermanent heuristically detects combo-enablers and engines.
func isHighUtilityPermanent(name string) bool {
	switch name {
	case "Sol Ring", "Mana Crypt", "Grim Monolith", "Basalt Monolith",
		"Gilded Lotus", "Thran Dynamo", "Chrome Mox", "Mox Diamond",
		"Mind's Eye", "Rhystic Study", "Mystic Remora",
		"Sylvan Library", "Sensei's Divining Top", "Skullclamp",
		"Ashnod's Altar", "Phyrexian Altar", "Earthcraft",
		"Intruder Alarm", "Training Grounds":
		return true
	}
	return false
}

// isLockPiece heuristically detects stax/prison pieces that shut down strategies.
func isLockPiece(name string) bool {
	switch name {
	case "Null Rod", "Stony Silence", "Collector Ouphe", "Kataki, War's Wage",
		"Rule of Law", "Eidolon of Rhetoric", "Arcane Laboratory",
		"Blood Moon", "Magus of the Moon", "Contamination",
		"Trinisphere", "Sphere of Resistance", "Thorn of Amethyst",
		"Thalia, Guardian of Thraben", "Glowrider", "Vryn Wingmare",
		"Armageddon", "Ravages of War", "Smokestack", "Tangle Wire",
		"Static Orb", "Winter Orb":
		return true
	}
	return false
}

// chooseTargets chooses targets for an ability using enhanced targeting validation.
func (ai *AIDecisionMaker) chooseTargets(ability *Ability, context DecisionContext) []interface{} {
	var targets []interface{}

	for _, effect := range ability.Effects {
		for _, targetReq := range effect.Targets {
			if targetReq.Required {
				var chosenTarget interface{}

				// Use enhanced targeting if available
				if targetReq.Enhanced != nil {
					chosenTarget = ai.chooseEnhancedTarget(*targetReq.Enhanced, effect, context)
				} else {
					// Fallback to basic targeting
					chosenTarget = ai.chooseBasicTarget(targetReq, effect, context)
				}

				if chosenTarget != nil {
					targets = append(targets, chosenTarget)
				}
			}
		}
	}

	return targets
}

// chooseEnhancedTarget chooses a target using the enhanced targeting system.
func (ai *AIDecisionMaker) chooseEnhancedTarget(enhancedTarget EnhancedTarget, effect Effect, context DecisionContext) interface{} {
	// Get all potential targets
	potentialTargets := ai.getPotentialTargets(enhancedTarget.Type, context)

	// Filter targets that pass validation
	var validTargets []interface{}
	for _, target := range potentialTargets {
		legality := ai.engine.targetValidator.ValidateTarget(target, enhancedTarget, context.Player)
		if legality.IsLegal {
			validTargets = append(validTargets, target)
		}
	}

	if len(validTargets) == 0 {
		return nil
	}

	// Choose the best target based on effect type and AI strategy
	return ai.chooseBestTarget(validTargets, effect, context)
}

// chooseBasicTarget provides fallback target selection for basic targets.
func (ai *AIDecisionMaker) chooseBasicTarget(targetReq Target, effect Effect, context DecisionContext) interface{} {
	switch targetReq.Type {
	case CreatureTarget:
		// Choose a creature (prefer opponent's creatures for damage, our creatures for pumping)
		if effect.Type == DealDamage || effect.Type == DestroyPermanent {
			// Target opponent's creatures (simplified)
			return "opponent_creature"
		} else {
			// Target our creatures
			if len(context.Player.GetCreatures()) > 0 {
				return context.Player.GetCreatures()[0]
			}
		}
	case PlayerTarget:
		// Target opponent for damage, self for beneficial effects
		if effect.Type == DealDamage || effect.Type == LoseLife {
			return "opponent"
		} else {
			return context.Player
		}
	case AnyTarget:
		// Choose the most beneficial target
		if effect.Type == DealDamage {
			return "opponent"
		} else {
			return context.Player
		}
	}
	return nil
}

// getPotentialTargets gets all potential targets of a given type for AI decision making.
func (ai *AIDecisionMaker) getPotentialTargets(targetType TargetType, context DecisionContext) []interface{} {
	var targets []interface{}

	switch targetType {
	case CreatureTarget:
		// Get all creatures on the battlefield
		targets = append(targets, context.Player.GetCreatures()...)
		for _, opponent := range context.Opponents {
			targets = append(targets, opponent.GetCreatures()...)
		}
	case PlayerTarget:
		// Get all players
		targets = append(targets, context.Player)
		for _, opponent := range context.Opponents {
			targets = append(targets, opponent)
		}
	case PermanentTarget:
		// Get all permanents
		targets = append(targets, context.Player.GetCreatures()...)
		targets = append(targets, context.Player.GetLands()...)
		for _, opponent := range context.Opponents {
			targets = append(targets, opponent.GetCreatures()...)
			targets = append(targets, opponent.GetLands()...)
		}
	case AnyTarget:
		// Get all valid targets (players and permanents)
		targets = append(targets, context.Player)
		targets = append(targets, context.Player.GetCreatures()...)
		targets = append(targets, context.Player.GetLands()...)
		for _, opponent := range context.Opponents {
			targets = append(targets, opponent)
			targets = append(targets, opponent.GetCreatures()...)
			targets = append(targets, opponent.GetLands()...)
		}
	}

	return targets
}

// chooseBestTarget selects the best target from valid options based on AI strategy.
func (ai *AIDecisionMaker) chooseBestTarget(validTargets []interface{}, effect Effect, context DecisionContext) interface{} {
	if len(validTargets) == 0 {
		return nil
	}

	// Helper: check if a card name is a combo piece for a given player name.
	isComboPieceFor := func(name, playerName string) bool {
		if ci := ai.comboIndices[playerName]; ci != nil {
			return ci.IsComboPiece(name)
		}
		return false
	}

	// Helper: check if a card name is a combo piece for any opponent.
	isOpponentComboPiece := func(name string) bool {
		for _, opp := range context.Opponents {
			if isComboPieceFor(name, opp.GetName()) {
				return true
			}
		}
		return false
	}

	// Simple strategy: prefer opponents for harmful effects, self for beneficial effects
	switch effect.Type {
	case DealDamage, DestroyPermanent, LoseLife, TapUntap:
		// Prefer opponent targets, especially lock pieces or combo pieces when destroying/tapping
		for _, target := range validTargets {
			if player, ok := target.(AbilityPlayer); ok {
				if player.GetName() != context.Player.GetName() {
					return target
				}
			}
		}
		// For permanents: prefer opponent combo pieces, then lock pieces
		for _, target := range validTargets {
			if permanent, ok := target.(AbilityPermanent); ok {
				if permanent.GetController().GetName() != context.Player.GetName() {
					if isOpponentComboPiece(permanent.GetName()) {
						return target
					}
				}
			}
		}
		for _, target := range validTargets {
			if permanent, ok := target.(AbilityPermanent); ok {
				if permanent.GetController().GetName() != context.Player.GetName() {
					if isLockPiece(permanent.GetName()) {
						return target
					}
				}
			}
		}
		// Fallback: any opponent permanent
		for _, target := range validTargets {
			if permanent, ok := target.(AbilityPermanent); ok {
				if permanent.GetController().GetName() != context.Player.GetName() {
					return target
				}
			}
		}
	case DrawCards, GainLife, PumpCreature:
		// Prefer own targets; for pump, prefer our combo-enabling creatures
		for _, target := range validTargets {
			if player, ok := target.(AbilityPlayer); ok {
				if player.GetName() == context.Player.GetName() {
					return target
				}
			}
		}
		// Prefer protecting our combo pieces, then utility permanents
		for _, target := range validTargets {
			if permanent, ok := target.(AbilityPermanent); ok {
				if permanent.GetController().GetName() == context.Player.GetName() {
					if isComboPieceFor(permanent.GetName(), context.Player.GetName()) {
						return target
					}
				}
			}
		}
		for _, target := range validTargets {
			if permanent, ok := target.(AbilityPermanent); ok {
				if permanent.GetController().GetName() == context.Player.GetName() {
					if isHighUtilityPermanent(permanent.GetName()) {
						return target
					}
				}
			}
		}
		for _, target := range validTargets {
			if permanent, ok := target.(AbilityPermanent); ok {
				if permanent.GetController().GetName() == context.Player.GetName() {
					return target
				}
			}
		}
	}

	// If no preference-based target found, return the first valid target
	return validTargets[0]
}
