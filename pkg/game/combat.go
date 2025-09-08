package game

import "fmt"

// combat holds state for the current combat declaration and resolution.
type combat struct {
    attackers map[*Permanent]*Player          // attacker -> defending player
    blocks    map[*Permanent]*Permanent       // attacker -> blocker (one-to-one)
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
    // Minimal rule set: tap the attacker on declaration
    attacker.Tap()
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
    g.combat.blocks[attacker] = blocker
    return nil
}

// ResolveCombatDamage performs a minimal combat damage assignment and applies SBAs.
// Rules implemented (simplified):
// - Unblocked attacker deals damage equal to its power to the defending player
// - Blocked attackers and their single blocker assign damage simultaneously
// - First strike: creatures with first strike deal damage in a first step; SBAs are applied;
//   surviving creatures without first strike then deal damage in the normal step.
// - Double strike: attacker deals in both steps (treated as having first strike and normal damage)
func (g *Game) ResolveCombatDamage() {
    if g.combat == nil {
        return
    }

    // Helper closures
    dealToPermanent := func(src *Permanent, tgt *Permanent) {
        if src == nil || tgt == nil { return }
        tgt.AddDamage(src.GetPower())
    }
    dealToPlayer := func(src *Permanent, pl *Player) {
        if src == nil || pl == nil { return }
        pl.SetLifeTotal(pl.GetLifeTotal() - src.GetPower())
    }

    // Collect sets for steps
    type pair struct{ a *Permanent; b *Permanent }
    var fsPairs, normPairs []pair
    var fsUnblocked, normUnblocked []struct{ a *Permanent; d *Player }

    for a, def := range g.combat.attackers {
        if b, blocked := g.combat.blocks[a]; blocked {
            if a.HasFirstStrike() || a.HasDoubleStrike() || b.HasFirstStrike() || b.HasDoubleStrike() {
                fsPairs = append(fsPairs, pair{a: a, b: b})
            } else {
                normPairs = append(normPairs, pair{a: a, b: b})
            }
        } else {
            // unblocked
            if a.HasFirstStrike() || a.HasDoubleStrike() {
                fsUnblocked = append(fsUnblocked, struct{ a *Permanent; d *Player }{a, def})
            } else {
                normUnblocked = append(normUnblocked, struct{ a *Permanent; d *Player }{a, def})
            }
        }
    }

    // First strike step
    for _, p := range fsPairs {
        a, b := p.a, p.b
        // Attacker with FS/DS deals first-strike damage to blocker
        if a.HasFirstStrike() || a.HasDoubleStrike() {
            dealToPermanent(a, b)
        }
        // Blocker with FS/DS deals first-strike damage to attacker
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
        a, b := p.a, p.b
        // If the attacker has double strike and both are still around, it deals again
        if a.HasDoubleStrike() && g.onBattlefield(a) && g.onBattlefield(b) {
            dealToPermanent(a, b)
        }
        // If the blocker has double strike, it deals again
        if b.HasDoubleStrike() && g.onBattlefield(a) && g.onBattlefield(b) {
            dealToPermanent(b, a)
        }
    }
    for _, p := range normPairs {
        a, b := p.a, p.b
        if g.onBattlefield(a) && g.onBattlefield(b) {
            dealToPermanent(a, b)
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

