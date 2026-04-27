package simulation

import "github.com/mtgsim/mtgsim/pkg/game"

// chooseAttackTarget picks the most threatening living opponent for the
// active player to attack. The score combines three signals:
//
//   - Opponent's combined creature power (raw board pressure)
//   - Opponent's life deficit from 40 (finish-the-leader incentive)
//   - Commander damage they have already inflicted on the attacker
//     (preempt the 21-damage SBA, CR 704.5u)
//
// Ties fall back to the next-living-opponent in seat order so behaviour
// remains deterministic for a given seed.
func chooseAttackTarget(g *game.Game, attacker *game.Player) *game.Player {
	type scored struct {
		p     *game.Player
		score int
		seat  int
	}
	var best *scored
	startSeat := indexOfPlayer(g, attacker)
	players := g.GetPlayersRaw()
	n := len(players)
	for i := 1; i < n; i++ {
		opp := players[(startSeat+i)%n]
		if opp == attacker || opp.HasLost() {
			continue
		}
		s := threatScore(attacker, opp)
		entry := &scored{p: opp, score: s, seat: i}
		if best == nil || entry.score > best.score || (entry.score == best.score && entry.seat < best.seat) {
			best = entry
		}
	}
	if best == nil {
		return nil
	}
	return best.p
}

// threatScore assigns a numeric danger level to opp from attacker's
// point of view. Heuristic only — kept deterministic and side-effect
// free so it can be unit tested in isolation.
func threatScore(attacker, opp *game.Player) int {
	board := 0
	for _, perm := range opp.GetCreatures() {
		board += perm.GetPower()
	}
	lifeDeficit := 40 - opp.GetLifeTotal()
	if lifeDeficit < 0 {
		lifeDeficit = 0
	}
	cmdrDmg := 0
	for _, name := range opp.GetCommanderNames() {
		cmdrDmg += attacker.CommanderDamageFrom(opp, name)
	}
	return board*2 + lifeDeficit/4 + cmdrDmg*3
}
