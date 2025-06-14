# VS Code Configuration for MTGSim

This directory contains VS Code configuration files to enhance your development experience with the MTGSim project.

## Files Overview

### `settings.json`
Project-specific VS Code settings that configure:
- Go language server settings
- Code formatting and linting
- File exclusions for build artifacts
- Test configuration
- Coverage settings

### `launch.json`
Debug configurations for:
- **Launch MTGSim**: Run the main application with sample parameters
- **Launch MTGSim (Welcome Decks)**: Test with welcome deck collection
- **Launch MTGSim (Many Games)**: Performance testing with 1000 games
- **Debug Current Test**: Debug a specific test function
- **Debug All Tests**: Debug all tests in the project
- **Launch Deck Generator**: Run the deck generation utility

### `tasks.json`
Build and test tasks:
- **Build MTGSim**: Compile the main application
- **Test All**: Run all tests with verbose output
- **Test with Coverage**: Run tests and generate coverage report
- **Run MTGSim (Quick Test)**: Build and run a quick simulation
- **Run MTGSim (Performance Test)**: Build and run performance testing
- **Clean Build Artifacts**: Remove compiled binaries and test files
- **Go Mod Tidy**: Clean up module dependencies
- **Generate Decks**: Run the deck generation tool

### `extensions.json`
Recommended VS Code extensions:
- **golang.go**: Official Go extension
- **ms-vscode.vscode-json**: JSON language support
- **redhat.vscode-yaml**: YAML language support
- **formulahendry.code-runner**: Quick code execution
- **streetsidesoftware.code-spell-checker**: Spell checking
- **ms-vscode.vscode-markdown**: Markdown support
- **davidanson.vscode-markdownlint**: Markdown linting

## Usage

### Quick Start
1. Open the project in VS Code
2. Install recommended extensions when prompted
3. Use `Ctrl+Shift+P` (or `Cmd+Shift+P` on Mac) to open the command palette
4. Type "Tasks: Run Task" to see available build/test tasks

### Debugging
1. Set breakpoints in your code
2. Press `F5` or go to Run and Debug view
3. Select a debug configuration from the dropdown
4. Click the green play button or press `F5`

### Running Tests
- **All tests**: `Ctrl+Shift+P` → "Tasks: Run Task" → "Test All"
- **Current file tests**: Use the Go extension's test runner
- **With coverage**: `Ctrl+Shift+P` → "Tasks: Run Task" → "Test with Coverage"

### Building
- **Quick build**: `Ctrl+Shift+B` (default build task)
- **Custom build**: `Ctrl+Shift+P` → "Tasks: Run Task" → select build option

### Code Quality
The configuration automatically:
- Formats code on save using `goimports`
- Organizes imports
- Runs linting and vetting
- Excludes build artifacts from file explorer

## Keyboard Shortcuts

| Action | Shortcut |
|--------|----------|
| Build (default task) | `Ctrl+Shift+B` |
| Run Task | `Ctrl+Shift+P` → "Tasks: Run Task" |
| Start Debugging | `F5` |
| Run Without Debugging | `Ctrl+F5` |
| Toggle Breakpoint | `F9` |
| Step Over | `F10` |
| Step Into | `F11` |
| Step Out | `Shift+F11` |

## Customization

You can customize these configurations by:
1. Modifying the JSON files directly
2. Using VS Code's settings UI (`Ctrl+,`)
3. Adding workspace-specific settings

### Adding New Debug Configurations
Edit `launch.json` and add new configurations following the existing pattern:

```json
{
    "name": "My Custom Debug",
    "type": "go",
    "request": "launch",
    "mode": "auto",
    "program": "${workspaceFolder}/cmd/mtgsim",
    "args": ["-games=50", "-decks=my-decks"],
    "env": {},
    "showLog": true
}
```

### Adding New Tasks
Edit `tasks.json` and add new tasks:

```json
{
    "label": "My Custom Task",
    "type": "shell",
    "command": "go",
    "args": ["run", "./my-tool"],
    "group": "build"
}
```

## Troubleshooting

### Go Extension Issues
- Ensure Go is installed and in your PATH
- Run "Go: Install/Update Tools" from the command palette
- Check the Go extension output panel for errors

### Debug Issues
- Verify the program path in launch configurations
- Check that the project builds successfully
- Ensure breakpoints are set in reachable code

### Test Issues
- Make sure all dependencies are installed (`go mod tidy`)
- Check test file naming follows Go conventions (`*_test.go`)
- Verify test functions start with `Test`

For more help, see the [Go extension documentation](https://github.com/golang/vscode-go/blob/master/README.md).
