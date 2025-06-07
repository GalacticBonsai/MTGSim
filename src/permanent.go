package main

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// Permanent represents a permanent on the battlefield.
type Permanent struct {
	source            Card
	owner             *Player
	id                uuid.UUID
	tokenType         PermanentType
	tapped            bool
	summoningSickness bool
	manaProducer      bool
	manaTypes         []ManaType
	attacking         *Player
	blocking          *Permanent
	blockedBy         []*Permanent
	power             int
	toughness         int
	damage_counters   int
	goaded            bool
}

// checkManaProducer sets manaProducer and manaTypes based on the card's Oracle text.
func (p *Permanent) checkManaProducer() {
	p.manaProducer, p.manaTypes = CheckManaProducer(p.source.OracleText)
}

// Display prints information about the permanent.
func (p Permanent) Display() {
	LogCard("Name: %s, Type: %s", p.source.Name, p.tokenType)
	if p.manaProducer {
		LogCard("This permanent is a mana producer and produces:")
		for _, manaType := range p.manaTypes {
			LogCard("%s mana", manaType)
		}
	}
}

// tap marks the permanent as tapped.
func (p *Permanent) tap() {
	if !p.tapped {
		p.tapped = true
	}
}

// untap marks the permanent as untapped.
func (p *Permanent) untap() {
	p.tapped = false
}

// DisplayPermanents prints all permanents in a slice.
func DisplayPermanents(permanents []*Permanent) {
	for _, Permanent := range permanents {
		DisplayCard(Permanent.source)
	}
}

// damages deals damage from this permanent to the target.
func (p *Permanent) damages(target *Permanent) int {
	LogCard("%s deals %d damage to %s", p.source.Name, p.power, target.source.Name)
	// Handle Lifelink
	if CardHasEvergreenAbility(p.source, "Lifelink") {
		p.owner.LifeTotal += p.power
		LogPlayer("%s deals damage with Lifelink, gaining %d life.", p.source.Name, p.power)
	}
	target.damage_counters += p.power
	if target.damage_counters > target.toughness {
		return target.damage_counters - target.toughness // Overkill
	}
	return 0
}

// checkLife destroys the permanent if it has lethal damage.
func (p *Permanent) checkLife() {
	if p.toughness <= p.damage_counters {
		destroyPermanent(p)
	}
}

// Fight makes two permanents deal damage to each other.
func (p *Permanent) Fight(other *Permanent) {
	p.damages(other)
	other.damages(p)
}

// destroyPermanent removes the permanent from the battlefield and puts it in the graveyard.
func destroyPermanent(p *Permanent) {
	if CardHasEvergreenAbility(p.source, "Indestructible") {
		LogCard("%s is indestructible and cannot be destroyed.", p.source.Name)
		return
	}

	// remove permanent from owner's board
	removePermanent := func(list []*Permanent, target *Permanent) []*Permanent {
		for i, c := range list {
			if c == target {
				return append(list[:i], list[i+1:]...)
			}
		}
		return list
	}

	switch p.tokenType {
	case Creature:
		p.owner.Creatures = removePermanent(p.owner.Creatures, p)
	case Land:
		p.owner.Lands = removePermanent(p.owner.Lands, p)
	case Artifact:
		p.owner.Artifacts = removePermanent(p.owner.Artifacts, p)
	case Enchantment:
		p.owner.Enchantments = removePermanent(p.owner.Enchantments, p)
	case Planeswalker:
		p.owner.Planeswalkers = removePermanent(p.owner.Planeswalkers, p)
	}
	
	// add permanent to owner's graveyard
	LogCard("%s sent to player %s's graveyard", p.source.Name, p.owner.Name)
	p.owner.Graveyard = append(p.owner.Graveyard, p.source)
}

// ParseTapEffect parses the Oracle text for a tap effect (e.g., "{T}: Add {G}") and returns whether a tap effect exists and the effect description.
func ParseTapEffect(oracle string) (bool, string) {
	// Regex matches lines like "{T}: ..." or "{T}, ...: ..."
	re := regexp.MustCompile(`(?m)\{T\}[^:]*: (.+)$`)
	lines := strings.Split(oracle, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := re.FindStringSubmatch(line); matches != nil {
			return true, matches[1]
		}
	}
	return false, ""
}

// HasTapAbility returns true if the permanent has a tap ability.
func (p *Permanent) HasTapAbility() bool {
	hasTap, _ := ParseTapEffect(p.source.OracleText)
	return hasTap
}

// GetTapEffect returns the effect string for the tap ability, if any.
func (p *Permanent) GetTapEffect() string {
	_, effect := ParseTapEffect(p.source.OracleText)
	return effect
}
