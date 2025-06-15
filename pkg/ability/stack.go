// Package ability provides comprehensive stack mechanics for MTG simulation.
package ability

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/internal/logger"
)

// StackItem represents an item on the stack (spell or ability)
type StackItem struct {
	ID          uuid.UUID
	Type        StackItemType
	Ability     *Ability
	Spell       *Spell
	Controller  AbilityPlayer
	Source      interface{}
	Targets     []interface{}
	Countered   bool
	Fizzled     bool
	Description string
}

// StackItemType represents the type of item on the stack
type StackItemType int

const (
	StackItemAbility StackItemType = iota
	StackItemSpell
)

// Spell represents a spell being cast
type Spell struct {
	ID         uuid.UUID
	Name       string
	ManaCost   string
	CMC        int
	TypeLine   string
	OracleText string
	Effects    []Effect
	Source     interface{} // The card being cast
}

// Stack represents the Magic: The Gathering stack
type Stack struct {
	items           []*StackItem
	priorityPlayer  AbilityPlayer
	passedPriority  map[string]bool
	allPlayers      []AbilityPlayer
	gameState       GameState
	executionEngine *ExecutionEngine
}

// NewStack creates a new stack instance
func NewStack(gameState GameState, executionEngine *ExecutionEngine) *Stack {
	return &Stack{
		items:           make([]*StackItem, 0),
		passedPriority:  make(map[string]bool),
		gameState:       gameState,
		executionEngine: executionEngine,
	}
}

// Push adds a spell or ability to the top of the stack
func (s *Stack) Push(item *StackItem) {
	s.items = append(s.items, item)
	logger.LogCard("Added to stack: %s (controlled by %s)", item.Description, item.Controller.GetName())
	
	// Reset priority passing when new item is added
	s.resetPriorityPassing()
	
	// Priority goes to the active player
	if s.gameState != nil {
		s.priorityPlayer = s.gameState.GetActivePlayer()
		if s.priorityPlayer != nil {
			logger.LogCard("Priority to %s", s.priorityPlayer.GetName())
		}
	}
}

// Pop removes and returns the top item from the stack
func (s *Stack) Pop() *StackItem {
	if len(s.items) == 0 {
		return nil
	}
	
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item
}

// Peek returns the top item without removing it
func (s *Stack) Peek() *StackItem {
	if len(s.items) == 0 {
		return nil
	}
	return s.items[len(s.items)-1]
}

// Size returns the number of items on the stack
func (s *Stack) Size() int {
	return len(s.items)
}

// IsEmpty returns true if the stack is empty
func (s *Stack) IsEmpty() bool {
	return len(s.items) == 0
}

// GetItems returns a copy of all items on the stack (bottom to top)
func (s *Stack) GetItems() []*StackItem {
	items := make([]*StackItem, len(s.items))
	copy(items, s.items)
	return items
}

// AddSpell adds a spell to the stack
func (s *Stack) AddSpell(spell *Spell, controller AbilityPlayer, targets []interface{}) {
	item := &StackItem{
		ID:          uuid.New(),
		Type:        StackItemSpell,
		Spell:       spell,
		Controller:  controller,
		Source:      spell.Source,
		Targets:     targets,
		Description: fmt.Sprintf("%s (spell)", spell.Name),
	}
	s.Push(item)
}

// AddAbility adds an ability to the stack
func (s *Stack) AddAbility(ability *Ability, controller AbilityPlayer, targets []interface{}) {
	item := &StackItem{
		ID:          uuid.New(),
		Type:        StackItemAbility,
		Ability:     ability,
		Controller:  controller,
		Source:      ability.Source,
		Targets:     targets,
		Description: fmt.Sprintf("%s (ability)", ability.Name),
	}
	s.Push(item)
}

// PassPriority handles a player passing priority
func (s *Stack) PassPriority(player AbilityPlayer) bool {
	playerName := player.GetName()
	s.passedPriority[playerName] = true
	logger.LogCard("%s passes priority", playerName)
	
	// Check if all players have passed priority
	if s.allPlayersPassedPriority() {
		logger.LogCard("All players passed priority")
		return true // Ready to resolve
	}
	
	// Pass priority to next player
	s.passPriorityToNextPlayer()
	return false
}

// SetPlayers sets the list of all players for priority passing
func (s *Stack) SetPlayers(players []AbilityPlayer) {
	s.allPlayers = players
}

// GetPriorityPlayer returns the player who currently has priority
func (s *Stack) GetPriorityPlayer() AbilityPlayer {
	return s.priorityPlayer
}

// ResolveTop resolves the top item on the stack
func (s *Stack) ResolveTop() error {
	if s.IsEmpty() {
		return fmt.Errorf("cannot resolve empty stack")
	}
	
	item := s.Pop()
	logger.LogCard("Resolving: %s", item.Description)
	
	// Check if the item was countered
	if item.Countered {
		logger.LogCard("%s was countered", item.Description)
		return nil
	}
	
	// Check if the item fizzled (no legal targets)
	if s.checkFizzle(item) {
		item.Fizzled = true
		logger.LogCard("%s fizzled (no legal targets)", item.Description)
		return nil
	}
	
	// Resolve the item
	var err error
	switch item.Type {
	case StackItemSpell:
		err = s.resolveSpell(item)
	case StackItemAbility:
		err = s.resolveAbility(item)
	}
	
	if err != nil {
		logger.LogCard("Error resolving %s: %v", item.Description, err)
		return err
	}
	
	// After resolution, priority returns to the active player
	s.resetPriorityPassing()
	if s.gameState != nil {
		s.priorityPlayer = s.gameState.GetActivePlayer()
	}
	
	return nil
}

// CounterSpell counters a spell on the stack
func (s *Stack) CounterSpell(targetSpell *StackItem, counteringPlayer AbilityPlayer) error {
	if targetSpell.Type != StackItemSpell {
		return fmt.Errorf("can only counter spells")
	}
	
	// Find the spell on the stack
	for _, item := range s.items {
		if item.ID == targetSpell.ID {
			item.Countered = true
			logger.LogCard("%s counters %s", counteringPlayer.GetName(), item.Description)
			return nil
		}
	}
	
	return fmt.Errorf("spell not found on stack")
}

// Helper methods

func (s *Stack) resetPriorityPassing() {
	s.passedPriority = make(map[string]bool)
}

func (s *Stack) allPlayersPassedPriority() bool {
	for _, player := range s.allPlayers {
		if !s.passedPriority[player.GetName()] {
			return false
		}
	}
	return len(s.allPlayers) > 0
}

func (s *Stack) passPriorityToNextPlayer() {
	if len(s.allPlayers) == 0 {
		return
	}
	
	// Find current priority player index
	currentIndex := -1
	for i, player := range s.allPlayers {
		if player.GetName() == s.priorityPlayer.GetName() {
			currentIndex = i
			break
		}
	}
	
	// Move to next player
	if currentIndex >= 0 {
		nextIndex := (currentIndex + 1) % len(s.allPlayers)
		s.priorityPlayer = s.allPlayers[nextIndex]
		logger.LogCard("Priority to %s", s.priorityPlayer.GetName())
	}
}

func (s *Stack) checkFizzle(item *StackItem) bool {
	// Check if all targets are still legal
	// This is a simplified implementation
	if len(item.Targets) == 0 {
		return false // No targets required
	}
	
	// TODO: Implement proper target legality checking
	// For now, assume targets are still legal
	return false
}

func (s *Stack) resolveSpell(item *StackItem) error {
	if item.Spell == nil {
		return fmt.Errorf("spell item has no spell")
	}
	
	// Apply spell effects
	for _, effect := range item.Spell.Effects {
		err := s.executionEngine.ApplyEffect(effect, item.Controller, item.Targets)
		if err != nil {
			return err
		}
	}
	
	logger.LogCard("%s resolved", item.Spell.Name)
	return nil
}

func (s *Stack) resolveAbility(item *StackItem) error {
	if item.Ability == nil {
		return fmt.Errorf("ability item has no ability")
	}
	
	// Apply ability effects
	for _, effect := range item.Ability.Effects {
		err := s.executionEngine.ApplyEffect(effect, item.Controller, item.Targets)
		if err != nil {
			return err
		}
	}
	
	logger.LogCard("%s resolved", item.Ability.Name)
	return nil
}
