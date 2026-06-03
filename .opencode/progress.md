# MTGSim Progress Tracker

## #1 Full Priority System — ✅ DONE (PR #53)
- **Branch:** `priority-system`
- **PR:** https://github.com/GalacticBonsai/MTGSim/pull/53
- **Files:** `pkg/simulation/priority_handler.go` (new), `pkg/simulation/edh_runner.go` (modified)
- **What:** Replaced NoopPriorityHandler with StackAwareHandler that runs APNAP priority rounds. Opponents activate abilities and cast instants during priority windows using AI.

## #2 The Stack — ✅ DONE (PR #54)
- **Branch:** `the-stack`
- **PR:** https://github.com/GalacticBonsai/MTGSim/pull/54
- **Files:** `pkg/simulation/priority_handler.go`, `pkg/ability/priority.go`, `pkg/ability/spell_casting.go`
- **What:** Integrated the ability package's Stack and PriorityManager into the simulation loop. ProcessPriorityRound with AI DecisionFunc, stack-backed priority rounds with LIFO resolution.

## #3 Instant-speed interaction
- **Status:** UP NEXT
- **Branch:** TBD
- **What:** Opponents hold priority, respond to each other's spells, cast protection spells in response to removal. Builds on #1/#2.

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

## #8 Mana burn / mana pool emptying
- **Status:** PENDING
