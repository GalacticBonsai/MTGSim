// Package ability provides comprehensive validation for welcome deck abilities.
package ability

import (
	"fmt"
	"strings"

	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/deck"
)

// WelcomeValidator validates abilities for all cards in welcome decks.
type WelcomeValidator struct {
	parser          *AbilityParser
	cardDB          *card.CardDB
	validationStats ValidationStats
}

// ValidationStats tracks validation results.
type ValidationStats struct {
	TotalCards           int
	CardsWithAbilities   int
	SuccessfullyParsed   int
	FailedToParse        int
	CreatureAbilities    int
	InstantAbilities     int
	SorceryAbilities     int
	EnchantmentAbilities int
	ArtifactAbilities    int
	LandAbilities        int
	PlaneswalkerAbilities int
	ValidationErrors     []ValidationError
}

// ValidationError represents a validation error.
type ValidationError struct {
	CardName    string
	CardType    string
	OracleText  string
	ErrorType   string
	Description string
}

// CardTypeValidator interface for type-specific validation.
type CardTypeValidator interface {
	ValidateCard(card card.Card) []ValidationError
	GetSupportedTypes() []string
}

// NewWelcomeValidator creates a new welcome deck validator.
func NewWelcomeValidator(cardDB *card.CardDB) *WelcomeValidator {
	return &WelcomeValidator{
		parser:          NewAbilityParser(),
		cardDB:          cardDB,
		validationStats: ValidationStats{ValidationErrors: make([]ValidationError, 0)},
	}
}

// ValidateWelcomeDecks validates abilities for all cards in welcome decks.
func (wv *WelcomeValidator) ValidateWelcomeDecks(deckManager *deck.WelcomeDeckManager) error {
	logger.LogMeta("Starting comprehensive welcome deck ability validation")
	
	// Reset stats
	wv.validationStats = ValidationStats{ValidationErrors: make([]ValidationError, 0)}
	
	// Get all deck info
	deckInfos := deckManager.GetDeckInfo()
	
	// Track unique cards to avoid duplicate validation
	validatedCards := make(map[string]bool)
	
	for _, deckInfo := range deckInfos {
		logger.LogCard("Validating deck: %s", deckInfo.Name)
		
		// Validate main deck cards
		if err := wv.validateDeckCards(deckInfo.MainDeck.Cards, validatedCards); err != nil {
			return fmt.Errorf("failed to validate main deck %s: %v", deckInfo.Name, err)
		}
		
		// Validate sideboard cards
		if len(deckInfo.Sideboard.Cards) > 0 {
			if err := wv.validateDeckCards(deckInfo.Sideboard.Cards, validatedCards); err != nil {
				return fmt.Errorf("failed to validate sideboard %s: %v", deckInfo.Name, err)
			}
		}
	}
	
	// Print validation summary
	wv.printValidationSummary()
	
	return nil
}

// validateDeckCards validates all cards in a deck.
func (wv *WelcomeValidator) validateDeckCards(cards []card.Card, validatedCards map[string]bool) error {
	for _, cardData := range cards {
		// Skip if already validated
		if validatedCards[cardData.Name] {
			continue
		}
		validatedCards[cardData.Name] = true
		
		wv.validationStats.TotalCards++
		
		// Skip basic lands (they typically don't have abilities)
		if wv.isBasicLand(cardData) {
			continue
		}
		
		// Validate card abilities
		if err := wv.validateCard(cardData); err != nil {
			logger.LogCard("Validation error for %s: %v", cardData.Name, err)
			wv.validationStats.ValidationErrors = append(wv.validationStats.ValidationErrors, ValidationError{
				CardName:    cardData.Name,
				CardType:    cardData.TypeLine,
				OracleText:  cardData.OracleText,
				ErrorType:   "ValidationError",
				Description: err.Error(),
			})
		}
	}
	
	return nil
}

// validateCard validates a single card's abilities.
func (wv *WelcomeValidator) validateCard(cardData card.Card) error {
	// Skip cards without oracle text
	if strings.TrimSpace(cardData.OracleText) == "" {
		return nil
	}
	
	wv.validationStats.CardsWithAbilities++
	
	// Parse abilities
	abilities, err := wv.parser.ParseAbilities(cardData.OracleText, cardData)
	if err != nil {
		wv.validationStats.FailedToParse++
		return fmt.Errorf("failed to parse abilities: %v", err)
	}
	
	// Count abilities by card type
	wv.countAbilitiesByType(cardData.TypeLine, len(abilities))
	
	if len(abilities) > 0 {
		wv.validationStats.SuccessfullyParsed++
		logger.LogCard("Successfully parsed %d abilities for %s", len(abilities), cardData.Name)
		
		// Validate each ability
		for _, ability := range abilities {
			if err := wv.validateAbility(ability, cardData); err != nil {
				return fmt.Errorf("ability validation failed: %v", err)
			}
		}
	} else {
		// Card has oracle text but no abilities were parsed
		wv.validationStats.FailedToParse++
		return fmt.Errorf("no abilities parsed from oracle text: %s", cardData.OracleText)
	}
	
	return nil
}

// validateAbility validates a single ability.
func (wv *WelcomeValidator) validateAbility(ability *Ability, cardData card.Card) error {
	// Basic ability validation
	if ability.Name == "" {
		return fmt.Errorf("ability has no name")
	}
	
	if len(ability.Effects) == 0 {
		return fmt.Errorf("ability %s has no effects", ability.Name)
	}
	
	// Validate each effect
	for i, effect := range ability.Effects {
		if err := wv.validateEffect(effect, cardData); err != nil {
			return fmt.Errorf("effect %d in ability %s failed validation: %v", i, ability.Name, err)
		}
	}
	
	return nil
}

// validateEffect validates a single effect.
func (wv *WelcomeValidator) validateEffect(effect Effect, cardData card.Card) error {
	// Check that effect has a type
	if effect.Type == 0 {
		return fmt.Errorf("effect has no type")
	}
	
	// Validate effect based on type
	switch effect.Type {
	case DealDamage:
		if effect.Value <= 0 {
			return fmt.Errorf("damage effect has invalid value: %d", effect.Value)
		}
	case GainLife:
		if effect.Value <= 0 {
			return fmt.Errorf("gain life effect has invalid value: %d", effect.Value)
		}
	case DrawCards:
		if effect.Value <= 0 {
			return fmt.Errorf("draw cards effect has invalid value: %d", effect.Value)
		}
	case AddMana:
		// Mana effects should have valid mana type
		// This would need more sophisticated validation
	}
	
	return nil
}

// countAbilitiesByType counts abilities by card type.
func (wv *WelcomeValidator) countAbilitiesByType(typeLine string, abilityCount int) {
	typeLine = strings.ToLower(typeLine)
	
	if strings.Contains(typeLine, "creature") {
		wv.validationStats.CreatureAbilities += abilityCount
	}
	if strings.Contains(typeLine, "instant") {
		wv.validationStats.InstantAbilities += abilityCount
	}
	if strings.Contains(typeLine, "sorcery") {
		wv.validationStats.SorceryAbilities += abilityCount
	}
	if strings.Contains(typeLine, "enchantment") {
		wv.validationStats.EnchantmentAbilities += abilityCount
	}
	if strings.Contains(typeLine, "artifact") {
		wv.validationStats.ArtifactAbilities += abilityCount
	}
	if strings.Contains(typeLine, "land") {
		wv.validationStats.LandAbilities += abilityCount
	}
	if strings.Contains(typeLine, "planeswalker") {
		wv.validationStats.PlaneswalkerAbilities += abilityCount
	}
}

// isBasicLand checks if a card is a basic land.
func (wv *WelcomeValidator) isBasicLand(cardData card.Card) bool {
	name := strings.ToLower(cardData.Name)
	return name == "plains" || name == "island" || name == "swamp" || 
		   name == "mountain" || name == "forest"
}

// printValidationSummary prints a summary of validation results.
func (wv *WelcomeValidator) printValidationSummary() {
	stats := wv.validationStats
	
	logger.LogMeta("=== WELCOME DECK ABILITY VALIDATION SUMMARY ===")
	logger.LogMeta("Total cards validated: %d", stats.TotalCards)
	logger.LogMeta("Cards with abilities: %d", stats.CardsWithAbilities)
	logger.LogMeta("Successfully parsed: %d", stats.SuccessfullyParsed)
	logger.LogMeta("Failed to parse: %d", stats.FailedToParse)
	
	if stats.CardsWithAbilities > 0 {
		successRate := float64(stats.SuccessfullyParsed) / float64(stats.CardsWithAbilities) * 100
		logger.LogMeta("Success rate: %.1f%%", successRate)
	}
	
	logger.LogMeta("\nAbilities by card type:")
	logger.LogMeta("  Creatures: %d", stats.CreatureAbilities)
	logger.LogMeta("  Instants: %d", stats.InstantAbilities)
	logger.LogMeta("  Sorceries: %d", stats.SorceryAbilities)
	logger.LogMeta("  Enchantments: %d", stats.EnchantmentAbilities)
	logger.LogMeta("  Artifacts: %d", stats.ArtifactAbilities)
	logger.LogMeta("  Lands: %d", stats.LandAbilities)
	logger.LogMeta("  Planeswalkers: %d", stats.PlaneswalkerAbilities)
	
	if len(stats.ValidationErrors) > 0 {
		logger.LogMeta("\nValidation errors (%d):", len(stats.ValidationErrors))
		for i, err := range stats.ValidationErrors {
			if i < 10 { // Limit to first 10 errors
				logger.LogMeta("  %s (%s): %s", err.CardName, err.CardType, err.Description)
			}
		}
		if len(stats.ValidationErrors) > 10 {
			logger.LogMeta("  ... and %d more errors", len(stats.ValidationErrors)-10)
		}
	}
}

// GetValidationStats returns the current validation statistics.
func (wv *WelcomeValidator) GetValidationStats() ValidationStats {
	return wv.validationStats
}

// GetValidationErrors returns all validation errors.
func (wv *WelcomeValidator) GetValidationErrors() []ValidationError {
	return wv.validationStats.ValidationErrors
}
