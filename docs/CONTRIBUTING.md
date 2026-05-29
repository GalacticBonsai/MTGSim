# Contributing to MTGSim

Thank you for your interest in contributing to MTGSim! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- Basic understanding of Magic: The Gathering rules (helpful but not required)

### Setting Up Development Environment

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```sh
   git clone https://github.com/yourusername/MTGSim.git
   cd MTGSim
   ```
3. Install dependencies:
   ```sh
   go mod tidy
   ```
4. Build and test:
   ```sh
   go build ./cmd/mtgsim
   go test ./...
   ```

## Code Organization

### Package Structure

- `cmd/mtgsim/`: Main application entry point
- `pkg/card/`: Card types, database, and card-related functionality
- `pkg/deck/`: Deck management and import/export
- `pkg/game/`: Core game types and constants
- `pkg/simulation/`: Simulation utilities and result tracking
- `internal/logger/`: Internal logging functionality

### Coding Standards

- Follow standard Go conventions and formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Write tests for new functionality
- Keep functions focused and small

## Making Changes

### Branching Strategy

1. Create a feature branch from main:
   ```sh
   git checkout -b feature/your-feature-name
   ```
2. Make your changes
3. Test thoroughly
4. Commit with clear, descriptive messages

### Commit Messages

Use clear, descriptive commit messages:
- `feat: add support for planeswalker cards`
- `fix: resolve deck import parsing issue`
- `docs: update README with new examples`
- `test: add unit tests for card database`

### Testing

- Write unit tests for new functionality
- Ensure all existing tests pass: `go test ./...`
- Test with different deck formats and configurations
- Verify the application builds successfully

## Types of Contributions

### Bug Reports

When reporting bugs, please include:
- Go version and operating system
- Steps to reproduce the issue
- Expected vs. actual behavior
- Relevant log output or error messages
- Sample deck files if applicable

### Feature Requests

For new features:
- Describe the use case and benefit
- Provide examples of how it would work
- Consider backward compatibility
- Discuss implementation approach if you have ideas

### Code Contributions

Areas where contributions are especially welcome:
- **Game Rules Engine**: Implementing more complete MTG rules
- **Card Abilities**: Adding support for specific card abilities
- **Deck Analysis**: Statistical analysis and deck optimization
- **Performance**: Optimizing simulation speed
- **Documentation**: Improving code documentation and examples

## Pull Request Process

1. Ensure your code follows Go conventions
2. Add or update tests as needed
3. Update documentation if you're changing behavior
4. Ensure all tests pass
5. Create a pull request with:
   - Clear title and description
   - Reference to any related issues
   - Screenshots or examples if applicable

### Review Process

- Maintainers will review your PR
- Address any feedback or requested changes
- Once approved, your PR will be merged

## Development Tips

### Working with Card Data

- The card database is automatically downloaded from Scryfall
- Use the existing `card.CardDB` interface for card lookups
- Test with various card types and edge cases

### Adding New Simulation Features

- Keep simulation logic modular and testable
- Consider performance impact for large numbers of games
- Maintain backward compatibility with existing deck formats

### Debugging

- Use the logging system with appropriate levels
- Test with different log levels to verify output
- Use Go's built-in debugging tools and profiler

## Questions and Support

- Open an issue for questions about contributing
- Check existing issues and PRs before creating new ones
- Be respectful and constructive in all interactions

## License

By contributing to MTGSim, you agree that your contributions will be licensed under the MIT License.

Thank you for contributing to MTGSim!
