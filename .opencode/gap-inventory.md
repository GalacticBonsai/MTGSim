# MTGSim Gap Inventory vs XMage
Generated from XMage Java → MTGSim Go comparison.

## PRIORITY 1: CRITICAL (blocks correct rules)

### 1. Unified Game Loop
**Status:** MISSING
**XMage:** `GameImpl.play()` → `playTurn()` → `playPriority()` integrates stack resolution with turn phases
**MTGSim:** `game.go` advances phases but never calls the stack. `ability/stack.go` exists but is disconnected.
**What to build:**
- `Game.Play()` main loop
- `Game.PlayTurn(player)` turn lifecycle
- `Game.PlayPriority(activePlayer)` priority passing loop
- `Game.Resolve()` stack resolution
- Wire `AdvancePhase()` to call `PlayPriority()` at each step

### 2. Turn Sub-Steps
**Status:** PARTIAL (8 flat phases)
**XMage:** 22 sub-steps with PRE/POST events
**MTGSim:** Untap, Upkeep, Draw, Main1, Combat, Main2, End, Cleanup
**Missing sub-steps:**
- BeginCombat (start of combat)
- DeclareAttackersStep
- DeclareBlockersStep
- CombatDamageStep
- EndOfCombatStep
- EndStep (before cleanup)

### 3. Replacement Effects (CR 614)
**Status:** MINIMAL (only "exile instead of die")
**XMage:** Full framework with `ReplacementEffect` interface, dependency resolution, consumed-map prevention
**What to build:**
- `ReplacementEffect` interface
- `ReplacementEffectEngine` with `ReplaceEvent()` pipeline
- `AppliedEffects` tracking on events (prevent infinite loops)
- Self-replacement priority (CR 616.1a-d for ETB)

### 4. Delayed/Reflexive Triggers
**Status:** MISSING
**XMage:** `DelayedTriggeredAbility` with duration + one-shot; `ReflexiveTriggeredAbility`
**What to build:**
- `DelayedTriggeredAbility` struct (duration, triggerOnlyOnce)
- `ReflexiveTriggeredAbility` struct (fires immediately, not on stack)
- Duration tracking (until end of turn, until next upkeep, etc.)

### 5. As-Though Effects
**Status:** MISSING
**XMage:** 20+ `AsThoughEffectType` constants
**What to build:**
- `AsThoughEffect` interface
- Types: CanCastFromNonHand, CanActivateFromNonManaAbilities, DamageNotBlocked, LoseAbility, Untap, etc.
- Query methods on `ContinuousEffects`

### 6. Full Event System
**Status:** MINIMAL (19 types)
**XMage:** 150+ event types with batch variants
**Missing events (critical):**
- EventSpellCast, EventSpellResolves
- EventActivatedAbility, EventTriggeredAbility
- EventDamagePlayer, EventDamagedPlayer, EventDamagedPermanent
- EventLoseLife, EventGainedLife
- EventTapped, EventUntapped
- EventLeavesBattlefield
- EventDestroyedPermanent
- EventAddCounter, EventRemoveCounter
- EventTargeted, EventCounter
- EventDeclareAttacker, EventBlockerDeclared
- EventGainControl, EventLoseControl
- EventCreateToken, EventTransformed
- Batch variants for all above

### 7. Copy Effects (Layer 1)
**Status:** MISSING
**XMage:** `CopyEffect`, `copyPermanent`, `copyCard`, `CopyApplier`
**What to build:**
- Copy permanent effect (Layer 1)
- Copy spell on stack
- Token copy of spells
- Copy tracking (`wasCopied`, `copySource`)

## PRIORITY 2: HIGH (significantly limits card implementation)

### 8. Non-Evergreen Keywords (228 missing)
**Status:** 21 evergreen only
**XMage:** 249 keyword implementations
**Critical missing for cEDH:**
- Kicker/Multikicker (cost tags)
- Storm (triggered, count spells)
- Cascade (exile until CMC match)
- Suspend (time counters, exile from hand)
- Flashback (cast from graveyard)
- Madness (cast on discard)
- Dredge (replace draw)
- Escape (exile from GY as cost)
- Forecast (reveal from hand)
- Bestow (cast as aura)
- Proliferate (already have Player.Proliferate, need keyword integration)
- Equip/Attach (activation costs)
- Indestructible (already have keyword, need rules enforcement)
- Protection (already have, need full DEBT)
- Hexproof (already have, need targeting enforcement)
- Shroud (already have, need targeting enforcement)

### 9. Restriction/Requirement Effects
**Status:** MISSING
**XMage:** `RestrictionEffect` (can't attack/block/untap), `RequirementEffect` (must attack/block)
**What to build:**
- `RestrictionEffect` interface
- `RequirementEffect` interface
- Integration with combat declaration

### 10. Watchers (Comprehensive)
**Status:** MINIMAL (1 watcher)
**XMage:** Dozens of watchers
**Critical missing:**
- LifeLostThisTurnWatcher
- CardsDrawnThisTurnWatcher
- CreaturesAttackedThisTurnWatcher
- SpellsCastThisTurnWatcher
- DamageDealtThisTurnWatcher
- CreatureDiedThisTurnWatcher

### 11. Targeting/Filter System
**Status:** BASIC
**XMage:** 100+ predicate types, `FilterPermanent`, `FilterCard`
**What to build:**
- `Filter[E]` generic interface
- `Predicate[E]` chain
- `FilterPermanent` with type/color/subtype/controller predicates
- `FilterCard` for hand/graveyard/library
- `StaticFilters` with pre-built instances

### 12. Zone-Change Tracking (LKI)
**Status:** SIMPLIFIED
**XMage:** `zoneChangeCounter`, full LKI by zone, `MageObjectReference`
**What to build:**
- Zone-change counter per card
- Persistent LKI storage (not just snapshots)
- `MageObjectReference` for stable cross-zone references

### 13. DFC/Split/Meld Cards
**Status:** MISSING
**XMage:** `SplitCard`, `DoubleFacedCard`, `MeldCard`, `CardState`
**What to build:**
- `CardState` (front/back face)
- `DoubleFacedCard` type
- `SplitCard` type
- Transform mechanics

## PRIORITY 3: MEDIUM

### 14. Permanent Attachment
**Status:** BASIC (pointer only)
**Missing:** Equip costs, Aura "enchant" restrictions, living weapon

### 15. Damage System
**Status:** BASIC
**Missing:** Batch damage events, damage redirection, preventable/non-preventable distinction

### 16. Game State Serialization
**Status:** MISSING
**Missing:** Save/restore, rollback, replay

### 17. Multiplayer
**Status:** BASIC (Commander only)
**Missing:** Range of influence, monarch, initiative

### 18. Mulligan Variants
**Status:** London only
**Missing:** Vancouver, Paris

### 19. Counter Replacement Effects
**Status:** MISSING
**Missing:** "If a counter would be placed," replacement effects

### 20. Special Actions
**Status:** MISSING
**Missing:** Cycling, foretell, suspend-from-hand
