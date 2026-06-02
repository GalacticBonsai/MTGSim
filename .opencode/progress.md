# MTGSim Progress Tracker

## #1 Full Priority System — ✅ DONE (PR #53)
- **Branch:** `priority-system`
- **PR:** https://github.com/GalacticBonsai/MTGSim/pull/53
- **Files:**
  - `pkg/simulation/priority_handler.go` (new)
  - `pkg/simulation/edh_runner.go` (modified)
- **What:** Replaced NoopPriorityHandler with StackAwareHandler that runs APNAP priority rounds. Opponents activate abilities and cast instants during priority windows using the AI decision-maker.

## #2 The Stack — Full implementation with targets, modes, resolution
- **Status:** PENDING
- **Branch:** TBD
- **What:** LIFO spell resolution, countermagic targeting stack items, triggered ability stacking, priority rounds that loop until stack empty.

## #3 Instant-speed interaction
- **Status:** PENDING (partially done in #1)
- **Branch:** TBD
- **What:** Opponents hold priority, respond to each other's spells, cast protection spells.

## #4 Countermagic AI
- **Status:** PENDING
- **Branch:** TBD
- **What:** Use CounterspellStrategy to decide which spells to counter during priority.

## #5 Deep threat assessment
- **Status:** PENDING
- **Branch:** TBD
- **What:** Score opponent boards for combo proximity, commander danger, card advantage engines.

## #6 Combat tricks
- **Status:** PENDING

## #7 Blocks
- **Status:** PENDING

... (continuing from README todo list)
