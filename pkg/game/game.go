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

// AdvancePhase steps to the next phase; on end step completion, rotate to next player's turn.
func (g *Game) AdvancePhase() {
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
		// end of turn -> next player
		g.clearUntilEndOfTurnEffects()
		if len(g.players) > 0 {
			g.currentIdx = (g.currentIdx + 1) % len(g.players)
			g.activeIdx = g.currentIdx
		}
		g.currentPhase = PhaseUntap
		if g.currentIdx == 0 { // wrapped around to first player
			g.turnNumber++
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
