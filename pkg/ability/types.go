// Package ability provides a comprehensive ability engine for MTG simulation.
package ability

import (
	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/types"
)

// AbilityType represents the different types of abilities in Magic: The Gathering.
type AbilityType int

const (
	// Triggered abilities activate when specific events occur
	Triggered AbilityType = iota
	// Activated abilities can be activated by paying costs
	Activated
	// Static abilities provide continuous effects
	Static
	// Replacement effects replace events with other events
	Replacement
	// Mana abilities are a special subset of activated abilities
	Mana
)

// TriggerCondition represents when a triggered ability triggers.
type TriggerCondition int

const (
	EntersTheBattlefield TriggerCondition = iota
	LeavesTheBattlefield
	Dies
	BeginningOfUpkeep
	EndOfTurn
	DealsCombatDamage
	BecomesTargeted
	AttacksOrBlocks
	SpellCast
	CreatureEnters
	LandPlayed
)

// EffectType represents the type of effect an ability produces.
type EffectType int

const (
	DrawCards EffectType = iota
	DealDamage
	GainLife
	LoseLife
	AddMana
	PumpCreature
	DestroyPermanent
	CounterSpell
	SearchLibrary
	DiscardCards
	ReturnToHand
	CreateToken
	TapUntap
	ChangeControl
	PreventDamage
)

// TimingRestriction represents when an ability can be activated.
type TimingRestriction int

const (
	AnyTime TimingRestriction = iota
	SorcerySpeed
	OncePerTurn
	OnlyOnYourTurn
	OnlyDuringCombat
	OnlyMainPhase
)

// Cost represents the cost to activate an ability.
type Cost struct {
	ManaCost     map[types.ManaType]int
	TapCost      bool
	SacrificeCost bool
	DiscardCost  int
	LifeCost     int
	OtherCosts   []string // For complex costs that need special handling
}

// Target represents a target for an ability.
type Target struct {
	Type         TargetType
	Required     bool
	Count        int
	Restrictions []string // e.g., "creature", "non-artifact", "with flying"
	Enhanced     *EnhancedTarget // New comprehensive targeting system
}

// TargetType represents what can be targeted.
type TargetType int

const (
	NoTarget TargetType = iota
	AnyTarget
	CreatureTarget
	PlayerTarget
	PermanentTarget
	SpellTarget
	CardInGraveyardTarget
	CardInHandTarget
)

// Effect represents the effect of an ability.
type Effect struct {
	Type        EffectType
	Value       int           // Amount of damage, cards drawn, etc.
	Duration    EffectDuration
	Targets     []Target
	Conditions  []string      // Additional conditions for the effect
	Description string        // Human-readable description
}

// EffectDuration represents how long an effect lasts.
type EffectDuration int

const (
	Instant EffectDuration = iota
	UntilEndOfTurn
	UntilEndOfCombat
	Permanent
	UntilLeavesPlay
)

// Ability represents a Magic: The Gathering ability.
type Ability struct {
	ID               uuid.UUID
	Name             string
	Type             AbilityType
	Source           interface{} // The card or permanent that has this ability
	Cost             Cost
	Effects          []Effect
	TriggerCondition TriggerCondition
	TimingRestriction TimingRestriction
	UsesPerTurn      int // 0 = unlimited, -1 = once per game
	UsedThisTurn     int
	IsOptional       bool
	OracleText       string
	ParsedFromText   bool
}

// CanActivate checks if an ability can be activated by the given player.
func (a *Ability) CanActivate(player interface{}) bool {
	// Check timing restrictions
	if a.TimingRestriction == OncePerTurn && a.UsedThisTurn >= 1 {
		return false
	}
	
	if a.UsesPerTurn > 0 && a.UsedThisTurn >= a.UsesPerTurn {
		return false
	}

	// TODO: Check if player can pay costs
	// TODO: Check timing restrictions based on game state
	// TODO: Check if valid targets exist

	return true
}

// Activate activates the ability with the given targets.
func (a *Ability) Activate(player interface{}, targets []interface{}) error {
	if !a.CanActivate(player) {
		return ErrCannotActivate
	}

	// TODO: Pay costs
	// TODO: Put ability on stack
	// TODO: Increment usage counter

	a.UsedThisTurn++
	return nil
}

// Reset resets the ability's per-turn usage counters.
func (a *Ability) Reset() {
	a.UsedThisTurn = 0
}

// AbilityEngine manages all abilities in the types.
type AbilityEngine struct {
	abilities       map[uuid.UUID]*Ability
	triggeredQueue  []*Ability
	stack          []*StackObject
	priorityPlayer interface{}
}

// StackObject represents an ability or spell on the stack.
type StackObject struct {
	ID       uuid.UUID
	Ability  *Ability
	Source   interface{}
	Targets  []interface{}
	Player   interface{}
}

// NewAbilityEngine creates a new ability engine.
func NewAbilityEngine() *AbilityEngine {
	return &AbilityEngine{
		abilities:      make(map[uuid.UUID]*Ability),
		triggeredQueue: make([]*Ability, 0),
		stack:         make([]*StackObject, 0),
	}
}

// RegisterAbility registers an ability with the engine.
func (ae *AbilityEngine) RegisterAbility(ability *Ability) {
	ae.abilities[ability.ID] = ability
}

// UnregisterAbility removes an ability from the engine.
func (ae *AbilityEngine) UnregisterAbility(abilityID uuid.UUID) {
	delete(ae.abilities, abilityID)
}

// TriggerAbilities checks for and triggers any abilities that should trigger.
func (ae *AbilityEngine) TriggerAbilities(condition TriggerCondition, source interface{}) {
	for _, ability := range ae.abilities {
		if ability.Type == Triggered && ability.TriggerCondition == condition {
			// Check if the trigger condition is met for this specific ability
			if ae.shouldTrigger(ability, source) {
				ae.triggeredQueue = append(ae.triggeredQueue, ability)
			}
		}
	}
}

// shouldTrigger determines if a specific ability should trigger.
func (ae *AbilityEngine) shouldTrigger(ability *Ability, source interface{}) bool {
	// TODO: Implement specific trigger condition checking
	// This would check things like "when a creature enters the battlefield"
	// vs "when this creature enters the battlefield"
	return true
}

// ProcessTriggeredAbilities processes all triggered abilities in the queue.
func (ae *AbilityEngine) ProcessTriggeredAbilities() {
	for len(ae.triggeredQueue) > 0 {
		ability := ae.triggeredQueue[0]
		ae.triggeredQueue = ae.triggeredQueue[1:]
		
		// Put triggered ability on stack
		stackObj := &StackObject{
			ID:      uuid.New(),
			Ability: ability,
			Source:  ability.Source,
			Targets: []interface{}{}, // TODO: Choose targets
		}
		ae.stack = append(ae.stack, stackObj)
	}
}

// ResolveStack resolves the top object on the stack.
func (ae *AbilityEngine) ResolveStack() error {
	if len(ae.stack) == 0 {
		return ErrEmptyStack
	}

	// Get top object
	topObject := ae.stack[len(ae.stack)-1]
	ae.stack = ae.stack[:len(ae.stack)-1]

	// Resolve the ability
	return ae.resolveAbility(topObject)
}

// resolveAbility resolves a specific ability.
func (ae *AbilityEngine) resolveAbility(stackObj *StackObject) error {
	// TODO: Implement ability resolution based on effect types
	for _, effect := range stackObj.Ability.Effects {
		err := ae.applyEffect(effect, stackObj.Targets)
		if err != nil {
			return err
		}
	}
	return nil
}

// applyEffect applies a specific effect.
func (ae *AbilityEngine) applyEffect(effect Effect, targets []interface{}) error {
	switch effect.Type {
	case DrawCards:
		// TODO: Implement card drawing
	case DealDamage:
		// TODO: Implement damage dealing
	case GainLife:
		// TODO: Implement life gain
	case AddMana:
		// TODO: Implement mana addition
	default:
		return ErrUnknownEffect
	}
	return nil
}

// ResetTurnCounters resets all per-turn ability usage counters.
func (ae *AbilityEngine) ResetTurnCounters() {
	for _, ability := range ae.abilities {
		ability.Reset()
	}
}
