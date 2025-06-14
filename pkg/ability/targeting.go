// Package ability provides comprehensive target validation for MTG abilities.
package ability

import (
	"strings"
)

// TargetRestriction represents a restriction on what can be targeted.
type TargetRestriction struct {
	Type        TargetRestrictionType
	Value       interface{} // For numeric restrictions like power/CMC
	Negated     bool        // For "non-" restrictions
	Description string      // Human-readable description
}

// TargetRestrictionType represents different types of targeting restrictions.
type TargetRestrictionType int

const (
	// Basic type restrictions
	NoRestriction TargetRestrictionType = iota
	CreatureRestriction
	ArtifactRestriction
	EnchantmentRestriction
	LandRestriction
	PlaneswalkerRestriction
	PermanentRestriction
	SpellRestriction
	PlayerRestriction

	// Ability-based restrictions
	FlyingRestriction
	TrampleRestriction
	VigilanceRestriction
	FirstStrikeRestriction
	DoubleStrikeRestriction
	DeathtouchRestriction
	LifelinkRestriction
	HexproofRestriction
	ShroudRestriction
	ProtectionRestriction

	// Numeric restrictions
	PowerRestriction
	ToughnessRestriction
	CMCRestriction
	PowerLessEqualRestriction
	ToughnessLessEqualRestriction
	CMCLessEqualRestriction
	PowerGreaterEqualRestriction
	ToughnessGreaterEqualRestriction
	CMCGreaterEqualRestriction

	// Control restrictions
	YouControlRestriction
	YouDontControlRestriction
	OpponentControlsRestriction

	// Color restrictions
	WhiteRestriction
	BlueRestriction
	BlackRestriction
	RedRestriction
	GreenRestriction
	ColorlessRestriction
	MonocoloredRestriction
	MulticoloredRestriction

	// Special restrictions
	TappedRestriction
	UntappedRestriction
	AttackingRestriction
	BlockingRestriction
	EnchantedRestriction
	EquippedRestriction
)

// EnhancedTarget extends the basic Target with comprehensive restrictions.
type EnhancedTarget struct {
	Type         TargetType
	Required     bool
	Count        int
	Restrictions []TargetRestriction
	IsEach       bool // True for "each" effects that don't target
	Description  string
}

// TargetValidator validates targets against restrictions.
type TargetValidator struct {
	gameState GameState
}

// NewTargetValidator creates a new target validator.
func NewTargetValidator(gameState GameState) *TargetValidator {
	return &TargetValidator{
		gameState: gameState,
	}
}

// TargetingLegality represents whether a target is legal and why.
type TargetingLegality struct {
	IsLegal bool
	Reason  string
}

// ValidateTarget checks if a potential target meets all restrictions.
func (tv *TargetValidator) ValidateTarget(target interface{}, enhancedTarget EnhancedTarget, controller AbilityPlayer) TargetingLegality {
	// Check basic type compatibility
	if !tv.isValidTargetType(target, enhancedTarget.Type) {
		return TargetingLegality{
			IsLegal: false,
			Reason:  "Target type mismatch",
		}
	}

	// Check targeting legality (hexproof, shroud, protection)
	if legality := tv.checkTargetingLegality(target, controller); !legality.IsLegal {
		return legality
	}

	// Check all restrictions
	for _, restriction := range enhancedTarget.Restrictions {
		if legality := tv.checkRestriction(target, restriction, controller); !legality.IsLegal {
			return legality
		}
	}

	return TargetingLegality{IsLegal: true, Reason: "Valid target"}
}

// isValidTargetType checks if the target matches the basic target type.
func (tv *TargetValidator) isValidTargetType(target interface{}, targetType TargetType) bool {
	switch targetType {
	case CreatureTarget:
		return tv.isCreature(target)
	case PlayerTarget:
		return tv.isPlayer(target)
	case PermanentTarget:
		return tv.isPermanent(target)
	case AnyTarget:
		return tv.isPlayer(target) || tv.isPermanent(target)
	default:
		return false
	}
}

// checkTargetingLegality checks hexproof, shroud, protection, etc.
func (tv *TargetValidator) checkTargetingLegality(target interface{}, controller AbilityPlayer) TargetingLegality {
	// Check if target has shroud (can't be targeted by anything)
	if tv.hasShroud(target) {
		return TargetingLegality{
			IsLegal: false,
			Reason:  "Target has shroud",
		}
	}

	// Check if target has hexproof (can't be targeted by opponents)
	if tv.hasHexproof(target) && !tv.isControlledBy(target, controller) {
		return TargetingLegality{
			IsLegal: false,
			Reason:  "Target has hexproof and you don't control it",
		}
	}

	// Check protection (simplified - would need color/type information)
	if tv.hasProtection(target, controller) {
		return TargetingLegality{
			IsLegal: false,
			Reason:  "Target has protection from this source",
		}
	}

	return TargetingLegality{IsLegal: true, Reason: "No targeting restrictions"}
}

// checkRestriction checks a specific targeting restriction.
func (tv *TargetValidator) checkRestriction(target interface{}, restriction TargetRestriction, controller AbilityPlayer) TargetingLegality {
	result := tv.evaluateRestriction(target, restriction, controller)
	
	// Apply negation if needed
	if restriction.Negated {
		result = !result
	}

	if !result {
		return TargetingLegality{
			IsLegal: false,
			Reason:  "Target doesn't meet restriction: " + restriction.Description,
		}
	}

	return TargetingLegality{IsLegal: true, Reason: "Restriction satisfied"}
}

// evaluateRestriction evaluates a specific restriction against a target.
func (tv *TargetValidator) evaluateRestriction(target interface{}, restriction TargetRestriction, controller AbilityPlayer) bool {
	switch restriction.Type {
	case CreatureRestriction:
		return tv.isCreature(target)
	case ArtifactRestriction:
		return tv.isArtifact(target)
	case EnchantmentRestriction:
		return tv.isEnchantment(target)
	case LandRestriction:
		return tv.isLand(target)
	case PlaneswalkerRestriction:
		return tv.isPlaneswalker(target)

	case FlyingRestriction:
		return tv.hasAbility(target, "flying")
	case TrampleRestriction:
		return tv.hasAbility(target, "trample")
	case VigilanceRestriction:
		return tv.hasAbility(target, "vigilance")
	case FirstStrikeRestriction:
		return tv.hasAbility(target, "first strike")
	case DeathtouchRestriction:
		return tv.hasAbility(target, "deathtouch")
	case LifelinkRestriction:
		return tv.hasAbility(target, "lifelink")

	case PowerLessEqualRestriction:
		if value, ok := restriction.Value.(int); ok {
			return tv.getPower(target) <= value
		}
	case ToughnessLessEqualRestriction:
		if value, ok := restriction.Value.(int); ok {
			return tv.getToughness(target) <= value
		}
	case CMCLessEqualRestriction:
		if value, ok := restriction.Value.(int); ok {
			return tv.getCMC(target) <= value
		}

	case PowerGreaterEqualRestriction:
		if value, ok := restriction.Value.(int); ok {
			return tv.getPower(target) >= value
		}
	case ToughnessGreaterEqualRestriction:
		if value, ok := restriction.Value.(int); ok {
			return tv.getToughness(target) >= value
		}
	case CMCGreaterEqualRestriction:
		if value, ok := restriction.Value.(int); ok {
			return tv.getCMC(target) >= value
		}

	case YouControlRestriction:
		return tv.isControlledBy(target, controller)
	case YouDontControlRestriction:
		return !tv.isControlledBy(target, controller)
	case OpponentControlsRestriction:
		return tv.isControlledByOpponent(target, controller)

	case TappedRestriction:
		return tv.isTapped(target)
	case UntappedRestriction:
		return !tv.isTapped(target)

	default:
		return true // Unknown restrictions default to true
	}
	return false
}

// TypeChecker interface for checking permanent types
type TypeChecker interface {
	IsCreature() bool
	IsArtifact() bool
	IsEnchantment() bool
	IsLand() bool
	IsPlaneswalker() bool
}

// Helper methods for checking target properties
func (tv *TargetValidator) isCreature(target interface{}) bool {
	// Check if target implements TypeChecker interface
	if checker, ok := target.(TypeChecker); ok {
		return checker.IsCreature()
	}
	// Fallback to name-based checking
	if permanent, ok := target.(AbilityPermanent); ok {
		return strings.Contains(strings.ToLower(permanent.GetName()), "creature")
	}
	return false
}

func (tv *TargetValidator) isPlayer(target interface{}) bool {
	_, ok := target.(AbilityPlayer)
	return ok
}

func (tv *TargetValidator) isPermanent(target interface{}) bool {
	_, ok := target.(AbilityPermanent)
	return ok
}

func (tv *TargetValidator) isArtifact(target interface{}) bool {
	// Check if target implements TypeChecker interface
	if checker, ok := target.(TypeChecker); ok {
		return checker.IsArtifact()
	}
	// Fallback to name-based checking
	if permanent, ok := target.(AbilityPermanent); ok {
		return strings.Contains(strings.ToLower(permanent.GetName()), "artifact")
	}
	return false
}

func (tv *TargetValidator) isEnchantment(target interface{}) bool {
	// Check if target implements TypeChecker interface
	if checker, ok := target.(TypeChecker); ok {
		return checker.IsEnchantment()
	}
	// Fallback to name-based checking
	if permanent, ok := target.(AbilityPermanent); ok {
		return strings.Contains(strings.ToLower(permanent.GetName()), "enchantment")
	}
	return false
}

func (tv *TargetValidator) isLand(target interface{}) bool {
	// Check if target implements TypeChecker interface
	if checker, ok := target.(TypeChecker); ok {
		return checker.IsLand()
	}
	// Fallback to name-based checking
	if permanent, ok := target.(AbilityPermanent); ok {
		return strings.Contains(strings.ToLower(permanent.GetName()), "land")
	}
	return false
}

func (tv *TargetValidator) isPlaneswalker(target interface{}) bool {
	// Check if target implements TypeChecker interface
	if checker, ok := target.(TypeChecker); ok {
		return checker.IsPlaneswalker()
	}
	// Fallback to name-based checking
	if permanent, ok := target.(AbilityPermanent); ok {
		return strings.Contains(strings.ToLower(permanent.GetName()), "planeswalker")
	}
	return false
}

func (tv *TargetValidator) hasAbility(target interface{}, abilityName string) bool {
	// This would check if the permanent has the specified ability
	// For now, simplified implementation
	return false
}

// CreatureStats interface for getting creature statistics
type CreatureStats interface {
	GetPower() int
	GetToughness() int
	GetCMC() int
}

func (tv *TargetValidator) getPower(target interface{}) int {
	// Check if target implements CreatureStats interface
	if stats, ok := target.(CreatureStats); ok {
		return stats.GetPower()
	}
	// For real implementation, this would get the power from the permanent
	return 0
}

func (tv *TargetValidator) getToughness(target interface{}) int {
	// Check if target implements CreatureStats interface
	if stats, ok := target.(CreatureStats); ok {
		return stats.GetToughness()
	}
	// For real implementation, this would get the toughness from the permanent
	return 0
}

func (tv *TargetValidator) getCMC(target interface{}) int {
	// Check if target implements CreatureStats interface
	if stats, ok := target.(CreatureStats); ok {
		return stats.GetCMC()
	}
	// For real implementation, this would get the CMC from the permanent
	return 0
}

func (tv *TargetValidator) isControlledBy(target interface{}, player AbilityPlayer) bool {
	if permanent, ok := target.(AbilityPermanent); ok {
		return permanent.GetController().GetName() == player.GetName()
	}
	return false
}

func (tv *TargetValidator) isControlledByOpponent(target interface{}, player AbilityPlayer) bool {
	if permanent, ok := target.(AbilityPermanent); ok {
		return permanent.GetController().GetName() != player.GetName()
	}
	return false
}

func (tv *TargetValidator) isTapped(target interface{}) bool {
	if permanent, ok := target.(AbilityPermanent); ok {
		return permanent.IsTapped()
	}
	return false
}

func (tv *TargetValidator) hasShroud(target interface{}) bool {
	// This would check if the target has shroud
	// For now, simplified implementation
	return false
}

func (tv *TargetValidator) hasHexproof(target interface{}) bool {
	// This would check if the target has hexproof
	// For now, simplified implementation
	return false
}

func (tv *TargetValidator) hasProtection(target interface{}, source AbilityPlayer) bool {
	// This would check if the target has protection from the source
	// For now, simplified implementation
	return false
}
