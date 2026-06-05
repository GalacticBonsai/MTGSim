// Package ability provides error definitions for the ability engine.
package ability

import "errors"

// Common errors for the ability engine.
var (
	ErrCannotActivate    = errors.New("ability cannot be activated")
	ErrInsufficientMana  = errors.New("insufficient mana to pay cost")
	ErrInvalidTarget     = errors.New("invalid target for ability")
	ErrNoValidTargets    = errors.New("no valid targets available")

	ErrParsingFailed     = errors.New("failed to parse ability from oracle text")
	ErrTimingRestriction = errors.New("ability cannot be activated at this time")
	ErrAlreadyUsed       = errors.New("ability already used this turn")
	ErrInvalidCost       = errors.New("invalid cost specification")
)
