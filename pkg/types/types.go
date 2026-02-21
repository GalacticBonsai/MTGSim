// Package types provides shared types and constants for MTG simulation.
package types

// LogLevel represents different levels of logging detail.
type LogLevel int

const (
	META LogLevel = iota
	GAME
	PLAYER
	CARD
)

// ManaType represents different types of mana in Magic: The Gathering.
type ManaType string

const (
	White     ManaType = "W"
	Blue      ManaType = "U"
	Black     ManaType = "B"
	Red       ManaType = "R"
	Green     ManaType = "G"
	Colorless ManaType = "C"
	Any       ManaType = "A"
	Phyrexian ManaType = "P"
	Snow      ManaType = "S"
	X         ManaType = "X"
)

// PermanentType represents different types of permanents on the battlefield.
type PermanentType int

const (
	Creature PermanentType = iota
	Artifact
	Enchantment
	Land
	Planeswalker
)
