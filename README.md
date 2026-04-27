# MTGSim

MTGSim is a Magic: The Gathering (MTG) deck simulation tool that simulates MTG decks and helps analyze deck performance. The tool can import decks, simulate games, and track the performance of different decks.

## Features

- 🎴 Import decks from `.deck` files with multiple format support
- ⚔️ Simulate games between decks with configurable parameters
- 📊 Track wins and losses for each deck with detailed statistics
- 🏆 Display top-performing decks based on win percentage
- 📚 Automatic card database management with Scryfall integration
- 🌐 **Web Dashboard** with real-time metrics and visualizations
- 🧪 Comprehensive test suite with high coverage
- 📈 Statistics tool with confidence intervals
- 🎯 Modular, well-organized codebase with proper Go package structure

## Getting Started

### Prerequisites

- Go 1.21 or later
- Internet connection (for downloading card data on first run)

### Installation

1. Clone the repository:
    ```sh
    git clone https://github.com/yourusername/MTGSim.git
    cd MTGSim
    ```

2. Install dependencies:
    ```sh
    go mod tidy
    ```

3. Build the project:
    ```sh
    go build -o mtgsim ./cmd/mtgsim
    ```

4. Run the project:
    ```sh
    ./mtgsim
    ```

## Usage

### Available Tools

| Tool | Purpose | Command |
|------|---------|---------|
| `mtgsim` | Basic game simulator | `./mtgsim [options]` |
| `mtgstats` | Detailed statistics tool | `./mtgstats [options]` |
| `mtgsim-dashboard` | Web-based dashboard | `./mtgsim-dashboard [options]` |
| `stack_demo` | Stack/abilities demo | `./stack_demo` |

### Command Line Options

```sh
./mtgsim [options]
```

**Options:**
- `-games N`: Number of games to simulate (default: 1)
- `-decks DIR`: Directory containing deck files (default: "decks/1v1")
- `-log LEVEL`: Log level - META, GAME, PLAYER, CARD (default: "CARD")

### Web Dashboard

Launch the web dashboard to browse simulation metrics in real-time:

```sh
./mtgsim-dashboard -games=100 -port=8080
```

Then open http://localhost:8080 in your browser to see:
- 📊 Total games played and aggregate metrics
- 🏆 Deck win rates with sortable rankings
- ⏱️ Average game duration and turn counts
- 📜 Recent game history with details
- 🔄 Auto-refresh every 5 seconds

### Deck Format

Decks should be stored as `.deck` files. The tool supports multiple formats:

**Standard Format:**
```
4 Lightning Bolt
3 Mountain
20 Forest
```

**Extended Format with Set Information:**
```
About
Name My Awesome Deck

Deck
4x Lightning Bolt (CLB) 401
3x Mountain (DSK) 283
20x Forest (TDM) 276

Sideboard
2x Naturalize (M19) 190
```

### Examples

**Simulate 100 games:**
```sh
./mtgsim -games=100
```

**Use different deck directory:**
```sh
./mtgsim -decks=decks/welcome -games=50
```

**Enable detailed logging:**
```sh
./mtgsim -log=PLAYER -games=10
```

## Project Structure

```
MTGSim/
├── cmd/mtgsim/          # Main application
├── pkg/                 # Public packages
│   ├── card/           # Card types and database
│   ├── deck/           # Deck management
│   ├── game/           # Game types and constants
│   └── simulation/     # Simulation utilities
├── internal/logger/    # Internal logging package
├── decks/              # Deck files organized by category
│   ├── 1v1/           # Two-player decks
│   ├── welcome/       # Beginner-friendly decks
│   ├── vanilla/       # Simple creature decks
│   └── novelty/       # Special theme decks
└── meta/              # Deck generation tools
```

## Card Database

MTGSim automatically downloads and caches card data from [Scryfall](https://scryfall.com/) on first run. The database is stored locally as `cardDB.json` and contains comprehensive information about Magic: The Gathering cards.

## Development

### Running Tests

```sh
go test ./...
```

### Building for Different Platforms

```sh
# Linux
GOOS=linux GOARCH=amd64 go build -o mtgsim-linux ./cmd/mtgsim

# Windows
GOOS=windows GOARCH=amd64 go build -o mtgsim.exe ./cmd/mtgsim

# macOS
GOOS=darwin GOARCH=amd64 go build -o mtgsim-mac ./cmd/mtgsim
```

## Architecture

The codebase follows Go best practices with a clean separation of concerns:

- **cmd/**: Application entry points
- **pkg/**: Reusable packages that could be imported by other projects
- **internal/**: Private packages not intended for external use


## Rules Engine (pkg/game)

A lightweight, headless rules engine powers automated simulations without UI/networking. It implements:

- Turn/phase progression with end-of-turn cleanup
- Combat (attackers, one-to-one blocks, first/double strike timing)
- Continuous effects: Set power/toughness and temporary pumps until EOT (timestamp last-wins for SetPT)
- Triggers and watchers: event-driven actions with per-turn watcher reset
- Replacement effects: e.g., "would die → exile instead" centralized in dies handling
- Damage prevention: shield pools for players/permanents that consume before damage is applied

Key entry points:
- Game.AdvancePhase() and IsMainPhase()/IsCombatPhase()
- SummonCreature/PlayLand (emit ETB events)
- ApplySetPTUntilEOT, ApplyTempPump
- AddWouldDieExileUntilEOT, ApplyDamageToPlayer/ApplyDamageToPermanent
- AddTrigger, AddWatcher (CreatureETBWatcher provided)

Example (integrated mini-scenario):

```go
p1, p2 := game.NewPlayer("P1", 20), game.NewPlayer("P2", 20)
g := game.NewGame(p1, p2)
// ETB trigger: draw a card when your creature enters
g.AddTrigger(&game.Trigger{ On: game.EventEntersBattlefield,
  Condition: func(e game.Event) bool { return e.ZoneChange.Permanent.GetController()==p1 },
  Action: func(g *game.Game, e game.Event){ p1.Draw(1) },
})
// Summon 2/2, set 5/5 until EOT, then EOT resets
p1.AddCardToHand(game.SimpleCard{Name:"Bear", TypeLine:"Creature", Power:"2", Toughness:"2"})
perm, _ := g.SummonCreature(p1, "Bear")
g.ApplySetPTUntilEOT(perm, 5, 5)
for i := 0; i < 7; i++ { g.AdvancePhase() } // clears EOT effects
```

AI and automation:
- pkg/bridge exposes AutoActivateMainPhaseAbilities(g) to safely call the ability AI during main phases
- pkg/simulation.RunOneTurnAuto(g) advances a full turn and calls the AI hook in each main phase

## Known Limitations

- Game simulation is currently simplified (basic damage dealing)
- Full MTG rules engine is not implemented
- Advanced card interactions are not simulated

## Future Enhancements

- **Complete Rules Engine**: Implement full Magic: The Gathering rules
- **Advanced Card Interactions**: Support for complex card abilities and interactions
- **Deck Analysis**: Statistical analysis of deck composition and performance
- **Tournament Simulation**: Multi-round tournament brackets
- **Web Interface**: Browser-based deck building and simulation

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Scryfall](https://scryfall.com/) for providing comprehensive MTG card data
- The Magic: The Gathering community for inspiration and feedback
