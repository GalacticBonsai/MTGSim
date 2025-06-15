// Package ability provides spell casting mechanics for MTG simulation.
package ability

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
)

// SpellCastingEngine handles the casting of spells and their integration with the stack
type SpellCastingEngine struct {
	stack           *Stack
	priorityManager *PriorityManager
	parser          *AbilityParser
	gameState       GameState
	executionEngine *ExecutionEngine
}

// NewSpellCastingEngine creates a new spell casting engine
func NewSpellCastingEngine(gameState GameState, executionEngine *ExecutionEngine) *SpellCastingEngine {
	stack := NewStack(gameState, executionEngine)
	priorityManager := NewPriorityManager(stack, gameState)
	
	return &SpellCastingEngine{
		stack:           stack,
		priorityManager: priorityManager,
		parser:          NewAbilityParser(),
		gameState:       gameState,
		executionEngine: executionEngine,
	}
}

// GetStack returns the stack instance
func (sce *SpellCastingEngine) GetStack() *Stack {
	return sce.stack
}

// GetPriorityManager returns the priority manager instance
func (sce *SpellCastingEngine) GetPriorityManager() *PriorityManager {
	return sce.priorityManager
}

// CastSpell handles casting a spell from a card
func (sce *SpellCastingEngine) CastSpell(cardToCast card.Card, caster AbilityPlayer, targets []interface{}) error {
	logger.LogCard("%s attempts to cast %s", caster.GetName(), cardToCast.Name)
	
	// Convert card to spell
	spell, err := sce.cardToSpell(cardToCast)
	if err != nil {
		return fmt.Errorf("failed to convert card to spell: %v", err)
	}
	
	// Check if spell can be cast
	if !sce.priorityManager.CanCastSpell(spell, caster) {
		return fmt.Errorf("cannot cast %s at this time", spell.Name)
	}
	
	// Validate targets
	if err := sce.validateSpellTargets(spell, targets); err != nil {
		return fmt.Errorf("invalid targets for %s: %v", spell.Name, err)
	}
	
	// Pay mana costs (simplified for now)
	if err := sce.paySpellCosts(spell, caster); err != nil {
		return fmt.Errorf("cannot pay costs for %s: %v", spell.Name, err)
	}
	
	// Cast the spell (add to stack)
	return sce.priorityManager.CastSpell(caster, spell, targets)
}

// CastInstantSpell specifically handles instant spells
func (sce *SpellCastingEngine) CastInstantSpell(cardToCast card.Card, caster AbilityPlayer, targets []interface{}) error {
	if !sce.isInstantSpell(cardToCast) {
		return fmt.Errorf("%s is not an instant spell", cardToCast.Name)
	}
	
	return sce.CastSpell(cardToCast, caster, targets)
}

// CastSorcerySpell specifically handles sorcery spells
func (sce *SpellCastingEngine) CastSorcerySpell(cardToCast card.Card, caster AbilityPlayer, targets []interface{}) error {
	if !sce.isSorcerySpell(cardToCast) {
		return fmt.Errorf("%s is not a sorcery spell", cardToCast.Name)
	}
	
	// Sorcery spells have additional timing restrictions
	if !sce.gameState.IsMainPhase() || !sce.stack.IsEmpty() {
		return fmt.Errorf("can only cast sorcery spells during main phase with empty stack")
	}
	
	return sce.CastSpell(cardToCast, caster, targets)
}

// CounterSpell handles counterspell effects
func (sce *SpellCastingEngine) CounterSpell(counterSpell card.Card, caster AbilityPlayer, targetSpell *StackItem) error {
	logger.LogCard("%s attempts to counter %s", caster.GetName(), targetSpell.Description)
	
	// Verify the counter spell can target the spell
	if targetSpell.Type != StackItemSpell {
		return fmt.Errorf("can only counter spells")
	}
	
	// Cast the counterspell first
	spell, err := sce.cardToSpell(counterSpell)
	if err != nil {
		return err
	}
	
	// Add counterspell to stack with target
	targets := []interface{}{targetSpell}
	if err := sce.priorityManager.CastSpell(caster, spell, targets); err != nil {
		return err
	}
	
	return nil
}

// ActivateAbility handles activating abilities that use the stack
func (sce *SpellCastingEngine) ActivateAbility(ability *Ability, controller AbilityPlayer, targets []interface{}) error {
	logger.LogCard("%s attempts to activate %s", controller.GetName(), ability.Name)
	
	// Check if ability can be activated
	if !sce.priorityManager.CanActivateAbility(ability, controller) {
		return fmt.Errorf("cannot activate %s at this time", ability.Name)
	}
	
	// Validate targets
	if err := sce.validateAbilityTargets(ability, targets); err != nil {
		return fmt.Errorf("invalid targets for %s: %v", ability.Name, err)
	}
	
	// Activate the ability
	return sce.priorityManager.ActivateAbility(controller, ability, targets)
}

// ProcessPriority processes a complete priority round
func (sce *SpellCastingEngine) ProcessPriority() error {
	return sce.priorityManager.ProcessPriorityRound()
}

// ResolveStack resolves all items on the stack
func (sce *SpellCastingEngine) ResolveStack() error {
	logger.LogCard("Resolving stack with %d items", sce.stack.Size())
	
	for !sce.stack.IsEmpty() {
		if err := sce.stack.ResolveTop(); err != nil {
			return err
		}
	}
	
	logger.LogCard("Stack resolution complete")
	return nil
}

// SetPlayers sets the players for priority management
func (sce *SpellCastingEngine) SetPlayers(players []AbilityPlayer) {
	sce.priorityManager.SetPlayers(players)
}

// SetActivePlayer sets the active player
func (sce *SpellCastingEngine) SetActivePlayer(player AbilityPlayer) {
	sce.priorityManager.SetActivePlayer(player)
}

// SetPhase sets the current game phase
func (sce *SpellCastingEngine) SetPhase(phase string) {
	sce.priorityManager.SetPhase(phase)
}

// Helper methods

func (sce *SpellCastingEngine) cardToSpell(cardToCast card.Card) (*Spell, error) {
	// Parse abilities from the card's oracle text
	abilities, err := sce.parser.ParseAbilities(cardToCast.OracleText, cardToCast)
	if err != nil {
		return nil, err
	}
	
	// Convert abilities to effects
	var effects []Effect
	for _, ability := range abilities {
		effects = append(effects, ability.Effects...)
	}
	
	spell := &Spell{
		ID:         uuid.New(),
		Name:       cardToCast.Name,
		ManaCost:   cardToCast.ManaCost,
		CMC:        int(cardToCast.CMC),
		TypeLine:   cardToCast.TypeLine,
		OracleText: cardToCast.OracleText,
		Effects:    effects,
		Source:     cardToCast,
	}
	
	return spell, nil
}

func (sce *SpellCastingEngine) isInstantSpell(cardToCast card.Card) bool {
	return strings.Contains(cardToCast.TypeLine, "Instant")
}

func (sce *SpellCastingEngine) isSorcerySpell(cardToCast card.Card) bool {
	return strings.Contains(cardToCast.TypeLine, "Sorcery")
}

func (sce *SpellCastingEngine) validateSpellTargets(spell *Spell, targets []interface{}) error {
	// Count required targets from effects
	requiredTargets := 0
	for _, effect := range spell.Effects {
		for _, target := range effect.Targets {
			if target.Required {
				requiredTargets += target.Count
			}
		}
	}
	
	if len(targets) < requiredTargets {
		return fmt.Errorf("spell requires %d targets, got %d", requiredTargets, len(targets))
	}
	
	// TODO: Implement detailed target validation (type checking, legality, etc.)
	return nil
}

func (sce *SpellCastingEngine) validateAbilityTargets(ability *Ability, targets []interface{}) error {
	// Count required targets from effects
	requiredTargets := 0
	for _, effect := range ability.Effects {
		for _, target := range effect.Targets {
			if target.Required {
				requiredTargets += target.Count
			}
		}
	}
	
	if len(targets) < requiredTargets {
		return fmt.Errorf("ability requires %d targets, got %d", requiredTargets, len(targets))
	}
	
	// TODO: Implement detailed target validation
	return nil
}

func (sce *SpellCastingEngine) paySpellCosts(spell *Spell, caster AbilityPlayer) error {
	// Simplified cost payment - just check if player can pay
	// TODO: Implement proper mana cost parsing and payment
	logger.LogCard("%s pays costs for %s", caster.GetName(), spell.Name)
	return nil
}

// GetStackState returns the current state of the stack for display/debugging
func (sce *SpellCastingEngine) GetStackState() []string {
	items := sce.stack.GetItems()
	state := make([]string, len(items))
	
	for i, item := range items {
		state[i] = fmt.Sprintf("%d: %s (controlled by %s)", 
			i+1, item.Description, item.Controller.GetName())
	}
	
	return state
}

// IsStackEmpty returns true if the stack is empty
func (sce *SpellCastingEngine) IsStackEmpty() bool {
	return sce.stack.IsEmpty()
}

// GetPriorityPlayer returns the player who currently has priority
func (sce *SpellCastingEngine) GetPriorityPlayer() AbilityPlayer {
	return sce.priorityManager.GetCurrentPlayer()
}
