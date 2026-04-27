package simulation

import (
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// stepOneEDHTurn drives the active player through a complete turn using
// the simplified runner AI. Returns false if no players are alive after
// the turn finishes. priority is invoked at instant-speed windows so
// future AI can respond on opponents' turns; log is optional.
func stepOneEDHTurn(g *game.Game, seats []EDHSeat, casts []int, priority PriorityHandler, log *EDHEventLog) bool {
	startTurn := g.GetTurnNumber()
	milledThisTurn := false
	for {
		ap := g.GetActivePlayerRaw()
		switch g.GetCurrentPhase() {
		case game.PhaseUntap:
			for _, perm := range ap.Battlefield {
				perm.Untap()
			}
		case game.PhaseUpkeep:
			offerOpponentPriority(g, ap, priority)
		case game.PhaseDraw:
			if g.GetTurnNumber() > 1 || ap != g.GetPlayerByIndex(0) {
				if ap.Draw(1) == 0 {
					ap.SetLifeTotal(0) // CR 704.5b: empty-library draw loss proxy
					milledThisTurn = true
				}
			}
			offerOpponentPriority(g, ap, priority)
		case game.PhaseMain1:
			if log != nil {
				log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventTurnStart, Actor: ap.GetName()})
			}
			runMainPhase(g, ap, casts, log)
			offerOpponentPriority(g, ap, priority)
		case game.PhaseCombat:
			runCombatPhase(g, ap, log)
			offerOpponentPriority(g, ap, priority)
		case game.PhaseEnd:
			offerOpponentPriority(g, ap, priority)
		}
		g.ApplyStateBasedActions()
		if milledThisTurn {
			markMillIfApplicable(ap)
		}
		recordEliminations(g, log, ap)
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

// offerOpponentPriority walks each living non-active opponent in APNAP
// order and invokes the priority handler. Default Noop short-circuits.
func offerOpponentPriority(g *game.Game, ap *game.Player, h PriorityHandler) {
	if h == nil {
		return
	}
	if _, ok := h.(NoopPriorityHandler); ok {
		return
	}
	players := g.GetPlayersRaw()
	n := len(players)
	start := indexOfPlayer(g, ap)
	for i := 1; i < n; i++ {
		opp := players[(start+i)%n]
		if opp == ap || opp.HasLost() {
			continue
		}
		h.OnOpponentPriority(g, ap, opp, g.GetCurrentPhase())
	}
}

// recordEliminations emits a player_eliminated event the first time a
// player is observed as lost. Tracking lives in the closure-free way:
// we just inspect HasLost and avoid duplicate emission by checking the
// last event of the matching kind for that actor.
func recordEliminations(g *game.Game, log *EDHEventLog, ap *game.Player) {
	if log == nil {
		return
	}
	existing := log.Events()
	already := map[string]bool{}
	for _, e := range existing {
		if e.Kind == EventPlayerEliminated {
			already[e.Actor] = true
		}
	}
	for _, p := range g.GetPlayersRaw() {
		if !p.HasLost() || already[p.GetName()] {
			continue
		}
		log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(g.GetCurrentPhase()), Kind: EventPlayerEliminated, Actor: p.GetName()})
	}
}

// runMainPhase plays one land, casts the commander when possible, and
// summons every creature in hand (no mana enforcement — a deliberate
// simplification mirrored from cmd/mtgsim's main loop). Optional log
// records every public action so a replay can be reproduced.
func runMainPhase(g *game.Game, ap *game.Player, casts []int, log *EDHEventLog) {
	for i, c := range ap.Hand {
		if c.IsLand() {
			ap.Hand = append(ap.Hand[:i], ap.Hand[i+1:]...)
			_, _ = g.PlayLand(ap, c.Name)
			if log != nil {
				log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventLandPlay, Actor: ap.GetName(), Detail: c.Name})
			}
			break
		}
	}

	idx := indexOfPlayer(g, ap)
	if idx >= 0 && len(ap.CommandZone) > 0 {
		name := ap.CommandZone[0].Name
		if perm := ap.CastCommander(name); perm != nil {
			perm.SetEnteredTurn(g.GetTurnNumber())
			casts[idx]++
			if log != nil {
				log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventCommanderCast, Actor: ap.GetName(), Detail: name})
			}
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
				if log != nil {
					log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseMain1), Kind: EventCreatureSummon, Actor: ap.GetName(), Detail: c.Name})
				}
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

// runCombatPhase declares all eligible attackers against the most
// threatening living opponent (Phase 4) and resolves combat damage.
// Falls back to seat-order rotation when threat scores are tied.
func runCombatPhase(g *game.Game, ap *game.Player, log *EDHEventLog) {
	defender := chooseAttackTarget(g, ap)
	if defender == nil {
		return
	}
	declared := 0
	for _, perm := range ap.GetCreatures() {
		if perm.IsTapped() {
			continue
		}
		if err := g.DeclareAttacker(perm, defender); err == nil {
			declared++
			if log != nil {
				log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseCombat), Kind: EventAttackDeclared, Actor: ap.GetName(), Target: defender.GetName(), Detail: perm.GetName()})
			}
		}
	}
	g.ResolveCombatDamage()
	if log != nil && declared > 0 {
		log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(game.PhaseCombat), Kind: EventCombatResolved, Actor: ap.GetName(), Target: defender.GetName()})
	}
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
