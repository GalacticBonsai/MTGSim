package main

import "strings"

// EvergreenAbility represents an evergreen keyword ability in MTG.
type EvergreenAbility struct {
	Name        string
	Description string
}

// List of evergreen abilities
var evergreenAbilities = []EvergreenAbility{
	{Name: "Deathtouch", Description: "Any amount of damage this deals to a creature is enough to destroy it."},
	{Name: "Defender", Description: "This creature can't attack."},
	{Name: "Double Strike", Description: "This creature deals both first-strike and regular combat damage."},
	{Name: "First Strike", Description: "This creature deals combat damage before creatures without first strike."},
	{Name: "Flash", Description: "You may cast this spell any time you could cast an instant."},
	{Name: "Flying", Description: "This creature can't be blocked except by creatures with flying or reach."},
	{Name: "Haste", Description: "This creature can attack and {T} as soon as it comes under your control."},
	{Name: "Hexproof", Description: "This creature can't be the target of spells or abilities your opponents control."},
	{Name: "Indestructible", Description: "This creature can't be destroyed."},
	{Name: "Lifelink", Description: "Damage dealt by this creature also causes you to gain that much life."},
	{Name: "Menace", Description: "This creature can't be blocked except by two or more creatures."},
	{Name: "Reach", Description: "This creature can block creatures with flying."},
	{Name: "Trample", Description: "Excess combat damage this creature deals to a player or planeswalker is assigned to the defending player or planeswalker."},
	{Name: "Vigilance", Description: "Attacking doesn't cause this creature to tap."},
	{Name: "Ward", Description: "Whenever this creature becomes the target of a spell or ability an opponent controls, counter it unless that player pays the ward cost."},
	{Name: "Equip", Description: "Attach this Equipment to a target creature you control."},
	{Name: "Enchant", Description: "Attach this Aura to a target permanent as specified in its text."},
	{Name: "Scry", Description: "Look at the top X cards of your library, then put any number of them on the bottom of your library and the rest on top in any order."},
	{Name: "Mill", Description: "Put the top X cards of a player's library into their graveyard."},
	{Name: "Fight", Description: "Two creatures deal damage equal to their power to each other."},
	{Name: "Goad", Description: "Until your next turn, this creature attacks each combat if able and attacks a player other than you if able."},
}

// GetEvergreenAbilityByName retrieves an evergreen ability by its name.
func GetEvergreenAbilityByName(name string) (EvergreenAbility, bool) {
	for _, ability := range evergreenAbilities {
		if strings.EqualFold(ability.Name, name) {
			return ability, true
		}
	}
	return EvergreenAbility{}, false
}

// CardHasEvergreenAbility checks if a card has a specific evergreen ability.
func CardHasEvergreenAbility(card Card, abilityName string) bool {
	for _, keyword := range card.Keywords {
		if strings.EqualFold(keyword, abilityName) {
			return true
		}
	}
	return false
}
