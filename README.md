# MTGSim — cEDH Deck Optimizer & Simulator

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

> A multiplayer Commander simulator purpose-built to discover the best cEDH deck. Import lists, simulate thousands of high-fidelity pods, and let data — not theory — tell you which cards win.

## Why this exists

Deckbuilding for cEDH has always been driven by intuition, heuristics, and collective table experience. MTGSim replaces guesswork with evidence:

- Simulate 50+ concurrent pods using real cEDH decklists
- Track every card's win rate, cast count, and performance across thousands of games
- Get data-driven cut/add recommendations for any deck
- Test sideboard variants, meta adjustments, and new brews at scale

This isn't a casual EDH simulator. Multiplayer pods, 40 life, commander tax, 21-damage SBA, London Mulligans, storm counts, mana tracking, turn-limited combo detection — every mechanic that matters in competitive Commander is accounted for.

## How it works

```
Import decks → Validate vs Scryfall oracle → Simulate pods with character-level AI → Collect per-card stats → Recommend cuts/adds → Repeat
```

### Three commands, three levels of fidelity

| Command | What it does | Best for |
|---|---|---|
| `mtgsim` | 1v1 simulator with stats, sideboards, and confidence intervals | Smoke-testing and comparing tuned lists |
| `mtgsim-edh` | Multiplayer Commander pods (2–6 players) | EDH meta analysis and deck optimization |
| `mtgsim-dashboard` | Live web dashboard with per-card stats and deck recommendations | Real-time analysis |

## Core features

### Simulation engine
- **Full multiplayer EDH** — 2–6 players, 40 life, command zone, commander tax, 21-damage SBA, APNAP ordering
- **cEDH mulligan AI** — Hand evaluation based on competitive mulligan framework (Sperling 2023): prioritizes truly broken openers (T1 Remora, Sol Ring + 2 lands, Ad Naus with mana), aggressive default-to-mulligan
- **Storm tracking** — records storm count for combo detection
- **Threat-based combat** — attack targeting uses board-state scoring; AI prioritizes dangerous commanders and open boards
- **Ability engine** — 250+ regex patterns parse oracle text into structured abilities (mana, triggered, activated, static)
- **Combo detection** — oracle-consultation, godo-helm, and extensible combo finish framework
- **EDH event log** — structured JSON per-pod replay (lands, summons, combat, commander casts, eliminations)
- **ETB triggers** — enter-the-battlefield effects resolve automatically
- **Mana production** — auto-tap lands, rocks, and dorks for main-phase casting
- **Search/tutor resolution** — Grim Monolith, Urborg/Vesuva effects, and search routines

### Deck optimization
- **Per-card win-rate tracking** — every card records casts and wins across thousands of games
- **Cut/add recommendations** — the dashboard highlights underperformers and suggests upgrades from the global card library
- **Sideboard variant generation** — automatically generate variants by swapping configurable numbers of sideboard cards
- **Uploaded deck comparison** — upload your list, compare its performance against the meta
- **Persistent card library** — aggregate stats survive restarts (`card_library.json`), reset on demand via API

### Dashboard
- **Dark-themed HTML UI** with auto-refresh
- **Per-deck rankings** with win rates and game counts
- **Per-card stats** — global win rate, cast counts, recommendations
- **Uploaded deck management** — upload, analyze, remove
- **Game log** — browse recent pods with player details
- **EDH telemetry** — average turns, storm counts, mana spent, combat damage, eliminations
- **Reset controls** — clear card library or game logs from the UI
- **JSON API** — all data accessible programmatically

### Deck import
- **Multiple formats** — plain lists, Cockatrice `.dck`, sectioned exports (Moxfield etc.)
- **Commander detection** — infers command zones from section headers, inline annotations, sideboard conventions
- **Color identity validation** — validates every card against commander(s) using Scryfall data
- **Recursive directory scanning** — import hundreds of decks at once

### Infrastructure
- **PostgreSQL persistence** — optional DSN-backed durable storage for deployments
- **Docker support** — containerized deployment with Postgres
- **Scryfall integration** — automatic card database download, image enrichment
- **Replay export** — per-pod structured JSON for external analysis
- **Performance** — 105x optimized (13.6ms per game), 14M→64K allocations per game via singleton regex compilation

## Quick start

```sh
git clone https://github.com/mtgsim/mtgsim.git
cd MTGSim
```

On first run the simulator downloads the Scryfall oracle snapshot and caches it at `.cache/cardDB.json`.

### Run a cEDH meta simulation

```sh
go run ./cmd/mtgsim-edh -games=200 -pod=4 -decks=decks/edh -port=8080
```

### Launch the dashboard

```sh
go run ./cmd/mtgsim-edh -games=0 -decks=decks/edh -port=8080
# Open http://localhost:8080
```

### Upload your deck for analysis

1. Start the dashboard with an EDH deck directory
2. Go to the "My Deck" tab
3. Name your deck, paste your list
4. Click "Save & Analyze" — the runner will include your deck in future pods

## Requirements

- Go 1.25+
- Internet access **only** if the card database is missing (one-time Scryfall download)
- PostgreSQL (optional, for persistent storage)

## Repository layout

```
MTGSim/
├── cmd/
│   ├── mtgsim/            # 1v1 batch simulator
│   ├── mtgsim-dashboard/  # Dashboard server with 1v1 runner
│   └── mtgsim-edh/        # EDH pod simulator + dashboard
├── pkg/
│   ├── ability/           # Oracle text parser, stack, targeting, effects engine
│   ├── bridge/            # Game state adapters for ability/AI systems
│   ├── card/              # Card models, mana parsing, Scryfall database
│   ├── dashboard/         # HTTP server, HTML UI, JSON API
│   ├── database/          # PostgreSQL persistence and queries
│   ├── deck/              # Deck import, commander validation
│   ├── game/              # Core rules engine (zones, phases, combat, triggers)
│   ├── simulation/        # 1v1 / EDH runners, mulligan AI, results, combos
│   ├── combo/             # Combo detection framework
│   └── stats/             # Card library and global stats
├── internal/logger/       # Structured logging
├── decks/                 # Sample cEDH decklists
├── .cache/cardDB.json     # Scryfall oracle cache (auto-downloaded)
└── meta/                  # Deck generation utilities
```

## Command reference

### `mtgsim-edh` flags

| Flag | Default | Description |
|---|---|---|
| `-games` | `0` | Number of pods (0 = continuous mode) |
| `-pod` | `4` | Players per pod (2–6) |
| `-decks` | `decks` | Deck directory (recursive) |
| `-max-turns` | `50` | Hard turn limit |
| `-port` | `8080` | Dashboard port (0 = no dashboard) |
| `-keep-alive` | `true` | Keep server running after batch |
| `-seed` | `0` | RNG seed (0 = time-based) |
| `-replay` | `` | Replay output directory |
| `-db` | `` | PostgreSQL DSN |
| `-card-stats` | `card_library.json` | Persistent card stats file |
| `-sideboard-variants` | `0` | Variants per deck |
| `-sideboard-swaps` | `3` | Cards swapped per variant |
| `-mulligans` | `0` | Force mulligan count (0 = AI) |

## API endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/` | GET | Dashboard UI |
| `/api/health` | GET | Health check |
| `/api/edh-results` | GET | Per-deck EDH stats |
| `/api/edh-summary` | GET | Global EDH aggregates |
| `/api/edh-games` | GET | Recent pod records |
| `/api/card-library` | GET | Global per-card stats |
| `/api/card-db` | GET | Card metadata from Scryfall |
| `/api/run-games` | POST | Trigger additional simulations |
| `/api/game-status` | GET | Running/polling state |
| `/api/uploaded-decks` | GET/POST/DELETE | Uploaded deck management |
| `/api/reset-card-library` | POST | Clear card stats |
| `/api/reset-game-logs` | POST | Clear game logs |
| `/api/deck-analysis` | GET | Cut/add recommendations for a deck |

---

## Todo

The following is the complete roadmap to make this the best cEDH deck construction and simulation application ever built.

### Rules engine completeness

- [ ] **Full priority system** — pass priority around the table in APNAP order; players get priority in every phase/step
- [ ] **The Stack** — full implementation with targets, modes, split-second, and triggered ability stacking
- [ ] **Instant-speed interaction** — AI holds priority and casts instants (removal, counterspells, protection) during opponents' turns
- [ ] **Combat tricks** — AI casts pump spells, combat tricks, and untap effects during combat
- [ ] **Blocks** — AI evaluates block decisions (trade, chump, double-block)
- [ ] **Mana burn / mana pool emptying** — CR 500.4: mana pool empties between steps/phases
- [ ] **Phasing** — implement phasing rules for cards like Teferi's Protection
- [ ] **MDFCs / modal double-faced cards** — both faces with correct mana costs and type lines
- [ ] **Split cards, fuse, aftermath** — multi-sided spell handling
- [ ] **Adventure, Bestow** — creature vs. non-creature spell modality
- [ ] **Cascade, discover, suspend** — alternative casting methods
- [ ] **Foretell, morph, manifest** — facedown card mechanics
- [ ] **Dice rolling, coin flipping** — for cards like Delina, Wild Mage
- [ ] **Monarch, Initiative** — tracking who is the monarch / has the initiative
- [ ] **City's Blessing** — tracking permanent permanents you control
- [ ] **Day/night cycle** — werewolf/curse tracking
- [ ] **Descend, delirium, threshold, hellbent** — graveyard/hand-count thresholds
- [ ] **Devotion** — counting mana symbols on permanents
- [ ] **Domain, basic land count** — land type and basic count checks

### Ability parsing and effects

- [ ] **All triggered abilities** — `whenever`, `at`, `when` for every event type
- [ ] **Replacement effects** — `if you would`, `instead` patterns
- [ ] **Cost reduction / increasing** — `spells you cast cost {1} less`
- [ ] **Mana cost modification** — `this spell costs {X} more to cast for each`
- [ ] **Protection from X** — protection from colors/creature types
- [ ] **Hexproof, shroud, ward** — targeting restrictions
- [ ] **Indestructible, deathtouch, lifelink, infect** — combat keyword effects
- [ ] **First strike, double strike, trample** — combat damage step ordering
- [ ] **Haste** — summoning sickness tracking
- [ ] **Flash** — timing restriction modification
- [ ] **Vigilance, menace, fear, intimidate** — attack/block restrictions
- [ ] **Prowess, heroic** — spell-based triggers
- [ ] **Delirium, threshold** — condition-based triggers
- [ ] **Leaves-the-battlefield triggers** — `when ~ leaves the battlefield`
- [ ] **Dies triggers** — `when ~ dies`
- [ ] **Graveyard recursion** — `return target card from graveyard to battlefield`
- [ ] **Exile interaction** — `exile ~, then you may cast it`
- [ ] **Token creation** — with correct subtypes, colors, and abilities
- [ ] **Copy effects** — `copy target permanent`, `copy target spell`
- [ ] **Flicker / blink** — `exile target creature, then return it to the battlefield`
- [ ] **Clones** — `you may have ~ enter as a copy of`
- [ ] **Theft / control change** — `gain control of target creature`
- [ ] **Phase-out** — `phase out target permanent`
- [ ] **Tutor effects** — `search your library for a card` (with all restriction types)
- [ ] **Wheel effects** — `each player discards their hand, then draws seven cards`
- [ ] **Mass discard / draw** — windfall, peer into the abyss patterns
- [ ] **Alternative casting costs** — `you may cast ~ without paying its mana cost`
- [ ] **Miracle, entwine, kicker** — modal cost payment
- [ ] **Buyback, flashback, unearth** — alternate zone casting
- [ ] **Convoke, delve, affinity** — cost reduction mechanics
- [ ] **Living weapon, modular** — enters-the-battlefield counter placement
- [ ] **Sagas** — chapter counter tracking and lore counter triggers
- [ ] **Classes (Dungeons & Dragons)** — level-up tracking
- [ ] **Contraptions, attractions** — Un-sets if desired
- [ ] **Companion** — deckbuilding restriction checking
- [ ] **Partner, Background** — multiple commander handling
- [ ] **Friends forever, doctor's companion** — special partner variants

### AI and decision-making

- [ ] **Full optimal mulligan** — per-deck hand evaluation using historical performance data
- [ ] **Mulligan simulation** — before deciding, simulate N random hands and pick the best one
- [ ] **Deep threat assessment** — score each opponent's board for: combo proximity, commander danger, card advantage engines
- [ ] **Kingmaking avoidance** — distribute attacks/removal evenly; prioritize the player most likely to win
- [ ] **Combo detection and prioritization** — recognize when the AI's hand contains a deterministic win, cast it
- [ ] **Stax piece sequencing** — play Rule of Law, Drannith Magistrate, Rest in Peace at optimal times
- [ ] **Countermagic AI** — decide which spells to counter based on known decklists, storm count, and win condition proximity
- [ ] **Removal targeting** — prioritize mana-positive rocks over mana-negative ones, engines over threats
- [ ] **Graveyard hate timing** — Surgical Extraction / Faerie Macabre at the right moment (response to tutor, response to breach)
- [ ] **Fetch land optimization** — decide which land to fetch based on current hand and commander colors
- [ ] **Brainstorm / topdeck manipulation** — optimal putting-back decisions
- [ ] **Rhystic Study / Mystic Remora payment AI** — decide whether opponents pay the {1} or let them draw
- [ ] **Opponent hand estimation** — infer bombs and interaction from known information (tutors, revealed cards)
- [ ] **Politic / table talk simulation** — who to attack, who to save interaction for
- [ ] **Learning / reinforcement learning** — train AI across thousands of games to improve decisions
- [ ] **Opening hand selection tuning** — automatically tune the mulligan AI per-deck using genetic algorithms

### Deck construction and optimization

- [ ] **Genetic deck optimizer** — mutate cards in/out, simulate pods, select for win rate, repeat across generations
- [ ] **Curve analysis** — interactive mana curve with simulation of color-screw probability
- [ ] **Color-fixing analysis** — land count + color source calculator (chrome mox, rainbow lands, basics)
- [ ] **Mana base solver** — given a deck's color requirements, output optimal land count and composition
- [ ] **Auto-sideboarding** — given a meta snapshot, generate optimal 15-card sideboard
- [ ] **Meta analysis** — cluster decks by archetype, compute which cards over/underperform against each archetype
- [ ] **Card synergy scoring** — compute pairwise synergy (e.g., Underworld Breach + Lion's Eye Diamond)
- [ ] **Deck similarity** — for any two decks, compute Jaccard similarity and shared strategies
- [ ] **Budget mode** — filter recommendations by budget constraints
- [ ] **Tournament meta import** — scrape or import decklists from MTGO/MTGTop8/EDHTop16
- [ ] **Custom card evaluation rules** — user-defined card weights and bans
- [ ] **Multi-archetype comparison** — import N decks in the same archetype, identify the highest-variance flex slots
- [ ] **Cut/add confidence** — highlight the statistical significance of each recommendation (p-value, confidence interval)

### Dashboard and visualization

- [ ] **Interactive card browser** — search, filter by color/CMC/type, view performance sparklines
- [ ] **Heatmap: card vs. archetype** — win rate of each card against each archetype in the meta
- [ ] **Time-series charts** — win rate over simulation batches (track meta shifts)
- [ ] **Combo graph visualization** — network diagram of known combos with piece frequency
- [ ] **Mana curve chart** — per-deck histogram overlayed with win-rate per CMC
- [ ] **Player dashboard** — track your uploaded decks, personal stats, leaderboard
- [ ] **Export analysis to PDF/CSV** — downloadable reports
- [ ] **Per-deck matchup matrix** — heatmap of deck A vs. deck B win rates
- [ ] **Event timeline viewer** — replay browser: step through each turn of a pod with board state snapshots
- [ ] **Game replay viewer** — visual replay of any recorded pod (turn-by-turn)
- [ ] **Leaderboard** — ranked decklist display with filters (time period, pod size, minimum games)
- [ ] **Card image integration** — show Scryfall card images on hover in all tables

### Data infrastructure

- [ ] **Persistent PostgreSQL schema migrations** — versioned, repeatable, rollback-capable
- [ ] **Tournament mode** — run N pods with M decks, produce a tournament bracket
- [ ] **Real-time streaming** — WebSocket push for live game results without polling
- [ ] **REST API documentation** — OpenAPI/Swagger spec for all endpoints
- [ ] **Authentication** — user accounts for multi-user deployments
- [ ] **Rate limiting and caching** — optimize for high-throughput deployments
- [ ] **Export API** — bulk export of all data in JSON/CSV/Parquet
- [ ] **Database indexing** — optimize the most common query patterns
- [ ] **Backup/restore** — automatic database snapshots

### Performance and scalability

- [ ] **Distributed simulation** — run workers across multiple machines, aggregate results
- [ ] **Lazy card parsing** — parse oracle text only when a card enters a relevant zone
- [ ] **Ability caching** — memoize parsed abilities per card name (not per card instance)
- [ ] **Game state delta compression** — only serialize state changes in replays
- [ ] **Adaptive turn limit** — stop pods when no player can win (board state is stalled)
- [ ] **Early game termination** — detect deterministic wins and end the pod immediately
- [ ] **Profile-guided optimization** — PGO builds for the simulation binary
- [ ] **SIMD mana pool operations** — batch mana payment for large boards

### Testing and validation

- [ ] **Rules regression tests** — for every implemented mechanic, a test that it interacts correctly with the stack, layers, and timing
- [ ] **Card-specific integration tests** — for the top 500 cEDH staples, a test that their oracle text parses and resolves correctly
- [ ] **Chaos mode** — run games with random actions and assert no crashes (fuzz testing)
- [ ] **Determinism test** — same seed → same game state (every step)
- [ ] **End-to-end simulation tests** — simulate N pods, compare aggregate stats to known good baselines
- [ ] **CI benchmark tracking** — track simulation speed over time, alert on regressions

### cEDH-specific features

- [ ] **Tournament-ready results** — produce bracket-busting predictions for actual cEDH tournaments
- [ ] **Flash / other banned-list awareness** — check legality against current ban lists
- [ ] **Custom ban list support** — allow user-defined ban lists for playgroup testing
- [ ] **Proxies** — untracked card library vs. real collection tracking
- [ ] **Deck weight analysis** — compute "how tuned" a deck is based on staple inclusion rates
- [ ] **Brew scorer** — given a novel decklist, predict its win rate within 5% based on similarity to known archetypes
- [ ] **cEDH metagame clock** — identify which archetypes are under/overrepresented and whether the meta is healthy
- [ ] **Rule zero / house rule support** — partial mulligan, extra starting life, etc.

### Quality of life

- [ ] **Prebuilt binary releases** — GitHub Actions publishing for Windows/macOS/Linux
- [ ] **Docker Compose** — one-command deployment with Postgres + dashboard
- [ ] **CLI interactive mode** — TUI for selecting decks, configuring runs, viewing results
- [ ] **Desktop app** — Electron or Tauri wrapper for the dashboard
- [ ] **Mobile-responsive dashboard** — make the UI usable on phones/tablets
- [ ] **Notifications** — email/Slack/Discord webhook when a simulation batch completes
- [ ] **Progress bar** — real-time simulation progress with estimated time remaining
- [ ] **Deck editor** — in-browser deck list editing with autocomplete and validation
