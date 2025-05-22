package main

import "strings"

// EvergreenAbility represents an evergreen keyword ability in MTG.
type EvergreenAbility struct {
	Name        string
	Description string
}

// List of evergreen abilities
var evergreenAbilities = []EvergreenAbility{
	// Evergreen keywords
	{Name: "Deathtouch", Description: "Any amount of damage this deals to a creature is enough to destroy it."},
	{Name: "Defender", Description: "This creature can't attack."},
	{Name: "First Strike", Description: "This creature deals combat damage before creatures without first strike."},
	{Name: "Double Strike", Description: "This creature deals both first-strike and regular combat damage."},
	{Name: "Enchant", Description: "Attach this Aura to a target permanent as specified in its text."},
	{Name: "Equip", Description: "Attach this Equipment to a target creature you control."},
	{Name: "Flash", Description: "You may cast this spell any time you could cast an instant."},
	{Name: "Flying", Description: "This creature can't be blocked except by creatures with flying or reach."},
	{Name: "Haste", Description: "This creature can attack and {T} as soon as it comes under your control."},
	{Name: "Hexproof", Description: "This creature can't be the target of spells or abilities your opponents control."},
	{Name: "Indestructible", Description: "This permanent can't be destroyed."},
	{Name: "Intimidate", Description: "This creature can't be blocked except by artifact creatures and/or creatures that share a color with it."},
	{Name: "Landwalk", Description: "This creature can't be blocked as long as defending player controls a specified land type."},
	{Name: "Lifelink", Description: "Damage dealt by this creature also causes you to gain that much life."},
	{Name: "Protection", Description: "This permanent can't be blocked, targeted, dealt damage, or enchanted by anything with the specified quality."},
	{Name: "Reach", Description: "This creature can block creatures with flying."},
	{Name: "Shroud", Description: "This permanent can't be the target of any spells or abilities."},
	{Name: "Trample", Description: "Excess combat damage is assigned to the defending player or planeswalker."},
	{Name: "Vigilance", Description: "Attacking doesn't cause this creature to tap."},
	// Keyword actions
	{Name: "Attach", Description: "Attach an Aura, Equipment, or Fortification to a permanent."},
	{Name: "Counter", Description: "To counter a spell or ability is to remove it from the stack."},
	{Name: "Exile", Description: "Put a card into the exile zone."},
	{Name: "Fight", Description: "Two creatures deal damage equal to their power to each other."},
	{Name: "Regenerate", Description: "Replace destruction with tapping, removing damage, and removing from combat."},
	{Name: "Sacrifice", Description: "Put a permanent you control into your graveyard."},
	{Name: "Tap", Description: "Rotate a permanent 90 degrees to indicate it is used."},
	{Name: "Untap", Description: "Return a tapped permanent to its upright position."},
	// Expert-level keywords (mechanics)
	{Name: "Absorb", Description: "If a creature with absorb would be dealt damage, prevent X of that damage."},
	{Name: "Affinity", Description: "This spell costs less to cast for each specified permanent you control."},
	{Name: "Amplify", Description: "As this enters the battlefield, reveal cards to add +1/+1 counters."},
	{Name: "Annihilator", Description: "Whenever this creature attacks, defending player sacrifices permanents."},
	{Name: "Aura swap", Description: "Exchange this Aura with one in your hand."},
	{Name: "Banding", Description: "Creatures with banding can attack or block in a group."},
	{Name: "Bands with other", Description: "Can band only with creatures that have the same ability."},
	{Name: "Battle cry", Description: "When this creature attacks, other attacking creatures get +1/+0."},
	{Name: "Bestow", Description: "Cast this card as an Aura or as a creature."},
	{Name: "Bloodthirst", Description: "Enters the battlefield with +1/+1 counters if an opponent was dealt damage."},
	{Name: "Bushido", Description: "Gets +X/+X until end of turn when blocking or blocked."},
	{Name: "Buyback", Description: "You may pay an additional cost to return this spell to your hand."},
	{Name: "Cascade", Description: "When you cast this spell, exile cards until you exile a nonland card with lesser cost."},
	{Name: "Champion", Description: "Exile another permanent you control when this enters the battlefield."},
	{Name: "Changeling", Description: "This card is every creature type."},
	{Name: "Cipher", Description: "You may exile this spell encoded on a creature you control."},
	{Name: "Clash", Description: "Each player reveals the top card of their library and compares costs."},
	{Name: "Conspire", Description: "You may tap creatures to copy this spell."},
	{Name: "Convoke", Description: "Your creatures can help cast this spell."},
	{Name: "Cumulative upkeep", Description: "Pay an increasing cost during your upkeep or sacrifice this permanent."},
	{Name: "Cycling", Description: "Pay a cost and discard this card to draw a card."},
	{Name: "Delve", Description: "Each card you exile from your graveyard pays for {1}."},
	{Name: "Detain", Description: "Until your next turn, target can't attack, block, or activate abilities."},
	{Name: "Devour", Description: "As this enters the battlefield, you may sacrifice creatures to put +1/+1 counters."},
	{Name: "Dredge", Description: "If you would draw a card, you may return this from your graveyard instead."},
	{Name: "Echo", Description: "Pay its echo cost at the beginning of your upkeep or sacrifice it."},
	{Name: "Entwine", Description: "You may choose all modes of a modal spell if you pay the entwine cost."},
	{Name: "Epic", Description: "For the rest of the game, you can't cast spells. At each upkeep, copy this spell."},
	{Name: "Evolve", Description: "Whenever a creature enters the battlefield under your control, if it has greater power or toughness, put a +1/+1 counter on this creature."},
	{Name: "Evoke", Description: "You may cast this spell for its evoke cost. If you do, it's sacrificed when it enters the battlefield."},
	{Name: "Exalted", Description: "Whenever a creature you control attacks alone, it gets +1/+1 until end of turn."},
	{Name: "Extort", Description: "Whenever you cast a spell, you may pay {W/B}. If you do, each opponent loses 1 life and you gain that much life."},
	{Name: "Fading", Description: "This permanent enters the battlefield with fade counters. Remove one each upkeep. Sacrifice it if you can't."},
	{Name: "Fateseal", Description: "Look at the top X cards of an opponent's library, then put any number on the bottom and the rest on top."},
	{Name: "Fear", Description: "This creature can't be blocked except by artifact creatures and/or black creatures."},
	{Name: "Flanking", Description: "Whenever a creature without flanking blocks this creature, the blocking creature gets -1/-1."},
	{Name: "Flashback", Description: "You may cast this card from your graveyard for its flashback cost."},
	{Name: "Flip", Description: "A keyword action for flip cards."},
	{Name: "Forecast", Description: "During your upkeep, you may reveal this card from your hand and pay its forecast cost to activate its ability."},
	{Name: "Fortify", Description: "Attach this Fortification to a land you control."},
	{Name: "Frenzy", Description: "When this creature attacks and isn't blocked, it gets +X/+0 until end of turn."},
	{Name: "Graft", Description: "This creature enters the battlefield with +1/+1 counters. You may move counters to other creatures."},
	{Name: "Gravestorm", Description: "When you cast this spell, copy it for each permanent put into a graveyard this turn."},
	{Name: "Haunt", Description: "When this card is put into a graveyard, exile it haunting a creature."},
	{Name: "Hideaway", Description: "This permanent enters the battlefield tapped. When it does, look at the top four cards of your library, exile one face down, then put the rest on the bottom."},
	{Name: "Horsemanship", Description: "This creature can't be blocked except by creatures with horsemanship."},
	{Name: "Infect", Description: "This creature deals damage to creatures in the form of -1/-1 counters and to players in the form of poison counters."},
	{Name: "Kicker", Description: "You may pay an additional cost as you cast this spell for an additional effect."},
	{Name: "Level up", Description: "Pay a cost to put a level counter on this creature."},
	{Name: "Living weapon", Description: "When this Equipment enters the battlefield, create a 0/0 black Germ creature token, then attach this to it."},
	{Name: "Madness", Description: "If you discard this card, you may cast it for its madness cost instead of putting it into your graveyard."},
	{Name: "Miracle", Description: "You may cast this card for its miracle cost if it's the first card you drew this turn."},
	{Name: "Modular", Description: "This creature enters the battlefield with +1/+1 counters. When it dies, you may put its counters on another artifact creature."},
	{Name: "Monstrosity", Description: "If this creature isn't monstrous, put X +1/+1 counters on it and it becomes monstrous."},
	{Name: "Morph", Description: "You may cast this card face down as a 2/2 creature for {3}. Turn it face up any time for its morph cost."},
	{Name: "Multikicker", Description: "You may pay the kicker cost any number of times as you cast this spell."},
	{Name: "Ninjutsu", Description: "Pay a cost, return an unblocked attacker you control to hand: Put this card onto the battlefield tapped and attacking."},
	{Name: "Offering", Description: "You may cast this card any time you could cast an instant by sacrificing a specified permanent and paying the difference in mana costs."},
	{Name: "Overload", Description: "You may pay the overload cost to change 'target' to 'each' in the spell's text."},
	{Name: "Persist", Description: "When this creature dies, if it had no -1/-1 counters, return it to the battlefield with a -1/-1 counter."},
	{Name: "Phasing", Description: "This permanent phases in and out of existence."},
	{Name: "Poisonous", Description: "Whenever this creature deals combat damage to a player, that player gets X poison counters."},
	{Name: "Populate", Description: "Create a token that's a copy of a creature token you control."},
	{Name: "Proliferate", Description: "Choose any number of permanents and/or players, then give each another counter of a kind already there."},
	{Name: "Provoke", Description: "When this creature attacks, you may have target creature untap and block it if able."},
	{Name: "Prowl", Description: "You may cast this card for its prowl cost if you dealt combat damage with a specified creature type this turn."},
	{Name: "Rampage", Description: "Whenever this creature becomes blocked, it gets +X/+X until end of turn for each creature blocking it beyond the first."},
	{Name: "Rebound", Description: "If you cast this spell from your hand, exile it as it resolves. At the beginning of your next upkeep, you may cast it from exile without paying its mana cost."},
	{Name: "Recover", Description: "When a creature is put into your graveyard from the battlefield, you may pay the recover cost to return this card from your graveyard to your hand."},
	{Name: "Reinforce", Description: "You may discard this card to put +1/+1 counters on a creature."},
	{Name: "Replicate", Description: "When you cast this spell, copy it for each time you paid its replicate cost."},
	{Name: "Retrace", Description: "You may cast this card from your graveyard by discarding a land card in addition to paying its other costs."},
	{Name: "Ripple", Description: "When you cast this spell, you may reveal the top X cards of your library. You may cast any revealed cards with the same name without paying their mana costs."},
	{Name: "Scavenge", Description: "Exile this card from your graveyard to put a number of +1/+1 counters on a creature equal to this card's power."},
	{Name: "Scry", Description: "Look at the top X cards of your library, then put any number on the bottom and the rest on top in any order."},
	{Name: "Shadow", Description: "This creature can block or be blocked only by creatures with shadow."},
	{Name: "Soulbond", Description: "You may pair this creature with another unpaired creature when either enters the battlefield."},
	{Name: "Soulshift", Description: "When this creature dies, you may return a Spirit card with converted mana cost X or less from your graveyard to your hand."},
	{Name: "Splice", Description: "As you cast a spell, you may reveal any number of cards with splice and pay their costs to add their effects to the spell."},
	{Name: "Split second", Description: "As long as this spell is on the stack, players can't cast spells or activate abilities that aren't mana abilities."},
	{Name: "Storm", Description: "When you cast this spell, copy it for each spell cast before it this turn."},
	{Name: "Sunburst", Description: "This permanent enters the battlefield with a counter for each color of mana spent to cast it."},
	{Name: "Suspend", Description: "Rather than cast this card from your hand, you may pay its suspend cost and exile it with time counters."},
	{Name: "Totem armor", Description: "If enchanted creature would be destroyed, instead remove all damage from it and destroy this Aura."},
	{Name: "Transfigure", Description: "Pay a cost and sacrifice this creature: Search your library for a creature card with the same converted mana cost and put it onto the battlefield."},
	{Name: "Transform", Description: "Turn this double-faced card over to its other face."},
	{Name: "Transmute", Description: "Pay a cost and discard this card: Search your library for a card with the same converted mana cost and put it into your hand."},
	{Name: "Typecycling", Description: "Pay a cost and discard this card: Search your library for a card of the specified type."},
	{Name: "Undying", Description: "When this creature dies, if it had no +1/+1 counters, return it to the battlefield with a +1/+1 counter."},
	{Name: "Unearth", Description: "Pay a cost: Return this card from your graveyard to the battlefield. Exile it at end of turn or if it would leave the battlefield."},
	{Name: "Unleash", Description: "You may have this creature enter the battlefield with a +1/+1 counter. It can't block as long as it has a +1/+1 counter."},
	{Name: "Vanishing", Description: "This permanent enters the battlefield with time counters. Remove one each upkeep. Sacrifice it if you can't."},
	{Name: "Wither", Description: "This deals damage to creatures in the form of -1/-1 counters."},
	// Ability words
	{Name: "Battalion", Description: "Whenever this and at least two other creatures attack, an effect happens."},
	{Name: "Bloodrush", Description: "You may discard this card to give an attacking creature a bonus."},
	{Name: "Channel", Description: "You may discard this card for a cost to yield a specified effect."},
	{Name: "Chroma", Description: "An effect based on the number of colored mana symbols among cards you control."},
	{Name: "Domain", Description: "An effect based on the number of basic land types among lands you control."},
	{Name: "Fateful hour", Description: "An effect if you have 5 or less life."},
	{Name: "Grandeur", Description: "Discard another card with the same name for an effect."},
	{Name: "Hellbent", Description: "An effect if you have no cards in hand."},
	{Name: "Heroic", Description: "Whenever you cast a spell that targets this creature, an effect happens."},
	{Name: "Imprint", Description: "Exile a card to grant abilities to this permanent."},
	{Name: "Join forces", Description: "All players may contribute to an effect."},
	{Name: "Kinship", Description: "At the beginning of your upkeep, you may reveal the top card of your library for an effect if it shares a creature type."},
	{Name: "Landfall", Description: "Whenever a land enters the battlefield under your control, an effect happens."},
	{Name: "Metalcraft", Description: "An effect if you control three or more artifacts."},
	{Name: "Morbid", Description: "An effect if a creature died this turn."},
	{Name: "Radiance", Description: "An effect that targets a permanent and all others that share a color with it."},
	{Name: "Sweep", Description: "An effect that can be strengthened by returning lands to your hand."},
	{Name: "Threshold", Description: "An effect if you have seven or more cards in your graveyard."},
	// Discontinued keywords
	{Name: "Bury", Description: "Destroy a permanent and it can't be regenerated."},
	{Name: "Landhome", Description: "A creature can only attack a player who controls a specified land type and must be sacrificed if you don't control one."},
	{Name: "Substance", Description: "A static ability with no effect, used for rules purposes only."},
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
