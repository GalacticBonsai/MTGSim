package game

import "fmt"

// combat holds state for the current combat declaration and resolution.
type combat struct {
	attackers map[*Permanent]*Player       // attacker -> defending player
	blocks    map[*Permanent][]*Permanent // attacker -> blockers (multiple allowed)
}

// BeginCombat starts a new combat instance for the current turn.
func (g *Game) BeginCombat() {
	g.combat = &combat{
		attackers: map[*Permanent]*Player{},
		blocks:    map[*Permanent][]*Permanent{},
	}
}

// DeclareAttacker declares a single attacker against the specified defending player.
func (g *Game) DeclareAttacker(attacker *Permanent, defendingPlayer *Player) error {
	if g.combat == nil {
		g.BeginCombat()
	}
	if attacker == nil || defendingPlayer == nil {
		return fmt.Errorf("invalid attacker or defender")
	}
	if !attacker.IsCreature() {
		return fmt.Errorf("attacker must be a creature")
	}
	if attacker.IsTapped() {
		return fmt.Errorf("attacker is tapped")
	}
	if attacker.GetController() != g.GetActivePlayerRaw() {
		return fmt.Errorf("only active player may declare attackers")
	}
	// CR 302.6 / 702.10: Creatures with summoning sickness can't attack unless they have haste.
	if attacker.GetEnteredTurn() == g.GetTurnNumber() && !attacker.HasKeyword(KWHaste) {
		return fmt.Errorf("summoning sickness: creature can't attack this turn")
	}
	// CR 702.20: defenders can't attack.
	if attacker.HasKeyword(KWDefender) {
		return fmt.Errorf("defenders can't attack")
	}
	// CR 508.1f: Attacking causes the creature to become tapped, except
	// creatures with vigilance (CR 702.20).
	if !attacker.HasKeyword(KWVigilance) {
		attacker.Tap()
	}
	g.combat.attackers[attacker] = defendingPlayer
	return nil
}

// DeclareBlocker declares a block: blocker blocks attacker (multiple blockers allowed).
func (g *Game) DeclareBlocker(blocker *Permanent, attacker *Permanent) error {
	if g.combat == nil {
		return fmt.Errorf("combat not started")
	}
	if blocker == nil || attacker == nil {
		return fmt.Errorf("invalid blocker or attacker")
	}
	if !blocker.IsCreature() || !attacker.IsCreature() {
		return fmt.Errorf("blocker and attacker must be creatures")
	}
	if attacker.GetController() == blocker.GetController() {
		return fmt.Errorf("cannot block your own creature")
	}
	if _, ok := g.combat.attackers[attacker]; !ok {
		return fmt.Errorf("target is not an attacker")
	}
	// Check if already blocking this attacker
	for _, b := range g.combat.blocks[attacker] {
		if b == blocker {
			return fmt.Errorf("creature is already blocking this attacker")
		}
	}
	if blocker.IsTapped() {
		return fmt.Errorf("tapped creatures can't block")
	}
	// CR 702.9: a creature with flying can be blocked only by creatures
	// with flying or reach.
	if attacker.HasKeyword(KWFlying) && !(blocker.HasKeyword(KWFlying) || blocker.HasKeyword(KWReach)) {
		return fmt.Errorf("flying creature can only be blocked by flying or reach")
	}
	g.combat.blocks[attacker] = append(g.combat.blocks[attacker], blocker)
	return nil
}

// ResolveCombatDamage performs a minimal combat damage assignment and applies SBAs.
// Rules implemented (simplified):
//   - Unblocked attacker deals damage equal to its power to the defending player
//   - Blocked attackers and their single blocker assign damage simultaneously
//   - First strike: creatures with first strike deal damage in a first step; SBAs are applied;
//     surviving creatures without first strike then deal damage in the normal step.
//   - Double strike: attacker deals in both steps (treated as having first strike and normal damage)
func (g *Game) ResolveCombatDamage() {
	if g.combat == nil {
		return
	}

	// Helper closures. Each application returns the damage actually dealt
	// so callers (e.g. trample) can compute leftover.
	applyLifelink := func(src *Permanent, dmg int) {
		// CR 702.15: lifelink causes its controller to gain life equal
		// to the damage dealt by that source.
		if dmg <= 0 || src == nil || !src.HasKeyword(KWLifelink) {
			return
		}
		if c := src.GetController(); c != nil {
			c.SetLifeTotal(c.GetLifeTotal() + dmg)
		}
	}
	dealToPermanent := func(src *Permanent, tgt *Permanent) int {
		if src == nil || tgt == nil {
			return 0
		}
		dmg := src.GetPower()
		if dmg <= 0 {
			return 0
		}
		tgt.AddDamage(dmg)
		// CR 702.2: any nonzero damage from a deathtouch source is lethal.
		if src.HasKeyword(KWDeathtouch) {
			tgt.markedLethal = true
		}
		applyLifelink(src, dmg)
		return dmg
	}
	dealToPlayer := func(src *Permanent, pl *Player) int {
		if src == nil || pl == nil {
			return 0
		}
		dmg := src.GetPower()
		if dmg <= 0 {
			return 0
		}
		pl.SetLifeTotal(pl.GetLifeTotal() - dmg)
		// CR 704.5u: track commander damage for the 21-damage SBA.
		if src.IsCommander() {
			pl.AddCommanderDamage(src.GetOwner(), src.GetName(), dmg)
		}
		applyLifelink(src, dmg)
		return dmg
	}
	// assignDamageToBlockers assigns attacker's damage to blockers and possibly tramples excess to player.
	// Simple strategy: assign lethal damage to blockers in order, then trample excess.
	assignDamageToBlockers := func(a *Permanent, blockers []*Permanent, defender *Player) {
		dmg := a.GetPower()
		if dmg <= 0 {
			return
		}
		remainingDmg := dmg
		for _, b := range blockers {
			if remainingDmg <= 0 {
				break
			}
			needed := b.GetToughness() - b.GetDamageCounters()
			if needed < 0 {
				needed = 0
			}
			if a.HasKeyword(KWDeathtouch) && needed > 1 {
				needed = 1
			}
			assigned := remainingDmg
			if assigned > needed {
				assigned = needed
			}
			if assigned > 0 {
				b.AddDamage(assigned)
				if a.HasKeyword(KWDeathtouch) {
					b.markedLethal = true
				}
				remainingDmg -= assigned
			}
		}
		// Trample excess
		if a.HasKeyword(KWTrample) && remainingDmg > 0 && defender != nil {
			defender.SetLifeTotal(defender.GetLifeTotal() - remainingDmg)
			if a.IsCommander() {
				defender.AddCommanderDamage(a.GetOwner(), a.GetName(), remainingDmg)
			}
		}
		applyLifelink(a, dmg)
	}

	// Collect sets for steps
	type blockedAttacker struct {
		a *Permanent
		blockers []*Permanent
		d *Player
	}
	var fsBlocked, normBlocked []blockedAttacker
	var fsUnblocked, normUnblocked []struct {
		a *Permanent
		d *Player
	}

	for a, def := range g.combat.attackers {
		if blockers, blocked := g.combat.blocks[a]; blocked && len(blockers) > 0 {
			ba := blockedAttacker{a: a, blockers: blockers, d: def}
			hasFirstStrike := a.HasFirstStrike() || a.HasDoubleStrike()
			for _, b := range blockers {
				if b.HasFirstStrike() || b.HasDoubleStrike() {
					hasFirstStrike = true
					break
				}
			}
			if hasFirstStrike {
				fsBlocked = append(fsBlocked, ba)
			} else {
				normBlocked = append(normBlocked, ba)
			}
		} else {
			// unblocked
			if a.HasFirstStrike() || a.HasDoubleStrike() {
				fsUnblocked = append(fsUnblocked, struct {
					a *Permanent
					d *Player
				}{a, def})
			} else {
				normUnblocked = append(normUnblocked, struct {
					a *Permanent
					d *Player
				}{a, def})
			}
		}
	}

	// First strike step
	for _, ba := range fsBlocked {
		a, blockers, def := ba.a, ba.blockers, ba.d
		if a.HasFirstStrike() || a.HasDoubleStrike() {
			assignDamageToBlockers(a, blockers, def)
		}
		// Blockers deal damage
		for _, b := range blockers {
			if b.HasFirstStrike() || b.HasDoubleStrike() {
				dealToPermanent(b, a)
			}
		}
	}
	for _, u := range fsUnblocked {
		dealToPlayer(u.a, u.d)
	}
	g.ApplyStateBasedActions()

	// Normal step
	for _, ba := range fsBlocked {
		a, blockers, def := ba.a, ba.blockers, ba.d
		// Attacker with double strike (or no first strike at all) deals normal damage if alive
		attackerStrikesNormal := a.HasDoubleStrike() || !a.HasFirstStrike()
		if attackerStrikesNormal && g.onBattlefield(a) {
			// Check if any blockers are still alive
			aliveBlockers := []*Permanent{}
			for _, b := range blockers {
				if g.onBattlefield(b) {
					aliveBlockers = append(aliveBlockers, b)
				}
			}
			if len(aliveBlockers) > 0 {
				assignDamageToBlockers(a, aliveBlockers, def)
			}
		}
		// Blockers with double strike (or no first strike at all) deal normal damage if both alive
		for _, b := range blockers {
			blockerStrikesNormal := b.HasDoubleStrike() || !b.HasFirstStrike()
			if blockerStrikesNormal && g.onBattlefield(a) && g.onBattlefield(b) {
				dealToPermanent(b, a)
			}
		}
	}
	for _, ba := range normBlocked {
		a, blockers, def := ba.a, ba.blockers, ba.d
		if g.onBattlefield(a) {
			// Check alive blockers
			aliveBlockers := []*Permanent{}
			for _, b := range blockers {
				if g.onBattlefield(b) {
					aliveBlockers = append(aliveBlockers, b)
				}
			}
			if len(aliveBlockers) > 0 {
				assignDamageToBlockers(a, aliveBlockers, def)
				for _, b := range aliveBlockers {
					dealToPermanent(b, a)
				}
			}
		}
	}
	for _, u := range normUnblocked {
		if g.onBattlefield(u.a) {
			dealToPlayer(u.a, u.d)
		}
	}
	g.ApplyStateBasedActions()

	// Combat finished
	g.combat = nil
}
