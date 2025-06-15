// Package ability provides priority system management for MTG simulation.
package ability

import (
	"fmt"

	"github.com/mtgsim/mtgsim/internal/logger"
)

// PriorityManager manages the priority system for Magic: The Gathering
type PriorityManager struct {
	stack          *Stack
	gameState      GameState
	currentPlayer  AbilityPlayer
	activePlayer   AbilityPlayer
	allPlayers     []AbilityPlayer
	priorityPassed map[string]bool
	phase          string
}

// PriorityAction represents an action a player can take when they have priority
type PriorityAction int

const (
	PriorityActionPass PriorityAction = iota
	PriorityActionCastSpell
	PriorityActionActivateAbility
	PriorityActionPlayLand
)

// PriorityDecision represents a player's decision when they have priority
type PriorityDecision struct {
	Action   PriorityAction
	Spell    *Spell
	Ability  *Ability
	Targets  []interface{}
	Player   AbilityPlayer
}

// NewPriorityManager creates a new priority manager
func NewPriorityManager(stack *Stack, gameState GameState) *PriorityManager {
	return &PriorityManager{
		stack:          stack,
		gameState:      gameState,
		priorityPassed: make(map[string]bool),
	}
}

// SetPlayers sets the list of all players
func (pm *PriorityManager) SetPlayers(players []AbilityPlayer) {
	pm.allPlayers = players
	pm.stack.SetPlayers(players)
}

// SetActivePlayer sets the active player (whose turn it is)
func (pm *PriorityManager) SetActivePlayer(player AbilityPlayer) {
	pm.activePlayer = player
	pm.currentPlayer = player
}

// SetPhase sets the current game phase
func (pm *PriorityManager) SetPhase(phase string) {
	pm.phase = phase
}

// GetCurrentPlayer returns the player who currently has priority
func (pm *PriorityManager) GetCurrentPlayer() AbilityPlayer {
	return pm.currentPlayer
}

// PassPriority handles a player passing priority
func (pm *PriorityManager) PassPriority(player AbilityPlayer) error {
	if pm.currentPlayer.GetName() != player.GetName() {
		return fmt.Errorf("player %s does not have priority", player.GetName())
	}
	
	pm.priorityPassed[player.GetName()] = true
	logger.LogCard("%s passes priority", player.GetName())
	
	// Check if all players have passed priority
	if pm.allPlayersPassedPriority() {
		return pm.handleAllPlayersPassed()
	}
	
	// Pass priority to next player
	pm.passPriorityToNextPlayer()
	return nil
}

// CastSpell handles a player casting a spell
func (pm *PriorityManager) CastSpell(player AbilityPlayer, spell *Spell, targets []interface{}) error {
	if pm.currentPlayer.GetName() != player.GetName() {
		return fmt.Errorf("player %s does not have priority", player.GetName())
	}
	
	// Check timing restrictions
	if err := pm.checkSpellTiming(spell); err != nil {
		return err
	}
	
	// Add spell to stack
	pm.stack.AddSpell(spell, player, targets)
	
	// Reset priority passing and give priority to active player
	pm.resetPriorityPassing()
	pm.currentPlayer = pm.activePlayer
	
	return nil
}

// ActivateAbility handles a player activating an ability
func (pm *PriorityManager) ActivateAbility(player AbilityPlayer, ability *Ability, targets []interface{}) error {
	if pm.currentPlayer.GetName() != player.GetName() {
		return fmt.Errorf("player %s does not have priority", player.GetName())
	}
	
	// Check timing restrictions
	if err := pm.checkAbilityTiming(ability); err != nil {
		return err
	}
	
	// Mana abilities don't use the stack
	if ability.Type == Mana {
		return pm.resolveManaAbility(ability, player, targets)
	}
	
	// Add ability to stack
	pm.stack.AddAbility(ability, player, targets)
	
	// Reset priority passing and give priority to active player
	pm.resetPriorityPassing()
	pm.currentPlayer = pm.activePlayer
	
	return nil
}

// ProcessPriorityRound processes a complete round of priority
func (pm *PriorityManager) ProcessPriorityRound() error {
	logger.LogCard("Starting priority round in %s", pm.phase)
	
	// Continue until all players pass priority or stack is resolved
	for {
		// If stack is empty and all players passed, we're done
		if pm.stack.IsEmpty() && pm.allPlayersPassedPriority() {
			logger.LogCard("Priority round complete - stack empty and all passed")
			break
		}
		
		// If all players passed with items on stack, resolve top item
		if pm.allPlayersPassedPriority() && !pm.stack.IsEmpty() {
			if err := pm.stack.ResolveTop(); err != nil {
				return err
			}
			pm.resetPriorityPassing()
			pm.currentPlayer = pm.activePlayer
			continue
		}
		
		// Get decision from current player
		decision := pm.getPlayerDecision(pm.currentPlayer)
		
		// Process the decision
		if err := pm.processDecision(decision); err != nil {
			return err
		}
	}
	
	return nil
}

// CanCastSpell checks if a spell can be cast at the current time
func (pm *PriorityManager) CanCastSpell(spell *Spell, player AbilityPlayer) bool {
	// Check if player has priority
	if pm.currentPlayer.GetName() != player.GetName() {
		return false
	}
	
	// Check timing restrictions
	return pm.checkSpellTiming(spell) == nil
}

// CanActivateAbility checks if an ability can be activated at the current time
func (pm *PriorityManager) CanActivateAbility(ability *Ability, player AbilityPlayer) bool {
	// Check if player has priority
	if pm.currentPlayer.GetName() != player.GetName() {
		return false
	}
	
	// Check timing restrictions
	return pm.checkAbilityTiming(ability) == nil
}

// Helper methods

func (pm *PriorityManager) resetPriorityPassing() {
	pm.priorityPassed = make(map[string]bool)
}

func (pm *PriorityManager) allPlayersPassedPriority() bool {
	for _, player := range pm.allPlayers {
		if !pm.priorityPassed[player.GetName()] {
			return false
		}
	}
	return len(pm.allPlayers) > 0
}

func (pm *PriorityManager) passPriorityToNextPlayer() {
	if len(pm.allPlayers) == 0 {
		return
	}
	
	// Find current player index
	currentIndex := -1
	for i, player := range pm.allPlayers {
		if player.GetName() == pm.currentPlayer.GetName() {
			currentIndex = i
			break
		}
	}
	
	// Move to next player
	if currentIndex >= 0 {
		nextIndex := (currentIndex + 1) % len(pm.allPlayers)
		pm.currentPlayer = pm.allPlayers[nextIndex]
		logger.LogCard("Priority to %s", pm.currentPlayer.GetName())
	}
}

func (pm *PriorityManager) handleAllPlayersPassed() error {
	if pm.stack.IsEmpty() {
		logger.LogCard("All players passed with empty stack")
		// Reset priority passing and give priority to active player
		pm.resetPriorityPassing()
		pm.currentPlayer = pm.activePlayer
		return nil
	}

	// Resolve top item on stack
	if err := pm.stack.ResolveTop(); err != nil {
		return err
	}

	// Reset priority passing and give priority to active player
	pm.resetPriorityPassing()
	pm.currentPlayer = pm.activePlayer

	return nil
}

func (pm *PriorityManager) checkSpellTiming(spell *Spell) error {
	// Instant spells can be cast anytime with priority
	if isInstantSpeed(spell) {
		return nil
	}
	
	// Sorcery speed spells can only be cast during main phases with empty stack
	if pm.phase == "Main Phase" && pm.stack.IsEmpty() {
		return nil
	}
	
	return fmt.Errorf("cannot cast %s at this time", spell.Name)
}

func (pm *PriorityManager) checkAbilityTiming(ability *Ability) error {
	switch ability.TimingRestriction {
	case AnyTime:
		return nil
	case SorcerySpeed:
		if pm.phase == "Main Phase" && pm.stack.IsEmpty() {
			return nil
		}
		return fmt.Errorf("can only activate at sorcery speed")
	case OnlyDuringCombat:
		if pm.phase == "Combat Phase" {
			return nil
		}
		return fmt.Errorf("can only activate during combat")
	default:
		return nil
	}
}

func (pm *PriorityManager) resolveManaAbility(ability *Ability, player AbilityPlayer, targets []interface{}) error {
	// Mana abilities resolve immediately without using the stack
	for _, effect := range ability.Effects {
		if effect.Type == AddMana {
			// Apply mana effect directly
			logger.LogCard("%s activates mana ability: %s", player.GetName(), ability.Name)
			// TODO: Add mana to player's mana pool
		}
	}
	return nil
}

func (pm *PriorityManager) getPlayerDecision(player AbilityPlayer) *PriorityDecision {
	// This would be implemented by the AI or human player interface
	// For now, return a pass decision
	return &PriorityDecision{
		Action: PriorityActionPass,
		Player: player,
	}
}

func (pm *PriorityManager) processDecision(decision *PriorityDecision) error {
	switch decision.Action {
	case PriorityActionPass:
		return pm.PassPriority(decision.Player)
	case PriorityActionCastSpell:
		return pm.CastSpell(decision.Player, decision.Spell, decision.Targets)
	case PriorityActionActivateAbility:
		return pm.ActivateAbility(decision.Player, decision.Ability, decision.Targets)
	default:
		return fmt.Errorf("unknown priority action")
	}
}

func isInstantSpeed(spell *Spell) bool {
	return spell.TypeLine == "Instant" || 
		   (spell.TypeLine == "Creature" && hasFlash(spell)) ||
		   hasInstantSpeedKeyword(spell)
}

func hasFlash(spell *Spell) bool {
	// Check if spell has flash keyword
	return false // Simplified for now
}

func hasInstantSpeedKeyword(spell *Spell) bool {
	// Check for other instant-speed keywords
	return false // Simplified for now
}
