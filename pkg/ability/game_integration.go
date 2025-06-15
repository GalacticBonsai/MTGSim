// Package ability provides integration between the stack system and the existing game engine.
package ability

import (
	"fmt"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
)

// GameStackIntegration provides integration between the stack system and the existing game
type GameStackIntegration struct {
	spellCastingEngine *SpellCastingEngine
	gameAdapter        *GameAdapter
	players            []AbilityPlayer
	currentPhase       string
	activePlayer       AbilityPlayer
}

// NewGameStackIntegration creates a new game stack integration
func NewGameStackIntegration(gameInterface GameInterface) *GameStackIntegration {
	gameAdapter := NewGameAdapter(gameInterface)
	spellCastingEngine := NewSpellCastingEngine(gameAdapter, gameAdapter.executionEngine)
	
	integration := &GameStackIntegration{
		spellCastingEngine: spellCastingEngine,
		gameAdapter:        gameAdapter,
		currentPhase:       "Main Phase",
	}
	
	// Set up players
	integration.setupPlayers()
	
	return integration
}

// setupPlayers initializes the player list for the stack system
func (gsi *GameStackIntegration) setupPlayers() {
	gamePlayers := gsi.gameAdapter.game.GetPlayers()
	gsi.players = make([]AbilityPlayer, len(gamePlayers))
	
	for i, gamePlayer := range gamePlayers {
		gsi.players[i] = &PlayerAdapter{
			player:  gamePlayer,
			adapter: gsi.gameAdapter,
		}
	}
	
	if len(gsi.players) > 0 {
		gsi.activePlayer = gsi.players[0]
		gsi.spellCastingEngine.SetPlayers(gsi.players)
		gsi.spellCastingEngine.SetActivePlayer(gsi.activePlayer)
	}
}

// SetActivePlayer sets the active player for the current turn
func (gsi *GameStackIntegration) SetActivePlayer(playerName string) error {
	for _, player := range gsi.players {
		if player.GetName() == playerName {
			gsi.activePlayer = player
			gsi.spellCastingEngine.SetActivePlayer(player)
			logger.LogCard("Active player set to %s", playerName)
			return nil
		}
	}
	return fmt.Errorf("player %s not found", playerName)
}

// SetPhase sets the current game phase
func (gsi *GameStackIntegration) SetPhase(phase string) {
	gsi.currentPhase = phase
	gsi.spellCastingEngine.SetPhase(phase)
	logger.LogCard("Phase set to %s", phase)
}

// CastSpell handles casting a spell from a card
func (gsi *GameStackIntegration) CastSpell(cardToCast card.Card, casterName string, targets []interface{}) error {
	caster := gsi.getPlayerByName(casterName)
	if caster == nil {
		return fmt.Errorf("player %s not found", casterName)
	}
	
	logger.LogCard("%s attempts to cast %s", casterName, cardToCast.Name)
	return gsi.spellCastingEngine.CastSpell(cardToCast, caster, targets)
}

// CastInstantSpell specifically handles instant spells
func (gsi *GameStackIntegration) CastInstantSpell(cardToCast card.Card, casterName string, targets []interface{}) error {
	caster := gsi.getPlayerByName(casterName)
	if caster == nil {
		return fmt.Errorf("player %s not found", casterName)
	}
	
	return gsi.spellCastingEngine.CastInstantSpell(cardToCast, caster, targets)
}

// CastSorcerySpell specifically handles sorcery spells
func (gsi *GameStackIntegration) CastSorcerySpell(cardToCast card.Card, casterName string, targets []interface{}) error {
	caster := gsi.getPlayerByName(casterName)
	if caster == nil {
		return fmt.Errorf("player %s not found", casterName)
	}
	
	return gsi.spellCastingEngine.CastSorcerySpell(cardToCast, caster, targets)
}

// CounterSpell handles counterspell effects
func (gsi *GameStackIntegration) CounterSpell(counterSpell card.Card, casterName string, targetSpellIndex int) error {
	caster := gsi.getPlayerByName(casterName)
	if caster == nil {
		return fmt.Errorf("player %s not found", casterName)
	}
	
	// Get the target spell from stack
	stackItems := gsi.spellCastingEngine.GetStack().GetItems()
	if targetSpellIndex < 0 || targetSpellIndex >= len(stackItems) {
		return fmt.Errorf("invalid target spell index %d", targetSpellIndex)
	}
	
	targetSpell := stackItems[targetSpellIndex]
	return gsi.spellCastingEngine.CounterSpell(counterSpell, caster, targetSpell)
}

// ActivateAbility handles activating abilities
func (gsi *GameStackIntegration) ActivateAbility(ability *Ability, controllerName string, targets []interface{}) error {
	controller := gsi.getPlayerByName(controllerName)
	if controller == nil {
		return fmt.Errorf("player %s not found", controllerName)
	}
	
	return gsi.spellCastingEngine.ActivateAbility(ability, controller, targets)
}

// PassPriority handles a player passing priority
func (gsi *GameStackIntegration) PassPriority(playerName string) error {
	player := gsi.getPlayerByName(playerName)
	if player == nil {
		return fmt.Errorf("player %s not found", playerName)
	}
	
	return gsi.spellCastingEngine.GetPriorityManager().PassPriority(player)
}

// ProcessPriorityRound processes a complete round of priority
func (gsi *GameStackIntegration) ProcessPriorityRound() error {
	logger.LogCard("Processing priority round in %s", gsi.currentPhase)
	return gsi.spellCastingEngine.ProcessPriority()
}

// ResolveStack resolves all items on the stack
func (gsi *GameStackIntegration) ResolveStack() error {
	return gsi.spellCastingEngine.ResolveStack()
}

// GetStackState returns the current state of the stack
func (gsi *GameStackIntegration) GetStackState() []string {
	return gsi.spellCastingEngine.GetStackState()
}

// IsStackEmpty returns true if the stack is empty
func (gsi *GameStackIntegration) IsStackEmpty() bool {
	return gsi.spellCastingEngine.IsStackEmpty()
}

// GetPriorityPlayer returns the player who currently has priority
func (gsi *GameStackIntegration) GetPriorityPlayer() string {
	player := gsi.spellCastingEngine.GetPriorityPlayer()
	if player == nil {
		return ""
	}
	return player.GetName()
}

// CanCastSpell checks if a spell can be cast at the current time
func (gsi *GameStackIntegration) CanCastSpell(cardToCast card.Card, casterName string) bool {
	caster := gsi.getPlayerByName(casterName)
	if caster == nil {
		return false
	}
	
	// Convert card to spell for checking
	spell, err := gsi.cardToSpell(cardToCast)
	if err != nil {
		return false
	}
	
	return gsi.spellCastingEngine.GetPriorityManager().CanCastSpell(spell, caster)
}

// CanActivateAbility checks if an ability can be activated at the current time
func (gsi *GameStackIntegration) CanActivateAbility(ability *Ability, controllerName string) bool {
	controller := gsi.getPlayerByName(controllerName)
	if controller == nil {
		return false
	}
	
	return gsi.spellCastingEngine.GetPriorityManager().CanActivateAbility(ability, controller)
}

// GetAvailableActions returns the actions available to the current priority player
func (gsi *GameStackIntegration) GetAvailableActions() []string {
	priorityPlayer := gsi.spellCastingEngine.GetPriorityPlayer()
	if priorityPlayer == nil {
		return []string{}
	}
	
	actions := []string{"Pass Priority"}
	
	// Add spell casting options based on phase and stack state
	if gsi.currentPhase == "Main Phase" && gsi.spellCastingEngine.IsStackEmpty() {
		actions = append(actions, "Cast Sorcery Spell")
	}
	
	actions = append(actions, "Cast Instant Spell")
	actions = append(actions, "Activate Ability")
	
	return actions
}

// HandleSpellResolution handles the resolution of a specific spell
func (gsi *GameStackIntegration) HandleSpellResolution(spellName string) error {
	logger.LogCard("Resolving spell: %s", spellName)
	
	// This would integrate with the existing game state to apply spell effects
	// For now, just log the resolution
	logger.LogCard("Spell %s resolved successfully", spellName)
	
	return nil
}

// HandleAbilityResolution handles the resolution of a specific ability
func (gsi *GameStackIntegration) HandleAbilityResolution(abilityName string) error {
	logger.LogCard("Resolving ability: %s", abilityName)
	
	// This would integrate with the existing game state to apply ability effects
	// For now, just log the resolution
	logger.LogCard("Ability %s resolved successfully", abilityName)
	
	return nil
}

// GetGameState returns the current game state for external inspection
func (gsi *GameStackIntegration) GetGameState() GameState {
	return gsi.gameAdapter
}

// Helper methods

func (gsi *GameStackIntegration) getPlayerByName(name string) AbilityPlayer {
	for _, player := range gsi.players {
		if player.GetName() == name {
			return player
		}
	}
	return nil
}

func (gsi *GameStackIntegration) cardToSpell(cardToCast card.Card) (*Spell, error) {
	// This is a simplified conversion - in a full implementation,
	// this would parse the card's oracle text and create appropriate effects
	spell := &Spell{
		Name:       cardToCast.Name,
		ManaCost:   cardToCast.ManaCost,
		CMC:        int(cardToCast.CMC),
		TypeLine:   cardToCast.TypeLine,
		OracleText: cardToCast.OracleText,
		Source:     cardToCast,
	}
	
	return spell, nil
}

// Integration with existing game phases

// MainPhaseWithStack handles main phase with stack integration
func (gsi *GameStackIntegration) MainPhaseWithStack(playerName string) error {
	gsi.SetPhase("Main Phase")
	gsi.SetActivePlayer(playerName)
	
	logger.LogCard("Main phase for %s with stack integration", playerName)
	
	// Process priority until all players pass
	return gsi.ProcessPriorityRound()
}

// CombatPhaseWithStack handles combat phase with stack integration
func (gsi *GameStackIntegration) CombatPhaseWithStack(playerName string) error {
	gsi.SetPhase("Combat Phase")
	gsi.SetActivePlayer(playerName)
	
	logger.LogCard("Combat phase for %s with stack integration", playerName)
	
	// Process priority for combat
	return gsi.ProcessPriorityRound()
}

// EndStepWithStack handles end step with stack integration
func (gsi *GameStackIntegration) EndStepWithStack(playerName string) error {
	gsi.SetPhase("End Step")
	gsi.SetActivePlayer(playerName)
	
	logger.LogCard("End step for %s with stack integration", playerName)
	
	// Process priority and resolve any remaining stack items
	if err := gsi.ProcessPriorityRound(); err != nil {
		return err
	}
	
	// Ensure stack is empty at end of turn
	if !gsi.IsStackEmpty() {
		logger.LogCard("Warning: Stack not empty at end of turn, resolving remaining items")
		return gsi.ResolveStack()
	}
	
	return nil
}
