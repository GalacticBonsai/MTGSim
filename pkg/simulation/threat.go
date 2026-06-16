package simulation

import (
	"strings"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// isCardAdvantageEngine returns true for permanents that generate
// incremental card advantage over time.
func isCardAdvantageEngine(name string) bool {
	lower := strings.ToLower(name)
	engines := []string{
		"rhystic study", "mystic remora", "the one ring",
		"consecrated sphinx", "dark confidant", "phyrexian arena",
		"necropotence", "esper sentinel", "trouble in pairs",
		"tatyova", "chulane", "beast whisperer", "guardian project",
		"the great henge", "sylvan library", "mangara, the diplomat",
		"smothering tithe", "monologue tax",
	}
	for _, e := range engines {
		if strings.Contains(lower, e) {
			return true
		}
	}
	return false
}

// chooseAttackTarget picks the most threatening living opponent for the
// active player to attack. The score combines signals:
//
//   - Opponent's combined creature power (raw board pressure)
//   - Opponent's life deficit from 40 (finish-the-leader incentive)
//   - Commander damage they have already inflicted on the attacker
//     (preempt the 21-damage SBA, CR 704.5u)
//   - Card advantage engines on the opponent's board (value threats)
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
	cardEngines := 0
	for _, perm := range opp.GetCreatures() {
		board += perm.GetPower()
		// Count card advantage engines on creatures (e.g., Consecrated Sphinx)
		if isCardAdvantageEngine(perm.GetName()) {
			cardEngines++
		}
	}
	// Count non-creature card advantage engines on battlefield
	for _, perm := range opp.Battlefield {
		if !perm.IsCreature() && isCardAdvantageEngine(perm.GetName()) {
			cardEngines++
		}
	}
	lifeDeficit := 40 - opp.GetLifeTotal()
	if lifeDeficit < 0 {
		lifeDeficit = 0
	}
	cmdrDmg := 0
	for _, name := range opp.GetCommanderNames() {
		cmdrDmg += attacker.CommanderDamageFrom(opp, name)
	}
	return board*2 + lifeDeficit/4 + cmdrDmg*3 + cardEngines*5
}
