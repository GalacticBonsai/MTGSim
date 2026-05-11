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

	// Keyword abilities — broad regex for comma-separated keyword lists on a single line.
	ap.addPattern(Static, `(?i)^(?:(?:Flying|Trample|Haste|Vigilance|First strike|Lifelink|Deathtouch|Menace|Reach|Hexproof|Defender|Flash|Indestructible|Double strike|Shroud|Intimidate|Fear|Shadow|Infect|Wither|Prowess|Cascade|Convoke|Delve|Dredge|Persist|Undying|Unearth|Morph|Manifest|Embalm|Eternalize|Aftermath|Adventure|Mutate|Foretell|Strive|Rebound|Suspend|Madness|Buyback|Replicate|Splice|Transmute|Regenerate|Ward\s*\{[^}]+\}|Bloodthirst\s*\d+|Annihilator\s*\d+|Protection from [^.,;()]+)(?:\s*\([^)]*\)|\s*[,;]\s*|)*)+$`, KeywordAbility, "Keyword abilities", ap.parseKeywordAbilities)

	// Aura enchantments
	ap.addPattern(Static, `(?i)^Enchant\s+(creature|land|artifact|enchantment|planeswalker|permanent)$`, PumpCreature, "Aura enchantment", ap.parseAuraEnchantment)
	ap.addPattern(Static, `(?i)^Enchant\s+(creature|land|artifact|enchantment|planeswalker|permanent)\s+with\s+mana\s+value\s+\d+\s+or\s+(?:less|greater)$`, PumpCreature, "Aura enchantment with restriction", ap.parseAuraEnchantment)

	// Equipped creature gets...
	ap.addPattern(Static, `(?i)^Equipped\s+creature\s+gets\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "Equipment pump", ap.parseEquippedPump)
	ap.addPattern(Static, `(?i)^Equipped\s+creature\s+has\s+.+`, KeywordAbility, "Equipment grant ability", ap.parseEquippedGrant)

	// Exile effects
	ap.addPattern(Activated, `(?i)^Exile\s+target\s+.+$`, Exile, "Exile target", ap.parseExileEffect)
	ap.addPattern(Activated, `(?i)^Exile\s+all\s+.+$`, Exile, "Exile all", ap.parseExileEffect)
	ap.addPattern(Activated, `(?i)^Exile\s+target\s+creature\s+you\s+control,\s+then\s+return\s+it\s+to\s+the\s+battlefield.+`, Exile, "Flicker", ap.parseFlickerEffect)
	ap.addPattern(Activated, `(?i)^Exile\s+target\s+creature.+then\s+return\s+that\s+card\s+to\s+the\s+battlefield.+`, Exile, "Flicker", ap.parseFlickerEffect)

	// More return patterns
	ap.addPattern(Activated, `(?i)^Return\s+target\s+.+\s+to\s+its\s+owner'?s\s+hand\.?$`, ReturnToHand, "Return target to hand", ap.parseReturnToHandGeneric)
	ap.addPattern(Activated, `(?i)^Return\s+all\s+.+\s+to\s+their\s+owners'?\s+hands\.?$`, ReturnToHand, "Return all to hands", ap.parseReturnAllToHands)

	// More token creation patterns
	ap.addPattern(Activated, `(?i)^Create\s+a\s+\d+/\d+\s+.+\s+creature\s+token`, CreateToken, "Create specific token", ap.parseCreateSpecificToken)
	ap.addPattern(Activated, `(?i)^Create\s+\d+\s+\d+/\d+\s+.+\s+creature\s+tokens?`, CreateToken, "Create multiple tokens", ap.parseCreateMultipleTokens)

	// Additional common spell patterns
	ap.addPattern(Activated, `(?i)^Target\s+(?:player|opponent)\s+loses\s+(\d+)\s+life`, LoseLife, "Target loses life", ap.parseTargetLosesLife)
	ap.addPattern(Activated, `(?i)^Target\s+creature\s+gets\s+-([0-9]+)/-([0-9]+)\s+until\s+end\s+of\s+turn`, PumpCreature, "Target debuff", ap.parseTargetDebuff)
	ap.addPattern(Activated, `(?i)^Target\s+creature\s+has\s+base\s+power\s+and\s+toughness\s+(\d+)/(\d+)`, PumpCreature, "Set base P/T", ap.parseSetBasePT)
	ap.addPattern(Activated, `(?i)^Each\s+player\s+loses\s+(\d+)\s+life`, LoseLife, "Each loses life", ap.parseEachLosesLife)
	ap.addPattern(Activated, `(?i)^Each\s+player\s+draws?\s+(\d+)\s+cards?`, DrawCards, "Each draws", ap.parseEachDraws)
	ap.addPattern(Activated, `(?i)^You\s+gain\s+(\d+)\s+life`, GainLife, "You gain life", ap.parseYouGainLife)

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

	// Broad ETB trigger patterns for common effects
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+exile\s+target\s+(.*)`, Exile, "ETB exile", ap.parseETBExile)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+return\s+target\s+(.*)\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "ETB return", ap.parseETBReturn)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+destroy\s+target\s+(.*)`, DestroyPermanent, "ETB destroy", ap.parseETBDestroy)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "ETB search", ap.parseETBSearch)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "ETB create token", ap.parseETBCreateToken)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+target\s+creature\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "ETB pump", ap.parseETBPump)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+target\s+creature\s+gets\s+-([0-9]+)/-([0-9]+)`, PumpCreature, "ETB debuff", ap.parseETBDebuff)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+target\s+player\s+loses\s+(\d+)\s+life`, LoseLife, "ETB lose life", ap.parseETBLoseLife)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+target\s+player\s+gains\s+(\d+)\s+life`, GainLife, "ETB gain life", ap.parseETBGainLife)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+draw\s+(a|\d+)\s+cards?`, DrawCards, "ETB draw", ap.parseDeathDraw)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "ETB damage", ap.parseAttackDamage)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+.*\s+gains\s+(\d+)\s+life`, GainLife, "ETB gain life broad", ap.parseETBGainLife)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+.*\s+lose\s+(\d+)\s+life`, LoseLife, "ETB lose life broad", ap.parseETBLoseLife)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+.*\s+tap\s+target\s+(.*)`, TapUntap, "ETB tap", ap.parseETBTap)

	// Attack triggers
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks(?:\s+and\s+isn'?t\s+blocked)?,\s+.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "Attack damage trigger", ap.parseAttackDamage)
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks(?:\s+and\s+isn'?t\s+blocked)?,\s+.*\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Attack pump trigger", ap.parseAttackPump)
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks(?:\s+and\s+isn'?t\s+blocked)?,\s+draw\s+(a|\d+)\s+cards?`, DrawCards, "Attack draw trigger", ap.parseAttackDraw)
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks(?:\s+and\s+isn'?t\s+blocked)?,\s+.*\s+gains\s+\+(\d+)/\+(\d+)`, PumpCreature, "Attack pump broad", ap.parseAttackPump)
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks(?:\s+and\s+isn'?t\s+blocked)?,\s+.*\s+gains\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|menace|reach|hexproof)`, KeywordAbility, "Attack gains keyword", ap.parseAttackGainKeyword)
	ap.addPattern(Triggered, `Whenever\s+a\s+creature\s+attacks(?:\s+this\s+turn)?,\s+(.*)`, KeywordAbility, "Any attack trigger", ap.parseAnyAttackTrigger)

	// Combat damage triggers
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+player,\s+.*\s+gains\s+(\d+)\s+life`, GainLife, "Combat damage gain life", ap.parseCombatDamageGainLife)
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+player,\s+.*\s+loses\s+(\d+)\s+life`, LoseLife, "Combat damage lose life", ap.parseCombatDamageLoseLife)
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+player,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "Combat damage token", ap.parseCombatDamageToken)
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+player,\s+exile\s+(.*)`, Exile, "Combat damage exile", ap.parseCombatDamageExile)

	// Step triggers
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+.*\s+loses\s+(\d+)\s+life`, LoseLife, "Upkeep lose life", ap.parseUpkeepLoseLife)
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+.*\s+gains\s+(\d+)\s+life`, GainLife, "Upkeep gain life", ap.parseUpkeepGainLife)
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "Upkeep damage", ap.parseUpkeepDamage)
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+combat\s+on\s+your\s+turn,\s+(.*)`, DrawCards, "Combat step trigger", ap.parseCombatStepTrigger)

	// Dies triggers
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+draw\s+(a|\d+)\s+cards?`, DrawCards, "Death draw", ap.parseDeathDraw)
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+return\s+it\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "Death return", ap.parseDeathReturn)
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "Death token", ap.parseDeathToken)
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+exile\s+(.*)`, Exile, "Death exile", ap.parseDeathExile)

	// Block triggers
	ap.addPattern(Triggered, `Whenever\s+.*\s+blocks,\s+.*\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Block pump", ap.parseBlockPump)
	ap.addPattern(Triggered, `Whenever\s+.*\s+blocks,\s+.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "Block damage", ap.parseBlockDamage)
	ap.addPattern(Triggered, `Whenever\s+.*\s+blocks,\s+draw\s+(a|\d+)\s+cards?`, DrawCards, "Block draw", ap.parseBlockDraw)

	// More spell patterns
	ap.addPattern(Activated, `(?i)^Target\s+creature\s+gets\s+\+(\d+)/\+(\d+)\s+and\s+gains\s+.+`, PumpCreature, "Target pump and gains keyword", ap.parseTargetPumpAndGain)
	ap.addPattern(Activated, `(?i)^Target\s+creature\s+you\s+control\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Target controlled pump", ap.parseTargetControlledPump)
	ap.addPattern(Activated, `(?i)^Target\s+creature\s+gains\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|menace|reach|hexproof|indestructible|flash|defender)\s+until\s+end\s+of\s+turn`, KeywordAbility, "Target gains keyword EOT", ap.parseTargetGainsKeyword)
	ap.addPattern(Activated, `(?i)^Target\s+creature\s+loses\s+.+`, GenericEffect, "Target loses ability", ap.parseGenericActivated)
	ap.addPattern(Activated, `(?i)^Return\s+target\s+.+\s+from\s+your\s+graveyard\s+to\s+your\s+hand`, ReturnToHand, "Return from graveyard", ap.parseReturnFromGraveyard)
	ap.addPattern(Activated, `(?i)^Create\s+(?:two|three|four|five|six|seven|eight|nine|ten|\d+|X)\s+\d+/\d+\s+.+\s+creature\s+tokens?`, CreateToken, "Create multiple tokens", ap.parseCreateMultipleTokens)
	ap.addPattern(Activated, `(?i)^Look\s+at\s+the\s+top\s+.+\s+cards?\s+of\s+your\s+library`, GenericEffect, "Look at library", ap.parseGenericActivated)
	ap.addPattern(Activated, `(?i)^Counter\s+target\s+(?:creature|artifact|enchantment|instant|sorcery|planeswalker|creature\s+or\s+enchantment|artifact\s+or\s+creature|instant\s+or\s+sorcery)\s+spell`, CounterSpell, "Counter type spell", ap.parseCounterTypeSpell)
	ap.addPattern(Activated, `(?i)^Counter\s+target\s+spell`, CounterSpell, "Counter any spell broad", ap.parseGenericActivated)
	ap.addPattern(Activated, `(?i)^Put\s+.+\s+counter`, GenericEffect, "Put counter", ap.parseGenericActivated)
	ap.addPattern(Activated, `(?i)^Prevent\s+all\s+damage`, PreventDamage, "Prevent all damage", ap.parsePreventAllDamage)
	ap.addPattern(Activated, `(?i)^Prevent\s+the\s+next\s+\d+\s+damage`, PreventDamage, "Prevent next damage", ap.parsePreventNextDamage)
	ap.addPattern(Activated, `(?i)^Each\s+player\s+.*\s+loses?\s+\d+\s+life`, LoseLife, "Each loses life broad", ap.parseEachLosesLife)
	ap.addPattern(Activated, `(?i)^All\s+creatures\s+get\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "All creatures pump", ap.parseAllPump)
	ap.addPattern(Activated, `(?i)^All\s+creatures\s+lose\s+.+`, GenericEffect, "All creatures lose ability", ap.parseGenericActivated)
	ap.addPattern(Activated, `(?i)^You\s+may\s+play\s+an\s+additional\s+land`, GenericEffect, "Additional land", ap.parseGenericActivated)
	ap.addPattern(Activated, `(?i)^You\s+have\s+(?:shroud|hexproof|protection)`, GenericEffect, "You have ability", ap.parseGenericActivated)
	ap.addPattern(Activated, `(?i)^You\s+may\s+discard\s+.+`, DiscardCards, "You may discard", ap.parseGenericActivated)
	ap.addPattern(Activated, `(?i)^Sacrifice\s+[^:]+:.*`, GenericEffect, "Sacrifice cost ability", ap.parseGenericActivated)

	// Static patterns
	ap.addPattern(Static, `(?i)^As\s+long\s+as\s+.+`, GenericEffect, "As long as static", ap.parseGenericStatic)
	ap.addPattern(Static, `(?i)^Each\s+creature\s+you\s+control\s+.*`, GenericEffect, "Each creature static", ap.parseGenericStatic)
	ap.addPattern(Static, `(?i)^Each\s+other\s+creature\s+.*`, GenericEffect, "Each other static", ap.parseGenericStatic)
	ap.addPattern(Static, `(?i)^Creatures\s+you\s+control\s+.*`, GenericEffect, "Creatures static", ap.parseGenericStatic)
	ap.addPattern(Static, `(?i)^Other\s+creatures\s+you\s+control\s+.*`, GenericEffect, "Other creatures static", ap.parseGenericStatic)
	ap.addPattern(Static, `(?i)^If\s+.+`, GenericEffect, "If conditional static", ap.parseGenericStatic)
	ap.addPattern(Static, `(?i)^This\s+creature\s+.*`, GenericEffect, "This creature static", ap.parseGenericStatic)
	ap.addPattern(Static, `(?i)^Until\s+.+`, GenericEffect, "Until static", ap.parseGenericStatic)

	// Broad catch-all patterns for common first words (lowest priority within type)
	ap.addPattern(Triggered, `^When\s+.*`, GenericEffect, "When catch-all", ap.parseGenericTriggered)
	ap.addPattern(Triggered, `^Whenever\s+.*`, GenericEffect, "Whenever catch-all", ap.parseGenericTriggered)
	ap.addPattern(Triggered, `^At\s+.*`, GenericEffect, "At catch-all", ap.parseGenericTriggered)

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
// It splits by newlines first (abilities are usually line-separated) and
// then by periods to catch multiple sentences on the same line.
func (ap *AbilityParser) splitOracleText(text string) []string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ".")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
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

func (ap *AbilityParser) parseKeywordAbilities(matches []string, fullText string) (*Ability, error) {
	parts := strings.Split(fullText, ",")
	var effects []Effect
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Strip reminder text in parentheses
		if idx := strings.Index(part, "("); idx != -1 {
			part = strings.TrimSpace(part[:idx])
		}
		if part == "" {
			continue
		}
		effects = append(effects, Effect{
			Type:        KeywordAbility,
			Duration:    Permanent,
			Description: part,
		})
	}
	if len(effects) == 0 {
			return nil, ErrParsingFailed
		}
		return &Ability{
			Name:    "Keyword Abilities",
			Type:    Static,
			Effects: effects,
		}, nil
	}

func (ap *AbilityParser) parseAuraEnchantment(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Aura Enchantment",
		Type: Static,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Duration:    Permanent,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseEquippedPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return &Ability{
		Name: "Equipment Pump",
		Type: Static,
		Effects: []Effect{
			{
				Type:     PumpCreature,
				Value:    power*1000 + toughness,
				Duration: Permanent,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseEquippedGrant(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Equipment Grant Ability",
		Type: Static,
		Effects: []Effect{
			{
				Type:        KeywordAbility,
				Duration:    Permanent,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseExileEffect(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Exile",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        Exile,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseFlickerEffect(matches []string, fullText string) (*Ability, error) {
	// Flicker: exile then return
	return &Ability{
		Name: "Flicker",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        Exile,
				Duration:    Instant,
				Description: fullText,
			},
			{
				Type:        ReturnToHand,
				Duration:    Instant,
				Description: "return to battlefield",
			},
		},
	}, nil
}

func (ap *AbilityParser) parseReturnToHandGeneric(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Return to Hand",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        ReturnToHand,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseReturnAllToHands(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Return All to Hands",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        ReturnToHand,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseCreateSpecificToken(matches []string, fullText string) (*Ability, error) {
	// Parse "Create a X/Y ... creature token"
	re := regexp.MustCompile(`(?i)Create a (\d+)/(\d+) (.+?) creature token`)
	m := re.FindStringSubmatch(fullText)
	if len(m) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(m[1])
	toughness, _ := strconv.Atoi(m[2])
	return &Ability{
		Name: "Create Token",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        CreateToken,
				Value:       1*1000000 + power*1000 + toughness,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseCreateMultipleTokens(matches []string, fullText string) (*Ability, error) {
	// Parse "Create N X/Y ... creature tokens"
	re := regexp.MustCompile(`(?i)Create (\d+) (\d+)/(\d+) (.+?) creature tokens?`)
	m := re.FindStringSubmatch(fullText)
	if len(m) < 4 {
		return nil, ErrParsingFailed
	}
	count, _ := strconv.Atoi(m[1])
	power, _ := strconv.Atoi(m[2])
	toughness, _ := strconv.Atoi(m[3])
	return &Ability{
		Name: "Create Tokens",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        CreateToken,
				Value:       count*1000000 + power*1000 + toughness,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseTargetLosesLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	value, _ := strconv.Atoi(matches[1])
	return &Ability{
		Name: "Target Loses Life",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        LoseLife,
				Value:       value,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseTargetDebuff(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return &Ability{
		Name: "Target Debuff",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Value:       -power*1000 - toughness,
				Duration:    UntilEndOfTurn,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseSetBasePT(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return &Ability{
		Name: "Set Base P/T",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Value:       power*1000 + toughness,
				Duration:    UntilEndOfTurn,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseEachLosesLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	value, _ := strconv.Atoi(matches[1])
	return &Ability{
		Name: "Each Loses Life",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        LoseLife,
				Value:       value,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseEachDraws(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	value, _ := strconv.Atoi(matches[1])
	return &Ability{
		Name: "Each Draws",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DrawCards,
				Value:       value,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseYouGainLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	value, _ := strconv.Atoi(matches[1])
	return &Ability{
		Name: "Gain Life",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        GainLife,
				Value:       value,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func makeTriggeredAbility(name string, effType EffectType, value int, dur EffectDuration, fullText string) *Ability {
	return &Ability{
		Name:             name,
		Type:             Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:        effType,
				Value:       value,
				Duration:    dur,
				Description: fullText,
			},
		},
		IsOptional: false,
	}
}

func makeTriggeredAbilityWithCondition(name string, effType EffectType, value int, dur EffectDuration, fullText string, cond TriggerCondition) *Ability {
	return &Ability{
		Name:             name,
		Type:             Triggered,
		TriggerCondition: cond,
		Effects: []Effect{
			{
				Type:        effType,
				Value:       value,
				Duration:    dur,
				Description: fullText,
			},
		},
		IsOptional: false,
	}
}

func parseIntOrOne(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "a" || s == "" {
		return 1
	}
	v, _ := strconv.Atoi(s)
	if v == 0 {
		return 1
	}
	return v
}

// ETB trigger parsers
func (ap *AbilityParser) parseETBExile(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbility("ETB Exile", Exile, 0, Instant, fullText), nil
}
func (ap *AbilityParser) parseETBReturn(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbility("ETB Return", ReturnToHand, 0, Instant, fullText), nil
}
func (ap *AbilityParser) parseETBDestroy(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbility("ETB Destroy", DestroyPermanent, 0, Instant, fullText), nil
}
func (ap *AbilityParser) parseETBSearch(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbility("ETB Search", SearchLibrary, 1, Instant, fullText), nil
}
func (ap *AbilityParser) parseETBCreateToken(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbility("ETB Create Token", CreateToken, 0, Instant, fullText), nil
}
func (ap *AbilityParser) parseETBPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return makeTriggeredAbility("ETB Pump", PumpCreature, power*1000+toughness, Instant, fullText), nil
}
func (ap *AbilityParser) parseETBDebuff(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return makeTriggeredAbility("ETB Debuff", PumpCreature, -power*1000-toughness, Instant, fullText), nil
}
func (ap *AbilityParser) parseETBLoseLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return makeTriggeredAbility("ETB Lose Life", LoseLife, v, Instant, fullText), nil
}
// Attack trigger parsers
func (ap *AbilityParser) parseAttackDamage(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return makeTriggeredAbilityWithCondition("Attack Damage", DealDamage, v, Instant, fullText, AttacksOrBlocks), nil
}
func (ap *AbilityParser) parseAttackPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return makeTriggeredAbilityWithCondition("Attack Pump", PumpCreature, power*1000+toughness, Instant, fullText, AttacksOrBlocks), nil
}
func (ap *AbilityParser) parseAttackDraw(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v := parseIntOrOne(matches[1])
	return makeTriggeredAbilityWithCondition("Attack Draw", DrawCards, v, Instant, fullText, AttacksOrBlocks), nil
}

// Combat damage trigger parsers
func (ap *AbilityParser) parseCombatDamageGainLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return makeTriggeredAbilityWithCondition("Combat Gain Life", GainLife, v, Instant, fullText, DealsCombatDamage), nil
}
func (ap *AbilityParser) parseCombatDamageLoseLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return makeTriggeredAbilityWithCondition("Combat Lose Life", LoseLife, v, Instant, fullText, DealsCombatDamage), nil
}
func (ap *AbilityParser) parseCombatDamageToken(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("Combat Token", CreateToken, 0, Instant, fullText, DealsCombatDamage), nil
}
func (ap *AbilityParser) parseCombatDamageExile(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("Combat Exile", Exile, 0, Instant, fullText, DealsCombatDamage), nil
}

// Upkeep trigger parsers
func (ap *AbilityParser) parseUpkeepLoseLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return makeTriggeredAbilityWithCondition("Upkeep Lose Life", LoseLife, v, Instant, fullText, BeginningOfUpkeep), nil
}
func (ap *AbilityParser) parseUpkeepGainLife(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return makeTriggeredAbilityWithCondition("Upkeep Gain Life", GainLife, v, Instant, fullText, BeginningOfUpkeep), nil
}
func (ap *AbilityParser) parseUpkeepDamage(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return makeTriggeredAbilityWithCondition("Upkeep Damage", DealDamage, v, Instant, fullText, BeginningOfUpkeep), nil
}
func (ap *AbilityParser) parseCombatStepTrigger(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("Combat Step", DrawCards, 0, Instant, fullText, AttacksOrBlocks), nil
}

// Death trigger parsers
func (ap *AbilityParser) parseDeathDraw(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v := parseIntOrOne(matches[1])
	return makeTriggeredAbilityWithCondition("Death Draw", DrawCards, v, Instant, fullText, Dies), nil
}

func (ap *AbilityParser) parseDeathReturn(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("Death Return", ReturnToHand, 0, Instant, fullText, Dies), nil
}
func (ap *AbilityParser) parseDeathToken(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("Death Token", CreateToken, 0, Instant, fullText, Dies), nil
}
func (ap *AbilityParser) parseDeathExile(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("Death Exile", Exile, 0, Instant, fullText, Dies), nil
}


func (ap *AbilityParser) parseETBTap(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbility("ETB Tap", TapUntap, 0, Instant, fullText), nil
}

func (ap *AbilityParser) parseAttackGainKeyword(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("Attack Gain Keyword", KeywordAbility, 0, Instant, fullText, AttacksOrBlocks), nil
}

func (ap *AbilityParser) parseAnyAttackTrigger(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("Any Attack Trigger", KeywordAbility, 0, Instant, fullText, AttacksOrBlocks), nil
}

func (ap *AbilityParser) parseEndStepTrigger(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("End Step Trigger", DrawCards, 0, Instant, fullText, EndOfTurn), nil
}

func (ap *AbilityParser) parseEndOfCombatTrigger(matches []string, fullText string) (*Ability, error) {
	return makeTriggeredAbilityWithCondition("End of Combat Trigger", DrawCards, 0, Instant, fullText, EndOfTurn), nil
}


func (ap *AbilityParser) parseBlockPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return makeTriggeredAbilityWithCondition("Block Pump", PumpCreature, power*1000+toughness, Instant, fullText, AttacksOrBlocks), nil
}

func (ap *AbilityParser) parseBlockDamage(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return makeTriggeredAbilityWithCondition("Block Damage", DealDamage, v, Instant, fullText, AttacksOrBlocks), nil
}

func (ap *AbilityParser) parseBlockDraw(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v := parseIntOrOne(matches[1])
	return makeTriggeredAbilityWithCondition("Block Draw", DrawCards, v, Instant, fullText, AttacksOrBlocks), nil
}

func (ap *AbilityParser) parseTargetPumpAndGain(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return &Ability{
		Name: "Target Pump and Gain",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Value:       power*1000 + toughness,
				Duration:    UntilEndOfTurn,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseTargetControlledPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return &Ability{
		Name: "Target Controlled Pump",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Value:       power*1000 + toughness,
				Duration:    UntilEndOfTurn,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseTargetGainsKeyword(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Target Gains Keyword",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        KeywordAbility,
				Duration:    UntilEndOfTurn,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseReturnFromGraveyard(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Return from Graveyard",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        ReturnToHand,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parsePreventAllDamage(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Prevent All Damage",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PreventDamage,
				Value:       9999,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parsePreventNextDamage(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Prevent Next Damage",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PreventDamage,
				Value:       0,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseAllPump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return &Ability{
		Name: "All Pump",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Value:       power*1000 + toughness,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseCounterTypeSpell(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Counter Type Spell",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        CounterSpell,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseGenericActivated(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Generic Activated",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        GenericEffect,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseGenericTriggered(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Generic Triggered",
		Type: Triggered,
		Effects: []Effect{
			{
				Type:        GenericEffect,
				Duration:    Instant,
				Description: fullText,
			},
		},
		IsOptional: false,
	}, nil
}

func (ap *AbilityParser) parseGenericStatic(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Generic Static",
		Type: Static,
		Effects: []Effect{
			{
				Type:        GenericEffect,
				Duration:    Permanent,
				Description: fullText,
			},
		},
	}, nil
}
