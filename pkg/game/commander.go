package game

// Commander rules support (CR 903).
//
// This file groups the player- and game-level helpers that implement the
// EDH/Commander format: the command zone, commander tax (CR 903.8), the
// zone-change replacement effect (CR 903.9 / 903.10), and the 21-damage
// state-based action (CR 704.5u). Combat-damage tracking lives in
// combat.go and the SBA itself in sba.go; this file owns the bookkeeping.

// commanderKey returns the unique key used to track a specific commander
// card across zone changes. EDH is singleton so owner + card name suffices.
func commanderKey(owner *Player, name string) string {
	if owner == nil {
		return "|" + name
	}
	return owner.name + "|" + name
}

// IsCommanderName reports whether the given card name is one of this
// player's commanders.
func (p *Player) IsCommanderName(name string) bool {
	if p == nil || p.commanderNames == nil {
		return false
	}
	return p.commanderNames[name]
}

// GetCommanderNames returns a copy of this player's commander card names.
func (p *Player) GetCommanderNames() []string {
	out := make([]string, 0, len(p.commanderNames))
	for n := range p.commanderNames {
		out = append(out, n)
	}
	return out
}

// RegisterCommander designates a card as this player's commander and
// places it in their command zone if it is not already represented there.
// Idempotent: calling twice with the same name is a no-op.
func (p *Player) RegisterCommander(c SimpleCard) {
	if p.commanderNames == nil {
		p.commanderNames = map[string]bool{}
	}
	if p.commanderCastCount == nil {
		p.commanderCastCount = map[string]int{}
	}
	if p.commanderNames[c.Name] {
		return
	}
	p.commanderNames[c.Name] = true
	p.CommandZone = append(p.CommandZone, c)
}

// CommanderTax returns the additional generic mana cost owed for casting
// the named commander from the command zone (CR 903.8): {2} for each
// previous time it has been cast from the command zone this game.
func (p *Player) CommanderTax(name string) int {
	if p == nil || p.commanderCastCount == nil {
		return 0
	}
	return 2 * p.commanderCastCount[name]
}

// IncrementCommanderCast records a cast of the named commander from the
// command zone. The next CommanderTax query will reflect the increment.
func (p *Player) IncrementCommanderCast(name string) {
	if p.commanderCastCount == nil {
		p.commanderCastCount = map[string]int{}
	}
	p.commanderCastCount[name]++
}

// MoveCommanderFromZoneToBattlefield removes the named commander from the
// command zone and creates a battlefield permanent flagged as commander.
// Returns nil if the commander is not currently in the command zone.
func (p *Player) MoveCommanderFromZoneToBattlefield() *Permanent {
	return nil // unused placeholder; callers should use CastCommander.
}

// CastCommander resolves a commander cast from the command zone: removes
// the card from CZ, increments the cast counter, and creates a battlefield
// permanent with the commander flag set. Returns nil if the named
// commander is not currently in the command zone.
func (p *Player) CastCommander(name string) *Permanent {
	for i, c := range p.CommandZone {
		if c.Name == name {
			p.CommandZone = append(p.CommandZone[:i], p.CommandZone[i+1:]...)
			p.IncrementCommanderCast(name)
			perm := NewPermanent(c, p, p)
			perm.SetIsCommander(true)
			p.Battlefield = append(p.Battlefield, perm)
			return perm
		}
	}
	return nil
}

// SendCommanderToCommandZone removes a commander permanent from the
// battlefield and places its source card back into its owner's command
// zone (CR 903.9, automated default choice).
func (p *Player) SendCommanderToCommandZone(perm *Permanent) bool {
	if perm == nil || !perm.IsCommander() {
		return false
	}
	for i, bp := range p.Battlefield {
		if bp == perm {
			p.Battlefield = append(p.Battlefield[:i], p.Battlefield[i+1:]...)
			owner := perm.GetOwner()
			if owner == nil {
				owner = p
			}
			owner.CommandZone = append(owner.CommandZone, perm.source)
			return true
		}
	}
	return false
}

// AddCommanderDamage records combat damage dealt by the named commander
// (owned by `cmdOwner`) to this player. CR 704.5u: a player who has been
// dealt 21+ combat damage by the same commander loses the game.
func (p *Player) AddCommanderDamage(cmdOwner *Player, cmdName string, dmg int) {
	if p == nil || dmg <= 0 {
		return
	}
	if p.commanderDamageReceived == nil {
		p.commanderDamageReceived = map[string]int{}
	}
	p.commanderDamageReceived[commanderKey(cmdOwner, cmdName)] += dmg
}

// CommanderDamageFrom returns total combat damage this player has received
// from the named commander.
func (p *Player) CommanderDamageFrom(cmdOwner *Player, cmdName string) int {
	if p == nil || p.commanderDamageReceived == nil {
		return 0
	}
	return p.commanderDamageReceived[commanderKey(cmdOwner, cmdName)]
}

// MaxCommanderDamageReceived returns the highest single-commander damage
// total this player has taken; useful for SBA checks and metrics.
func (p *Player) MaxCommanderDamageReceived() int {
	max := 0
	for _, v := range p.commanderDamageReceived {
		if v > max {
			max = v
		}
	}
	return max
}
