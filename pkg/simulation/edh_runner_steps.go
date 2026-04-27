package simulation

import (
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// stepOneEDHTurn drives the active player through a complete turn using
// the simplified runner AI. Returns false if no players are alive after
// the turn finishes.
func stepOneEDHTurn(g *game.Game, seats []EDHSeat, casts []int) bool {
	startTurn := g.GetTurnNumber()
	milledThisTurn := false
	for {
		ap := g.GetActivePlayerRaw()
		switch g.GetCurrentPhase() {
		case game.PhaseUntap:
			for _, perm := range ap.Battlefield {
				perm.Untap()
			}
		case game.PhaseDraw:
			if g.GetTurnNumber() > 1 || ap != g.GetPlayerByIndex(0) {
				if ap.Draw(1) == 0 {
					ap.SetLifeTotal(0) // CR 704.5b: empty-library draw loss proxy
					milledThisTurn = true
				}
			}
		case game.PhaseMain1:
			runMainPhase(g, ap, casts)
		case game.PhaseCombat:
			runCombatPhase(g, ap)
		case game.PhaseEnd:
			// Game.AdvancePhase rotates active player and handles EOT cleanup.
		}
		g.ApplyStateBasedActions()
		if milledThisTurn {
			markMillIfApplicable(ap)
		}
		g.AdvancePhase()
		if survivors(g) <= 1 {
			return survivors(g) >= 1
		}
		if g.GetTurnNumber() != startTurn {
			break
		}
	}
	return survivors(g) >= 1
}

// runMainPhase plays one land, casts the commander when possible, and
// summons every creature in hand (no mana enforcement — a deliberate
// simplification mirrored from cmd/mtgsim's main loop).
func runMainPhase(g *game.Game, ap *game.Player, casts []int) {
	for i, c := range ap.Hand {
		if c.IsLand() {
			ap.Hand = append(ap.Hand[:i], ap.Hand[i+1:]...)
			_, _ = g.PlayLand(ap, c.Name)
			break
		}
	}

	idx := indexOfPlayer(g, ap)
	if idx >= 0 && len(ap.CommandZone) > 0 {
		name := ap.CommandZone[0].Name
		if perm := ap.CastCommander(name); perm != nil {
			perm.SetEnteredTurn(g.GetTurnNumber())
			casts[idx]++
		}
	}

	again := true
	for again {
		again = false
		for i, c := range ap.Hand {
			if !c.IsCreature() {
				continue
			}
			ap.Hand = append(ap.Hand[:i], ap.Hand[i+1:]...)
			if perm, err := summonByName(ap, c.Name); err == nil && perm != nil {
				perm.SetEnteredTurn(g.GetTurnNumber())
			}
			again = true
			break
		}
	}

	bridge.AutoActivateMainPhaseAbilities(g)
}

// summonByName wraps Player.SummonCreature so callers can avoid the
// extra Hand lookup when the card is already known.
func summonByName(p *game.Player, name string) (*game.Permanent, error) {
	return p.SummonCreature(name)
}

// runCombatPhase declares all eligible attackers against the next
// surviving opponent and resolves combat damage.
func runCombatPhase(g *game.Game, ap *game.Player) {
	defender := nextLivingOpponent(g, ap)
	if defender == nil {
		return
	}
	for _, perm := range ap.GetCreatures() {
		if perm.IsTapped() {
			continue
		}
		_ = g.DeclareAttacker(perm, defender)
	}
	g.ResolveCombatDamage()
}

// nextLivingOpponent returns the next non-eliminated player after the
// active player in seat order.
func nextLivingOpponent(g *game.Game, ap *game.Player) *game.Player {
	players := g.GetPlayersRaw()
	n := len(players)
	start := indexOfPlayer(g, ap)
	if start < 0 {
		return nil
	}
	for i := 1; i < n; i++ {
		cand := players[(start+i)%n]
		if cand != ap && !cand.HasLost() {
			return cand
		}
	}
	return nil
}

func indexOfPlayer(g *game.Game, p *game.Player) int {
	for i, q := range g.GetPlayersRaw() {
		if q == p {
			return i
		}
	}
	return -1
}

// markMillIfApplicable lets classifyElimination distinguish mill from
// life-loss. We tag the player by zeroing life and rely on a sentinel
// (empty library + lost) at finalize time.
func markMillIfApplicable(p *game.Player) {
	// no-op; classifyElimination uses Library size as the heuristic
	_ = p
}
