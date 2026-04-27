package game

import "fmt"

// combat holds state for the current combat declaration and resolution.
type combat struct {
	attackers map[*Permanent]*Player    // attacker -> defending player
	blocks    map[*Permanent]*Permanent // attacker -> blocker (one-to-one)
}

// BeginCombat starts a new combat instance for the current turn.
func (g *Game) BeginCombat() {
	g.combat = &combat{
		attackers: map[*Permanent]*Player{},
		blocks:    map[*Permanent]*Permanent{},
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

// DeclareBlocker declares a one-to-one block: blocker blocks attacker.
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
	if _, already := g.combat.blocks[attacker]; already {
		return fmt.Errorf("attacker already blocked")
	}
	if blocker.IsTapped() {
		return fmt.Errorf("tapped creatures can't block")
	}
	// CR 702.9: a creature with flying can be blocked only by creatures
	// with flying or reach.
	if attacker.HasKeyword(KWFlying) && !(blocker.HasKeyword(KWFlying) || blocker.HasKeyword(KWReach)) {
		return fmt.Errorf("flying creature can only be blocked by flying or reach")
	}
	g.combat.blocks[attacker] = blocker
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
	// dealAttackerToBlocker handles trample: the attacker assigns just
	// enough damage to be lethal to the blocker, and the remainder goes
	// to the defending player (CR 702.19). With deathtouch, "lethal" is
	// any 1 damage (CR 702.2c).
	dealAttackerToBlocker := func(a, b *Permanent, defender *Player) {
		if a == nil || b == nil {
			return
		}
		dmg := a.GetPower()
		if dmg <= 0 {
			return
		}
		needed := b.GetToughness() - b.GetDamageCounters()
		if needed < 0 {
			needed = 0
		}
		if a.HasKeyword(KWDeathtouch) && needed > 1 {
			needed = 1
		}
		assignedToBlocker := dmg
		if a.HasKeyword(KWTrample) && dmg > needed {
			assignedToBlocker = needed
		}
		if assignedToBlocker > 0 {
			b.AddDamage(assignedToBlocker)
			if a.HasKeyword(KWDeathtouch) {
				b.markedLethal = true
			}
		}
		excess := dmg - assignedToBlocker
		if a.HasKeyword(KWTrample) && excess > 0 && defender != nil {
			defender.SetLifeTotal(defender.GetLifeTotal() - excess)
			if a.IsCommander() {
				defender.AddCommanderDamage(a.GetOwner(), a.GetName(), excess)
			}
		}
		applyLifelink(a, dmg)
	}

	// Collect sets for steps
	type pair struct {
		a *Permanent
		b *Permanent
		d *Player
	}
	var fsPairs, normPairs []pair
	var fsUnblocked, normUnblocked []struct {
		a *Permanent
		d *Player
	}

	for a, def := range g.combat.attackers {
		if b, blocked := g.combat.blocks[a]; blocked {
			if a.HasFirstStrike() || a.HasDoubleStrike() || b.HasFirstStrike() || b.HasDoubleStrike() {
				fsPairs = append(fsPairs, pair{a: a, b: b, d: def})
			} else {
				normPairs = append(normPairs, pair{a: a, b: b, d: def})
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
	for _, p := range fsPairs {
		a, b, def := p.a, p.b, p.d
		if a.HasFirstStrike() || a.HasDoubleStrike() {
			dealAttackerToBlocker(a, b, def)
		}
		if b.HasFirstStrike() || b.HasDoubleStrike() {
			dealToPermanent(b, a)
		}
	}
	for _, u := range fsUnblocked {
		dealToPlayer(u.a, u.d)
	}
	g.ApplyStateBasedActions()

	// Normal step
	for _, p := range fsPairs {
		a, b, def := p.a, p.b, p.d
		// Attacker with double strike (or no first strike at all) deals normal damage if both alive
		attackerStrikesNormal := a.HasDoubleStrike() || !a.HasFirstStrike()
		if attackerStrikesNormal && g.onBattlefield(a) && g.onBattlefield(b) {
			dealAttackerToBlocker(a, b, def)
		}
		// Blocker with double strike (or no first strike at all) deals normal damage if both alive
		blockerStrikesNormal := b.HasDoubleStrike() || !b.HasFirstStrike()
		if blockerStrikesNormal && g.onBattlefield(a) && g.onBattlefield(b) {
			dealToPermanent(b, a)
		}
	}
	for _, p := range normPairs {
		a, b, def := p.a, p.b, p.d
		if g.onBattlefield(a) && g.onBattlefield(b) {
			dealAttackerToBlocker(a, b, def)
			dealToPermanent(b, a)
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
