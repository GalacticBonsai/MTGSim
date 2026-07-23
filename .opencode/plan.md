# MTGSim Game Engine Completeness Plan

## Objective
Ensure MTGSim's game engine covers 100% of MTG rules relevant to cEDH simulation, matching XMage's rule implementation while staying dynamic (regex-parsed oracle text, not per-card Java classes). Add comprehensive unit tests (~80+) validating card/effect parsing and engine correctness.

---

## Phase 1: Fix Existing Build Break + Foundation (2 files)

### 1.1 Fix stack_test.go build break
- **File:** `pkg/ability/stack_test.go:418,422`
- Add missing `containsSubstring` helper function
- Verify full ability test suite passes

### 1.2 Add missing keywords to keywords.go
- **File:** `pkg/game/keywords.go`
- Add: `KWFlash`, `KWShroud`, `KWWard`, `KWProtection`, `KWPhasing`, `KWMenace` (already there), `KWIntimidate`, `KWShadow`, `KWProwess`, `KWDeathtouch` (already there), `KWDefender` (already there)
- At minimum add: `KWFlash`, `KWShroud`, `KWWard`
- Update `parseKeywordsFromOracle` to recognize "flash", "shroud", "ward {X}"
- Update `keywordFromToken` switch

### 1.3 Add missing permanent types
- **File:** `pkg/game/simple_card.go`
- Add `IsBattle()`, `IsInstant()` already there, etc.
- Add `IsTribal()` if needed

---

## Phase 2: Game Engine Phase Automation (2 files)

### 2.1 Add phase hook events
- **File:** `pkg/game/events.go`
- Add event types: `EventUntapStep`, `EventUpkeepStep`, `EventDrawStep`, `EventMainStep`, `EventBeginCombatStep`, `EventEndStep`, `EventCleanupStep`
- These fire when `AdvancePhase()` enters each phase

### 2.2 Automate phase actions in AdvancePhase
- **File:** `pkg/game/game.go`
- **Untap step (CR 502):** Auto-untap all permanents controlled by active player. No priority given.
- **Upkeep step (CR 503):** Fire `EventUpkeepStep` for triggers. Give priority.
- **Draw step (CR 504):** Active player draws 1 card. Fire `EventDrawStep`. Check deck-out (CR 704.5b).
- **Main phases:** Fire phase events. Sorcery-speed timing available.
- **End step (CR 513):** Fire `EventEndStep` for "at the beginning of end step" triggers.
- **Cleanup step (CR 514):** Remove damage, clear temp effects, discard to max hand size (7). Only give priority if SBAs would be performed or triggers are pending; if so, run another cleanup after.

### 2.3 Add discard-to-max-hand-size
- **File:** `pkg/game/player.go`
- Add `MaxHandSize()` (returns 7, overridable for Reliquary Tower effects)
- Add `DiscardToMax()` method

---

## Phase 3: Missing SBAs (1 file)

### 3.1 Expand state-based actions
- **File:** `pkg/game/sba.go`
- Add in correct CR 704.5 order:
  - **CR 704.5a:** Player at ≤0 life loses ✓ (already there)
  - **CR 704.5b:** Player who tried to draw from empty library loses ✓ (partially - draw step handles this)
  - **CR 704.5c:** Poison counters ≥10 → player loses. Add `poisonCounters` field to Player.
  - **CR 704.5d:** Not relevant for Commander
  - **CR 704.5e:** Not relevant for Commander
  - **CR 704.5f/g/h:** Lethal damage / 0 toughness ✓ (already there)
  - **CR 704.5i:** Planeswalker with 0 loyalty → sacrifice. Add loyalty counter check.
  - **CR 704.5k:** Legend rule ✓ (already there)
  - **CR 704.5n:** Planeswalker uniqueness ✓ (already there)
  - **CR 704.5p:** Aura attached to illegal object → graveyard ✓ (already there)
  - **CR 704.5r:** +1/+1 and -1/-1 counter cancellation. Add counter removal logic.
  - **CR 704.5u:** Commander 21 damage ✓ (already there)
- Add `ApplyStateBasedActions` loop (keep running until no changes occur, CR 117.5)

---

## Phase 4: Missing Keywords (3 files)

### 4.1 Flash (CR 702.7)
- **File:** `pkg/game/keywords.go` — add keyword constant
- **File:** `pkg/ability/priority.go` — modify `isInstantSpeed` to check Flash keyword on permanents
- **File:** `pkg/ability/spell_casting.go` — modify sorcery-speed timing check to allow Flash
- **File:** `pkg/game/permanent.go` — add `HasFlash()` convenience method

### 4.2 Shroud (CR 702.18)
- **File:** `pkg/game/keywords.go` — add keyword constant
- **File:** `pkg/ability/targeting.go` — update `hasShroud` to actually check permanent keywords

### 4.3 Ward (CR 702.21)
- **File:** `pkg/game/keywords.go` — add keyword constant + ward cost tracking
- **File:** `pkg/game/permanent.go` — add `wardCost` field, `SetWardCost()`, `GetWardCost()`
- **File:** `pkg/ability/targeting.go` — when targeting a warded permanent by opponent, trigger ward ability (counter unless cost paid)

### 4.4 Protection (CR 702.16)
- **File:** `pkg/game/keywords.go` — add protection from color/card type tracking
- **File:** `pkg/game/permanent.go` — add `protectionFrom` field (colors, types)
- **File:** `pkg/game/combat.go` — prevent blocked by creatures with protected quality
- **File:** `pkg/ability/targeting.go` — prevent targeting by spells with protected quality
- **File:** `pkg/game/sba.go` — remove auras/equipment from protected sources

---

## Phase 5: Missing Game Actions (2 files)

### 5.1 Sacrifice (CR 701.13)
- **File:** `pkg/game/player.go`
- Add `SacrificePermanent(perm *Permanent) bool` — removes from battlefield, moves to graveyard
- **File:** `pkg/game/transitions.go`
- Add `Sacrifice(p *Player, perm *Permanent) bool` — wraps with event emission (EventSacrificed)
- **File:** `pkg/game/events.go`
- Add `EventSacrificed` event type

### 5.2 Controller Change (CR 109.5)
- **File:** `pkg/game/permanent.go`
- Add `ChangeController(newController *Player)` — used by Control-changing Layer 2 effects

### 5.3 Copy Spell (CR 706)
- **File:** `pkg/game/permanent.go`
- Add `CopyPermanent(source *Permanent) *Permanent` — creates a copy
- **File:** `pkg/ability/types.go`
- Ensure CopySpell EffectType is wired through the execution engine

---

## Phase 6: Missing Effect Types (2 files)

### 6.1 Add new effect types
- **File:** `pkg/ability/types.go`
- Add: `Proliferate`, `CreateClueToken`, `CreateTreasureToken`, `GrantFlash`, `PhasingOut`

### 6.2 Wire effect execution
- **File:** `pkg/ability/engine.go`
- Add execution cases for:
  - `Proliferate` — add 1 counter of each type to target
  - `CreateTreasureToken` — create a Treasure artifact token with "{T}, Sacrifice this artifact: Add one mana of any color"
  - `CreateClueToken` — create a Clue artifact with "{2}, Sacrifice this artifact: Draw a card"
  - `GrantFlash` — add Flash keyword to target until EOT or permanent
  - `PhasingOut` — phase out target permanent

### 6.3 Add Treasure/Clue token SimpleCard templates
- **File:** `pkg/game/simple_card.go`
- Add `TreasureToken() SimpleCard` and `ClueToken() SimpleCard` factory functions

---

## Phase 7: Comprehensive Unit Tests (~80+ tests)

### 7.1 Fix existing build break
- **File:** `pkg/ability/stack_test.go`
- Add `containsSubstring` helper

### 7.2 Parser validation tests (20+ tests)
- **New file:** `pkg/ability/parser_cedh_test.go`
- Test parsing of cEDH staple cards:
  - **Counterspells:** Force of Will, Force of Negation, Swan Song, Fierce Guardianship, Mana Drain, Pact of Negation, Dispel, Flusterstorm
  - **Tutors:** Demonic Tutor, Vampiric Tutor, Mystical Tutor, Imperial Seal, Enlightened Tutor, Worldly Tutor, Sylvan Library
  - **Fast Mana:** Mana Crypt, Sol Ring, Jeweled Lotus, Chrome Mox, Mox Diamond, Lotus Petal, Grim Monolith, Basalt Monolith, Mana Vault
  - **Win Conditions:** Thassa's Oracle (ETB trigger), Demonic Consultation (exile library), Underworld Breach (escape), Dualcaster Mage (copy)
  - **Stax:** Rule of Law, Drannith Magistrate, Collector Ouphe, Cursed Totem, Aven Mindcensor, Void Mirror
  - **Card Draw:** Ad Nauseam, Necropotence, Rhystic Study, Mystic Remora
  - **Combo pieces:** Worldgorger Dragon,Animate Dead, Lion's Eye Diamond, Brain Freeze, Brainstorm, Demonic Consultation + Thassa's Oracle
- Each test: provide oracle text → verify parsed AbilityType, EffectType, TriggerCondition, targets

### 7.3 Game engine rule tests (20+ tests)
- **New file:** `pkg/game/game_engine_test.go`
- Test phase transitions: untap auto-untap, draw step auto-draw, cleanup clears damage
- Test SBA loop: +1/+1 and -1/-1 cancellation, planeswalker death at 0 loyalty
- Test sacrifice: fires events, moves to graveyard
- Test controller change: permanent changes controller
- Test flash timing: can cast creature at instant speed
- Test ward: opponent targeting triggers ward ability
- Test shroud: no one can target
- Test protection: DEBT (damage prevention, targeting prevention, blocking prevention, aura prevention)
- Test discard to max hand size
- Test poison counter loss at 10

### 7.4 Combat keyword tests (15+ tests)
- **New file:** `pkg/game/combat_keywords_test.go`
- Test flash + combat interaction
- Test protection preventing blocks
- Test menace requiring 2+ blockers
- Test first strike damage step ordering
- Test double strike dealing in both steps
- Test trample over planeswalker
- Test deathtouch + trample (assign 1 damage, trample rest)

### 7.5 Effect execution tests (15+ tests)
- **New file:** `pkg/ability/effect_execution_test.go`
- Test CopySpell execution
- Test Proliferate execution
- Test Treasure token creation + mana production
- Test Clue token creation + card draw
- Test GrantFlash effect
- Test PhasingOut effect
- Test full stack resolution with counterspell interaction
- Test priority passing with ward trigger on stack
- Test multiple triggers APNAP ordering

### 7.6 Integration tests (10+ tests)
- **New file:** `pkg/game/cedh_integration_test.go`
- Test Thassa's Oracle + Demonic Consultation win (full game loop)
- Test Ad Nauseam + high CMC draw
- Test stax piece (Rule of Law) limiting spells per turn
- Test commander tax accumulation
- Test full EDH game loop (4 players, 3+ turns)
- Test graveyard recursion (Underworld Breach + LED + Brain Freeze)

---

## File Change Summary

| File | Action | Changes |
|------|--------|---------|
| `pkg/ability/stack_test.go` | Fix | Add `containsSubstring` helper |
| `pkg/game/keywords.go` | Modify | Add Flash, Shroud, Ward, Protection keywords; update parser |
| `pkg/game/events.go` | Modify | Add phase events, Sacrificed event, controller change event |
| `pkg/game/game.go` | Modify | Add phase automation in AdvancePhase, SBA loop |
| `pkg/game/player.go` | Modify | Add sacrifice, discard-to-max, poison counters, max hand size |
| `pkg/game/sba.go` | Modify | Add counter cancellation, planeswalker death, poison loss, SBA loop |
| `pkg/game/permanent.go` | Modify | Add ward cost, protection, controller change, copy, flash helper |
| `pkg/game/combat.go` | Modify | Protection blocking prevention, menace 2+ blocker check |
| `pkg/game/transitions.go` | Modify | Add Sacrifice wrapper with event emission |
| `pkg/game/simple_card.go` | Modify | Add Treasure/Clue token factories, IsBattle |
| `pkg/ability/types.go` | Modify | Add Proliferate, CreateClueToken, CreateTreasureToken, GrantFlash, PhasingOut |
| `pkg/ability/engine.go` | Modify | Wire execution for new effect types |
| `pkg/ability/priority.go` | Modify | Flash timing check |
| `pkg/ability/targeting.go` | Modify | Ward, shroud, protection enforcement |
| `pkg/ability/spell_casting.go` | Modify | Flash timing in sorcery check |
| `pkg/ability/parser_cedh_test.go` | New | 20+ parser validation tests |
| `pkg/game/game_engine_test.go` | New | 20+ engine rule tests |
| `pkg/game/combat_keywords_test.go` | New | 15+ combat keyword tests |
| `pkg/ability/effect_execution_test.go` | New | 15+ effect execution tests |
| `pkg/game/cedh_integration_test.go` | New | 10+ integration tests |

**Total: 15 files modified, 5 new test files, ~80+ new tests**

---

## Execution Order
1. Phase 1 (foundation) — must come first
2. Phase 2 (phase automation) — depends on Phase 1 events
3. Phase 3 (SBAs) — depends on Phase 1
4. Phase 4 (keywords) — depends on Phase 1
5. Phase 5 (game actions) — depends on Phase 1 events
6. Phase 6 (effect types) — depends on Phase 4
7. Phase 7 (tests) — validates everything above
