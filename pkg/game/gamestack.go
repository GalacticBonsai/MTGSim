package game

import "github.com/google/uuid"

// GameStack is a minimal stack interface used by the game loop.
// It wraps the ability.Stack to avoid import cycles while providing
// the subset of stack operations the game engine needs.
type GameStack interface {
	Size() int
	IsEmpty() bool
	Push(item *GameStackItem)
	Pop() *GameStackItem
	Peek() *GameStackItem
	ResolveTop() error
}

// GameStackItem represents an item on the game-level stack.
type GameStackItem struct {
	ID          uuid.UUID
	Type        GameStackItemType
	Name        string
	Controller  *Player
	Source      *Permanent
	Targets     []interface{}
	Countered   bool
	Fizzled     bool
	Description string
	// Resolve is called when this item resolves off the stack.
	Resolve func(g *Game) error
}

type GameStackItemType int

const (
	GameStackSpell    GameStackItemType = iota
	GameStackAbility
	GameStackTriggeredAbility
)

// simpleGameStack is a minimal implementation of GameStack for testing
// and as a fallback when no external stack is injected.
type simpleGameStack struct {
	items []*GameStackItem
}

func (s *simpleGameStack) Size() int              { return len(s.items) }
func (s *simpleGameStack) IsEmpty() bool           { return len(s.items) == 0 }
func (s *simpleGameStack) Push(item *GameStackItem) { s.items = append(s.items, item) }
func (s *simpleGameStack) Pop() *GameStackItem {
	if len(s.items) == 0 {
		return nil
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item
}
func (s *simpleGameStack) Peek() *GameStackItem {
	if len(s.items) == 0 {
		return nil
	}
	return s.items[len(s.items)-1]
}
func (s *simpleGameStack) ResolveTop() error {
	item := s.Pop()
	if item == nil || item.Resolve == nil {
		return nil
	}
	// Game reference is passed via closure; this is only used in the
	// minimal fallback path.
	return nil
}
