package main

// Log levels
type LogLevel int

const (
	META LogLevel = iota
	GAME
	PLAYER
	CARD
)

// Mana types
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

// Permanent types
type PermanentType int

const (
	Creature PermanentType = iota
	Artifact
	Enchantment
	Land
	Planeswalker
)
