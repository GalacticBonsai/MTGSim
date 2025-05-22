package main

import (
	"fmt"
	"strings"
)

type Player struct {
	Name          string
	LifeTotal     int
	Deck          Deck
	Hand          []Card
	Graveyard     []Card
	Exile         []Card
	Creatures     []Permanant
	Enchantments  []Permanant
	Artifacts     []Permanant
	Planeswalkers []Permanant
	Lands         []Permanant
	// mana          mana
	Opponents []*Player
}

func NewPlayer(decklist string) *Player {
	deck, _, err := importDeckfile(decklist)
	if err != nil {
		// handle the error appropriately, e.g., log it or return it
		panic(err)
	}
	return &Player{
		Name:      deck.Name,
		LifeTotal: 20,
		Deck:      deck,
	}
}

func (p *Player) PlayTurn() {
	// p.Display()
	t := newTurn()
	for _, phase := range t.phases {
		for _, s := range phase.steps {
			p.PlayStep(s, t)
		}
	}
}

func (p *Player) PlayStep(s step, t *turn) {
	switch s.name {
	case "Untap Step":
		for i := range p.Lands {
			p.Lands[i].untap()
		}
		for i := range p.Creatures {
			p.Creatures[i].untap()
			if !CardHasEvergreenAbility(p.Creatures[i].source, "Haste") {
				p.Creatures[i].summoningSickness = false
			}
		}
		for i := range p.Artifacts {
			p.Artifacts[i].untap()
		}
	case "Upkeep Step":
	case "Draw Step":
		p.Hand = append(p.Hand, p.Deck.DrawCard())
	case "Play Land":
		p.PlayLand(t)
	case "Cast Spells":
		p.PlaySpell()
	case "Beginning of Combat Step":
	case "Declare Attackers Step":
		p.DeclareAttackers()
	case "Declare Blockers Step":
		p.Opponents[0].DeclareBlockers()
	case "Combat Damage Step":
		p.DealDamage()
	case "End of Combat Step":
		p.CleanupCombat()
		p.Opponents[0].CleanupCombat()
	case "End Step":
		p.EndStep()
		p.Opponents[0].EndStep()
	case "Cleanup Step":
		// discard down to 7
		var c Card
		for len(p.Hand) > 7 {
			c, p.Hand = sliceGet(p.Hand, 0)
			LogPlayer("Discarding: %s", c.Name)
			p.Graveyard = append(p.Graveyard, c)
		}
	}
}

func (p *Player) CleanupCombat() {
	for i := range p.Creatures {
		p.Creatures[i].attacking = nil
		p.Creatures[i].blocking = nil
		p.Creatures[i].blocked = false
	}
}

func (p *Player) DealDamage() {
	// Handle first-strike damage
	LogPlayer("First Strike Damage Step")
	for _, creature := range p.Creatures {
		if creature.attacking != nil && (CardHasEvergreenAbility(creature.source, "First Strike") || CardHasEvergreenAbility(creature.source, "Double Strike")) {
			p.resolveCombatDamage(&creature)
		}
	}
	for _, creature := range p.Opponents[0].Creatures {
		if creature.blocking != nil && (CardHasEvergreenAbility(creature.source, "First Strike") || CardHasEvergreenAbility(creature.source, "Double Strike")) {
			p.Opponents[0].resolveCombatDamage(&creature)
		}
	}

	// Remove creatures with 0 or less toughness after first-strike damage
	p.cleanupDeadCreatures()
	p.Opponents[0].cleanupDeadCreatures()

	// Handle regular damage
	LogPlayer("Regular Damage Step")
	for _, creature := range p.Creatures {
		if creature.attacking != nil && (!CardHasEvergreenAbility(creature.source, "First Strike")) {
			p.resolveCombatDamage(&creature)
		}
	}
	for _, creature := range p.Opponents[0].Creatures {
		if creature.blocking != nil && !CardHasEvergreenAbility(creature.source, "First Strike") {
			p.Opponents[0].resolveCombatDamage(&creature)
		}
	}

	// Remove creatures with 0 or less toughness after regular damage
	p.cleanupDeadCreatures()
	p.Opponents[0].cleanupDeadCreatures()
}

func (p *Player) resolveCombatDamage(creature *Permanant) {
	if creature.blocking != nil {
		// Handle Deathtouch: Any amount of damage is enough to destroy the blocker
		if CardHasEvergreenAbility(creature.source, "Deathtouch") {
			LogPlayer("%s deals damage with Deathtouch to %s.", creature.source.Name, creature.blocking.source.Name)
			creature.blocking.damage_counters = creature.blocking.toughness // Ensure the blocker is destroyed
		} else {
			creature.damages(creature.blocking)
		}

		creature.blocking.damages(creature)
		creature.checkLife()
		creature.blocking.checkLife()
	} else if creature.attacking != nil {
		damage := creature.power

		// Handle Trample
		if CardHasEvergreenAbility(creature.source, "Trample") && creature.blocking != nil {
			excessDamage := damage - creature.blocking.toughness
			if excessDamage > 0 {
				creature.attacking.LifeTotal -= excessDamage
				LogPlayer("%s deals %d excess damage to %s with Trample.", creature.source.Name, excessDamage, creature.attacking.Name)
			}
		} else {
			creature.attacking.LifeTotal -= damage
		}

		LogPlayer("%s deals %d damage to %s", creature.source.Name, damage, creature.attacking.Name)
	}
}

func (p *Player) cleanupDeadCreatures() {
	for i := 0; i < len(p.Creatures); i++ {
		if p.Creatures[i].toughness <= p.Creatures[i].damage_counters {
			LogPlayer("%s dies due to 0 toughness.", p.Creatures[i].source.Name)
			destroyPermanant(&p.Creatures[i])
			i-- // Adjust index after removing an element
		}
	}
}

// CanBlock determines if blocker can block attacker, considering abilities like Flying, Reach, Intimidate, Shadow, Fear, etc.
// TODO: Implement Menace (must be blocked by two or more creatures), Protection, and other complex abilities in the combat assignment logic.
func CanBlock(attacker, blocker Permanant) bool {
	// Flying: can only be blocked by creatures with flying or reach
	if CardHasEvergreenAbility(attacker.source, "Flying") {
		if CardHasEvergreenAbility(blocker.source, "Flying") || CardHasEvergreenAbility(blocker.source, "Reach") {
			return true
		}
		return false
	}
	// Intimidate: can only be blocked by artifact creatures and/or creatures that share a color
	if CardHasEvergreenAbility(attacker.source, "Intimidate") {
		if strings.Contains(blocker.source.TypeLine, "Artifact") {
			return true
		}
		for _, color := range attacker.source.Colors {
			for _, bcolor := range blocker.source.Colors {
				if color == bcolor {
					return true
				}
			}
		}
		return false
	}
	// Menace: must be blocked by two or more creatures (handled in block assignment logic)
	// Shadow: can only be blocked by creatures with shadow
	if CardHasEvergreenAbility(attacker.source, "Shadow") {
		if CardHasEvergreenAbility(blocker.source, "Shadow") {
			return true
		}
		return false
	}
	// Fear: can only be blocked by artifact creatures and/or black creatures
	if CardHasEvergreenAbility(attacker.source, "Fear") {
		if strings.Contains(blocker.source.TypeLine, "Artifact") {
			return true
		}
		for _, bcolor := range blocker.source.Colors {
			if bcolor == "B" {
				return true
			}
		}
		return false
	}
	// Protection: can't be blocked by creatures with the protected quality
	if CardHasEvergreenAbility(attacker.source, "Protection") {
		// Check for color protection
		for _, kw := range attacker.source.Keywords {
			if strings.HasPrefix(kw, "Protection from ") {
				prot := strings.TrimPrefix(kw, "Protection from ")
				// Check if blocker matches the protection
				if prot == "Artifacts" && strings.Contains(blocker.source.TypeLine, "Artifact") {
					return false
				}
				if prot == "Black" || prot == "White" || prot == "Blue" || prot == "Red" || prot == "Green" {
					for _, bcolor := range blocker.source.Colors {
						if strings.EqualFold(bcolor, string(prot[0])) { // e.g. "B" for Black
							return false
						}
					}
				}
				// TODO: Add more protection types as needed
			}
		}
	}
	// Default: can be blocked
	return true
}

func (p *Player) DeclareBlockers() {
	for i, creature := range p.Creatures {
		if creature.tapped {
			continue
		}
		for j, attacker := range p.Opponents[0].Creatures {
			if attacker.attacking == p && !attacker.blocked {
				if CanBlock(attacker, creature) {
					p.Creatures[i].blocking = &p.Opponents[0].Creatures[j]
					p.Opponents[0].Creatures[j].blocked = true
					LogPlayer("%s blocked by %s", attacker.source.Name, creature.source.Name)
					break // exit out to not block all attackers
				}
			}
		}
	}
}

func (p *Player) DeclareAttackers() {
	LogPlayer("Declare attacker:")
	attacking := false
	for i, creature := range p.Creatures {
		if creature.tapped || (creature.summoningSickness && !CardHasEvergreenAbility(creature.source, "Haste") || CardHasEvergreenAbility(creature.source, "Defender")) {
			continue
		}

		creature.Display()
		p.Creatures[i].attacking = p.Opponents[0]
		attacking = true

		// Handle Vigilance: Attacking doesn't cause this creature to tap
		if !CardHasEvergreenAbility(creature.source, "Vigilance") {
			creature.tap()
		} else {
			LogPlayer("%s attacks with Vigilance and does not tap.", creature.source.Name)
		}
	}
	if !attacking {
		LogPlayer("No Attackers declared.")
	}
}

func (p *Player) PlayLand(t *turn) {
	for i := 0; i < len(p.Hand); i++ {
		c := p.Hand[i]
		if t.landPerTurn <= 0 {
			return
		}
		if strings.Contains(c.TypeLine, "Land") {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			LogCard("Playing land: %s", c.Name)

			// adds land to board
			land := Permanant{
				source:            c,
				owner:             p,
				tokenType:         Land,
				tapped:            false,
				summoningSickness: false,
			}
			land.checkManaProducer()
			p.Lands = append(p.Lands, land)

			// pops card from hand
			t.landPerTurn--
			i-- // Adjust index after removing an element
		}
	}
}

func (p *Player) ManaAvailable() *ManaPool {
	manaPool := NewManaPool()

	for _, land := range p.Lands {
		if !land.tapped && land.manaProducer {
			for _, manaType := range land.manaTypes {
				manaPool.Add(manaType, 1)
			}
		}
	}

	for _, creature := range p.Creatures {
		if !creature.tapped && creature.manaProducer && !creature.summoningSickness {
			for _, manaType := range creature.manaTypes {
				manaPool.Add(manaType, 1)
			}
		}
	}

	for _, artifact := range p.Artifacts {
		if !artifact.tapped && artifact.manaProducer {
			for _, manaType := range artifact.manaTypes {
				manaPool.Add(manaType, 1)
			}
		}
	}

	return manaPool
}

func (p *Player) PlaySpell() {
	for i := 0; i < len(p.Hand); i++ {
		c := p.Hand[i]
		// check if spell can be cast
		if strings.Contains(c.TypeLine, "Land") {
			continue
		}

		// check if mana available
		cost := ParseManaCost(c.ManaCost)
		err := p.tapForMana(cost)
		if err != nil {
			continue
		}

		LogCard("Casting spell: %s", c.Name)
		// pops card from hand
		p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
		c.Cast(nil, p)
		i-- // Adjust index after removing an element
	}
}

func (p *Player) CastSpell(card *Card, target *Permanant) {
	// Handle Ward
	if target != nil && CardHasEvergreenAbility(target.source, "Ward") {
		LogPlayer("%s has Ward. The spell is countered unless the opponent pays the Ward cost.", target.source.Name)
		// Implement Ward cost logic here
		// For now, assume the opponent cannot pay the cost
		LogPlayer("The opponent cannot pay the Ward cost. The spell is countered.")
		p.Graveyard = append(p.Graveyard, *card)
		return
	}

	// Handle Equip
	if strings.Contains(card.TypeLine, "Equipment") && CardHasEvergreenAbility(*card, "Equip") {
		LogPlayer("Equipping %s to a target creature.", card.Name)
		// Implement Equip logic here
		return
	}

	// Handle Enchant
	if strings.Contains(card.TypeLine, "Aura") && CardHasEvergreenAbility(*card, "Enchant") {
		LogPlayer("Enchanting %s with %s.", target.source.Name, card.Name)
		// Implement Enchant logic here
		return
	}

	// Handle Scry
	if CardHasEvergreenAbility(*card, "Scry") {
		LogPlayer("Scrying with %s.", card.Name)
		// Implement Scry logic: Look at the top X cards of the library
		topCards := p.Deck.DrawCards(2) // Example: Scry 2
		LogPlayer("Top cards: %v", topCards)
		// Add logic to reorder or put cards on the bottom of the library
		return
	}

	// Handle Mill
	if CardHasEvergreenAbility(*card, "Mill") {
		LogPlayer("Milling with %s.", card.Name)
		// Implement Mill logic: Put the top X cards of the opponent's library into their graveyard
		milledCards := p.Opponents[0].Deck.DrawCards(3) // Example: Mill 3
		p.Opponents[0].Graveyard = append(p.Opponents[0].Graveyard, milledCards...)
		LogPlayer("Milled cards: %v", milledCards)
		return
	}

	// Handle Goad
	if CardHasEvergreenAbility(*card, "Goad") {
		LogPlayer("Goading with %s.", card.Name)
		if target != nil && target.tokenType == Creature {
			LogPlayer("%s is goaded and must attack next turn.", target.source.Name)
			// Implement Goad logic: Force the target creature to attack next turn
			target.attacking = p.Opponents[0] // Example: Force attack
		} else {
			LogPlayer("No valid target for Goad. The spell fails.")
		}
		return
	}

	// Default spell casting logic
	card.Cast(target, p)
}

func (p *Player) tapForMana(cost mana) error {
	manaPool := p.ManaAvailable()

	if !manaPool.CanPay(cost) {
		return fmt.Errorf("not enough mana available to pay the cost")
	}

	// Create a copy of the cost to avoid modifying the original during iteration
	remainingCost := newMana()
	for manaType, amount := range cost.pool {
		remainingCost.pool[manaType] = amount
	}

	// Helper function to tap permanents for specific mana
	tapForSpecificMana := func(permanents []Permanant, manaType ManaType) {
		for i := range permanents {
			if !permanents[i].tapped && permanents[i].manaProducer {
				for _, producedMana := range permanents[i].manaTypes {
					if producedMana == manaType && remainingCost.pool[manaType] > 0 {
						permanents[i].tap()
						remainingCost.pool[manaType]--
						break
					}
				}
			}
		}
	}

	// Tap lands for specific mana
	for manaType := range remainingCost.pool {
		tapForSpecificMana(p.Lands, manaType)
	}

	// Tap creatures for specific mana
	for manaType := range remainingCost.pool {
		tapForSpecificMana(p.Creatures, manaType)
	}

	// Tap artifacts for specific mana
	for manaType := range remainingCost.pool {
		tapForSpecificMana(p.Artifacts, manaType)
	}

	// Ensure all costs are paid
	if remainingCost.total() > 0 {
		return fmt.Errorf("not enough mana available to pay the cost")
	}

	return nil
}

func (p *Player) Display() {
	LogPlayer("Player: %s", p.Name)
	LogPlayer("Life: %d", p.LifeTotal)
	LogPlayer("Hand:")
	DisplayCards(p.Hand)
	LogPlayer("Board:")
	DisplayPermanants(p.Creatures)
	DisplayPermanants(p.Lands)
}

func (p *Player) EndStep() {
	for i := range p.Creatures {
		p.Creatures[i].damage_counters = 0
	}
}
