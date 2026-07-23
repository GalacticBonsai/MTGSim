package game

import "github.com/google/uuid"

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
	step         SubStep

	// event listeners
	listeners []func(Event)

	casting      *casting
	combat       *combat
	continuous   *continuous
	prevention   *prevention

	// CR 616: replacement effects engine
	replacementEngine *replacementEngine

	// CR 609.4: as-though effects engine
	asThoughEngine *asThoughEngine

	// CR 613.1f: copy effects engine
	copyEngine *copyEngine

	// CR 717: game-level stack (injected by the caller)
	gameStack GameStack

	// triggers and watchers
	triggers        []*Trigger
	pendingTriggers []PendingTrigger
	watchers        []Watcher

	// delayed and reflexive triggers (CR 603.7, 603.12)
	delayedTriggers   []*DelayedTriggeredAbility
	reflexiveTriggers []*ReflexiveTriggeredAbility

	// extra turns queued by card effects (e.g. Time Warp)
	extraTurns int

	// phaseAction is called at the start of each phase/step to perform
	// the mandatory actions for that phase. It is injectable for testing.
	phaseAction func(g *Game)

	// playerAction is called when a player has priority. It should return
	// true if the player takes an action (casts a spell or activates an
	// ability), false if they pass. It is injectable for testing.
	playerAction func(g *Game, p *Player) bool
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
	g.clearReplacementEffectsEOT()
	g.clearPreventionEOT()
	// clear as-though effects
	g.clearAsThoughEffectsEOT()
	// clear delayed triggers
	g.clearDelayedTriggersEOT()
	// reset watchers
	g.resetWatchersEOT()
}

// --- Unified Game Loop (ported from XMage GameImpl) ---

// SetGameStack injects a GameStack implementation for the unified game loop.
func (g *Game) SetGameStack(s GameStack) { g.gameStack = s }

// GetGameStack returns the game-level stack, creating a minimal fallback if none was set.
func (g *Game) GetGameStack() GameStack {
	if g.gameStack == nil {
		g.gameStack = &simpleGameStack{}
	}
	return g.gameStack
}

// PlayTurn executes a single complete turn: all phases from untap through
// cleanup, with priority passing and stack resolution at each appropriate
// point (CR 500).
func (g *Game) PlayTurn() {
	for !g.HasWinner() {
		g.PlayPhase()
	}
}

// PlayPhase executes the current phase including all its sub-steps,
// with priority passing at the appropriate points.
func (g *Game) PlayPhase() {
	// CR 500.1: phases happen in order; each step starts with the
	// mandatory action, then players get priority.
	switch g.currentPhase {
	case PhaseUntap:
		// CR 502: no priority in untap step
		g.untapStep()
		g.emit(Event{Type: EventUntapStep})
		g.currentPhase = PhaseUpkeep

	case PhaseUpkeep:
		g.playStep()

	case PhaseDraw:
		// CR 504.1: draw first, then priority
		g.drawStep()
		g.emit(Event{Type: EventDrawStep})
		g.playStep()

	case PhaseMain1:
		g.playStep()

	case PhaseCombat:
		g.playCombatPhase()

	case PhaseMain2:
		g.playStep()

	case PhaseEnd:
		g.playStep()

	case PhaseCleanup:
		// CR 514: cleanup step — mandatory actions, no priority normally
		g.cleanupStep()
		g.emit(Event{Type: EventCleanupStep})
		// CR 514.3: if anything triggers or a player gains priority during
		// cleanup, an extra cleanup step happens (we don't loop here; the
		// caller can check for pending triggers).
		g.advanceToNextTurn()
	}
}

// playStep runs the standard priority-passing loop for a single step
// (CR 500.4). It passes priority starting with the active player and
// continues until all players pass in succession with an empty stack.
func (g *Game) playStep() {
	// Fire step-begin event
	g.emitStepEvent()

	// Priority round: APNAP (CR 500.4)
	for {
		// Run SBAs first (CR 704)
		g.ApplyStateBasedActions()

		// Process pending triggers (CR 603.3)
		g.processPendingTriggers()

		// If the stack is not empty, resolve the top item
		stack := g.GetGameStack()
		if !stack.IsEmpty() {
			item := stack.Pop()
			if item != nil && item.Resolve != nil {
				_ = item.Resolve(g)
			}
			// After resolution, return to priority loop
			continue
		}

		// Stack is empty; pass priority to each player
		allPassed := true
		for i := 0; i < len(g.players); i++ {
			idx := (g.activeIdx + i) % len(g.players)
			p := g.players[idx]
			if p == nil || p.HasLost() {
				continue
			}
			if g.playerAction != nil && g.playerAction(g, p) {
				allPassed = false
				break // someone acted; restart the loop
			}
		}
		if allPassed {
			return // all players passed with empty stack
		}
	}
}

// playCombatPhase handles the full combat phase with its sub-steps
// (CR 507-510).
func (g *Game) playCombatPhase() {
	// CR 507.1: beginning of combat — triggers
	g.step = StepBeginCombat
	g.emit(Event{Type: EventBeginCombatStep})
	g.BeginCombat()
	g.playStep()

	// CR 508: declare attackers
	g.step = StepDeclareAttackers
	g.emit(Event{Type: EventDeclareAttackersStep})
	g.playStep()

	// CR 509: declare blockers
	g.step = StepDeclareBlockers
	g.emit(Event{Type: EventDeclareBlockersStep})
	g.playStep()

	// CR 510: combat damage
	g.step = StepCombatDamage
	g.emit(Event{Type: EventCombatDamageStep})
	g.ResolveCombatDamage()
	g.playStep()

	// CR 511: end of combat
	g.step = StepEndOfCombat
	g.emit(Event{Type: EventEndOfCombatStep})
	g.playStep()

	// Move to main 2
	g.currentPhase = PhaseMain2
}

// emitStepEvent fires the appropriate event for the current phase/step.
func (g *Game) emitStepEvent() {
	switch g.currentPhase {
	case PhaseUpkeep:
		g.emit(Event{Type: EventUpkeepStep})
	case PhaseDraw:
		g.emit(Event{Type: EventDrawStep})
	case PhaseMain1:
		// no specific step event for main phase
	case PhaseMain2:
		// no specific step event for main phase
	case PhaseEnd:
		g.emit(Event{Type: EventEndStep})
	case PhaseCleanup:
		g.emit(Event{Type: EventCleanupStep})
	}
}

// processPendingTriggers drains all pending triggers and resolves them
// through the stack with a priority round (CR 603.3).
func (g *Game) processPendingTriggers() {
	if !g.HasPendingTriggers() {
		return
	}
	triggers := g.DrainPendingTriggers()
	for _, pt := range triggers {
		if pt.Trigger == nil || pt.Trigger.Action == nil {
			continue
		}
		// Create a stack item for the triggered ability
		item := &GameStackItem{
			ID:          uuid.New(),
			Type:        GameStackTriggeredAbility,
			Controller:  pt.Trigger.Controller,
			Description: "triggered ability",
			Resolve: func(g *Game) error {
				pt.Trigger.Action(g, pt.Event)
				return nil
			},
		}
		g.GetGameStack().Push(item)
	}
	// Now run a priority round to resolve these triggers
	g.playStep()
}

// advanceToNextTurn rotates to the next living player and increments
// the turn number.
func (g *Game) advanceToNextTurn() {
	g.clearManaPools()
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
	g.step = StepUntapNone
}

// HasWinner returns true if only one player remains.
func (g *Game) HasWinner() bool {
	living := 0
	for _, p := range g.players {
		if p != nil && !p.HasLost() {
			living++
		}
	}
	return living <= 1
}

// GetWinner returns the last remaining player, or nil if the game is not over.
func (g *Game) GetWinner() *Player {
	var winner *Player
	living := 0
	for _, p := range g.players {
		if p != nil && !p.HasLost() {
			winner = p
			living++
		}
	}
	if living == 1 {
		return winner
	}
	return nil
}
