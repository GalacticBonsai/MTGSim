# MTGSim

MTGSim is a Magic: The Gathering (MTG) deck simulation tool that uses AI to simulate MTG decks and help make decisions on card picks. The tool can import decks, simulate games, and track the performance of different decks.

## Features

- Import decks from text files.
- Simulate games between decks.
- Track wins and losses for each deck.
- Display top-performing decks based on win percentage.
- Generate random decks for simulation.

## Getting Started

### Prerequisites

- Go 1.16 or later
- Internet connection (for downloading card data)

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
    go build -o mtgsim ./src
    ```

4. Run the project:
    ```sh
    ./mtgsim
    ```

## Usage

### Importing Decks

Decks should be stored in the `decks/Generated` directory. Each deck should be a text file with the format:
```
4 Lightning Bolt
3 Mountain
...
```

### Simulating Games

The main simulation logic is in the `main.go` file. It reads decks from the `decks/Generated` directory, simulates games, and tracks the results.

### Viewing Results

Results are printed to the console, showing the top-performing decks based on win percentage.

## Project Structure

- `src/`: Contains the source code for the project.
- `decks/`: Contains the deck files used for simulation.
- `meta/`: Contains scripts for generating decks and other meta operations.

## Examples

### Simulating a Game

To simulate a game between two decks, run the following command:
```sh
./mtgsim
```
The results will be printed to the console.

### Importing a Deck

To import a deck, place a text file in the `decks/Generated` directory with the following format:
```
4 Lightning Bolt
3 Mountain
...
```

## Configuration

Currently, there are no configuration options available. Future versions may include configurable options for simulation parameters.

## Known Issues

- The tool does not yet support parsing and handling of evergreen creature abilities.
- Activated and triggered abilities are not yet implemented.
- Card effects are not fully simulated.

## Future Enhancements

- **Evergreen Creature Abilities**: Implement parsing and handling of evergreen creature abilities like Flying, Trample, etc.
- **Activated and Triggered Abilities**: Parse and simulate activated and triggered abilities of cards.
- **Card Effects**: Implement parsing and simulation of various card effects.
- **Winrate Comparison**: Compare the win rates of individual cards within different decks.

## Contributing

Contributions are welcome! Please fork the repository and submit a pull request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contact Information

For support or questions, please contact the maintainers at [your-email@example.com].

## Acknowledgments

- [Scryfall](https://scryfall.com/) for providing the card data.
