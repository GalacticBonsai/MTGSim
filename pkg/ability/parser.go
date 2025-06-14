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
		for abilityType, patterns := range ap.patterns {
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
			ManaCost: map[game.ManaType]int{game.Any: manaCost},
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
			ManaCost: map[game.ManaType]int{game.Any: manaCost},
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
			ManaCost: map[game.ManaType]int{game.Any: manaCost},
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
