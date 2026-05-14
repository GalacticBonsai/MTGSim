package ability

import (
	"strconv"
	"strings"
)

func playerControlsMatching(player AbilityPlayer, value string) bool {
	needle := strings.ToLower(strings.TrimSpace(value))
	for _, permanent := range append(player.GetCreatures(), player.GetLands()...) {
		if named, ok := permanent.(interface{ GetName() string }); ok && strings.Contains(strings.ToLower(named.GetName()), needle) {
			return true
		}
		if typeLine, ok := permanent.(interface{ GetTypeLine() string }); ok && strings.Contains(strings.ToLower(typeLine.GetTypeLine()), needle) {
			return true
		}
	}
	return false
}

func playerControlsCreatureWithPowerGreater(player AbilityPlayer, value string) bool {
	threshold, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	for _, creature := range player.GetCreatures() {
		if powered, ok := creature.(interface{ GetPower() int }); ok && powered.GetPower() > threshold {
			return true
		}
	}
	return false
}

func (ee *ExecutionEngine) opponentsOf(controller AbilityPlayer) []AbilityPlayer {
	var opponents []AbilityPlayer
	if ee == nil || ee.gameState == nil || controller == nil {
		return opponents
	}
	for _, player := range ee.gameState.GetAllPlayers() {
		if player != nil && player.GetName() != controller.GetName() {
			opponents = append(opponents, player)
		}
	}
	return opponents
}

func (ee *ExecutionEngine) hasMoreLifeThanAnOpponent(controller AbilityPlayer) bool {
	for _, opponent := range ee.opponentsOf(controller) {
		if controller.GetLifeTotal() > opponent.GetLifeTotal() {
			return true
		}
	}
	return false
}

func (ee *ExecutionEngine) opponentHasMoreCreatures(controller AbilityPlayer) bool {
	for _, opponent := range ee.opponentsOf(controller) {
		if len(opponent.GetCreatures()) > len(controller.GetCreatures()) {
			return true
		}
	}
	return false
}

func (ee *ExecutionEngine) hasMoreLandsThanAnOpponent(controller AbilityPlayer) bool {
	for _, opponent := range ee.opponentsOf(controller) {
		if len(controller.GetLands()) > len(opponent.GetLands()) {
			return true
		}
	}
	return false
}

func (ee *ExecutionEngine) hasMoreCardsInHandThanAnOpponent(controller AbilityPlayer) bool {
	for _, opponent := range ee.opponentsOf(controller) {
		if len(controller.GetHand()) > len(opponent.GetHand()) {
			return true
		}
	}
	return false
}
