// Package game defines core game primitives for MTG simulation.
package game

// Zone represents a location a card/permanent can exist in.
type Zone int

const (
	Library Zone = iota
	Hand
	Battlefield
	Graveyard
	Exile
	Command
)

func (z Zone) String() string {
	switch z {
	case Library:
		return "Library"
	case Hand:
		return "Hand"
	case Battlefield:
		return "Battlefield"
	case Graveyard:
		return "Graveyard"
	case Exile:
		return "Exile"
	case Command:
		return "Command"
	default:
		return "Unknown"
	}
}
