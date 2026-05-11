// Package ability provides oracle text parsing for MTG abilities.
package ability

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/game"
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
	ap.addPattern(Activated, `Choose one —.*`, ChooseMode, "Modal spell", ap.parseModalSpell)
	ap.addPattern(Activated, `Choose two —.*`, ChooseMode, "Modal spell - choose two", ap.parseModalSpellTwo)
	ap.addPattern(Activated, `Choose three.*`, ChooseMode, "Modal spell - choose three", ap.parseModalSpellThree)
	ap.addPattern(Activated, `Choose any number —.*`, ChooseMode, "Modal spell - choose any", ap.parseModalSpellAny)

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
	ap.addPattern(Activated, `Take\s+an\s+extra\s+turn\s+after\s+this\s+one`, TakeExtraTurn, "Extra turn", ap.parseExtraTurn)
	ap.addPattern(Activated, `Create\s+(\d+)\s+(\d+)/(\d+)\s+.*\s+creature\s+tokens?`, CreateToken, "Token creation", ap.parseTokenCreation)

	// X-cost spell effects
	ap.addPattern(Activated, `.*\s+deals\s+X\s+damage\s+to\s+(.*)`, DealDamage, "X-cost spell damage", ap.parseXCostSpellDamage)
	ap.addPattern(Activated, `.*\s+deals\s+X\s+damage\s+divided\s+as\s+you\s+choose\s+among\s+(.*)`, DealDamage, "X-cost divided damage", ap.parseXCostDividedDamage)
	ap.addPattern(Activated, `Draw\s+X\s+cards?`, DrawCards, "X-cost spell draw", ap.parseXCostSpellDraw)

	// Complex spell effects
	ap.addPattern(Activated, `.*\s+deals\s+(\d+)\s+damage\s+divided\s+as\s+you\s+choose\s+among\s+(.*)`, DealDamage, "Divided damage", ap.parseDividedDamage)
	ap.addPattern(Activated, `.*\s+deals\s+X\s+damage\s+to\s+(.*)\.\s+You\s+gain\s+life\s+equal\s+to\s+the\s+damage\s+dealt`, DealDamage, "Drain life", ap.parseDrainLife)
	ap.addPattern(Activated, `.*\s+deals\s+X\s+damage\s+to\s+(.*)\s+You\s+gain\s+life\s+equal\s+to\s+the\s+damage\s+dealt`, DealDamage, "Drain life no period", ap.parseDrainLife)

	// New spell patterns for bounce, debuff, control change, and bite effects
	ap.addPattern(Activated, `Return\s+target\s+creature\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "Bounce creature", ap.parseBounceCreature)
	ap.addPattern(Activated, `Return\s+up\s+to\s+three\s+target\s+creatures\s+to\s+their\s+owners'?\s+hands`, ReturnToHand, "Bounce up to three creatures", ap.parseBounceUpToThree)
	ap.addPattern(Activated, `Target\s+creature\s+gets\s+-([0-9]+)\/-([0-9]+)\s+until\s+end\s+of\s+turn`, PumpCreature, "Debuff creature until EOT", ap.parseSpellDebuff)
	ap.addPattern(Activated, `Draw\s+a\s+card`, DrawCards, "Draw a card", ap.parseSpellDrawACard)
	ap.addPattern(Activated, `Gain\s+control\s+of\s+target\s+creature\s+until\s+end\s+of\s+turn.*Untap\s+that\s+creature.*`, ChangeControl, "Act of Treason style", ap.parseActOfTreason)
	ap.addPattern(Activated, `Target\s+creature\s+you\s+control\s+deals\s+damage\s+equal\s+to\s+its\s+power\s+to\s+target\s+creature\s+you\s+don'?t\s+control`, SourcePowerDamage, "Rabid Bite style", ap.parseRabidBite)

	// Tap/Untap effects
	ap.addPattern(Activated, `Tap\s+target\s+(creature|permanent|land|artifact|enchantment|planeswalker)`, TapUntap, "Tap single target", ap.parseTapTarget)
	ap.addPattern(Activated, `Tap\s+all\s+(creatures|lands|permanents|artifacts|enchantments|planeswalkers)`, TapUntap, "Tap all of type", ap.parseTapAll)
	ap.addPattern(Activated, `Untap\s+target\s+(creature|permanent|land|artifact|enchantment|planeswalker)`, TapUntap, "Untap single target", ap.parseUntapTarget)
	ap.addPattern(Activated, `Untap\s+all\s+(creatures|lands|permanents|artifacts|enchantments|planeswalkers)\s+you\s+control`, TapUntap, "Untap all controlled", ap.parseUntapAllControlled)
	ap.addPattern(Activated, `\{T\}:\s*Target\s+creature\s+doesn't\s+untap\s+during\s+its\s+controller's\s+next\s+untap\s+step`, TapUntap, "Freeze target creature", ap.parseFreezeTarget)

	// Discard effects
	ap.addPattern(Activated, `Target\s+player\s+discards\s+(a|two|three|four|five|\d+)\s+cards?`, DiscardCards, "Target player discards", ap.parseTargetPlayerDiscard)
	ap.addPattern(Activated, `Target\s+opponent\s+discards\s+(a|two|three|four|five|\d+)\s+cards?`, DiscardCards, "Target opponent discards", ap.parseTargetOpponentDiscard)
	ap.addPattern(Activated, `Each\s+player\s+discards\s+(a|two|three|four|five|\d+)\s+cards?`, DiscardCards, "Each player discards", ap.parseEachPlayerDiscard)
	ap.addPattern(Activated, `Target\s+creature'?s?\s+controller\s+discards\s+(a|two|three|four|five|\d+)\s+cards?`, DiscardCards, "Controller discards", ap.parseControllerDiscard)
	ap.addPattern(Triggered, `When(ever)?\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+player,\s+that\s+player\s+discards\s+(a|two|three|four|five|\d+)\s+cards?`, DiscardCards, "Combat damage discard trigger", ap.parseCombatDamageDiscard)

	// Search library effects
	ap.addPattern(Activated, `Search\s+your\s+library\s+for\s+a\s+basic\s+land\s+card`, SearchLibrary, "Search basic land", ap.parseSearchBasicLand)
	ap.addPattern(Activated, `Search\s+your\s+library\s+for\s+a\s+land\s+card`, SearchLibrary, "Search any land", ap.parseSearchLand)
	ap.addPattern(Activated, `Search\s+your\s+library\s+for\s+up\s+to\s+(\d+)\s+basic\s+land\s+cards?`, SearchLibrary, "Search multiple basic lands", ap.parseSearchMultipleBasicLands)
	ap.addPattern(Activated, `Search\s+your\s+library\s+for\s+a\s+card`, SearchLibrary, "Search any card", ap.parseSearchAnyCard)
	ap.addPattern(Activated, `Search\s+your\s+library\s+for\s+.*\s+and\s+put\s+(it|them)\s+onto\s+the\s+battlefield`, SearchLibrary, "Search to battlefield", ap.parseSearchToBattlefield)

	// Conditional abilities - activated/static
	ap.addPattern(Activated, `If\s+you\s+control\s+a\s+(.*),\s*(.*)`, DrawCards, "Conditional control permanent", ap.parseConditionalControl)
	ap.addPattern(Activated, `If\s+an\s+opponent\s+controls\s+more\s+creatures\s+than\s+you,\s*(.*)`, DealDamage, "Conditional opponent creatures", ap.parseConditionalOpponentCreatures)
	ap.addPattern(Activated, `If\s+you\s+have\s+no\s+cards\s+in\s+hand,\s*(.*)`, DealDamage, "Conditional hellbent", ap.parseConditionalHellbent)

	// Conditional ETB triggers
	ap.addPattern(Triggered, `When\s+.*\s+enters\s+the\s+battlefield,\s+if\s+you\s+control\s+a\s+(.*),\s*(.*)`, DrawCards, "Conditional ETB control", ap.parseConditionalETBControl)

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

	// Split oracle text by sentences/lines for better parsing
	sentences := ap.splitOracleText(oracleText)

	for _, sentence := range sentences {
		matched := false
		// Try ability type groups in deterministic precedence. Go map
		// iteration is intentionally randomized; without this, broad spell
		// patterns like "Draw a card" can beat a more specific ETB trigger.
		for _, abilityType := range []AbilityType{Mana, Triggered, Activated, Static} {
			if matched {
				break // Already found a match for this sentence
			}
			patterns := ap.patterns[abilityType]
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
					matched = true
					break // Found a match, don't try other patterns for this sentence
				}
			}
		}
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

	manaType := game.ManaType(matches[1])

	return &Ability{
		Name: "Mana Ability",
		Type: Mana,
		Cost: Cost{
			TapCost: true,
		},
		Effects: []Effect{
			{
				Type:        AddMana,
				Value:       1,
				Duration:    Instant,
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
				Type:        AddMana,
				Value:       1,
				Duration:    Instant,
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
				Type:        AddMana,
				Value:       2,
				Duration:    Instant,
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
		Name:             "ETB Draw Cards",
		Type:             Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       cardCount,
				Duration:    Instant,
				Description: "Draw " + matches[1] + " cards",
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseETBDrawCard(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name:             "ETB Draw Card",
		Type:             Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       1,
				Duration:    Instant,
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
		Name:             "ETB Draw Cards",
		Type:             Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       cardCount,
				Duration:    Instant,
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
		Name:             "ETB Deal Damage",
		Type:             Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     ap.parseTargetType(target),
						Required: true,
						Count:    1,
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
		Name:             "ETB Gain Life",
		Type:             Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:        GainLife,
				Value:       life,
				Duration:    Instant,
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
		Name:             "Death Draw Cards",
		Type:             Triggered,
		TriggerCondition: Dies,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       cardCount,
				Duration:    Instant,
				Description: "Draw " + matches[1] + " cards",
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseDeathDrawCard(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name:             "Death Draw Card",
		Type:             Triggered,
		TriggerCondition: Dies,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       1,
				Duration:    Instant,
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
		Name:             "Death Deal Damage",
		Type:             Triggered,
		TriggerCondition: Dies,
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     ap.parseTargetType(target),
						Required: true,
						Count:    1,
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
			ManaCost: map[game.ManaType]int{game.Any: manaCost},
			TapCost:  true,
		},
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       cardCount,
				Duration:    Instant,
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
			ManaCost: map[game.ManaType]int{game.Any: manaCost},
			TapCost:  true,
		},
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       1,
				Duration:    Instant,
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
			ManaCost: map[game.ManaType]int{game.Any: manaCost},
			TapCost:  true,
		},
		Effects: []Effect{
			{
				Type:     DealDamage,
				Value:    damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     ap.parseTargetType(target),
						Required: true,
						Count:    1,
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
				Type:     DealDamage,
				Value:    damage,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     ap.parseTargetType(target),
						Required: true,
						Count:    1,
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
				Type:     PumpCreature,
				Value:    power*100 + toughness, // Encode both values
				Duration: UntilEndOfTurn,
				Targets: []Target{
					{
						Type:     CreatureTarget,
						Required: true,
						Count:    1,
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
				Type:        PumpCreature,
				Value:       power*100 + toughness, // Encode both values
				Duration:    UntilLeavesPlay,
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
				Type:        PumpCreature,
				Value:       power*100 + toughness, // Encode both values
				Duration:    UntilLeavesPlay,
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
				Type:     GainLife,
				Value:    lifeGain,
				Duration: Instant,
				Targets: []Target{
					{
						Type:     PlayerTarget,
						Required: true,
						Count:    1,
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
				Type:        ChooseMode,
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
				Type:        ChooseMode,
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
				Type:        ChooseMode,
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
				Type:        ChooseMode,
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
			ManaCost: map[game.ManaType]int{game.Any: -1}, // -1 indicates X cost
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
			ManaCost: map[game.ManaType]int{game.Any: -1}, // -1 indicates X cost
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
				Type:        TakeExtraTurn,
				Value:       1,
				Duration:    Instant,
				Description: "Take an extra turn after this one",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseConditionalControl(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	conditionValue := matches[1]
	effectText := strings.TrimSpace(matches[2])

	// Determine effect type from the effect clause
	effectType, value := ap.inferEffectFromText(effectText)

	return &Ability{
		Name: "Conditional Control",
		Type: Activated,
		Effects: []Effect{
			{
				Type: effectType,
				Value: value,
				Duration: Instant,
				Conditions: []Condition{
					{Type: ControlPermanentType, Value: conditionValue},
				},
				Description: effectText,
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseConditionalOpponentCreatures(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	effectText := strings.TrimSpace(matches[1])
	effectType, value := ap.inferEffectFromText(effectText)

	return &Ability{
		Name: "Conditional Opponent Creatures",
		Type: Activated,
		Effects: []Effect{
			{
				Type: effectType,
				Value: value,
				Duration: Instant,
				Conditions: []Condition{
					{Type: OpponentHasMoreCreatures},
				},
				Description: effectText,
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseConditionalHellbent(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	effectText := strings.TrimSpace(matches[1])
	effectType, value := ap.inferEffectFromText(effectText)

	return &Ability{
		Name: "Conditional Hellbent",
		Type: Activated,
		Effects: []Effect{
			{
				Type: effectType,
				Value: value,
				Duration: Instant,
				Conditions: []Condition{
					{Type: NoCardsInHand},
				},
				Description: effectText,
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseConditionalETBControl(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	conditionValue := matches[1]
	effectText := strings.TrimSpace(matches[2])
	effectType, value := ap.inferEffectFromText(effectText)

	return &Ability{
		Name: "Conditional ETB Control",
		Type: Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type: effectType,
				Value: value,
				Duration: Instant,
				Conditions: []Condition{
					{Type: ControlPermanentType, Value: conditionValue},
				},
				Description: effectText,
			},
		},
	}, nil
}

func (ap *AbilityParser) inferEffectFromText(text string) (EffectType, int) {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "draw") {
		count := ap.parseIntValue(text)
		if count == 0 {
			count = 1
		}
		return DrawCards, count
	}
	if strings.Contains(lower, "damage") {
		count := ap.parseIntValue(text)
		if count == 0 {
			count = 1
		}
		return DealDamage, count
	}
	if strings.Contains(lower, "gain") && strings.Contains(lower, "life") {
		count := ap.parseIntValue(text)
		if count == 0 {
			count = 1
		}
		return GainLife, count
	}
	if strings.Contains(lower, "create") && strings.Contains(lower, "token") {
		return CreateToken, 1
	}
	if strings.Contains(lower, "search") && strings.Contains(lower, "library") {
		return SearchLibrary, 1
	}
	if strings.Contains(lower, "destroy") {
		return DestroyPermanent, 1
	}
	if strings.Contains(lower, "counter") && strings.Contains(lower, "spell") {
		return CounterSpell, 1
	}
	if strings.Contains(lower, "discard") {
		count := ap.parseIntValue(text)
		if count == 0 {
			count = 1
		}
		return DiscardCards, count
	}
	if strings.Contains(lower, "return") && strings.Contains(lower, "hand") {
		return ReturnToHand, 1
	}
	if strings.Contains(lower, "prevent") && strings.Contains(lower, "damage") {
		return PreventDamage, 0
	}
	return DealDamage, 1 // Default fallback
}

func (ap *AbilityParser) parseTokenCreation(matches []string, fullText string) (*Ability, error) {
	count := ap.parseIntValue(matches[1])
	power := ap.parseIntValue(matches[2])
	toughness := ap.parseIntValue(matches[3])

	// Encode count, power, toughness into Value: count*1000000 + power*1000 + toughness
	encodedValue := count*1000000 + power*1000 + toughness

	return &Ability{
		Name: "Token Creation",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        CreateToken,
				Value:       encodedValue,
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

// --- New parser functions for specific card-style effects ---

func (ap *AbilityParser) parseBounceCreature(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Bounce Creature",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        ReturnToHand,
				Value:       1,
				Duration:    Instant,
				Targets:     []Target{{Type: CreatureTarget, Required: true, Count: 1}},
				Description: "Return target creature to its owner's hand",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseBounceUpToThree(matches []string, fullText string) (*Ability, error) {
	// Simplified: model as a single-target bounce
	return &Ability{
		Name: "Mass Bounce (Simplified)",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        ReturnToHand,
				Value:       1,
				Duration:    Instant,
				Targets:     []Target{{Type: CreatureTarget, Required: false, Count: 1}},
				Description: "Return up to three target creatures to their owners' hands (simplified)",
			},
		},
		TimingRestriction: SorcerySpeed, // Captivating Gyre is a sorcery
	}, nil
}

func (ap *AbilityParser) parseSpellDebuff(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	pow, _ := strconv.Atoi(matches[1])
	tgh, _ := strconv.Atoi(matches[2])
	pow = -pow
	tgh = -tgh
	return &Ability{
		Name: "Spell Debuff",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Value:       pow*100 + tgh,
				Duration:    UntilEndOfTurn,
				Targets:     []Target{{Type: CreatureTarget, Required: true, Count: 1}},
				Description: "Target creature gets " + fullText[strings.Index(strings.ToLower(fullText), "target creature gets")+len("Target creature gets "):],
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseSpellDrawACard(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name:              "Draw a Card",
		Type:              Activated,
		Effects:           []Effect{{Type: DrawCards, Value: 1, Duration: Instant, Description: "Draw a card"}},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseActOfTreason(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Act of Treason",
		Type: Activated,
		Effects: []Effect{
			{ // Change control until EOT
				Type:        ChangeControl,
				Value:       1,
				Duration:    UntilEndOfTurn,
				Targets:     []Target{{Type: CreatureTarget, Required: true, Count: 1}},
				Description: "Gain control of target creature until end of turn",
			},
			{ // Untap that creature
				Type:        TapUntap,
				Value:       0, // 0 => untap
				Duration:    Instant,
				Description: "Untap that creature",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseRabidBite(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Rabid Bite",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     SourcePowerDamage,
				Value:    0,
				Duration: Instant,
				Targets: []Target{
					{Type: CreatureTarget, Required: true, Count: 1}, // your creature (simplified)
					{Type: CreatureTarget, Required: true, Count: 1}, // opponent creature (simplified)
				},
				Description: "Target creature you control deals damage equal to its power to target creature you don't control",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}


// --- Phase 1: Tap/Untap parsers ---

func (ap *AbilityParser) parseTapTarget(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])
	return &Ability{
		Name: "Tap Target",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     TapUntap,
				Value:    1, // positive => tap
				Duration: Instant,
				Targets:  []Target{{Type: targetType, Required: true, Count: 1}},
				Description: "Tap target " + matches[1],
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseTapAll(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])
	return &Ability{
		Name: "Tap All",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     TapUntap,
				Value:    1, // positive => tap
				Duration: Instant,
				Targets:  []Target{{Type: targetType, Required: false, Count: 0}}, // mass effect
				Description: "Tap all " + matches[1],
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseUntapTarget(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])
	return &Ability{
		Name: "Untap Target",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     TapUntap,
				Value:    -1, // negative => untap
				Duration: Instant,
				Targets:  []Target{{Type: targetType, Required: true, Count: 1}},
				Description: "Untap target " + matches[1],
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseUntapAllControlled(matches []string, fullText string) (*Ability, error) {
	targetType := ap.parseTargetType(matches[1])
	return &Ability{
		Name: "Untap All Controlled",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     TapUntap,
				Value:    -1,
				Duration: Instant,
				Targets:  []Target{{Type: targetType, Required: false, Count: 0}},
				Description: "Untap all " + matches[1] + " you control",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseFreezeTarget(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Freeze Target",
		Type: Activated,
		Effects: []Effect{
			{
				Type:     TapUntap,
				Value:    1,
				Duration: Instant,
				Targets:  []Target{{Type: CreatureTarget, Required: true, Count: 1}},
				Description: "Target creature doesn't untap during its controller's next untap step",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

// --- Phase 1: Discard parsers ---

func (ap *AbilityParser) parseDiscardCount(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "a", "an", "one":
		return 1
	case "two":
		return 2
	case "three":
		return 3
	case "four":
		return 4
	case "five":
		return 5
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return 1
}

func (ap *AbilityParser) parseTargetPlayerDiscard(matches []string, fullText string) (*Ability, error) {
	count := ap.parseDiscardCount(matches[1])
	return &Ability{
		Name: "Target Player Discards",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DiscardCards,
				Value:       count,
				Duration:    Instant,
				Targets:     []Target{{Type: PlayerTarget, Required: true, Count: 1}},
				Description: "Target player discards " + matches[1] + " cards",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseTargetOpponentDiscard(matches []string, fullText string) (*Ability, error) {
	count := ap.parseDiscardCount(matches[1])
	return &Ability{
		Name: "Target Opponent Discards",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DiscardCards,
				Value:       count,
				Duration:    Instant,
				Targets:     []Target{{Type: PlayerTarget, Required: true, Count: 1}},
				Description: "Target opponent discards " + matches[1] + " cards",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseEachPlayerDiscard(matches []string, fullText string) (*Ability, error) {
	count := ap.parseDiscardCount(matches[1])
	return &Ability{
		Name: "Each Player Discards",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DiscardCards,
				Value:       count,
				Duration:    Instant,
				Targets:     []Target{{Type: PlayerTarget, Required: false, Count: 0}}, // each is non-targeting
				Description: "Each player discards " + matches[1] + " cards",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseControllerDiscard(matches []string, fullText string) (*Ability, error) {
	count := ap.parseDiscardCount(matches[1])
	return &Ability{
		Name: "Controller Discards",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DiscardCards,
				Value:       count,
				Duration:    Instant,
				Targets:     []Target{{Type: CreatureTarget, Required: true, Count: 1}}, // target creature whose controller discards
				Description: "Target creature's controller discards " + matches[1] + " cards",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseCombatDamageDiscard(matches []string, fullText string) (*Ability, error) {
	count := ap.parseDiscardCount(matches[2])
	return &Ability{
		Name: "Combat Damage Discard",
		Type: Triggered,
		TriggerCondition: DealsCombatDamage,
		Effects: []Effect{
			{
				Type:        DiscardCards,
				Value:       count,
				Duration:    Instant,
				Targets:     []Target{{Type: PlayerTarget, Required: false, Count: 0}}, // player damaged
				Description: "That player discards " + matches[2] + " cards",
			},
		},
		IsOptional: false,
	}, nil
}

// --- Phase 1: Search library parsers ---

func (ap *AbilityParser) parseSearchBasicLand(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Search Basic Land",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        SearchLibrary,
				Value:       1,
				Duration:    Instant,
				Targets:     []Target{{Type: NoTarget, Required: false, Count: 0}},
				Description: "Search your library for a basic land card",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseSearchLand(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Search Land",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        SearchLibrary,
				Value:       1,
				Duration:    Instant,
				Targets:     []Target{{Type: NoTarget, Required: false, Count: 0}},
				Description: "Search your library for a land card",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseSearchMultipleBasicLands(matches []string, fullText string) (*Ability, error) {
	count := 1
	if len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			count = n
		}
	}
	return &Ability{
		Name: "Search Multiple Basic Lands",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        SearchLibrary,
				Value:       count,
				Duration:    Instant,
				Targets:     []Target{{Type: NoTarget, Required: false, Count: 0}},
				Description: "Search your library for up to " + matches[1] + " basic land cards",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseSearchAnyCard(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Search Any Card",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        SearchLibrary,
				Value:       1,
				Duration:    Instant,
				Targets:     []Target{{Type: NoTarget, Required: false, Count: 0}},
				Description: "Search your library for a card",
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}

func (ap *AbilityParser) parseSearchToBattlefield(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Search To Battlefield",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        SearchLibrary,
				Value:       1,
				Duration:    Instant,
				Targets:     []Target{{Type: NoTarget, Required: false, Count: 0}},
				Description: fullText,
			},
		},
		TimingRestriction: AnyTime,
	}, nil
}
