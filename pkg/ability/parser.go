// Package ability provides oracle text parsing for MTG abilities.
package ability

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/types"
)

// AbilityParser parses oracle text to extract abilities.
type AbilityParser struct {
	patterns map[AbilityType][]*AbilityPattern
}

// AbilityPattern represents a regex pattern for matching abilities.
type AbilityPattern struct {
	Regex       *regexp.Regexp
	Type        AbilityType
	EffectType  EffectType
	Description string
	Parser      func(matches []string, fullText string) (*Ability, error)
}

// NewAbilityParser creates a new ability parser with predefined patterns.
func NewAbilityParser() *AbilityParser {
	parser := &AbilityParser{
		patterns: make(map[AbilityType][]*AbilityPattern),
	}
	parser.initializePatterns()
	return parser
}

// initializePatterns sets up all the regex patterns for ability parsing.
func (ap *AbilityParser) initializePatterns() {
	// Mana abilities
	ap.addPattern(Mana, `\{T\}:\s*Add\s*\{([WUBRGC])\}`, AddMana, "Tap for mana", ap.parseManaAbility)
	ap.addPattern(Mana, `\{T\}:\s*Add\s*one\s*mana\s*of\s*any\s*color`, AddMana, "Tap for any color", ap.parseAnyColorMana)
	ap.addPattern(Mana, `\{T\}:\s*Add\s*\{C\}\{C\}`, AddMana, "Tap for colorless mana", ap.parseColorlessMana)

	// Triggered abilities - ETB effects
	ap.addPattern(Triggered, `When\s+.*\s+enters\s+the\s+battlefield,\s+draw\s+(\d+)\s+cards?`, DrawCards, "ETB draw cards", ap.parseETBDrawCards)
	ap.addPattern(Triggered, `When\s+.*\s+enters\s+the\s+battlefield,\s+draw\s+(two|three|four|five)\s+cards?`, DrawCards, "ETB draw word cards", ap.parseETBDrawWordsCards)
	ap.addPattern(Triggered, `When\s+.*\s+enters\s+the\s+battlefield,\s+draw\s+a\s+card`, DrawCards, "ETB draw a card", ap.parseETBDrawCard)

	// Life gain abilities
	ap.addPattern(Activated, `\{T\}:\s*You\s+gain\s+(\d+)\s+life`, GainLife, "Tap to gain life", ap.parseTapGainLife)
	ap.addPattern(Triggered, `When\s+.*\s+enters\s+the\s+battlefield,\s+.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "ETB deal damage", ap.parseETBDamage)
	ap.addPattern(Triggered, `When\s+.*\s+enters\s+the\s+battlefield,\s+you\s+gain\s+(\d+)\s+life`, GainLife, "ETB gain life", ap.parseETBGainLife)

	// Triggered abilities - Death triggers
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+draw\s+(\d+)\s+cards?`, DrawCards, "Death draw cards", ap.parseDeathDrawCards)
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+draw\s+a\s+card`, DrawCards, "Death draw a card", ap.parseDeathDrawCard)
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "Death deal damage", ap.parseDeathDamage)

	// Activated abilities
	ap.addPattern(Activated, `\{(\d+)\},\s*\{T\}:\s*Draw\s+(\d+)\s+cards?`, DrawCards, "Pay and tap to draw", ap.parseActivatedDraw)
	ap.addPattern(Activated, `\{(\d+)\},\s*\{T\}:\s*Draw\s+a\s+card`, DrawCards, "Pay and tap to draw a card", ap.parseActivatedDrawCard)
	ap.addPattern(Activated, `\{(\d+)\},\s*\{T\}:\s*.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "Pay and tap to deal damage", ap.parseActivatedDamage)
	ap.addPattern(Activated, `\{T\}:\s*.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "Tap to deal damage", ap.parseTapDamage)
	ap.addPattern(Activated, `\{T\}:\s*Target\s+creature\s+gets\s+\+(\d+)/\+(\d+)\s+until\s+end\s+of\s+turn`, PumpCreature, "Tap to pump creature", ap.parsePumpAbility)

	// Static abilities (these don't use the stack)
	ap.addPattern(Static, `Creatures\s+you\s+control\s+get\s+\+(\d+)/\+(\d+)`, PumpCreature, "Static pump", ap.parseStaticPump)
	ap.addPattern(Static, `Other\s+creatures\s+you\s+control\s+get\s+\+(\d+)/\+(\d+)`, PumpCreature, "Static pump others", ap.parseStaticPumpOthers)

	// Modal spells - improved patterns
	ap.addPattern(Activated, `Choose one —.*`, DrawCards, "Modal spell", ap.parseModalSpell)
	ap.addPattern(Activated, `Choose two —.*`, DrawCards, "Modal spell - choose two", ap.parseModalSpellTwo)
	ap.addPattern(Activated, `Choose three.*`, DrawCards, "Modal spell - choose three", ap.parseModalSpellThree)
	ap.addPattern(Activated, `Choose any number —.*`, DrawCards, "Modal spell - choose any", ap.parseModalSpellAny)

	// Variable X-cost abilities
	ap.addPattern(Activated, `\{X\}.*:\s*Draw\s+X\s+cards?`, DrawCards, "X-cost draw", ap.parseXCostDraw)
	ap.addPattern(Activated, `\{X\}.*:\s*.*\s+deals\s+X\s+damage\s+to\s+(.*)`, DealDamage, "X-cost damage", ap.parseXCostDamage)

	// Spell effects (for instants and sorceries)
	ap.addPattern(Activated, `.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)\.?`, DealDamage, "Spell damage", ap.parseSpellDamage)
	ap.addPattern(Activated, `^([A-Za-z\s]+)\s+deals\s+(\d+)\s+damage\s+to\s+(.*)\.?`, DealDamage, "Named spell damage", ap.parseNamedSpellDamage)
	ap.addPattern(Activated, `Draw\s+(two|three|four|five)\s+cards?\.?`, DrawCards, "Spell draw words", ap.parseSpellDrawWords)
	ap.addPattern(Activated, `Draw\s+(\d+)\s+cards?\.?`, DrawCards, "Spell draw", ap.parseSpellDraw)
	ap.addPattern(Activated, `Target\s+player\s+draws\s+(three|four|five)\s+cards?`, DrawCards, "Targeted spell draw words", ap.parseTargetedSpellDrawWords)
	ap.addPattern(Activated, `Target\s+player\s+draws\s+(\d+)\s+cards?`, DrawCards, "Targeted spell draw", ap.parseTargetedSpellDraw)
	ap.addPattern(Activated, `Destroy\s+target\s+(.*)\.?`, DestroyPermanent, "Spell destroy", ap.parseSpellDestroy)
	ap.addPattern(Activated, `Destroy\s+all\s+(.*)\.?`, DestroyPermanent, "Mass destroy", ap.parseMassDestroy)
	ap.addPattern(Activated, `Counter\s+target\s+spell\.?`, CounterSpell, "Counterspell", ap.parseCounterspell)
	ap.addPattern(Activated, `Counter\s+target\s+spell\s+unless\s+its\s+controller\s+pays\s+\{(\d+)\}`, CounterSpell, "Conditional counterspell", ap.parseConditionalCounterspell)
	ap.addPattern(Activated, `Target\s+player\s+gains\s+(\d+)\s+life`, GainLife, "Targeted life gain", ap.parseTargetedLifeGain)
	ap.addPattern(Activated, `Target\s+creature\s+gets\s+\+(\d+)/\+(\d+)\s+until\s+end\s+of\s+turn`, PumpCreature, "Spell pump", ap.parseSpellPump)

	// Additional spell effects for common cards
	ap.addPattern(Activated, `Add\s+\{([WUBRGC])\}\{([WUBRGC])\}\{([WUBRGC])\}\.?`, AddMana, "Triple mana", ap.parseTripleMana)
	ap.addPattern(Activated, `Prevent\s+all\s+combat\s+damage\s+that\s+would\s+be\s+dealt\s+this\s+turn`, PreventDamage, "Fog effect", ap.parseFogEffect)
	ap.addPattern(Activated, `Take\s+an\s+extra\s+turn\s+after\s+this\s+one`, DrawCards, "Extra turn", ap.parseExtraTurn) // Using DrawCards as placeholder
	ap.addPattern(Activated, `Create\s+(\d+)\s+(\d+)/(\d+)\s+.*\s+creature\s+tokens?`, CreateToken, "Token creation", ap.parseTokenCreation)

	// X-cost spell effects
	ap.addPattern(Activated, `.*\s+deals\s+X\s+damage\s+to\s+(.*)`, DealDamage, "X-cost spell damage", ap.parseXCostSpellDamage)
	ap.addPattern(Activated, `.*\s+deals\s+X\s+damage\s+divided\s+as\s+you\s+choose\s+among\s+(.*)`, DealDamage, "X-cost divided damage", ap.parseXCostDividedDamage)
	ap.addPattern(Activated, `Draw\s+X\s+cards?`, DrawCards, "X-cost spell draw", ap.parseXCostSpellDraw)

	// Complex spell effects
	ap.addPattern(Activated, `.*\s+deals\s+(\d+)\s+damage\s+divided\s+as\s+you\s+choose\s+among\s+(.*)`, DealDamage, "Divided damage", ap.parseDividedDamage)
	ap.addPattern(Activated, `.*\s+deals\s+X\s+damage\s+to\s+(.*)\.\s+You\s+gain\s+life\s+equal\s+to\s+the\s+damage\s+dealt`, DealDamage, "Drain life", ap.parseDrainLife)
	ap.addPattern(Activated, `.*\s+deals\s+X\s+damage\s+to\s+(.*)\s+You\s+gain\s+life\s+equal\s+to\s+the\s+damage\s+dealt`, DealDamage, "Drain life no period", ap.parseDrainLife)

	// Enhanced patterns for common failing cases

	// Simple keyword abilities (these often fail because they're just the keyword)
	ap.addPattern(Static, `^Flying$`, EvergreenAbility, "Flying keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^Deathtouch$`, EvergreenAbility, "Deathtouch keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^Haste$`, EvergreenAbility, "Haste keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^Trample$`, EvergreenAbility, "Trample keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^Reach$`, EvergreenAbility, "Reach keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^Defender$`, EvergreenAbility, "Defender keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^Vigilance$`, EvergreenAbility, "Vigilance keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^Lifelink$`, EvergreenAbility, "Lifelink keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^First\s+strike$`, EvergreenAbility, "First strike keyword", ap.parseSimpleKeyword)
	ap.addPattern(Static, `^Double\s+strike$`, EvergreenAbility, "Double strike keyword", ap.parseSimpleKeyword)

	// Multi-keyword patterns
	ap.addPattern(Static, `^Flying,\s*haste$`, EvergreenAbility, "Flying, haste", ap.parseMultiKeyword)
	ap.addPattern(Static, `^Flying,\s*vigilance$`, EvergreenAbility, "Flying, vigilance", ap.parseMultiKeyword)
	ap.addPattern(Static, `^Flying,\s*lifelink$`, EvergreenAbility, "Flying, lifelink", ap.parseMultiKeyword)

	// ETB abilities that are failing
	ap.addPattern(Triggered, `When\s+.*\s+enters,\s+draw\s+a\s+card`, DrawCards, "ETB draw simplified", ap.parseETBDrawCard)
	ap.addPattern(Triggered, `When\s+.*\s+enters,\s+you\s+gain\s+(\d+)\s+life`, GainLife, "ETB gain life simplified", ap.parseETBGainLife)
	ap.addPattern(Triggered, `When\s+.*\s+enters,\s+target\s+creature\s+you\s+control\s+gets\s+\+(\d+)/\+(\d+)\s+until\s+end\s+of\s+turn`, PumpCreature, "ETB pump", ap.parseETBPump)
	ap.addPattern(Triggered, `When\s+.*\s+enters,\s+tap\s+target\s+creature`, TapUntap, "ETB tap", ap.parseETBTap)

	// Activated abilities that are failing
	ap.addPattern(Activated, `\{(\d+)\}\{([WUBRGC])\}:\s*.*\s+gets\s+\+(\d+)/\+(\d+)\s+until\s+end\s+of\s+turn`, PumpCreature, "Activated pump", ap.parseActivatedPump)
	ap.addPattern(Activated, `\{([WUBRGC])\}:\s*.*\s+gets\s+\+(\d+)/\+(\d+)\s+until\s+end\s+of\s+turn`, PumpCreature, "Single mana pump", ap.parseSingleManaPump)

	// Enchant abilities
	ap.addPattern(Static, `Enchant\s+creature`, EvergreenAbility, "Enchant creature", ap.parseEnchantCreature)
	ap.addPattern(Static, `Enchanted\s+creature\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Enchanted pump", ap.parseEnchantedPump)
	ap.addPattern(Static, `Enchanted\s+creature\s+can't\s+attack\s+or\s+block`, EvergreenAbility, "Enchanted restriction", ap.parseEnchantedRestriction)

	// Unblockable and evasion abilities
	ap.addPattern(Static, `.*\s+can't\s+be\s+blocked`, EvergreenAbility, "Unblockable", ap.parseUnblockable)
	ap.addPattern(Static, `.*\s+can't\s+be\s+blocked\s+by\s+more\s+than\s+one\s+creature`, EvergreenAbility, "Limited blocking", ap.parseLimitedBlocking)

	// Spell effects that are failing
	ap.addPattern(Activated, `Target\s+creature\s+gets\s+\-(\d+)/\-(\d+)\s+until\s+end\s+of\s+turn`, PumpCreature, "Negative pump", ap.parseNegativePump)
	ap.addPattern(Activated, `Return\s+target\s+creature\s+to\s+its\s+owner's\s+hand`, ReturnToHand, "Bounce creature", ap.parseBounceCreature)
	ap.addPattern(Activated, `Return\s+up\s+to\s+(\d+)\s+target\s+creatures\s+to\s+their\s+owners'\s+hands`, ReturnToHand, "Bounce multiple", ap.parseBounceMultiple)
	ap.addPattern(Activated, `Return\s+target\s+creature\s+card\s+from\s+your\s+graveyard\s+to\s+your\s+hand`, ReturnToHand, "Reanimate to hand", ap.parseReanimateToHand)

	// Combat abilities
	ap.addPattern(Static, `.*\s+attacks\s+each\s+combat\s+if\s+able`, EvergreenAbility, "Must attack", ap.parseMustAttack)
	ap.addPattern(Static, `.*\s+can't\s+attack\s+or\s+block\s+alone`, EvergreenAbility, "Can't act alone", ap.parseCantActAlone)

	// Conditional abilities
	ap.addPattern(Static, `During\s+your\s+turn,\s+.*\s+has\s+flying`, EvergreenAbility, "Conditional flying", ap.parseConditionalFlying)
}

// addPattern adds a new pattern to the parser.
func (ap *AbilityParser) addPattern(abilityType AbilityType, pattern string, effectType EffectType, description string, parser func([]string, string) (*Ability, error)) {
	regex := regexp.MustCompile(`(?i)` + pattern) // Case insensitive
	abilityPattern := &AbilityPattern{
		Regex:       regex,
		Type:        abilityType,
		EffectType:  effectType,
		Description: description,
		Parser:      parser,
	}
	ap.patterns[abilityType] = append(ap.patterns[abilityType], abilityPattern)
}

// ParseAbilities parses oracle text and returns a list of abilities.
func (ap *AbilityParser) ParseAbilities(oracleText string, source interface{}) ([]*Ability, error) {
	var abilities []*Ability
	var unparsedSentences []string

	// Split oracle text by sentences/lines for better parsing
	sentences := ap.splitOracleText(oracleText)

	for _, sentence := range sentences {
		found := false
		parseMethod := "none"

		// Try ability types in priority order: Triggered, Mana, Static, Activated
		priorityOrder := []AbilityType{Triggered, Mana, Static, Activated}

		for _, abilityType := range priorityOrder {
			patterns, exists := ap.patterns[abilityType]
			if !exists {
				continue
			}

			for _, pattern := range patterns {
				if matches := pattern.Regex.FindStringSubmatch(sentence); matches != nil {
					ability, err := pattern.Parser(matches, sentence)
					if err != nil {
						continue // Skip this pattern and try others
					}

					ability.ID = uuid.New()
					ability.Type = abilityType
					ability.Source = source
					ability.OracleText = sentence
					ability.ParsedFromText = true

					// Parse enhanced targeting information
					if err := ap.parseEnhancedTargets(ability, sentence); err != nil {
						// Log error but don't fail - fall back to basic targeting
						logger.LogCard("Failed to parse enhanced targets for %s: %v", ability.Name, err)
					}

					abilities = append(abilities, ability)
					found = true
					parseMethod = "full_parsing"
					break // Found a match, don't try other patterns for this sentence
				}
			}
			if found {
				break
			}
		}

		// If full parsing failed, try keyword-based fallback parsing
		if !found {
			keywordAbilities := ap.parseKeywordFallback(sentence, source)
			if len(keywordAbilities) > 0 {
				abilities = append(abilities, keywordAbilities...)
				found = true
				parseMethod = "keyword_fallback"
			}
		}

		// Track unparsed sentences for logging
		if !found && strings.TrimSpace(sentence) != "" {
			unparsedSentences = append(unparsedSentences, sentence)
		} else if found {
			// Log successful parsing method for debugging
			logger.LogCard("Parsed '%s' using %s", strings.TrimSpace(sentence), parseMethod)
		}
	}

	// Try to extract keywords from card's official Keywords field if available
	if source != nil {
		officialKeywords := ap.extractOfficialKeywords(source)
		keywordAbilities := ap.parseOfficialKeywords(officialKeywords, source)
		abilities = append(abilities, keywordAbilities...)
	}

	// Log parsing failures for cards with unparsed abilities
	if len(unparsedSentences) > 0 {
		cardName := "Unknown Card"
		if source != nil {
			// Try to extract card name from source
			if card, ok := source.(interface{ GetName() string }); ok {
				cardName = card.GetName()
			} else if hasName := fmt.Sprintf("%v", source); hasName != "" {
				cardName = hasName
			}
		}

		errorDetails := fmt.Sprintf("Failed to parse %d sentences: %v",
			len(unparsedSentences), unparsedSentences)
		logger.LogParsingFailure(cardName, oracleText, errorDetails)
	}

	return abilities, nil
}

// parseEnhancedTargets parses enhanced targeting information for an ability.
func (ap *AbilityParser) parseEnhancedTargets(ability *Ability, oracleText string) error {
	targetParser := NewTargetParser()
	enhancedTargets, err := targetParser.ParseTargetRestrictions(oracleText)
	if err != nil {
		return err
	}

	// Update ability effects with enhanced targeting information
	for i, effect := range ability.Effects {
		for j := range effect.Targets {
			// Try to match with enhanced targets
			if j < len(enhancedTargets) {
				enhanced := enhancedTargets[j]
				ability.Effects[i].Targets[j].Enhanced = &enhanced
			}
		}
	}

	return nil
}

// splitOracleText splits oracle text into individual sentences for parsing.
func (ap *AbilityParser) splitOracleText(text string) []string {
	// Split by periods, but be careful about mana symbols like {T}
	sentences := strings.Split(text, ".")
	var result []string
	
	for _, sentence := range sentences {
		trimmed := strings.TrimSpace(sentence)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	
	return result
}

// Specific parser functions for different ability types

func (ap *AbilityParser) parseManaAbility(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	manaType := types.ManaType(matches[1])
	
	return &Ability{
		Name: "Mana Ability",
		Type: Mana,
		Cost: Cost{
			TapCost: true,
		},
		Effects: []Effect{
			{
				Type:  AddMana,
				Value: 1,
				Duration: Instant,
				Description: "Add " + string(manaType),
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseAnyColorMana(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Any Color Mana",
		Type: Mana,
		Cost: Cost{
			TapCost: true,
		},
		Effects: []Effect{
			{
				Type:  AddMana,
				Value: 1,
				Duration: Instant,
				Description: "Add one mana of any color",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseColorlessMana(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Colorless Mana",
		Type: Mana,
		Cost: Cost{
			TapCost: true,
		},
		Effects: []Effect{
			{
				Type:  AddMana,
				Value: 2,
				Duration: Instant,
				Description: "Add {C}{C}",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseETBDrawCards(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	cardCount, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "ETB Draw Cards",
		Type: Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:  DrawCards,
				Value: cardCount,
				Duration: Instant,
				Description: "Draw " + matches[1] + " cards",
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseETBDrawCard(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "ETB Draw Card",
		Type: Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:  DrawCards,
				Value: 1,
				Duration: Instant,
				Description: "Draw a card",
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseETBDrawWordsCards(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	// Convert word numbers to integers
	var cardCount int
	switch strings.ToLower(matches[1]) {
	case "two":
		cardCount = 2
	case "three":
		cardCount = 3
	case "four":
		cardCount = 4
	case "five":
		cardCount = 5
	default:
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "ETB Draw Cards",
		Type: Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:  DrawCards,
				Value: cardCount,
				Duration: Instant,
				Description: "Draw " + matches[1] + " cards",
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseETBDamage(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	damage, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	target := matches[2]
	
	return &Ability{
		Name: "ETB Deal Damage",
		Type: Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:  DealDamage,
				Value: damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type: ap.parseTargetType(target),
						Required: true,
						Count: 1,
					},
				},
				Description: "Deal " + matches[1] + " damage to " + target,
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseETBGainLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	life, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "ETB Gain Life",
		Type: Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:  GainLife,
				Value: life,
				Duration: Instant,
				Description: "Gain " + matches[1] + " life",
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseDeathDrawCards(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	cardCount, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Death Draw Cards",
		Type: Triggered,
		TriggerCondition: Dies,
		Effects: []Effect{
			{
				Type:  DrawCards,
				Value: cardCount,
				Duration: Instant,
				Description: "Draw " + matches[1] + " cards",
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseDeathDrawCard(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Death Draw Card",
		Type: Triggered,
		TriggerCondition: Dies,
		Effects: []Effect{
			{
				Type:  DrawCards,
				Value: 1,
				Duration: Instant,
				Description: "Draw a card",
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseDeathDamage(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	damage, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	target := matches[2]
	
	return &Ability{
		Name: "Death Deal Damage",
		Type: Triggered,
		TriggerCondition: Dies,
		Effects: []Effect{
			{
				Type:  DealDamage,
				Value: damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type: ap.parseTargetType(target),
						Required: true,
						Count: 1,
					},
				},
				Description: "Deal " + matches[1] + " damage to " + target,
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseActivatedDraw(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	manaCost, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	cardCount, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Activated Draw",
		Type: Activated,
		Cost: Cost{
			ManaCost: map[types.ManaType]int{types.Any: manaCost},
			TapCost:  true,
		},
		Effects: []Effect{
			{
				Type:  DrawCards,
				Value: cardCount,
				Duration: Instant,
				Description: "Draw " + matches[2] + " cards",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseActivatedDrawCard(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	manaCost, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Activated Draw",
		Type: Activated,
		Cost: Cost{
			ManaCost: map[types.ManaType]int{types.Any: manaCost},
			TapCost:  true,
		},
		Effects: []Effect{
			{
				Type:  DrawCards,
				Value: 1,
				Duration: Instant,
				Description: "Draw a card",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseActivatedDamage(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 4 {
		return nil, ErrParsingFailed
	}

	manaCost, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	damage, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	target := matches[3]

	return &Ability{
		Name: "Activated Damage",
		Type: Activated,
		Cost: Cost{
			ManaCost: map[types.ManaType]int{types.Any: manaCost},
			TapCost:  true,
		},
		Effects: []Effect{
			{
				Type:  DealDamage,
				Value: damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type: ap.parseTargetType(target),
						Required: true,
						Count: 1,
					},
				},
				Description: "Deal " + matches[2] + " damage to " + target,
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseTapDamage(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	damage, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	target := matches[2]

	return &Ability{
		Name: "Tap Damage",
		Type: Activated,
		Cost: Cost{
			TapCost: true,
		},
		Effects: []Effect{
			{
				Type:  DealDamage,
				Value: damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type: ap.parseTargetType(target),
						Required: true,
						Count: 1,
					},
				},
				Description: "Deal " + matches[1] + " damage to " + target,
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parsePumpAbility(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	power, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Pump Creature",
		Type: Activated,
		Cost: Cost{
			TapCost: true,
		},
		Effects: []Effect{
			{
				Type:  PumpCreature,
				Value: power*100 + toughness, // Encode both values
				Duration: UntilEndOfTurn,
				Targets: []Target{
					{
						Type: CreatureTarget,
						Required: true,
						Count: 1,
					},
				},
				Description: "Target creature gets +" + matches[1] + "/+" + matches[2] + " until end of turn",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseStaticPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	power, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Static Pump",
		Type: Static,
		Effects: []Effect{
			{
				Type:  PumpCreature,
				Value: power*100 + toughness, // Encode both values
				Duration: UntilLeavesPlay,
				Description: "Creatures you control get +" + matches[1] + "/+" + matches[2],
			},
		},
	}, nil
}

func (ap *AbilityParser) parseStaticPumpOthers(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	power, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Static Pump Others",
		Type: Static,
		Effects: []Effect{
			{
				Type:  PumpCreature,
				Value: power*100 + toughness, // Encode both values
				Duration: UntilLeavesPlay,
				Description: "Other creatures you control get +" + matches[1] + "/+" + matches[2],
			},
		},
	}, nil
}

// parseTargetType converts a target string to a TargetType.
func (ap *AbilityParser) parseTargetType(target string) TargetType {
	target = strings.ToLower(target)

	if strings.Contains(target, "creature") {
		return CreatureTarget
	}
	if strings.Contains(target, "player") {
		return PlayerTarget
	}
	if strings.Contains(target, "permanent") {
		return PermanentTarget
	}
	if strings.Contains(target, "any target") {
		return AnyTarget
	}

	return AnyTarget // Default
}

func (ap *AbilityParser) parseTapGainLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	lifeGain, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Tap Gain Life",
		Type: Activated,
		Cost: Cost{
			TapCost: true,
		},
		Effects: []Effect{
			{
				Type:  GainLife,
				Value: lifeGain,
				Duration: Instant,
				Targets: []Target{
					{
						Type: PlayerTarget,
						Required: true,
						Count: 1,
					},
				},
				Description: "You gain " + matches[1] + " life",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// Modal spell parsers
func (ap *AbilityParser) parseModalSpell(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Modal Spell",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards, // Placeholder - would need more complex parsing
				Value:       1,
				Duration:    Instant,
				Description: "Choose one modal effect",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseModalSpellTwo(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Modal Spell - Choose Two",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards, // Placeholder - would need more complex parsing
				Value:       2,
				Duration:    Instant,
				Description: "Choose two modal effects",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseModalSpellThree(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Modal Spell - Choose Three",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards, // Placeholder - would need more complex parsing
				Value:       3,
				Duration:    Instant,
				Description: "Choose three modal effects",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseModalSpellAny(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Modal Spell - Choose Any",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards, // Placeholder - would need more complex parsing
				Value:       0, // Variable
				Duration:    Instant,
				Description: "Choose any number of modal effects",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// X-cost parsers
func (ap *AbilityParser) parseXCostDraw(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "X-Cost Draw",
		Type: Activated,
		Cost: Cost{
			ManaCost: map[types.ManaType]int{types.Any: -1}, // -1 indicates X cost
		},
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       -1, // -1 indicates variable X value
				Duration:    Instant,
				Description: "Draw X cards",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseXCostDamage(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])

	return &Ability{
		Name: "X-Cost Damage",
		Type: Activated,
		Cost: Cost{
			ManaCost: map[types.ManaType]int{types.Any: -1}, // -1 indicates X cost
		},
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    -1, // -1 indicates variable X value
				Duration: Instant,
				Targets: []Target{
					{
						Type:     targetType,
						Required: true,
						Count:    1,
					},
				},
				Description: "Deal X damage to " + matches[1],
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// Spell effect parsers
func (ap *AbilityParser) parseSpellDamage(matches []string, fullText string) (*Ability, error) {
	damage := ap.parseIntValue(matches[1])
	targetType := ap.parseTargetType(matches[2])

	return &Ability{
		Name: "Spell Damage",
		Type: Activated, // Spells are treated as activated abilities for parsing
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     targetType,
						Required: true,
						Count:    1,
					},
				},
				Description: "Deal " + matches[1] + " damage to " + matches[2],
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseNamedSpellDamage(matches []string, fullText string) (*Ability, error) {
	spellName := strings.TrimSpace(matches[1])
	damage := ap.parseIntValue(matches[2])
	targetType := ap.parseTargetType(matches[3])

	return &Ability{
		Name: spellName,
		Type: Activated,
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     targetType,
						Required: true,
						Count:    1,
					},
				},
				Description: spellName + " deals " + matches[2] + " damage to " + matches[3],
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseSpellDraw(matches []string, fullText string) (*Ability, error) {
	cards := ap.parseIntValue(matches[1])

	return &Ability{
		Name: "Spell Draw",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       cards,
				Duration:    Instant,
				Description: "Draw " + matches[1] + " cards",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseTargetedSpellDraw(matches []string, fullText string) (*Ability, error) {
	cards := ap.parseIntValue(matches[1])

	return &Ability{
		Name: "Targeted Spell Draw",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     DrawCards,
				Value:    cards,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     PlayerTarget,
						Required: true,
						Count:    1,
					},
				},
				Description: "Target player draws " + matches[1] + " cards",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseSpellDestroy(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])

	return &Ability{
		Name: "Spell Destroy",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     DestroyPermanent,
				Value:    1,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     targetType,
						Required: true,
						Count:    1,
					},
				},
				Description: "Destroy target " + matches[1],
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseCounterspell(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Counterspell",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     CounterSpell,
				Value:    1,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     SpellTarget,
						Required: true,
						Count:    1,
					},
				},
				Description: "Counter target spell",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseTargetedLifeGain(matches []string, fullText string) (*Ability, error) {
	life := ap.parseIntValue(matches[1])

	return &Ability{
		Name: "Targeted Life Gain",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     GainLife,
				Value:    life,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     PlayerTarget,
						Required: true,
						Count:    1,
					},
				},
				Description: "Target player gains " + matches[1] + " life",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseSpellPump(matches []string, fullText string) (*Ability, error) {
	power := ap.parseIntValue(matches[1])
	toughness := ap.parseIntValue(matches[2])

	return &Ability{
		Name: "Spell Pump",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     PumpCreature,
				Value:    power, // Store toughness in description for now
				Duration: UntilEndOfTurn,
				Targets: []Target{
					{
						Type:     CreatureTarget,
						Required: true,
						Count:    1,
					},
				},
				Description: "Target creature gets +" + matches[1] + "/+" + matches[2] + " until end of turn (toughness: " + strconv.Itoa(toughness) + ")",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// Helper function to parse integer values from regex matches
func (ap *AbilityParser) parseIntValue(s string) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return 0
}

// Additional spell parsers
func (ap *AbilityParser) parseSpellDrawWords(matches []string, fullText string) (*Ability, error) {
	// Convert word numbers to integers
	var cardCount int
	switch strings.ToLower(matches[1]) {
	case "two":
		cardCount = 2
	case "three":
		cardCount = 3
	case "four":
		cardCount = 4
	case "five":
		cardCount = 5
	default:
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Spell Draw Words",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       cardCount,
				Duration:    Instant,
				Description: "Draw " + matches[1] + " cards",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseTargetedSpellDrawWords(matches []string, fullText string) (*Ability, error) {
	// Convert word numbers to integers
	var cardCount int
	switch strings.ToLower(matches[1]) {
	case "three":
		cardCount = 3
	case "four":
		cardCount = 4
	case "five":
		cardCount = 5
	default:
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Targeted Spell Draw Words",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     DrawCards,
				Value:    cardCount,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     PlayerTarget,
						Required: true,
						Count:    1,
					},
				},
				Description: "Target player draws " + matches[1] + " cards",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseConditionalCounterspell(matches []string, fullText string) (*Ability, error) {
	cost := ap.parseIntValue(matches[1])

	return &Ability{
		Name: "Conditional Counterspell",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     CounterSpell,
				Value:    cost, // Store the mana cost to pay
				Duration: Instant,
				Targets: []Target{
					{
						Type:     SpellTarget,
						Required: true,
						Count:    1,
					},
				},
				Description: "Counter target spell unless its controller pays {" + matches[1] + "}",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseXCostSpellDamage(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])

	return &Ability{
		Name: "X-Cost Spell Damage",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    -1, // -1 indicates variable X value
				Duration: Instant,
				Targets: []Target{
					{
						Type:     targetType,
						Required: true,
						Count:    1,
					},
				},
				Description: "Deal X damage to " + matches[1],
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseXCostSpellDraw(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "X-Cost Spell Draw",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       -1, // -1 indicates variable X value
				Duration:    Instant,
				Description: "Draw X cards",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseDividedDamage(matches []string, fullText string) (*Ability, error) {
	damage := ap.parseIntValue(matches[1])
	targetDesc := matches[2]

	return &Ability{
		Name: "Divided Damage",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     AnyTarget, // Can target multiple things
						Required: true,
						Count:    2, // Up to two targets
					},
				},
				Description: "Deal " + matches[1] + " damage divided as you choose among " + targetDesc,
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseDrainLife(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])

	return &Ability{
		Name: "Drain Life",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    -1, // X damage
				Duration: Instant,
				Targets: []Target{
					{
						Type:     targetType,
						Required: true,
						Count:    1,
					},
				},
				Description: "Deal X damage to " + matches[1],
			},
			{
				Type:        GainLife,
				Value:       -1, // Equal to damage dealt
				Duration:    Instant,
				Description: "Gain life equal to the damage dealt",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// Additional spell parsers for enhanced coverage

func (ap *AbilityParser) parseMassDestroy(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Mass Destroy",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DestroyPermanent,
				Value:       0, // All matching permanents
				Duration:    Instant,
				Description: "Destroy all " + matches[1],
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseTripleMana(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Triple Mana",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        AddMana,
				Value:       3,
				Duration:    Instant,
				Description: "Add " + matches[1] + matches[2] + matches[3],
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseFogEffect(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Fog Effect",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PreventDamage,
				Value:       0, // All combat damage
				Duration:    UntilEndOfTurn,
				Description: "Prevent all combat damage this turn",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseExtraTurn(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Extra Turn",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards, // Placeholder effect type
				Value:       1,
				Duration:    Instant,
				Description: "Take an extra turn after this one",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseTokenCreation(matches []string, fullText string) (*Ability, error) {
	count := ap.parseIntValue(matches[1])
	power := ap.parseIntValue(matches[2])
	toughness := ap.parseIntValue(matches[3])

	return &Ability{
		Name: "Token Creation",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        CreateToken,
				Value:       count,
				Duration:    Instant,
				Description: "Create " + matches[1] + " " + matches[2] + "/" + matches[3] + " creature tokens (power: " + strconv.Itoa(power) + ", toughness: " + strconv.Itoa(toughness) + ")",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseXCostDividedDamage(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])

	return &Ability{
		Name: "X-Cost Divided Damage",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    -1, // -1 indicates variable X value
				Duration: Instant,
				Targets: []Target{
					{
						Type:     targetType,
						Required: true,
						Count:    -1, // Variable number of targets
					},
				},
				Description: "Deal X damage divided as you choose among " + matches[1],
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// parseKeywordFallback attempts to parse abilities using keyword recognition
func (ap *AbilityParser) parseKeywordFallback(sentence string, source interface{}) []*Ability {
	var abilities []*Ability

	// Define known MTG keywords and their patterns
	keywordPatterns := map[string]func() *Ability{
		"flying": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Flying",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Flying",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"deathtouch": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Deathtouch",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Deathtouch",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"haste": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Haste",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Haste",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"trample": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Trample",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Trample",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"reach": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Reach",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Reach",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"defender": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Defender",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Defender",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"vigilance": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Vigilance",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Vigilance",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"lifelink": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Lifelink",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Lifelink",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"first strike": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "First Strike",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "First Strike",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
		"double strike": func() *Ability {
			return &Ability{
				ID:   uuid.New(),
				Name: "Double Strike",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Double Strike",
						Duration:    Permanent,
					},
				},
				Source:          source,
				OracleText:      sentence,
				ParsedFromText:  true,
				TimingRestriction: AnyTime,
			}
		},
	}

	// Check for keywords in the sentence (case insensitive)
	lowerSentence := strings.ToLower(sentence)

	// Handle comma-separated keywords like "Flying, haste"
	if strings.Contains(lowerSentence, ",") {
		parts := strings.Split(lowerSentence, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if creator, exists := keywordPatterns[trimmed]; exists {
				abilities = append(abilities, creator())
			}
		}
	} else {
		// Check for single keywords
		for keyword, creator := range keywordPatterns {
			if strings.Contains(lowerSentence, keyword) {
				// Make sure it's a whole word match, not part of another word
				if ap.isWholeWordMatch(lowerSentence, keyword) {
					abilities = append(abilities, creator())
				}
			}
		}
	}

	return abilities
}

// isWholeWordMatch checks if a keyword appears as a whole word in the text
func (ap *AbilityParser) isWholeWordMatch(text, keyword string) bool {
	// Simple word boundary check
	keywordLen := len(keyword)
	index := strings.Index(text, keyword)

	if index == -1 {
		return false
	}

	// Check character before keyword
	if index > 0 {
		prevChar := text[index-1]
		if (prevChar >= 'a' && prevChar <= 'z') || (prevChar >= 'A' && prevChar <= 'Z') {
			return false
		}
	}

	// Check character after keyword
	if index+keywordLen < len(text) {
		nextChar := text[index+keywordLen]
		if (nextChar >= 'a' && nextChar <= 'z') || (nextChar >= 'A' && nextChar <= 'Z') {
			return false
		}
	}

	return true
}

// extractOfficialKeywords extracts keywords from the card's official Keywords field
func (ap *AbilityParser) extractOfficialKeywords(source interface{}) []string {
	var keywords []string

	// Try to extract keywords from different possible source types
	if card, ok := source.(interface{ GetKeywords() []string }); ok {
		keywords = card.GetKeywords()
	} else if hasKeywords := fmt.Sprintf("%v", source); hasKeywords != "" {
		// Try to extract from string representation if it contains keyword info
		// This is a fallback for when the source doesn't have a GetKeywords method
		// We'll look for common patterns in the string representation
	}

	return keywords
}

// parseOfficialKeywords creates abilities from official keyword list
func (ap *AbilityParser) parseOfficialKeywords(keywords []string, source interface{}) []*Ability {
	var abilities []*Ability

	for _, keyword := range keywords {
		// Clean up the keyword (remove extra spaces, convert to lowercase)
		cleanKeyword := strings.ToLower(strings.TrimSpace(keyword))

		// Create ability based on keyword
		var ability *Ability

		switch cleanKeyword {
		case "flying":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Flying",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Flying",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false, // This came from official keywords, not oracle text
				TimingRestriction: AnyTime,
			}
		case "deathtouch":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Deathtouch",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Deathtouch",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		case "haste":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Haste",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Haste",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		case "trample":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Trample",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Trample",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		case "reach":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Reach",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Reach",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		case "defender":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Defender",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Defender",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		case "vigilance":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Vigilance",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Vigilance",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		case "lifelink":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Lifelink",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Lifelink",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		case "first strike":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "First Strike",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "First Strike",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		case "double strike":
			ability = &Ability{
				ID:   uuid.New(),
				Name: "Double Strike",
				Type: Static,
				Effects: []Effect{
					{
						Type:        EvergreenAbility,
						Description: "Double Strike",
						Duration:    Permanent,
					},
				},
				Source:            source,
				OracleText:        keyword,
				ParsedFromText:    false,
				TimingRestriction: AnyTime,
			}
		}

		if ability != nil {
			abilities = append(abilities, ability)
		}
	}

	return abilities
}

// parseSimpleKeyword parses simple keyword abilities like "Flying", "Haste", etc.
func (ap *AbilityParser) parseSimpleKeyword(matches []string, fullText string) (*Ability, error) {
	keyword := strings.ToLower(strings.TrimSpace(fullText))

	// Capitalize first letter manually
	capitalizedKeyword := ""
	if len(keyword) > 0 {
		capitalizedKeyword = strings.ToUpper(string(keyword[0])) + keyword[1:]
	}

	return &Ability{
		Name: capitalizedKeyword,
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: capitalizedKeyword,
				Duration:    Permanent,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseMultiKeyword parses multiple keywords like "Flying, haste"
func (ap *AbilityParser) parseMultiKeyword(matches []string, fullText string) (*Ability, error) {
	// This is a simplified approach - in reality we'd want to create separate abilities
	// For now, we'll create one ability with a combined description
	return &Ability{
		Name: "Multiple Keywords",
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: fullText,
				Duration:    Permanent,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseETBPump parses ETB abilities that pump creatures
func (ap *AbilityParser) parseETBPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	power, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "ETB Pump",
		Type: Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:  PumpCreature,
				Value: power*100 + toughness, // Encode both values
				Duration: UntilEndOfTurn,
				Targets: []Target{
					{
						Type: CreatureTarget,
						Required: true,
						Count: 1,
						Restrictions: []string{"you control"},
					},
				},
				Description: fmt.Sprintf("Target creature you control gets +%d/+%d until end of turn", power, toughness),
			},
		},
		IsOptional: false,
	}, nil
}

// parseETBTap parses ETB abilities that tap creatures
func (ap *AbilityParser) parseETBTap(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "ETB Tap",
		Type: Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:  TapUntap,
				Value: 1, // 1 for tap, 0 for untap
				Duration: Instant,
				Targets: []Target{
					{
						Type: CreatureTarget,
						Required: true,
						Count: 1,
					},
				},
				Description: "Tap target creature",
			},
		},
		IsOptional: false,
	}, nil
}

// parseActivatedPump parses activated abilities that pump creatures
func (ap *AbilityParser) parseActivatedPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 4 {
		return nil, ErrParsingFailed
	}

	genericCost, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	manaType := types.ManaType(matches[2])
	power, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[4])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Activated Pump",
		Type: Activated,
		Cost: Cost{
			ManaCost: map[types.ManaType]int{
				types.Any: genericCost,
				manaType:  1,
			},
		},
		Effects: []Effect{
			{
				Type:  PumpCreature,
				Value: power*100 + toughness,
				Duration: UntilEndOfTurn,
				Targets: []Target{
					{
						Type: CreatureTarget,
						Required: true,
						Count: 1,
					},
				},
				Description: fmt.Sprintf("Target creature gets +%d/+%d until end of turn", power, toughness),
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseSingleManaPump parses single mana activated pump abilities
func (ap *AbilityParser) parseSingleManaPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 4 {
		return nil, ErrParsingFailed
	}

	manaType := types.ManaType(matches[1])
	power, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Single Mana Pump",
		Type: Activated,
		Cost: Cost{
			ManaCost: map[types.ManaType]int{
				manaType: 1,
			},
		},
		Effects: []Effect{
			{
				Type:  PumpCreature,
				Value: power*100 + toughness,
				Duration: UntilEndOfTurn,
				Description: fmt.Sprintf("This creature gets +%d/+%d until end of turn", power, toughness),
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseEnchantCreature parses "Enchant creature" abilities
func (ap *AbilityParser) parseEnchantCreature(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Enchant Creature",
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: "Enchant creature",
				Duration:    Permanent,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseEnchantedPump parses "Enchanted creature gets +X/+X" abilities
func (ap *AbilityParser) parseEnchantedPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	power, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Enchanted Pump",
		Type: Static,
		Effects: []Effect{
			{
				Type:  PumpCreature,
				Value: power*100 + toughness,
				Duration: Permanent,
				Description: fmt.Sprintf("Enchanted creature gets +%d/+%d", power, toughness),
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseEnchantedRestriction parses "Enchanted creature can't attack or block" abilities
func (ap *AbilityParser) parseEnchantedRestriction(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Enchanted Restriction",
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: "Enchanted creature can't attack or block",
				Duration:    Permanent,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseUnblockable parses "can't be blocked" abilities
func (ap *AbilityParser) parseUnblockable(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Unblockable",
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: "Can't be blocked",
				Duration:    Permanent,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseLimitedBlocking parses "can't be blocked by more than one creature" abilities
func (ap *AbilityParser) parseLimitedBlocking(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Limited Blocking",
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: "Can't be blocked by more than one creature",
				Duration:    Permanent,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseNegativePump parses "Target creature gets -X/-X" abilities
func (ap *AbilityParser) parseNegativePump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}

	power, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Negative Pump",
		Type: Activated,
		Effects: []Effect{
			{
				Type:  PumpCreature,
				Value: -power*100 - toughness, // Negative values
				Duration: UntilEndOfTurn,
				Targets: []Target{
					{
						Type: CreatureTarget,
						Required: true,
						Count: 1,
					},
				},
				Description: fmt.Sprintf("Target creature gets -%d/-%d until end of turn", power, toughness),
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// parseBounceCreature parses "Return target creature to its owner's hand" abilities
func (ap *AbilityParser) parseBounceCreature(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Bounce Creature",
		Type: Activated,
		Effects: []Effect{
			{
				Type:  ReturnToHand,
				Value: 1,
				Duration: Instant,
				Targets: []Target{
					{
						Type: CreatureTarget,
						Required: true,
						Count: 1,
					},
				},
				Description: "Return target creature to its owner's hand",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// parseBounceMultiple parses "Return up to X target creatures to their owners' hands" abilities
func (ap *AbilityParser) parseBounceMultiple(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	count, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Bounce Multiple",
		Type: Activated,
		Effects: []Effect{
			{
				Type:  ReturnToHand,
				Value: count,
				Duration: Instant,
				Targets: []Target{
					{
						Type: CreatureTarget,
						Required: false, // "up to" means optional
						Count: count,
					},
				},
				Description: fmt.Sprintf("Return up to %d target creatures to their owners' hands", count),
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// parseReanimateToHand parses "Return target creature card from your graveyard to your hand" abilities
func (ap *AbilityParser) parseReanimateToHand(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Reanimate to Hand",
		Type: Activated,
		Effects: []Effect{
			{
				Type:  ReturnToHand,
				Value: 1,
				Duration: Instant,
				Targets: []Target{
					{
						Type: CardInGraveyardTarget,
						Required: true,
						Count: 1,
						Restrictions: []string{"creature card", "your graveyard"},
					},
				},
				Description: "Return target creature card from your graveyard to your hand",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

// parseMustAttack parses "attacks each combat if able" abilities
func (ap *AbilityParser) parseMustAttack(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Must Attack",
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: "Attacks each combat if able",
				Duration:    Permanent,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseCantActAlone parses "can't attack or block alone" abilities
func (ap *AbilityParser) parseCantActAlone(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Can't Act Alone",
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: "Can't attack or block alone",
				Duration:    Permanent,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// parseConditionalFlying parses "During your turn, X has flying" abilities
func (ap *AbilityParser) parseConditionalFlying(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Conditional Flying",
		Type: Static,
		Effects: []Effect{
			{
				Type:        EvergreenAbility,
				Description: "During your turn, this creature has flying",
				Duration:    Permanent,
				Conditions:  []string{"during your turn"},
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}
