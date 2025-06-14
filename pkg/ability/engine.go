// Package ability provides the execution engine for MTG abilities.
package ability

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// GameState represents the current state of the game for ability resolution.
type GameState interface {
	GetPlayer(name string) AbilityPlayer
	GetAllPlayers() []AbilityPlayer
	GetCurrentPlayer() AbilityPlayer
	GetActivePlayer() AbilityPlayer
	IsMainPhase() bool
	IsCombatPhase() bool
	CanActivateAbilities() bool
	AddManaToPool(player AbilityPlayer, manaType game.ManaType, amount int)
	DealDamage(source interface{}, target interface{}, amount int)
	DrawCards(player AbilityPlayer, count int)
	GainLife(player AbilityPlayer, amount int)
	LoseLife(player AbilityPlayer, amount int)
}

// AbilityPlayer represents a player in the game for ability purposes.
type AbilityPlayer interface {
	GetName() string
	GetLifeTotal() int
	SetLifeTotal(life int)
	GetHand() []interface{}
	AddCardToHand(card interface{})
	GetCreatures() []interface{}
	GetLands() []interface{}
	CanPayCost(cost Cost) bool
	PayCost(cost Cost) error
	GetManaPool() map[game.ManaType]int
}

// AbilityPermanent represents a permanent for ability purposes.
type AbilityPermanent interface {
	GetID() uuid.UUID
	GetName() string
	GetOwner() AbilityPlayer
	GetController() AbilityPlayer
	IsTapped() bool
	Tap()
	Untap()
	GetAbilities() []*Ability
	AddAbility(ability *Ability)
	RemoveAbility(abilityID uuid.UUID)
}

// ExecutionEngine handles the execution of abilities and their effects.
type ExecutionEngine struct {
	gameState       GameState
	parser          *AbilityParser
	targetValidator *TargetValidator
	targetParser    *TargetParser
}

// NewExecutionEngine creates a new execution engine.
func NewExecutionEngine(gameState GameState) *ExecutionEngine {
	return &ExecutionEngine{
		gameState:       gameState,
		parser:          NewAbilityParser(),
		targetValidator: NewTargetValidator(gameState),
		targetParser:    NewTargetParser(),
	}
}

// ExecuteAbility executes an ability with the given targets.
func (ee *ExecutionEngine) ExecuteAbility(ability *Ability, controller AbilityPlayer, targets []interface{}) error {
	logger.LogCard("Executing ability: %s", ability.Name)

	// Check if ability can be activated
	if !ee.canActivateAbility(ability, controller) {
		return ErrCannotActivate
	}

	// Validate targets before execution
	if err := ee.validateTargets(ability, controller, targets); err != nil {
		return err
	}

	// Pay costs
	if err := ee.payCosts(ability, controller, ability.Source); err != nil {
		return err
	}

	// For mana abilities, resolve immediately
	if ability.Type == Mana {
		return ee.resolveManaAbility(ability, controller)
	}

	// For other abilities, they would go on the stack
	// For simplicity, we'll resolve them immediately in this implementation
	return ee.resolveAbility(ability, controller, targets)
}

// canActivateAbility checks if an ability can be activated.
func (ee *ExecutionEngine) canActivateAbility(ability *Ability, controller AbilityPlayer) bool {
	// Check timing restrictions
	switch ability.TimingRestriction {
	case SorcerySpeed:
		if !ee.gameState.IsMainPhase() {
			return false
		}
	case OnlyDuringCombat:
		if !ee.gameState.IsCombatPhase() {
			return false
		}
	case OnlyOnYourTurn:
		if ee.gameState.GetCurrentPlayer().GetName() != controller.GetName() {
			return false
		}
	}

	// Check usage limits
	if !ability.CanActivate(controller) {
		return false
	}

	// Check if player can pay costs
	if !controller.CanPayCost(ability.Cost) {
		return false
	}

	// Check if valid targets exist using the new target validation system
	if ee.requiresTargets(ability) && !ee.hasValidTargets(ability, controller) {
		return false
	}

	return true
}

// payCosts pays the costs for an ability.
func (ee *ExecutionEngine) payCosts(ability *Ability, controller AbilityPlayer, source interface{}) error {
	// Handle tap costs by tapping the source permanent
	if ability.Cost.TapCost {
		if permanent, ok := source.(AbilityPermanent); ok {
			permanent.Tap()
		}
	}

	// Handle mana costs
	return controller.PayCost(ability.Cost)
}

// resolveManaAbility resolves a mana ability immediately.
func (ee *ExecutionEngine) resolveManaAbility(ability *Ability, controller AbilityPlayer) error {
	for _, effect := range ability.Effects {
		if effect.Type == AddMana {
			// Determine mana type from ability description or effect
			manaType := ee.determineManaType(ability, effect)
			ee.gameState.AddManaToPool(controller, manaType, effect.Value)
			logger.LogCard("%s adds %d %s mana", controller.GetName(), effect.Value, string(manaType))
		}
	}
	return nil
}

// resolveAbility resolves a non-mana ability.
func (ee *ExecutionEngine) resolveAbility(ability *Ability, controller AbilityPlayer, targets []interface{}) error {
	for _, effect := range ability.Effects {
		if err := ee.applyEffect(effect, controller, targets); err != nil {
			return err
		}
	}
	return nil
}

// applyEffect applies a specific effect.
func (ee *ExecutionEngine) applyEffect(effect Effect, controller AbilityPlayer, targets []interface{}) error {
	switch effect.Type {
	case DrawCards:
		ee.gameState.DrawCards(controller, effect.Value)
		logger.LogCard("%s draws %d cards", controller.GetName(), effect.Value)

	case DealDamage:
		if len(targets) > 0 {
			ee.gameState.DealDamage(controller, targets[0], effect.Value)
			logger.LogCard("Deal %d damage to target", effect.Value)
		}

	case GainLife:
		ee.gameState.GainLife(controller, effect.Value)
		logger.LogCard("%s gains %d life", controller.GetName(), effect.Value)

	case LoseLife:
		ee.gameState.LoseLife(controller, effect.Value)
		logger.LogCard("%s loses %d life", controller.GetName(), effect.Value)

	case PumpCreature:
		if len(targets) > 0 {
			power := effect.Value / 100
			toughness := effect.Value % 100
			ee.applyPumpEffect(targets[0], power, toughness, effect.Duration)
			logger.LogCard("Target creature gets +%d/+%d", power, toughness)
		}

	case TapUntap:
		if len(targets) > 0 {
			ee.applyTapEffect(targets[0], effect.Value > 0)
		}

	default:
		return fmt.Errorf("unimplemented effect type: %v", effect.Type)
	}

	return nil
}

// determineManaType determines what type of mana an ability produces.
func (ee *ExecutionEngine) determineManaType(ability *Ability, effect Effect) game.ManaType {
	// Parse from ability description or oracle text
	text := ability.OracleText
	
	if contains(text, "{W}") {
		return game.White
	}
	if contains(text, "{U}") {
		return game.Blue
	}
	if contains(text, "{B}") {
		return game.Black
	}
	if contains(text, "{R}") {
		return game.Red
	}
	if contains(text, "{G}") {
		return game.Green
	}
	if contains(text, "{C}") {
		return game.Colorless
	}
	if contains(text, "any color") {
		return game.Any
	}

	return game.Colorless // Default
}

// requiresTargets checks if an ability requires targets.
func (ee *ExecutionEngine) requiresTargets(ability *Ability) bool {
	for _, effect := range ability.Effects {
		if len(effect.Targets) > 0 {
			for _, target := range effect.Targets {
				if target.Required {
					return true
				}
			}
		}
	}
	return false
}

// hasValidTargets checks if valid targets exist for an ability.
func (ee *ExecutionEngine) hasValidTargets(ability *Ability, controller AbilityPlayer) bool {
	for _, effect := range ability.Effects {
		for _, target := range effect.Targets {
			if target.Required {
				// Use enhanced targeting if available
				if target.Enhanced != nil {
					if !ee.hasValidTargetsForEnhanced(*target.Enhanced, controller) {
						return false
					}
				} else {
					// Fallback to basic target validation
					if !ee.hasValidTargetsBasic(target, controller) {
						return false
					}
				}
			}
		}
	}
	return true // If no required targets or all have valid targets, it's valid
}

// hasValidTargetsForEnhanced checks if valid targets exist for an enhanced target.
func (ee *ExecutionEngine) hasValidTargetsForEnhanced(enhancedTarget EnhancedTarget, controller AbilityPlayer) bool {
	// Get all potential targets based on type
	potentialTargets := ee.getPotentialTargets(enhancedTarget.Type)

	// Check if any potential target passes validation
	for _, target := range potentialTargets {
		legality := ee.targetValidator.ValidateTarget(target, enhancedTarget, controller)
		if legality.IsLegal {
			return true
		}
	}

	return false
}

// hasValidTargetsBasic provides fallback validation for basic targets.
func (ee *ExecutionEngine) hasValidTargetsBasic(target Target, controller AbilityPlayer) bool {
	switch target.Type {
	case CreatureTarget:
		// Check if any creatures exist
		for _, player := range ee.gameState.GetAllPlayers() {
			if len(player.GetCreatures()) > 0 {
				return true
			}
		}
	case PlayerTarget:
		// Players always exist
		return true
	case AnyTarget:
		// Check if any valid targets exist
		for _, player := range ee.gameState.GetAllPlayers() {
			if len(player.GetCreatures()) > 0 {
				return true
			}
		}
		return true // Players are always valid targets
	}
	return false
}

// getPotentialTargets returns all potential targets of a given type.
func (ee *ExecutionEngine) getPotentialTargets(targetType TargetType) []interface{} {
	var targets []interface{}

	switch targetType {
	case CreatureTarget:
		for _, player := range ee.gameState.GetAllPlayers() {
			targets = append(targets, player.GetCreatures()...)
		}
	case PlayerTarget:
		for _, player := range ee.gameState.GetAllPlayers() {
			targets = append(targets, player)
		}
	case PermanentTarget:
		// Get all permanents (creatures, lands, artifacts, etc.)
		for _, player := range ee.gameState.GetAllPlayers() {
			targets = append(targets, player.GetCreatures()...)
			targets = append(targets, player.GetLands()...)
		}
	case AnyTarget:
		// Get all players and permanents
		for _, player := range ee.gameState.GetAllPlayers() {
			targets = append(targets, player)
			targets = append(targets, player.GetCreatures()...)
			targets = append(targets, player.GetLands()...)
		}
	}

	return targets
}

// validateTargets validates that the provided targets are legal for the ability.
func (ee *ExecutionEngine) validateTargets(ability *Ability, controller AbilityPlayer, targets []interface{}) error {
	targetIndex := 0

	for _, effect := range ability.Effects {
		for _, targetReq := range effect.Targets {
			if targetReq.Required {
				// Check if we have enough targets
				if targetIndex >= len(targets) {
					return ErrNoValidTargets
				}

				target := targets[targetIndex]

				// Use enhanced targeting if available
				if targetReq.Enhanced != nil {
					legality := ee.targetValidator.ValidateTarget(target, *targetReq.Enhanced, controller)
					if !legality.IsLegal {
						return ErrInvalidTarget
					}
				} else {
					// Fallback to basic validation
					if !ee.isValidBasicTarget(target, targetReq) {
						return ErrInvalidTarget
					}
				}

				targetIndex++
			}
		}
	}

	return nil
}

// isValidBasicTarget provides basic target validation for legacy targets.
func (ee *ExecutionEngine) isValidBasicTarget(target interface{}, targetReq Target) bool {
	switch targetReq.Type {
	case CreatureTarget:
		return ee.targetValidator.isCreature(target)
	case PlayerTarget:
		return ee.targetValidator.isPlayer(target)
	case PermanentTarget:
		return ee.targetValidator.isPermanent(target)
	case AnyTarget:
		return ee.targetValidator.isPlayer(target) || ee.targetValidator.isPermanent(target)
	default:
		return false
	}
}

// applyPumpEffect applies a pump effect to a creature.
func (ee *ExecutionEngine) applyPumpEffect(target interface{}, power, toughness int, duration EffectDuration) {
	// This would need to be implemented based on how creatures are represented
	// For now, just log the effect
	logger.LogCard("Applying +%d/+%d effect (duration: %v)", power, toughness, duration)
}

// applyTapEffect applies a tap/untap effect.
func (ee *ExecutionEngine) applyTapEffect(target interface{}, shouldTap bool) {
	if permanent, ok := target.(AbilityPermanent); ok {
		if shouldTap {
			permanent.Tap()
			logger.LogCard("Tapping %s", permanent.GetName())
		} else {
			permanent.Untap()
			logger.LogCard("Untapping %s", permanent.GetName())
		}
	}
}

// ParseAndRegisterAbilities parses abilities from oracle text and registers them.
func (ee *ExecutionEngine) ParseAndRegisterAbilities(oracleText string, source interface{}) ([]*Ability, error) {
	abilities, err := ee.parser.ParseAbilities(oracleText, source)
	if err != nil {
		return nil, err
	}

	logger.LogCard("Parsed %d abilities from oracle text", len(abilities))
	for _, ability := range abilities {
		logger.LogCard("  - %s: %s", ability.Name, ability.Effects[0].Description)
	}

	return abilities, nil
}

// ActivateManaAbilities activates all available mana abilities for a player.
func (ee *ExecutionEngine) ActivateManaAbilities(player AbilityPlayer) int {
	totalManaAdded := 0
	
	// Get all lands controlled by the player
	for _, land := range player.GetLands() {
		if permanent, ok := land.(AbilityPermanent); ok {
			if !permanent.IsTapped() {
				// Check if this land has mana abilities
				for _, ability := range permanent.GetAbilities() {
					if ability.Type == Mana && ee.canActivateAbility(ability, player) {
						if err := ee.ExecuteAbility(ability, player, nil); err == nil {
							totalManaAdded++
							break // Only activate one mana ability per land
						}
					}
				}
			}
		}
	}
	
	return totalManaAdded
}

// GetActivatableAbilities returns all abilities that can be activated by a player.
func (ee *ExecutionEngine) GetActivatableAbilities(player AbilityPlayer) []*Ability {
	var activatable []*Ability
	
	// Check creatures for activated abilities
	for _, creature := range player.GetCreatures() {
		if permanent, ok := creature.(AbilityPermanent); ok {
			for _, ability := range permanent.GetAbilities() {
				if ability.Type == Activated && ee.canActivateAbility(ability, player) {
					activatable = append(activatable, ability)
				}
			}
		}
	}

	// Check lands for non-mana activated abilities
	for _, land := range player.GetLands() {
		if permanent, ok := land.(AbilityPermanent); ok {
			for _, ability := range permanent.GetAbilities() {
				if ability.Type == Activated && ability.Type != Mana && ee.canActivateAbility(ability, player) {
					activatable = append(activatable, ability)
				}
			}
		}
	}
	
	return activatable
}

// Helper function to check if a string contains a substring (case-insensitive).
func contains(text, substr string) bool {
	return len(text) >= len(substr) && 
		   (text == substr || 
		    (len(text) > len(substr) && 
		     (text[:len(substr)] == substr || 
		      text[len(text)-len(substr):] == substr ||
		      containsSubstring(text, substr))))
}

func containsSubstring(text, substr string) bool {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
