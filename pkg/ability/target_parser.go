// Package ability provides target restriction parsing for MTG abilities.
package ability

import (
	"regexp"
	"strconv"
	"strings"
)

// TargetParser parses targeting restrictions from oracle text.
type TargetParser struct {
	patterns []*TargetPattern
}

// TargetPattern represents a regex pattern for parsing target restrictions.
type TargetPattern struct {
	Regex       *regexp.Regexp
	Parser      func(matches []string, fullText string) ([]TargetRestriction, error)
	Description string
}

// NewTargetParser creates a new target parser with predefined patterns.
func NewTargetParser() *TargetParser {
	parser := &TargetParser{
		patterns: make([]*TargetPattern, 0),
	}
	parser.initializePatterns()
	return parser
}

// initializePatterns sets up all the regex patterns for target restriction parsing.
// More specific patterns should be added first to ensure they match before generic ones.
func (tp *TargetParser) initializePatterns() {
	// Complex restrictions first (most specific)
	tp.addPattern(`target\s+non-artifact\s+creature`, tp.parseNonArtifactCreature, "Target non-artifact creature")
	tp.addPattern(`target\s+non-land\s+permanent`, tp.parseNonLandPermanent, "Target non-land permanent")
	tp.addPattern(`target\s+nonland\s+permanent`, tp.parseNonLandPermanent, "Target nonland permanent")
	tp.addPattern(`target\s+non-creature\s+permanent`, tp.parseNonCreaturePermanent, "Target non-creature permanent")

	// Ability-based restrictions
	tp.addPattern(`target\s+creature\s+with\s+flying`, tp.parseCreatureWithFlying, "Target creature with flying")
	tp.addPattern(`target\s+creature\s+with\s+trample`, tp.parseCreatureWithTrample, "Target creature with trample")
	tp.addPattern(`target\s+creature\s+with\s+vigilance`, tp.parseCreatureWithVigilance, "Target creature with vigilance")
	tp.addPattern(`target\s+creature\s+with\s+first\s+strike`, tp.parseCreatureWithFirstStrike, "Target creature with first strike")
	tp.addPattern(`target\s+creature\s+with\s+deathtouch`, tp.parseCreatureWithDeathtouch, "Target creature with deathtouch")

	// Power/toughness restrictions
	tp.addPattern(`target\s+creature\s+with\s+power\s+(\d+)\s+or\s+less`, tp.parseCreaturePowerLessEqual, "Target creature with power X or less")
	tp.addPattern(`target\s+creature\s+with\s+power\s+(\d+)\s+or\s+greater`, tp.parseCreaturePowerGreaterEqual, "Target creature with power X or greater")
	tp.addPattern(`target\s+creature\s+with\s+toughness\s+(\d+)\s+or\s+less`, tp.parseCreatureToughnessLessEqual, "Target creature with toughness X or less")
	tp.addPattern(`target\s+creature\s+with\s+toughness\s+(\d+)\s+or\s+greater`, tp.parseCreatureToughnessGreaterEqual, "Target creature with toughness X or greater")

	// CMC restrictions
	tp.addPattern(`target\s+creature\s+with\s+converted\s+mana\s+cost\s+(\d+)\s+or\s+less`, tp.parseCreatureCMCLessEqual, "Target creature with CMC X or less")
	tp.addPattern(`target\s+creature\s+with\s+mana\s+cost\s+(\d+)\s+or\s+less`, tp.parseCreatureCMCLessEqual, "Target creature with mana cost X or less")
	tp.addPattern(`target\s+permanent\s+with\s+converted\s+mana\s+cost\s+(\d+)\s+or\s+less`, tp.parsePermanentCMCLessEqual, "Target permanent with CMC X or less")

	// Control restrictions
	tp.addPattern(`target\s+creature\s+you\s+control`, tp.parseCreatureYouControl, "Target creature you control")
	tp.addPattern(`target\s+creature\s+you\s+don't\s+control`, tp.parseCreatureYouDontControl, "Target creature you don't control")
	tp.addPattern(`target\s+permanent\s+you\s+control`, tp.parsePermanentYouControl, "Target permanent you control")
	tp.addPattern(`target\s+permanent\s+you\s+don't\s+control`, tp.parsePermanentYouDontControl, "Target permanent you don't control")

	// State-based restrictions
	tp.addPattern(`target\s+tapped\s+creature`, tp.parseTappedCreature, "Target tapped creature")
	tp.addPattern(`target\s+untapped\s+creature`, tp.parseUntappedCreature, "Target untapped creature")
	tp.addPattern(`target\s+attacking\s+creature`, tp.parseAttackingCreature, "Target attacking creature")
	tp.addPattern(`target\s+blocking\s+creature`, tp.parseBlockingCreature, "Target blocking creature")

	// Multiple target types
	tp.addPattern(`target\s+player\s+or\s+planeswalker`, tp.parsePlayerOrPlaneswalker, "Target player or planeswalker")
	tp.addPattern(`any\s+target`, tp.parseAnyTarget, "Any target")

	// Basic type restrictions (most generic - should be last)
	tp.addPattern(`target\s+creature`, tp.parseBasicCreature, "Target creature")
	tp.addPattern(`target\s+player`, tp.parseBasicPlayer, "Target player")
	tp.addPattern(`target\s+permanent`, tp.parseBasicPermanent, "Target permanent")
	tp.addPattern(`target\s+artifact`, tp.parseBasicArtifact, "Target artifact")
	tp.addPattern(`target\s+enchantment`, tp.parseBasicEnchantment, "Target enchantment")
	tp.addPattern(`target\s+land`, tp.parseBasicLand, "Target land")
	tp.addPattern(`target\s+planeswalker`, tp.parseBasicPlaneswalker, "Target planeswalker")

	// Each patterns (non-targeting)
	tp.addPattern(`each\s+creature`, tp.parseEachCreature, "Each creature")
	tp.addPattern(`each\s+player`, tp.parseEachPlayer, "Each player")
	tp.addPattern(`each\s+opponent`, tp.parseEachOpponent, "Each opponent")
	tp.addPattern(`each\s+artifact`, tp.parseEachArtifact, "Each artifact")

	// Complex compound restrictions
	tp.addPattern(`target\s+non-artifact,\s+non-black\s+creature`, tp.parseComplexRestrictions, "Complex restrictions")
}

// addPattern adds a new pattern to the parser.
func (tp *TargetParser) addPattern(pattern string, parser func([]string, string) ([]TargetRestriction, error), description string) {
	regex := regexp.MustCompile(`(?i)` + pattern) // Case insensitive
	targetPattern := &TargetPattern{
		Regex:       regex,
		Parser:      parser,
		Description: description,
	}
	tp.patterns = append(tp.patterns, targetPattern)
}

// ParseTargetRestrictions parses target restrictions from oracle text.
func (tp *TargetParser) ParseTargetRestrictions(oracleText string) ([]EnhancedTarget, error) {
	var enhancedTargets []EnhancedTarget

	// Split oracle text by sentences for better parsing
	sentences := tp.splitOracleText(oracleText)

	for _, sentence := range sentences {
		for _, pattern := range tp.patterns {
			if matches := pattern.Regex.FindStringSubmatch(sentence); matches != nil {
				restrictions, err := pattern.Parser(matches, sentence)
				if err != nil {
					continue // Skip this pattern and try others
				}

				// Determine target type and properties
				enhancedTarget := tp.createEnhancedTarget(sentence, restrictions)
				if enhancedTarget != nil {
					enhancedTargets = append(enhancedTargets, *enhancedTarget)
				}
				break // Found a match, don't try other patterns for this sentence
			}
		}
	}

	return enhancedTargets, nil
}

// createEnhancedTarget creates an EnhancedTarget from parsed restrictions.
func (tp *TargetParser) createEnhancedTarget(sentence string, restrictions []TargetRestriction) *EnhancedTarget {
	sentence = strings.ToLower(sentence)

	// Determine if this is an "each" effect (non-targeting)
	isEach := strings.Contains(sentence, "each ")

	// Determine basic target type
	var targetType TargetType
	if strings.Contains(sentence, "creature") {
		targetType = CreatureTarget
	} else if strings.Contains(sentence, "player") {
		targetType = PlayerTarget
	} else if strings.Contains(sentence, "permanent") {
		targetType = PermanentTarget
	} else if strings.Contains(sentence, "any target") {
		targetType = AnyTarget
	} else {
		targetType = AnyTarget // Default
	}

	// Determine if targeting is required
	required := strings.Contains(sentence, "target") && !isEach

	return &EnhancedTarget{
		Type:         targetType,
		Required:     required,
		Count:        1, // Most abilities target one thing
		Restrictions: restrictions,
		IsEach:       isEach,
		Description:  sentence,
	}
}

// splitOracleText splits oracle text into individual sentences for parsing.
func (tp *TargetParser) splitOracleText(text string) []string {
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

// Parser functions for different restriction types

func (tp *TargetParser) parseBasicCreature(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
	}, nil
}

func (tp *TargetParser) parseBasicPlayer(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PlayerRestriction, Description: "must be a player"},
	}, nil
}

func (tp *TargetParser) parseBasicPermanent(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PermanentRestriction, Description: "must be a permanent"},
	}, nil
}

func (tp *TargetParser) parseBasicArtifact(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: ArtifactRestriction, Description: "must be an artifact"},
	}, nil
}

func (tp *TargetParser) parseBasicEnchantment(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: EnchantmentRestriction, Description: "must be an enchantment"},
	}, nil
}

func (tp *TargetParser) parseBasicLand(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: LandRestriction, Description: "must be a land"},
	}, nil
}

func (tp *TargetParser) parseBasicPlaneswalker(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PlaneswalkerRestriction, Description: "must be a planeswalker"},
	}, nil
}

func (tp *TargetParser) parseNonArtifactCreature(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: ArtifactRestriction, Negated: true, Description: "must not be an artifact"},
	}, nil
}

func (tp *TargetParser) parseNonLandPermanent(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PermanentRestriction, Description: "must be a permanent"},
		{Type: LandRestriction, Negated: true, Description: "must not be a land"},
	}, nil
}

func (tp *TargetParser) parseNonCreaturePermanent(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PermanentRestriction, Description: "must be a permanent"},
		{Type: CreatureRestriction, Negated: true, Description: "must not be a creature"},
	}, nil
}

func (tp *TargetParser) parseCreatureWithFlying(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: FlyingRestriction, Description: "must have flying"},
	}, nil
}

func (tp *TargetParser) parseCreatureWithTrample(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: TrampleRestriction, Description: "must have trample"},
	}, nil
}

func (tp *TargetParser) parseCreatureWithVigilance(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: VigilanceRestriction, Description: "must have vigilance"},
	}, nil
}

func (tp *TargetParser) parseCreatureWithFirstStrike(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: FirstStrikeRestriction, Description: "must have first strike"},
	}, nil
}

func (tp *TargetParser) parseCreatureWithDeathtouch(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: DeathtouchRestriction, Description: "must have deathtouch"},
	}, nil
}

func (tp *TargetParser) parseCreaturePowerLessEqual(matches []string, fullText string) ([]TargetRestriction, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	power, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: PowerLessEqualRestriction, Value: power, Description: "power " + matches[1] + " or less"},
	}, nil
}

func (tp *TargetParser) parseCreaturePowerGreaterEqual(matches []string, fullText string) ([]TargetRestriction, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	power, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: PowerGreaterEqualRestriction, Value: power, Description: "power " + matches[1] + " or greater"},
	}, nil
}

func (tp *TargetParser) parseCreatureToughnessLessEqual(matches []string, fullText string) ([]TargetRestriction, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: ToughnessLessEqualRestriction, Value: toughness, Description: "toughness " + matches[1] + " or less"},
	}, nil
}

func (tp *TargetParser) parseCreatureToughnessGreaterEqual(matches []string, fullText string) ([]TargetRestriction, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	toughness, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: ToughnessGreaterEqualRestriction, Value: toughness, Description: "toughness " + matches[1] + " or greater"},
	}, nil
}

func (tp *TargetParser) parseCreatureCMCLessEqual(matches []string, fullText string) ([]TargetRestriction, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	cmc, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: CMCLessEqualRestriction, Value: cmc, Description: "CMC " + matches[1] + " or less"},
	}, nil
}

func (tp *TargetParser) parsePermanentCMCLessEqual(matches []string, fullText string) ([]TargetRestriction, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}

	cmc, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, ErrParsingFailed
	}

	return []TargetRestriction{
		{Type: PermanentRestriction, Description: "must be a permanent"},
		{Type: CMCLessEqualRestriction, Value: cmc, Description: "CMC " + matches[1] + " or less"},
	}, nil
}

func (tp *TargetParser) parseCreatureYouControl(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: YouControlRestriction, Description: "you must control"},
	}, nil
}

func (tp *TargetParser) parseCreatureYouDontControl(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: YouDontControlRestriction, Description: "you must not control"},
	}, nil
}

func (tp *TargetParser) parsePermanentYouControl(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PermanentRestriction, Description: "must be a permanent"},
		{Type: YouControlRestriction, Description: "you must control"},
	}, nil
}

func (tp *TargetParser) parsePermanentYouDontControl(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PermanentRestriction, Description: "must be a permanent"},
		{Type: YouDontControlRestriction, Description: "you must not control"},
	}, nil
}

func (tp *TargetParser) parseTappedCreature(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: TappedRestriction, Description: "must be tapped"},
	}, nil
}

func (tp *TargetParser) parseUntappedCreature(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: UntappedRestriction, Description: "must be untapped"},
	}, nil
}

func (tp *TargetParser) parseAttackingCreature(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: AttackingRestriction, Description: "must be attacking"},
	}, nil
}

func (tp *TargetParser) parseBlockingCreature(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: BlockingRestriction, Description: "must be blocking"},
	}, nil
}

func (tp *TargetParser) parsePlayerOrPlaneswalker(matches []string, fullText string) ([]TargetRestriction, error) {
	// This creates a special case where either players or planeswalkers are valid
	return []TargetRestriction{
		{Type: NoRestriction, Description: "player or planeswalker"},
	}, nil
}

func (tp *TargetParser) parseAnyTarget(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: NoRestriction, Description: "any target"},
	}, nil
}

func (tp *TargetParser) parseEachCreature(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "each creature"},
	}, nil
}

func (tp *TargetParser) parseEachPlayer(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PlayerRestriction, Description: "each player"},
	}, nil
}

func (tp *TargetParser) parseEachOpponent(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: PlayerRestriction, Description: "each opponent"},
		{Type: OpponentControlsRestriction, Description: "must be an opponent"},
	}, nil
}

func (tp *TargetParser) parseEachArtifact(matches []string, fullText string) ([]TargetRestriction, error) {
	return []TargetRestriction{
		{Type: ArtifactRestriction, Description: "each artifact"},
	}, nil
}

func (tp *TargetParser) parseComplexRestrictions(matches []string, fullText string) ([]TargetRestriction, error) {
	// This would handle complex compound restrictions
	// For now, return a basic implementation
	return []TargetRestriction{
		{Type: CreatureRestriction, Description: "must be a creature"},
		{Type: ArtifactRestriction, Negated: true, Description: "must not be an artifact"},
	}, nil
}
