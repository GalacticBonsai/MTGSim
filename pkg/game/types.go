// Package game provides core game types and constants for MTG simulation.
package game

import "github.com/mtgsim/mtgsim/pkg/types"

// Re-export types for backward compatibility
type LogLevel = types.LogLevel
type ManaType = types.ManaType
type PermanentType = types.PermanentType

// Re-export constants for backward compatibility
const (
	META   = types.META
	GAME   = types.GAME
	PLAYER = types.PLAYER
	CARD   = types.CARD
)

const (
	White     = types.White
	Blue      = types.Blue
	Black     = types.Black
	Red       = types.Red
	Green     = types.Green
	Colorless = types.Colorless
	Any       = types.Any
	Phyrexian = types.Phyrexian
	Snow      = types.Snow
	X         = types.X
)

const (
	Creature     = types.Creature
	Artifact     = types.Artifact
	Enchantment  = types.Enchantment
	Land         = types.Land
	Planeswalker = types.Planeswalker
)
