package logger

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected game.LogLevel
	}{
		{"META", game.META},
		{"GAME", game.GAME},
		{"PLAYER", game.PLAYER},
		{"CARD", game.CARD},
		{"invalid", game.CARD}, // default case
		{"", game.CARD},        // default case
	}

	for _, test := range tests {
		result := ParseLogLevel(test.input)
		if result != test.expected {
			t.Errorf("ParseLogLevel(%s) = %d; expected %d", test.input, result, test.expected)
		}
	}
}

func TestSetLogLevel(t *testing.T) {
	// Test setting different log levels
	originalLevel := currentLogLevel
	defer func() {
		currentLogLevel = originalLevel // restore original level
	}()

	SetLogLevel(game.META)
	if currentLogLevel != game.META {
		t.Errorf("Expected log level to be META, got %d", currentLogLevel)
	}

	SetLogLevel(game.PLAYER)
	if currentLogLevel != game.PLAYER {
		t.Errorf("Expected log level to be PLAYER, got %d", currentLogLevel)
	}
}

func TestLoggingFunctions(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	originalLogger := logger.logger
	logger.logger = log.New(&buf, "", 0) // No timestamp for testing
	defer func() {
		logger.logger = originalLogger // restore original logger
	}()

	// Test with CARD level (should log everything)
	SetLogLevel(game.CARD)
	buf.Reset()

	LogMeta("Meta message")
	LogGame("Game message")
	LogPlayer("Player message")
	LogCard("Card message")
	LogDeck("Deck message")

	output := buf.String()
	expectedMessages := []string{
		"META: Meta message",
		"GAME: Game message",
		"PLAYER: Player message",
		"CARD: Card message",
		"DECK: Deck message",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain '%s', got: %s", expected, output)
		}
	}

	// Test with GAME level (should only log META and GAME)
	SetLogLevel(game.GAME)
	buf.Reset()

	LogMeta("Meta message 2")
	LogGame("Game message 2")
	LogPlayer("Player message 2")
	LogCard("Card message 2")

	output = buf.String()
	
	if !strings.Contains(output, "META: Meta message 2") {
		t.Errorf("Expected META message to be logged at GAME level")
	}
	if !strings.Contains(output, "GAME: Game message 2") {
		t.Errorf("Expected GAME message to be logged at GAME level")
	}
	if strings.Contains(output, "PLAYER: Player message 2") {
		t.Errorf("Expected PLAYER message NOT to be logged at GAME level")
	}
	if strings.Contains(output, "CARD: Card message 2") {
		t.Errorf("Expected CARD message NOT to be logged at GAME level")
	}

	// Test with META level (should only log META)
	SetLogLevel(game.META)
	buf.Reset()

	LogMeta("Meta message 3")
	LogGame("Game message 3")
	LogPlayer("Player message 3")

	output = buf.String()
	
	if !strings.Contains(output, "META: Meta message 3") {
		t.Errorf("Expected META message to be logged at META level")
	}
	if strings.Contains(output, "GAME: Game message 3") {
		t.Errorf("Expected GAME message NOT to be logged at META level")
	}
	if strings.Contains(output, "PLAYER: Player message 3") {
		t.Errorf("Expected PLAYER message NOT to be logged at META level")
	}
}

func TestLoggingWithFormatting(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	originalLogger := logger.logger
	logger.logger = log.New(&buf, "", 0) // No timestamp for testing
	defer func() {
		logger.logger = originalLogger // restore original logger
	}()

	SetLogLevel(game.CARD)
	buf.Reset()

	LogGame("Player %s has %d life", "Alice", 20)
	LogCard("Drawing card: %s", "Lightning Bolt")

	output := buf.String()
	
	if !strings.Contains(output, "GAME: Player Alice has 20 life") {
		t.Errorf("Expected formatted GAME message, got: %s", output)
	}
	if !strings.Contains(output, "CARD: Drawing card: Lightning Bolt") {
		t.Errorf("Expected formatted CARD message, got: %s", output)
	}
}
