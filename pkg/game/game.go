package game

// Phase represents the current phase of a turn (skeleton for Task 4).
type Phase int

const (
	PhaseUntap Phase = iota
	PhaseUpkeep
	PhaseDraw
	PhaseMain1
	PhaseCombat
	PhaseMain2
	PhaseEnd
	PhaseCleanup
)

// Game is the core game container for players and shared state.
type Game struct {
	players []*Player

	currentIdx   int
	activeIdx    int
	turnNumber   int
	currentPhase Phase

	// event listeners
	listeners []func(Event)

	casting      *casting
	combat       *combat
	continuous   *continuous
	replacements *replacements
	prevention   *prevention

	// triggers and watchers
	triggers        []*Trigger
	pendingTriggers []PendingTrigger
	watchers        []Watcher

	// extra turns queued by card effects (e.g. Time Warp)
	extraTurns int
}

// ApplyTempPump grants a temporary power/toughness boost until end of turn.
func (g *Game) ApplyTempPump(p *Permanent, dp, dt int) {
	if p == nil {
		return
	}
	p.addTempPump(dp, dt)
}

func NewGame(players ...*Player) *Game {
	g := &Game{players: players}
	g.currentIdx = 0
	g.activeIdx = 0
	g.turnNumber = 1
	g.currentPhase = PhaseUntap
	return g
}

func (g *Game) GetPlayersRaw() []*Player { return g.players }
func (g *Game) NumPlayers() int          { return len(g.players) }
func (g *Game) GetPlayerByIndex(i int) *Player {
	if i < 0 || i >= len(g.players) {
		return nil
	}
	return g.players[i]
}

func (g *Game) GetCurrentPlayerRaw() *Player { return g.GetPlayerByIndex(g.currentIdx) }
func (g *Game) GetActivePlayerRaw() *Player  { return g.GetPlayerByIndex(g.activeIdx) }

func (g *Game) GetTurnNumber() int     { return g.turnNumber }
func (g *Game) GetCurrentPhase() Phase { return g.currentPhase }

// TakeExtraTurn queues one extra turn for the active player.
func (g *Game) TakeExtraTurn() { g.extraTurns++ }
func (g *Game) IsMainPhase() bool {
	return g.currentPhase == PhaseMain1 || g.currentPhase == PhaseMain2
}
func (g *Game) IsCombatPhase() bool { return g.currentPhase == PhaseCombat }

// WinGame marks all opponents of the given winner as lost by the specified win condition.
// Common conditions: "combat", "commander_damage", "deckout", "effect".
func (g *Game) WinGame(winner *Player, condition string) {
	for _, pl := range g.players {
		if pl != winner && !pl.HasLost() {
			pl.Lose(condition)
		}
	}
}

// LoseGame marks the specified player as lost with the given reason.
func (g *Game) LoseGame(loser *Player, reason string) {
	if loser != nil && !loser.HasLost() {
		loser.Lose(reason)
	}
}

// findNextLivingPlayer searches clockwise from startIdx for the next player
// who has not lost. It wraps around the player slice. If the only living
// player is startIdx itself, startIdx is returned. If no living players exist,
// -1 is returned.
func (g *Game) findNextLivingPlayer(startIdx int) int {
	n := len(g.players)
	if n == 0 {
		return -1
	}
	for i := 1; i <= n; i++ {
		idx := (startIdx + i) % n
		if !g.players[idx].HasLost() {
			return idx
		}
	}
	if !g.players[startIdx].HasLost() {
		return startIdx
	}
	return -1
}

// AdvancePhase steps to the next phase; on cleanup step completion, rotate to next player's turn.
// Mana pools are emptied at the end of the current phase before advancing (CR 500.4).
// Phase-specific mandatory actions (untap, draw, cleanup) are performed automatically.
func (g *Game) AdvancePhase() {
	g.clearManaPools()
	switch g.currentPhase {
	case PhaseUntap:
		g.currentPhase = PhaseUpkeep
		// CR 502: no priority during untap step, just untap all permanents
		g.untapStep()
		g.emit(Event{Type: EventUntapStep})
	case PhaseUpkeep:
		g.currentPhase = PhaseDraw
		g.emit(Event{Type: EventUpkeepStep})
	case PhaseDraw:
		g.currentPhase = PhaseMain1
		// CR 504.1: active player draws a card at the beginning of the draw step
		g.drawStep()
		g.emit(Event{Type: EventDrawStep})
	case PhaseMain1:
		g.currentPhase = PhaseCombat
		g.emit(Event{Type: EventBeginCombatStep})
	case PhaseCombat:
		g.currentPhase = PhaseMain2
	case PhaseMain2:
		g.currentPhase = PhaseEnd
	case PhaseEnd:
		g.currentPhase = PhaseCleanup
		// CR 514: cleanup step — remove damage, end duration effects, discard to max
		g.cleanupStep()
		g.emit(Event{Type: EventCleanupStep})
	case PhaseCleanup:
		// end of turn -> next player (or extra turn)
		if g.extraTurns > 0 {
			g.extraTurns--
			g.turnNumber++
		} else if len(g.players) > 0 {
			nextIdx := g.findNextLivingPlayer(g.currentIdx)
			if nextIdx >= 0 {
				if nextIdx < g.currentIdx {
					g.turnNumber++
				}
				g.currentIdx = nextIdx
				g.activeIdx = nextIdx
			}
		}
		g.currentPhase = PhaseUntap
	}
}

func (g *Game) clearManaPools() {
	for _, pl := range g.players {
		if pl != nil {
			pl.ClearManaPool()
		}
	}
}

// untapStep performs CR 502 mandatory actions: untap all permanents
// controlled by the active player. No priority is given.
func (g *Game) untapStep() {
	ap := g.GetActivePlayerRaw()
	if ap == nil {
		return
	}
	for _, perm := range ap.Battlefield {
		perm.Untap()
	}
}

// firstPlayerIdx stores the index of the player who went first (for CR 103.4a).
func (g *Game) firstPlayerIdx() int {
	return 0
}

// drawStep performs CR 504.1: active player draws one card.
// CR 103.4a: the player who goes first doesn't draw on turn 1.
func (g *Game) drawStep() {
	ap := g.GetActivePlayerRaw()
	if ap == nil || ap.HasLost() {
		return
	}
	// CR 103.4a: first player skips draw on turn 1
	if g.turnNumber == 1 && g.activeIdx == g.firstPlayerIdx() {
		return
	}
	ap.Draw(1)
}

// cleanupStep performs CR 514 mandatory actions:
// - Remove all damage from permanents (CR 514.2)
// - End all "until end of turn" effects (CR 514.2)
// - Active player discards to maximum hand size (CR 514.1)
func (g *Game) cleanupStep() {
	g.clearUntilEndOfTurnEffects()
	ap := g.GetActivePlayerRaw()
	if ap != nil {
		ap.DiscardToMax()
	}
}

// clearUntilEndOfTurnEffects resets temporary effects that expire at EOT
func (g *Game) clearUntilEndOfTurnEffects() {
	// reset temp pumps and clear damage
	for _, pl := range g.players {
		for _, perm := range pl.Battlefield {
			perm.clearTempPump()
			perm.ClearDamage()
		}
	}
	// drop EOT-duration layered effects and recompute views
	g.clearLayeredEffectsEOT()
	g.RecomputeContinuous()
	// clear replacements and prevention
	g.clearReplacementsEOT()
	g.clearPreventionEOT()
	// reset watchers
	g.resetWatchersEOT()
}
