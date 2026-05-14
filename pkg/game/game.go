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
	triggers []*Trigger
	watchers []Watcher
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

// removeDamageFromPermanents removes all damage from permanents during cleanup step.
func (g *Game) removeDamageFromPermanents() {
	for _, pl := range g.players {
		for _, perm := range pl.Battlefield {
			perm.ClearDamage()
		}
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
func (g *Game) AdvancePhase() {
	g.clearManaPools()
	switch g.currentPhase {
	case PhaseUntap:
		g.currentPhase = PhaseUpkeep
	case PhaseUpkeep:
		g.currentPhase = PhaseDraw
	case PhaseDraw:
		g.currentPhase = PhaseMain1
	case PhaseMain1:
		g.currentPhase = PhaseCombat
	case PhaseCombat:
		g.currentPhase = PhaseMain2
	case PhaseMain2:
		g.currentPhase = PhaseEnd
	case PhaseEnd:
		g.currentPhase = PhaseCleanup
		g.clearUntilEndOfTurnEffects()
	case PhaseCleanup:
		// end of turn -> next player
		if len(g.players) > 0 {
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
