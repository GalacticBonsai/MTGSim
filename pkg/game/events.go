package game

import "github.com/google/uuid"

// EventType identifies the kind of game event. Events come in replace/fire
// pairs where applicable. Batch events wrap multiple single events for
// "whenever one or more..." triggers.
type EventType int

const (
	// --- Zone changes ---
	EventZoneChange EventType = iota
	EventZoneChangeGroup
	EventDrawCard
	EventDrewCard
	EventDiscardCard
	EventDiscardedCard
	EventExileCard
	EventExiledCard
	EventPlayLand
	EventPlayedLand

	// --- Battlefield ---
	EventEntersBattlefield
	EventEntersBattlefieldSelf
	EventEntersBattlefieldControl
	EventEntersBattlefieldCopy
	EventLeavesBattlefield
	EventLeavesBattlefieldDied
	EventLeavesBattlefieldExiled
	EventLeavesBattlefieldReturned

	// --- Combat ---
	EventDeclareAttacker
	EventAttackersDeclared
	EventDeclareBlocker
	EventBlockersDeclared
	EventCreatureBlocked
	EventCombatDamageStepStart
	EventCombatDamageStepEnd
	EventEndOfCombatStep

	// --- Step events (backward compat) ---
	EventUntapStep
	EventUpkeepStep
	EventDrawStep
	EventBeginCombatStep
	EventDeclareAttackersStep
	EventDeclareBlockersStep
	EventCombatDamageStep
	EventEndStep
	EventCleanupStep

	// --- Damage ---
	EventDamagePlayer
	EventDamagedPlayer
	EventDamagePermanent
	EventDamagedPermanent
	EventDamageBatchForPlayers
	EventDamageBatchForPermanents
	EventNonCombatDamagePlayer

	// --- Life ---
	EventLoseLife
	EventLostLife
	EventLoseLifeBatch
	EventGainLife
	EventGainedLife
	EventGainLifeBatch

	// --- Spells & Abilities ---
	EventSpellCast
	EventSpellResolves
	EventSpellCountered
	EventActivatedAbility
	EventActivatedAbilitySource
	EventTriggeredAbility
	EventManaAbility

	// --- Counters ---
	EventAddCounter
	EventAddedCounter
	EventRemoveCounter
	EventRemovedCounter
	EventAddCounterBatch

	// --- Tap/Untap ---
	EventTapped
	EventTappedForMana
	EventUntapped

	// --- Targeting ---
	EventTargeted
	EventTargeting

	// --- Destroy/Sacrifice/Exile ---
	EventDestroyPermanent
	EventDestroyedPermanent
	EventSacrificedPermanent
	EventSacrificePermanent
	EventExilePermanent
	EventExiledPermanent
	EventSacrificed

	// --- Control ---
	EventGainControl
	EventGainedControl
	EventLoseControl
	EventControllerChanged

	// --- Tokens ---
	EventCreateToken
	EventCreateTokenCopies
	EventTokenCreated

	// --- Transform ---
	EventTransform
	EventTransformed
	EventFlip
	EventFlipped

	// --- Phasing ---
	EventPhaseIn
	EventPhaseOut

	// --- Face-down ---
	EventTurnFaceDown
	EventTurnedFaceDown
	EventTurnFaceUp
	EventTurnedFaceUp

	// --- Copy ---
	EventCopySpell
	EventCopiedSpell
	EventCopyPermanent
	EventCopiedPermanent

	// --- Special ---
	EventShuffleLibrary
	EventShuffledLibrary
	EventRevealCard
	EventRevealedCard
	EventScry
	EventSurveil
	EventFateseal
	EventProliferate

	// --- Player ---
	EventLoseGame
	EventWinGame
	EventMulligan
	EventSkipStep

	// --- Day/Night ---
	EventDayBecomesNight
	EventNightBecomesDay

	// --- Commander ---
	EventCommanderDamage

	// --- Misc ---
	EventOther
)

// Event is the core event object dispatched through the game engine.
// It carries context about what happened and supports replacement effect
// tracking via AppliedEffects to prevent infinite loops (CR 614.5).
type ZoneChange struct {
	Card      SimpleCard
	Permanent *Permanent
	From      Zone
	To        Zone
	LKI       *PermanentSnapshot
}

type Event struct {
	ID              uuid.UUID
	Type            EventType
	TargetID        uuid.UUID    // the object affected
	SourceID        uuid.UUID    // the source ability/object
	ControllerID    string       // controller of the affected object
	PlayerID        string       // player involved (for life/damage events)
	Amount          int          // generic amount (damage, counters, life)
	Flag            bool         // context-dependent boolean
	Data            string       // context-dependent string (counter name, etc.)
	Zone            Zone         // origin zone for zone changes
	ZoneChange      *ZoneChange  // zone change details
	Player          *Player      // player reference (for backward compat)
	Permanent       *Permanent   // permanent reference (for backward compat)
	Card            *SimpleCard  // card reference (for backward compat)
	AppliedEffects  []uuid.UUID  // replacement effects already applied (CR 614.5)
	BatchEvents     []*Event     // sub-events for batch processing
	IsBatch         bool         // true for batch events
	Preventable     bool         // whether damage can be prevented
	IsCombatDamage  bool         // whether damage is from combat
}

// BatchEvent creates a batch wrapper for multiple single events.
func BatchEvent(parentType EventType, events []*Event) *Event {
	return &Event{
		ID:          uuid.New(),
		Type:        parentType,
		IsBatch:     true,
		BatchEvents: events,
	}
}

// HasAppliedEffect checks if a replacement effect has already been applied
// to this event (CR 614.5).
func (e *Event) HasAppliedEffect(effectID uuid.UUID) bool {
	for _, id := range e.AppliedEffects {
		if id == effectID {
			return true
		}
	}
	return false
}

// AddAppliedEffect records that a replacement effect has processed this event.
func (e *Event) AddAppliedEffect(effectID uuid.UUID) {
	e.AppliedEffects = append(e.AppliedEffects, effectID)
}

// Listener registration
func (g *Game) AddListener(l func(Event)) { g.listeners = append(g.listeners, l) }

func (g *Game) emit(e Event) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	// Notify listeners first
	for _, l := range g.listeners {
		l(e)
	}
	// Then process triggers and watchers
	g.handleTriggers(e)
	g.handleWatchers(e)
}

// emitEvent is the public wrapper that also runs replacement effects first.
func (g *Game) emitEvent(e Event) *Event {
	// Run replacement effects pipeline (CR 614)
	replaced := g.applyReplacementEffects(e)
	if replaced == nil {
		return nil // event was fully replaced / cancelled
	}
	// Fire the event
	g.emit(*replaced)
	return replaced
}

func snapshotPermanent(p *Permanent) *PermanentSnapshot {
	if p == nil {
		return nil
	}
	return &PermanentSnapshot{
		Name:      p.GetName(),
		Power:     p.GetPower(),
		Toughness: p.GetToughness(),
		Damage:    p.GetDamageCounters(),
	}
}

// PermanentSnapshot stores a point-in-time copy of a permanent's stats
// for Last Known Information (LKI) references.
type PermanentSnapshot struct {
	Name      string
	Power     int
	Toughness int
	Damage    int
}

// HasAppliedEffect checks if this permanent snapshot was affected by a
// specific replacement effect.
func (ps *PermanentSnapshot) HasAppliedEffect(effectID uuid.UUID) bool {
	return false // snapshots don't track this
}
