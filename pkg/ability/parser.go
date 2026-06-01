// Package ability provides oracle text parsing for MTG abilities.
package ability

import (
	"fmt"
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
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+draw\s+(\d+)\s+cards?`, DrawCards, "ETB draw cards", ap.parseETBDrawCards)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+draw\s+(two|three|four|five)\s+cards?`, DrawCards, "ETB draw word cards", ap.parseETBDrawWordsCards)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+draw\s+a\s+card`, DrawCards, "ETB draw a card", ap.parseETBDrawCard)

	// Life gain abilities
	ap.addPattern(Activated, `\{T\}:\s*You\s+gain\s+(\d+)\s+life`, GainLife, "Tap to gain life", ap.parseTapGainLife)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+.*\s+deals\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "ETB deal damage", ap.parseETBDamage)
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+you\s+gain\s+(\d+)\s+life`, GainLife, "ETB gain life", ap.parseETBGainLife)

	// Alternate win/loss abilities
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+.*you\s+win\s+the\s+game`, WinGame, "ETB win the game", ap.triggeredParserFactory(WinGame, EntersTheBattlefield, "ETB Win Game"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+.*you\s+win\s+the\s+game`, WinGame, "Upkeep win the game", ap.triggeredParserFactory(WinGame, BeginningOfUpkeep, "Upkeep Win Game"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+.*you\s+win\s+the\s+game`, WinGame, "End step win the game", ap.triggeredParserFactory(WinGame, EndOfTurn, "End Step Win Game"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+.*player\s+loses\s+the\s+game`, LoseGame, "Combat damage player loses", ap.triggeredParserFactory(LoseGame, DealsCombatDamage, "Combat Damage Lose Game"))
	ap.addPattern(Activated, `.*you\s+win\s+the\s+game`, WinGame, "Win the game", ap.activatedParserFactory(WinGame, "Win Game"))
	ap.addPattern(Activated, `.*each\s+opponent\s+.*loses\s+the\s+game`, LoseGame, "Each opponent loses", ap.activatedParserFactory(LoseGame, "Each Opponent Loses"))
	ap.addPattern(Activated, `.*target\s+(?:player|opponent)\s+.*loses\s+the\s+game`, LoseGame, "Target player loses", ap.activatedParserFactory(LoseGame, "Target Player Loses"))

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

	// Broad {T}: activated abilities for common effects (catches many 262 {t}: failures)
	ap.addPattern(Activated, `\{T\}:\s*Draw\s+(a|\d+)\s+cards?`, DrawCards, "Tap to draw", ap.activatedParserFactory(DrawCards, "Tap to Draw"))
	ap.addPattern(Activated, `\{T\}:\s*.*\s+gains?\s+(\d+)\s+life`, GainLife, "Tap to gain life broad", ap.activatedParserFactory(GainLife, "Tap to Gain Life"))
	ap.addPattern(Activated, `\{T\}:\s*.*\s+loses?\s+(\d+)\s+life`, LoseLife, "Tap to lose life broad", ap.activatedParserFactory(LoseLife, "Tap to Lose Life"))
	ap.addPattern(Activated, `\{T\}:\s*.*\s+deals\s+(\d+)\s+damage`, DealDamage, "Tap to deal damage broad", ap.activatedParserFactory(DealDamage, "Tap to Deal Damage"))
	ap.addPattern(Activated, `\{T\}:\s*Exile\s+(.*)`, Exile, "Tap to exile", ap.activatedParserFactory(Exile, "Tap to Exile"))
	ap.addPattern(Activated, `\{T\}:\s*Destroy\s+target\s+(.*)`, DestroyPermanent, "Tap to destroy", ap.activatedParserFactory(DestroyPermanent, "Tap to Destroy"))
	ap.addPattern(Activated, `\{T\}:\s*Return\s+target\s+(.*)\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "Tap to return", ap.activatedParserFactory(ReturnToHand, "Tap to Return"))
	ap.addPattern(Activated, `\{T\}:\s*Search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "Tap to search", ap.activatedParserFactory(SearchLibrary, "Tap to Search"))
	ap.addPattern(Activated, `\{T\}:\s*Create\s+(.*)\s+token`, CreateToken, "Tap to create token", ap.activatedParserFactory(CreateToken, "Tap to Create Token"))
	ap.addPattern(Activated, `\{T\}:\s*Mill\s+(a|\d+)\s+cards?`, MillCards, "Tap to mill", ap.activatedParserFactory(MillCards, "Tap to Mill"))
	ap.addPattern(Activated, `\{T\}:\s*Scry\s+(\d+)`, ScryCards, "Tap to scry", ap.activatedParserFactory(ScryCards, "Tap to Scry"))
	ap.addPattern(Activated, `\{T\}:\s*Put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Tap to +1/+1", ap.activatedParserFactory(AddCounters, "Tap to +1/+1"))
	ap.addPattern(Activated, `\{T\}:\s*Proliferate`, AddCounters, "Tap to proliferate", ap.activatedParserFactory(AddCounters, "Tap to Proliferate"))
	ap.addPattern(Activated, `\{T\}:\s*Explore`, AddCounters, "Tap to explore", ap.activatedParserFactory(AddCounters, "Tap to Explore"))
	ap.addPattern(Activated, `\{T\}:\s*Untap\s+target\s+(.*)`, UntapPermanent, "Tap to untap", ap.activatedParserFactory(UntapPermanent, "Tap to Untap"))
	ap.addPattern(Activated, `\{T\}:\s*Copy\s+target\s+(.*)`, CopySpell, "Tap to copy", ap.activatedParserFactory(CopySpell, "Tap to Copy"))
	ap.addPattern(Activated, `\{T\}:\s*Look\s+at\s+the\s+top\s+(\d+)\s+cards`, ScryCards, "Tap to look library", ap.activatedParserFactory(ScryCards, "Tap to Look Library"))
	ap.addPattern(Activated, `\{T\}:\s*Amass\s+(\d+)`, CreateToken, "Tap to amass", ap.activatedParserFactory(CreateToken, "Tap to Amass"))

	// Fetchlands: sacrifice-to-search-and-put-onto-battlefield (must precede broad Sacrifice patterns)
	ap.addPattern(Activated, `\{T\},\s*Pay\s*1\s*life,\s*Sacrifice\s+[^:]+:\s*Search\s+your\s+library\s+for\s+an?\s+.*\s+card.*put\s+it\s+onto\s+the\s+battlefield.*`, SearchLibrary, "Fetchland search", ap.parseFetchland)

	// Sacrifice activated abilities
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Draw\s+(a|\d+)\s+cards?`, DrawCards, "Sacrifice to draw", ap.activatedParserFactory(DrawCards, "Sacrifice to Draw"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*.*\s+gains?\s+(\d+)\s+life`, GainLife, "Sacrifice to gain life", ap.activatedParserFactory(GainLife, "Sacrifice to Gain Life"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*.*\s+deals?\s+(\d+)\s+damage`, DealDamage, "Sacrifice to deal damage", ap.activatedParserFactory(DealDamage, "Sacrifice to Deal Damage"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Exile\s+(.*)`, Exile, "Sacrifice to exile", ap.activatedParserFactory(Exile, "Sacrifice to Exile"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Destroy\s+target\s+(.*)`, DestroyPermanent, "Sacrifice to destroy", ap.activatedParserFactory(DestroyPermanent, "Sacrifice to Destroy"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Return\s+target\s+(.*)\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "Sacrifice to return", ap.activatedParserFactory(ReturnToHand, "Sacrifice to Return"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "Sacrifice to search", ap.activatedParserFactory(SearchLibrary, "Sacrifice to Search"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Create\s+(.*)\s+token`, CreateToken, "Sacrifice to create token", ap.activatedParserFactory(CreateToken, "Sacrifice to Create Token"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Mill\s+(a|\d+)\s+cards?`, MillCards, "Sacrifice to mill", ap.activatedParserFactory(MillCards, "Sacrifice to Mill"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Target\s+creature\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Sacrifice to pump", ap.activatedParserFactory(PumpCreature, "Sacrifice to Pump"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Target\s+player\s+loses\s+(\d+)\s+life`, LoseLife, "Sacrifice to lose life", ap.activatedParserFactory(LoseLife, "Sacrifice to Lose Life"))
	ap.addPattern(Activated, `Sacrifice\s+[^,:]+:\s*Target\s+player\s+gains\s+(\d+)\s+life`, GainLife, "Sacrifice to gain life target", ap.activatedParserFactory(GainLife, "Sacrifice to Target Gain Life"))

	// Static abilities (these don't use the stack)
	ap.addPattern(Static, `Creatures\s+you\s+control\s+get\s+\+(\d+)/\+(\d+)`, PumpCreature, "Static pump", ap.parseStaticPump)
	ap.addPattern(Static, `Other\s+creatures\s+you\s+control\s+get\s+\+(\d+)/\+(\d+)`, PumpCreature, "Static pump others", ap.parseStaticPumpOthers)

	// Keyword abilities — broad regex for comma-separated keyword lists on a single line.
	ap.addPattern(Static, `(?i)^(?:(?:Flying|Trample|Haste|Vigilance|First strike|Lifelink|Deathtouch|Menace|Reach|Hexproof|Defender|Flash|Indestructible|Double strike|Shroud|Intimidate|Fear|Shadow|Infect|Wither|Prowess|Cascade|Convoke|Delve|Dredge|Persist|Undying|Unearth|Morph|Manifest|Embalm|Eternalize|Aftermath|Adventure|Mutate|Foretell|Strive|Rebound|Suspend|Madness|Buyback|Replicate|Splice|Transmute|Regenerate|Ward\s*\{[^}]+\}|Bloodthirst\s*\d+|Annihilator\s*\d+|Protection from [^.,;()]+)(?:\s*\([^)]*\)|\s*[,;]\s*)*)+$`, KeywordAbility, "Keyword abilities", ap.parseKeywordAbilities)

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

	// Mill spells (modern wording)
	ap.addPattern(Activated, `Target\s+player\s+mills\s+(one|two|three|four|five|six|seven|eight|nine|ten|\d+)\s+cards?`, MillCards, "Target player mills", ap.parseTargetPlayerMills)
	ap.addPattern(Activated, `Each\s+player\s+mills\s+(one|two|three|four|five|six|seven|eight|nine|ten|\d+)\s+cards?`, MillCards, "Each player mills", ap.parseEachPlayerMills)

	// Reanimation from graveyard
	ap.addPattern(Activated, `(?:Put|Return)\s+target\s+creature\s+card\s+from\s+(?:a|your)\s+graveyard\s+(?:onto\s+the\s+battlefield|to\s+the\s+battlefield).*`, ReanimateCreature, "Reanimate creature", ap.parseReanimate)

	// Tutor to top of library
	ap.addPattern(Activated, `Search\s+your\s+library\s+for\s+.*\s+card.*reveal\s+it.*then\s+shuffle.*put\s+that\s+card\s+on\s+top.*`, SearchLibrary, "Tutor to top", ap.parseTutorToTop)

	// "This spell can't be countered" static rider
	ap.addPattern(Static, `This\s+spell\s+can'?t\s+be\s+countered`, KeywordAbility, "Cannot be countered", ap.parseCantBeCountered)

	// Conditional abilities - activated/static
	ap.addPattern(Activated, `If\s+you\s+control\s+a\s+(.*),\s*(.*)`, DrawCards, "Conditional control permanent", ap.parseConditionalControl)
	ap.addPattern(Activated, `If\s+an\s+opponent\s+controls\s+more\s+creatures\s+than\s+you,\s*(.*)`, DealDamage, "Conditional opponent creatures", ap.parseConditionalOpponentCreatures)
	ap.addPattern(Activated, `If\s+you\s+have\s+no\s+cards\s+in\s+hand,\s*(.*)`, DealDamage, "Conditional hellbent", ap.parseConditionalHellbent)

	// Conditional ETB triggers
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+if\s+you\s+control\s+a\s+(.*),\s*(.*)`, DrawCards, "Conditional ETB control", ap.parseConditionalETBControl)

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
	ap.addPattern(Activated, `(?i)^Target\s+creature\s+gains\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)\s+until\s+end\s+of\s+turn`, KeywordAbility, "Target gains keyword EOT", ap.parseTargetGainsKeyword)
	// Target creature loses/gains keyword until EOT (handled by broader pattern above; narrow "loses" removed — only 2 cards)
	ap.addPattern(Activated, `(?i)^Return\s+target\s+.+\s+from\s+your\s+graveyard\s+to\s+your\s+hand`, ReturnToHand, "Return from graveyard", ap.parseReturnFromGraveyard)
	ap.addPattern(Activated, `(?i)^Create\s+(?:two|three|four|five|six|seven|eight|nine|ten|\d+|X)\s+\d+/\d+\s+.+\s+creature\s+tokens?`, CreateToken, "Create multiple tokens", ap.parseCreateMultipleTokens)
	// Library manipulation (scry/look)
	ap.addPattern(Activated, `(?i)^Look\s+at\s+the\s+top\s+(\d+)\s+cards?\s+of\s+your\s+library`, ScryCards, "Look at library", ap.parseLookAtLibrary)
	ap.addPattern(Activated, `(?i)^Look\s+at\s+the\s+top\s+.+\s+cards?\s+of\s+your\s+library`, ScryCards, "Look at library broad", ap.parseLookAtLibraryBroad)
	// Counterspell patterns
	ap.addPattern(Activated, `(?i)^Counter\s+target\s+(?:creature|artifact|enchantment|instant|sorcery|planeswalker|creature\s+or\s+enchantment|artifact\s+or\s+creature|instant\s+or\s+sorcery)\s+spell`, CounterSpell, "Counter type spell", ap.parseCounterTypeSpell)
	ap.addPattern(Activated, `(?i)^Counter\s+target\s+spell`, CounterSpell, "Counter any spell broad", ap.parseCounterSpellBroad)
	// Counter/Proliferate mechanics
	ap.addPattern(Activated, `(?i)^Put\s+(?:a|\d+|X)\s+\+1/\+1\s+counter`, AddCounters, "Put +1/+1 counter", ap.parsePutCounter)
	ap.addPattern(Activated, `(?i)^Put\s+.+\s+counter`, AddCounters, "Put counter broad", ap.parsePutCounterBroad)
	// Damage prevention
	ap.addPattern(Activated, `(?i)^Prevent\s+all\s+damage`, PreventDamage, "Prevent all damage", ap.parsePreventAllDamage)
	ap.addPattern(Activated, `(?i)^Prevent\s+the\s+next\s+\d+\s+damage`, PreventDamage, "Prevent next damage", ap.parsePreventNextDamage)
	// Mass life loss and pump
	ap.addPattern(Activated, `(?i)^Each\s+player\s+.*\s+loses?\s+\d+\s+life`, LoseLife, "Each loses life broad", ap.parseEachLosesLife)
	ap.addPattern(Activated, `(?i)^All\s+creatures\s+get\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "All creatures pump", ap.parseAllPump)
	// Restriction removal (very few cards)
	ap.addPattern(Activated, `(?i)^All\s+creatures\s+lose\s+(?:flying|trample|first strike)`, CantAttackBlock, "All creatures lose keyword", ap.parseAllCreaturesLoseKeyword)
	// Land and player abilities
	ap.addPattern(Activated, `(?i)^You\s+may\s+play\s+an\s+additional\s+land`, AdditionalLand, "Additional land", ap.parseAdditionalLand)
	ap.addPattern(Activated, `(?i)^You\s+have\s+(?:shroud|hexproof|protection)`, KeywordAbility, "You have keyword", ap.parseStaticKeyword)
	// Discard
	ap.addPattern(Activated, `(?i)^You\s+may\s+discard\s+.+`, DiscardCards, "You may discard", ap.parseDiscardEffect)
	// Sacrifice cost abilities — broad catch-all removed; too diverse to model generically (280 cards)

	// Static patterns — narrowed replacements for former GenericEffect catch-alls
	// "As long as" conditional static abilities
	ap.addPattern(Static, `(?i)^As\s+long\s+as\s+.+\s+gets\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "As long as pump", ap.parseStaticPump)
	ap.addPattern(Static, `(?i)^As\s+long\s+as\s+.+\s+has\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "As long as keyword", ap.parseStaticKeyword)
	ap.addPattern(Static, `(?i)^As\s+long\s+as\s+.+\s+can't\s+(?:attack|block)`, CantAttackBlock, "As long as restriction", ap.parseStaticRestriction)
	// "Each creature you control" static abilities
	ap.addPattern(Static, `(?i)^Each\s+creature\s+you\s+control.*gets\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "Each creature pump", ap.parseStaticPump)
	ap.addPattern(Static, `(?i)^Each\s+creature\s+you\s+control.*has\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "Each creature keyword", ap.parseStaticKeyword)
	ap.addPattern(Static, `(?i)^Each\s+creature\s+you\s+control.*can't\s+(?:attack|block|be\s+blocked)`, CantAttackBlock, "Each creature restriction", ap.parseStaticRestriction)
	// "Each other creature" static abilities
	ap.addPattern(Static, `(?i)^Each\s+other\s+creature.*gets\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "Each other pump", ap.parseStaticPump)
	ap.addPattern(Static, `(?i)^Each\s+other\s+creature.*has\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "Each other keyword", ap.parseStaticKeyword)
	ap.addPattern(Static, `(?i)^Each\s+other\s+creature.*can't\s+(?:attack|block|be\s+blocked)`, CantAttackBlock, "Each other restriction", ap.parseStaticRestriction)
	// "Creatures you control" static keyword/restriction abilities (pump already covered by existing pattern)
	ap.addPattern(Static, `(?i)^Creatures\s+you\s+control.*have\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "Creatures keyword", ap.parseStaticKeyword)
	ap.addPattern(Static, `(?i)^Creatures\s+you\s+control.*can't\s+(?:attack|block|be\s+blocked)`, CantAttackBlock, "Creatures restriction", ap.parseStaticRestriction)
	// "Other creatures you control" static abilities (pump already covered by existing pattern)
	ap.addPattern(Static, `(?i)^Other\s+creatures\s+you\s+control.*have\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "Other creatures keyword", ap.parseStaticKeyword)
	ap.addPattern(Static, `(?i)^Other\s+creatures\s+you\s+control.*can't\s+(?:attack|block|be\s+blocked)`, CantAttackBlock, "Other creatures restriction", ap.parseStaticRestriction)
	// "Other [type] you control" static keyword/restriction abilities
	ap.addPattern(Static, `(?i)^Other\s+[A-Za-z]+\s+you\s+control.*get\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "Other typed pump", ap.parseStaticPump)
	ap.addPattern(Static, `(?i)^Other\s+[A-Za-z]+\s+you\s+control.*have\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "Other typed keyword", ap.parseStaticKeyword)
	ap.addPattern(Static, `(?i)^Creatures\s+with\s+(?:flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender).*get\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "Creatures with keyword pump", ap.parseStaticPump)
	// "If" conditional static abilities (very narrow to avoid catching spell riders)
	ap.addPattern(Static, `(?i)^If\s+.*gets\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "If pump", ap.parseIfPump)
	ap.addPattern(Static, `(?i)^If\s+.*has\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "If keyword", ap.parseIfKeyword)
	ap.addPattern(Static, `(?i)^If\s+.*can't\s+(?:attack|block)`, CantAttackBlock, "If restriction", ap.parseStaticRestriction)
	// "This creature" static abilities
	ap.addPattern(Static, `(?i)^This\s+creature\s+gets\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "This creature pump", ap.parseThisCreaturePump)
	ap.addPattern(Static, `(?i)^This\s+creature\s+has\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "This creature keyword", ap.parseStaticKeyword)
	ap.addPattern(Static, `(?i)^This\s+creature\s+can't\s+(?:attack|block)`, CantAttackBlock, "This creature restriction", ap.parseStaticRestriction)
	ap.addPattern(Static, `(?i)^This\s+creature\s+can\s+block\s+only\s+creatures\s+with\s+flying`, CantAttackBlock, "This creature block restriction", ap.parseStaticRestriction)
	// "Until" sentences are spells/triggers, not static — removed broad catch-all

	// Spell/activated catch-alls for common unparsed first-words
	ap.addPattern(Activated, `(?i)^Search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "Search library spell", ap.activatedParserFactory(SearchLibrary, "Search Library Spell"))
	ap.addPattern(Activated, `(?i)^Exile\s+target\s+(.*)`, Exile, "Exile target spell", ap.activatedParserFactory(Exile, "Exile Target Spell"))
	ap.addPattern(Activated, `(?i)^Exile\s+the\s+top\s+(.*)\s+cards?\s+of\s+your\s+library`, MillCards, "Exile top library spell", ap.activatedParserFactory(MillCards, "Exile Top Library"))
	ap.addPattern(Activated, `(?i)^Tap\s+target\s+(.*)`, TapUntap, "Tap target spell", ap.activatedParserFactory(TapUntap, "Tap Target Spell"))
	ap.addPattern(Activated, `(?i)^Reveal\s+the\s+top\s+(\d+)\s+cards?\s+of\s+your\s+library`, ScryCards, "Reveal top library spell", ap.activatedParserFactory(ScryCards, "Reveal Top Library"))
	ap.addPattern(Activated, `(?i)^Reveal\s+cards\s+from\s+the\s+top\s+of\s+your\s+library`, ScryCards, "Reveal library spell", ap.activatedParserFactory(ScryCards, "Reveal Library"))
	ap.addPattern(Activated, `(?i)^Put\s+target\s+(.*)\s+on\s+top\s+of\s+its\s+owner'?s\s+library`, ReturnToHand, "Put on top of library spell", ap.activatedParserFactory(ReturnToHand, "Put on Top of Library"))
	ap.addPattern(Activated, `(?i)^Until\s+end\s+of\s+turn,\s+target\s+creature\s+gets\s+\+?(\d+)/\+?(\d+)`, PumpCreature, "Until EOT pump spell", ap.activatedParserFactory(PumpCreature, "Until EOT Pump"))
	ap.addPattern(Activated, `(?i)^Until\s+end\s+of\s+turn,\s+target\s+creature\s+gains\s+(flying|trample|lifelink|deathtouch|haste|vigilance|first strike|double strike|menace|reach|hexproof|indestructible|flash|defender)`, KeywordAbility, "Until EOT keyword spell", ap.activatedParserFactory(KeywordAbility, "Until EOT Keyword"))

	// ETB mill / scry / counters / explore / proliferate / untap / amass / library
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "ETB mill", ap.triggeredParserFactory(MillCards, EntersTheBattlefield, "ETB Mill"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+scry\s+(\d+)`, ScryCards, "ETB scry", ap.triggeredParserFactory(ScryCards, EntersTheBattlefield, "ETB Scry"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "ETB +1/+1 counter", ap.triggeredParserFactory(AddCounters, EntersTheBattlefield, "ETB Counter"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+proliferate`, AddCounters, "ETB proliferate", ap.triggeredParserFactory(AddCounters, EntersTheBattlefield, "ETB Proliferate"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+explore`, AddCounters, "ETB explore", ap.triggeredParserFactory(AddCounters, EntersTheBattlefield, "ETB Explore"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+untap\s+target`, UntapPermanent, "ETB untap", ap.triggeredParserFactory(UntapPermanent, EntersTheBattlefield, "ETB Untap"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "ETB Food/Treasure", ap.triggeredParserFactory(CreateToken, EntersTheBattlefield, "ETB Food/Treasure"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+amass\s+(\d+)`, CreateToken, "ETB amass", ap.triggeredParserFactory(CreateToken, EntersTheBattlefield, "ETB Amass"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+look\s+at\s+the\s+top\s+(\d+)\s+cards`, ScryCards, "ETB look library", ap.triggeredParserFactory(ScryCards, EntersTheBattlefield, "ETB Look"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+copy\s+target`, CopySpell, "ETB copy", ap.triggeredParserFactory(CopySpell, EntersTheBattlefield, "ETB Copy"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+exile\s+target\s+.*\s+until`, Exile, "ETB flicker", ap.triggeredParserFactory(Exile, EntersTheBattlefield, "ETB Flicker"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+return\s+target\s+.*\s+from\s+your\s+graveyard\s+to\s+the\s+battlefield`, ReturnToHand, "ETB reanimate", ap.triggeredParserFactory(ReturnToHand, EntersTheBattlefield, "ETB Reanimate"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+destroy\s+target`, DestroyPermanent, "ETB destroy", ap.triggeredParserFactory(DestroyPermanent, EntersTheBattlefield, "ETB Destroy"))
	ap.addPattern(Triggered, `When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+(?:tap|untap)\s+target`, TapUntap, "ETB tap untap", ap.triggeredParserFactory(TapUntap, EntersTheBattlefield, "ETB Tap/Untap"))

	// Dies triggers — mill / scry / counters / explore / proliferate / untap / amass
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Death mill", ap.triggeredParserFactory(MillCards, Dies, "Death Mill"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+scry\s+(\d+)`, ScryCards, "Death scry", ap.triggeredParserFactory(ScryCards, Dies, "Death Scry"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Death +1/+1 counter", ap.triggeredParserFactory(AddCounters, Dies, "Death Counter"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+proliferate`, AddCounters, "Death proliferate", ap.triggeredParserFactory(AddCounters, Dies, "Death Proliferate"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+explore`, AddCounters, "Death explore", ap.triggeredParserFactory(AddCounters, Dies, "Death Explore"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+untap\s+target`, UntapPermanent, "Death untap", ap.triggeredParserFactory(UntapPermanent, Dies, "Death Untap"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+amass\s+(\d+)`, CreateToken, "Death amass", ap.triggeredParserFactory(CreateToken, Dies, "Death Amass"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+look\s+at\s+the\s+top\s+(\d+)\s+cards`, ScryCards, "Death look library", ap.triggeredParserFactory(ScryCards, Dies, "Death Look"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+copy\s+target`, CopySpell, "Death copy", ap.triggeredParserFactory(CopySpell, Dies, "Death Copy"))

	// Additional common death triggers
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "Death create token", ap.triggeredParserFactory(CreateToken, Dies, "Death Create Token"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+.*\s+gains?\s+(\d+)\s+life`, GainLife, "Death gain life", ap.triggeredParserFactory(GainLife, Dies, "Death Gain Life"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+.*\s+loses?\s+(\d+)\s+life`, LoseLife, "Death lose life", ap.triggeredParserFactory(LoseLife, Dies, "Death Lose Life"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+exile\s+target\s+(.*)`, Exile, "Death exile", ap.triggeredParserFactory(Exile, Dies, "Death Exile"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+return\s+target\s+(.*)\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "Death return to hand", ap.triggeredParserFactory(ReturnToHand, Dies, "Death Return"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+destroy\s+target\s+(.*)`, DestroyPermanent, "Death destroy", ap.triggeredParserFactory(DestroyPermanent, Dies, "Death Destroy"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "Death search", ap.triggeredParserFactory(SearchLibrary, Dies, "Death Search"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+target\s+creature\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Death pump", ap.triggeredParserFactory(PumpCreature, Dies, "Death Pump"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+target\s+creature\s+gets\s+-([0-9]+)/-([0-9]+)`, PumpCreature, "Death debuff", ap.triggeredParserFactory(PumpCreature, Dies, "Death Debuff"))
	ap.addPattern(Triggered, `When\s+.*\s+dies,\s+(?:tap|untap)\s+target\s+(.*)`, TapUntap, "Death tap untap", ap.triggeredParserFactory(TapUntap, Dies, "Death Tap/Untap"))

	// Attack triggers — mill / scry / counters / explore / proliferate / untap / amass / library
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Attack mill", ap.triggeredParserFactory(MillCards, AttacksOrBlocks, "Attack Mill"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+scry\s+(\d+)`, ScryCards, "Attack scry", ap.triggeredParserFactory(ScryCards, AttacksOrBlocks, "Attack Scry"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Attack +1/+1 counter", ap.triggeredParserFactory(AddCounters, AttacksOrBlocks, "Attack Counter"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+proliferate`, AddCounters, "Attack proliferate", ap.triggeredParserFactory(AddCounters, AttacksOrBlocks, "Attack Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+explore`, AddCounters, "Attack explore", ap.triggeredParserFactory(AddCounters, AttacksOrBlocks, "Attack Explore"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+untap\s+target`, UntapPermanent, "Attack untap", ap.triggeredParserFactory(UntapPermanent, AttacksOrBlocks, "Attack Untap"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+amass\s+(\d+)`, CreateToken, "Attack amass", ap.triggeredParserFactory(CreateToken, AttacksOrBlocks, "Attack Amass"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+look\s+at\s+the\s+top\s+(\d+)\s+cards`, ScryCards, "Attack look library", ap.triggeredParserFactory(ScryCards, AttacksOrBlocks, "Attack Look"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+copy\s+target`, CopySpell, "Attack copy", ap.triggeredParserFactory(CopySpell, AttacksOrBlocks, "Attack Copy"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "Attack Food/Treasure", ap.triggeredParserFactory(CreateToken, AttacksOrBlocks, "Attack Food/Treasure"))

	// Combat damage to player — mill / scry / counters / explore / proliferate / untap / amass / library
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Combat damage mill", ap.triggeredParserFactory(MillCards, DealsCombatDamage, "Combat Mill"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+scry\s+(\d+)`, ScryCards, "Combat damage scry", ap.triggeredParserFactory(ScryCards, DealsCombatDamage, "Combat Scry"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Combat damage +1/+1 counter", ap.triggeredParserFactory(AddCounters, DealsCombatDamage, "Combat Counter"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+proliferate`, AddCounters, "Combat damage proliferate", ap.triggeredParserFactory(AddCounters, DealsCombatDamage, "Combat Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+explore`, AddCounters, "Combat damage explore", ap.triggeredParserFactory(AddCounters, DealsCombatDamage, "Combat Explore"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+untap\s+target`, UntapPermanent, "Combat damage untap", ap.triggeredParserFactory(UntapPermanent, DealsCombatDamage, "Combat Untap"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+amass\s+(\d+)`, CreateToken, "Combat damage amass", ap.triggeredParserFactory(CreateToken, DealsCombatDamage, "Combat Amass"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+look\s+at\s+the\s+top\s+(\d+)\s+cards`, ScryCards, "Combat damage look library", ap.triggeredParserFactory(ScryCards, DealsCombatDamage, "Combat Look"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+copy\s+target`, CopySpell, "Combat damage copy", ap.triggeredParserFactory(CopySpell, DealsCombatDamage, "Combat Copy"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "Combat damage Food/Treasure", ap.triggeredParserFactory(CreateToken, DealsCombatDamage, "Combat Food/Treasure"))

	// Combat damage to a creature (not just player)
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+creature,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Combat damage creature mill", ap.triggeredParserFactory(MillCards, DealsCombatDamage, "Combat Creature Mill"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+creature,\s+draw\s+(a|\d+)\s+cards?`, DrawCards, "Combat damage creature draw", ap.triggeredParserFactory(DrawCards, DealsCombatDamage, "Combat Creature Draw"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+creature,\s+.*\s+gains?\s+(\d+)\s+life`, GainLife, "Combat damage creature gain life", ap.triggeredParserFactory(GainLife, DealsCombatDamage, "Combat Creature Gain Life"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+creature,\s+.*\s+loses?\s+(\d+)\s+life`, LoseLife, "Combat damage creature lose life", ap.triggeredParserFactory(LoseLife, DealsCombatDamage, "Combat Creature Lose Life"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+creature,\s+exile\s+(.*)`, Exile, "Combat damage creature exile", ap.triggeredParserFactory(Exile, DealsCombatDamage, "Combat Creature Exile"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+creature,\s+destroy\s+target\s+(.*)`, DestroyPermanent, "Combat damage creature destroy", ap.triggeredParserFactory(DestroyPermanent, DealsCombatDamage, "Combat Creature Destroy"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+creature,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Combat damage creature +1/+1", ap.triggeredParserFactory(AddCounters, DealsCombatDamage, "Combat Creature Counter"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+a\s+creature,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "Combat damage creature token", ap.triggeredParserFactory(CreateToken, DealsCombatDamage, "Combat Creature Token"))

	// Whenever a creature becomes blocked
	ap.addPattern(Triggered, `Whenever\s+.*\s+becomes?\s+blocked,\s+.*\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Blocked pump", ap.triggeredParserFactory(PumpCreature, AttacksOrBlocks, "Blocked Pump"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+becomes?\s+blocked,\s+.*\s+deals\s+(\d+)\s+damage`, DealDamage, "Blocked damage", ap.triggeredParserFactory(DealDamage, AttacksOrBlocks, "Blocked Damage"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+becomes?\s+blocked,\s+draw\s+(a|\d+)\s+cards?`, DrawCards, "Blocked draw", ap.triggeredParserFactory(DrawCards, AttacksOrBlocks, "Blocked Draw"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+becomes?\s+blocked,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Blocked mill", ap.triggeredParserFactory(MillCards, AttacksOrBlocks, "Blocked Mill"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+becomes?\s+blocked,\s+.*\s+gains?\s+(\d+)\s+life`, GainLife, "Blocked gain life", ap.triggeredParserFactory(GainLife, AttacksOrBlocks, "Blocked Gain Life"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+becomes?\s+blocked,\s+.*\s+loses?\s+(\d+)\s+life`, LoseLife, "Blocked lose life", ap.triggeredParserFactory(LoseLife, AttacksOrBlocks, "Blocked Lose Life"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+becomes?\s+blocked,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Blocked +1/+1", ap.triggeredParserFactory(AddCounters, AttacksOrBlocks, "Blocked Counter"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+becomes?\s+blocked,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "Blocked token", ap.triggeredParserFactory(CreateToken, AttacksOrBlocks, "Blocked Token"))

	// Additional attack trigger effects not covered by specific parsers
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+.*\s+gains?\s+(\d+)\s+life`, GainLife, "Attack gain life", ap.triggeredParserFactory(GainLife, AttacksOrBlocks, "Attack Gain Life"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+.*\s+loses?\s+(\d+)\s+life`, LoseLife, "Attack lose life", ap.triggeredParserFactory(LoseLife, AttacksOrBlocks, "Attack Lose Life"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+exile\s+target\s+(.*)`, Exile, "Attack exile", ap.triggeredParserFactory(Exile, AttacksOrBlocks, "Attack Exile"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+return\s+target\s+(.*)\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "Attack return", ap.triggeredParserFactory(ReturnToHand, AttacksOrBlocks, "Attack Return"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+destroy\s+target\s+(.*)`, DestroyPermanent, "Attack destroy", ap.triggeredParserFactory(DestroyPermanent, AttacksOrBlocks, "Attack Destroy"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "Attack search", ap.triggeredParserFactory(SearchLibrary, AttacksOrBlocks, "Attack Search"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+target\s+creature\s+gets\s+-([0-9]+)/-([0-9]+)`, PumpCreature, "Attack debuff", ap.triggeredParserFactory(PumpCreature, AttacksOrBlocks, "Attack Debuff"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+(?:tap|untap)\s+target\s+(.*)`, TapUntap, "Attack tap untap", ap.triggeredParserFactory(TapUntap, AttacksOrBlocks, "Attack Tap/Untap"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "Attack create token", ap.triggeredParserFactory(CreateToken, AttacksOrBlocks, "Attack Create Token"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+attacks,\s+each\s+player\s+sacrifices\s+(.*)`, SacrificePermanent, "Attack sacrifice", ap.triggeredParserFactory(SacrificePermanent, AttacksOrBlocks, "Attack Sacrifice"))

	// Additional combat damage to player effects not covered by specific parsers
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+.*\s+gains?\s+(\d+)\s+life`, GainLife, "Combat damage gain life broad", ap.triggeredParserFactory(GainLife, DealsCombatDamage, "Combat Gain Life"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+.*\s+loses?\s+(\d+)\s+life`, LoseLife, "Combat damage lose life broad", ap.triggeredParserFactory(LoseLife, DealsCombatDamage, "Combat Lose Life"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+exile\s+target\s+(.*)`, Exile, "Combat damage exile target", ap.triggeredParserFactory(Exile, DealsCombatDamage, "Combat Exile"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+return\s+target\s+(.*)\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "Combat damage return", ap.triggeredParserFactory(ReturnToHand, DealsCombatDamage, "Combat Return"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+destroy\s+target\s+(.*)`, DestroyPermanent, "Combat damage destroy", ap.triggeredParserFactory(DestroyPermanent, DealsCombatDamage, "Combat Destroy"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "Combat damage search", ap.triggeredParserFactory(SearchLibrary, DealsCombatDamage, "Combat Search"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+target\s+creature\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Combat damage pump", ap.triggeredParserFactory(PumpCreature, DealsCombatDamage, "Combat Pump"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+target\s+creature\s+gets\s+-([0-9]+)/-([0-9]+)`, PumpCreature, "Combat damage debuff", ap.triggeredParserFactory(PumpCreature, DealsCombatDamage, "Combat Debuff"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+(?:tap|untap)\s+target\s+(.*)`, TapUntap, "Combat damage tap untap", ap.triggeredParserFactory(TapUntap, DealsCombatDamage, "Combat Tap/Untap"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "Combat damage create token", ap.triggeredParserFactory(CreateToken, DealsCombatDamage, "Combat Create Token"))
	ap.addPattern(Triggered, `Whenever\s+.*\s+deals\s+combat\s+damage\s+to\s+(?:a\s+)?player,\s+each\s+player\s+sacrifices\s+(.*)`, SacrificePermanent, "Combat damage sacrifice", ap.triggeredParserFactory(SacrificePermanent, DealsCombatDamage, "Combat Sacrifice"))

	// Upkeep triggers — mill / scry / counters / explore / proliferate / untap / amass / library
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Upkeep mill", ap.triggeredParserFactory(MillCards, BeginningOfUpkeep, "Upkeep Mill"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+scry\s+(\d+)`, ScryCards, "Upkeep scry", ap.triggeredParserFactory(ScryCards, BeginningOfUpkeep, "Upkeep Scry"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Upkeep +1/+1 counter", ap.triggeredParserFactory(AddCounters, BeginningOfUpkeep, "Upkeep Counter"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+proliferate`, AddCounters, "Upkeep proliferate", ap.triggeredParserFactory(AddCounters, BeginningOfUpkeep, "Upkeep Proliferate"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+explore`, AddCounters, "Upkeep explore", ap.triggeredParserFactory(AddCounters, BeginningOfUpkeep, "Upkeep Explore"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+untap\s+target`, UntapPermanent, "Upkeep untap", ap.triggeredParserFactory(UntapPermanent, BeginningOfUpkeep, "Upkeep Untap"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+amass\s+(\d+)`, CreateToken, "Upkeep amass", ap.triggeredParserFactory(CreateToken, BeginningOfUpkeep, "Upkeep Amass"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+look\s+at\s+the\s+top\s+(\d+)\s+cards`, ScryCards, "Upkeep look library", ap.triggeredParserFactory(ScryCards, BeginningOfUpkeep, "Upkeep Look"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+copy\s+target`, CopySpell, "Upkeep copy", ap.triggeredParserFactory(CopySpell, BeginningOfUpkeep, "Upkeep Copy"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "Upkeep Food/Treasure", ap.triggeredParserFactory(CreateToken, BeginningOfUpkeep, "Upkeep Food/Treasure"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+draw\s+(a|\d+)\s+cards?`, DrawCards, "Upkeep draw", ap.triggeredParserFactory(DrawCards, BeginningOfUpkeep, "Upkeep Draw"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+exile\s+target\s+(.*)`, Exile, "Upkeep exile", ap.triggeredParserFactory(Exile, BeginningOfUpkeep, "Upkeep Exile"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+return\s+target\s+(.*)\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "Upkeep return", ap.triggeredParserFactory(ReturnToHand, BeginningOfUpkeep, "Upkeep Return"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+destroy\s+target\s+(.*)`, DestroyPermanent, "Upkeep destroy", ap.triggeredParserFactory(DestroyPermanent, BeginningOfUpkeep, "Upkeep Destroy"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "Upkeep search", ap.triggeredParserFactory(SearchLibrary, BeginningOfUpkeep, "Upkeep Search"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+target\s+creature\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "Upkeep pump", ap.triggeredParserFactory(PumpCreature, BeginningOfUpkeep, "Upkeep Pump"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+target\s+creature\s+gets\s+-([0-9]+)/-([0-9]+)`, PumpCreature, "Upkeep debuff", ap.triggeredParserFactory(PumpCreature, BeginningOfUpkeep, "Upkeep Debuff"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+(?:tap|untap)\s+target\s+(.*)`, TapUntap, "Upkeep tap untap", ap.triggeredParserFactory(TapUntap, BeginningOfUpkeep, "Upkeep Tap/Untap"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+each\s+player\s+sacrifices\s+(.*)`, SacrificePermanent, "Upkeep sacrifice", ap.triggeredParserFactory(SacrificePermanent, BeginningOfUpkeep, "Upkeep Sacrifice"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+.*\s+gains?\s+(\d+)\s+life`, GainLife, "Upkeep gain life broad", ap.triggeredParserFactory(GainLife, BeginningOfUpkeep, "Upkeep Gain Life"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+.*\s+loses?\s+(\d+)\s+life`, LoseLife, "Upkeep lose life broad", ap.triggeredParserFactory(LoseLife, BeginningOfUpkeep, "Upkeep Lose Life"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+.*\s+deals?\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "Upkeep damage broad", ap.triggeredParserFactory(DealDamage, BeginningOfUpkeep, "Upkeep Damage"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+upkeep,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "Upkeep create token", ap.triggeredParserFactory(CreateToken, BeginningOfUpkeep, "Upkeep Create Token"))

	// End step triggers
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+create\s+(?:a|\d+)\s+.*\s+token`, CreateToken, "End step token", ap.triggeredParserFactory(CreateToken, EndOfTurn, "End Step Token"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "End step mill", ap.triggeredParserFactory(MillCards, EndOfTurn, "End Step Mill"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+scry\s+(\d+)`, ScryCards, "End step scry", ap.triggeredParserFactory(ScryCards, EndOfTurn, "End Step Scry"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "End step +1/+1 counter", ap.triggeredParserFactory(AddCounters, EndOfTurn, "End Step Counter"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+proliferate`, AddCounters, "End step proliferate", ap.triggeredParserFactory(AddCounters, EndOfTurn, "End Step Proliferate"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+draw\s+(a|\d+)\s+cards?`, DrawCards, "End step draw", ap.triggeredParserFactory(DrawCards, EndOfTurn, "End Step Draw"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+.*\s+gains?\s+(\d+)\s+life`, GainLife, "End step gain life", ap.triggeredParserFactory(GainLife, EndOfTurn, "End Step Gain Life"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+.*\s+loses?\s+(\d+)\s+life`, LoseLife, "End step lose life", ap.triggeredParserFactory(LoseLife, EndOfTurn, "End Step Lose Life"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+.*\s+deals?\s+(\d+)\s+damage\s+to\s+(.*)`, DealDamage, "End step damage", ap.triggeredParserFactory(DealDamage, EndOfTurn, "End Step Damage"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+exile\s+target\s+(.*)`, Exile, "End step exile", ap.triggeredParserFactory(Exile, EndOfTurn, "End Step Exile"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+return\s+target\s+(.*)\s+to\s+its\s+owner'?s\s+hand`, ReturnToHand, "End step return", ap.triggeredParserFactory(ReturnToHand, EndOfTurn, "End Step Return"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+destroy\s+target\s+(.*)`, DestroyPermanent, "End step destroy", ap.triggeredParserFactory(DestroyPermanent, EndOfTurn, "End Step Destroy"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+search\s+your\s+library\s+for\s+(.*)`, SearchLibrary, "End step search", ap.triggeredParserFactory(SearchLibrary, EndOfTurn, "End Step Search"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+target\s+creature\s+gets\s+\+(\d+)/\+(\d+)`, PumpCreature, "End step pump", ap.triggeredParserFactory(PumpCreature, EndOfTurn, "End Step Pump"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+target\s+creature\s+gets\s+-([0-9]+)/-([0-9]+)`, PumpCreature, "End step debuff", ap.triggeredParserFactory(PumpCreature, EndOfTurn, "End Step Debuff"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+(?:tap|untap)\s+target\s+(.*)`, TapUntap, "End step tap untap", ap.triggeredParserFactory(TapUntap, EndOfTurn, "End Step Tap/Untap"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "End step Food/Treasure", ap.triggeredParserFactory(CreateToken, EndOfTurn, "End Step Food/Treasure"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+create\s+(.*)\s+creature\s+token`, CreateToken, "End step create token", ap.triggeredParserFactory(CreateToken, EndOfTurn, "End Step Create Token"))
	ap.addPattern(Triggered, `At\s+the\s+beginning\s+of\s+your\s+end\s+step,\s+each\s+player\s+sacrifices\s+(.*)`, SacrificePermanent, "End step sacrifice", ap.triggeredParserFactory(SacrificePermanent, EndOfTurn, "End Step Sacrifice"))

	// Other creature enters triggers
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Creature enters mill", ap.triggeredParserFactory(MillCards, CreatureEnters, "Creature Enters Mill"))
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+scry\s+(\d+)`, ScryCards, "Creature enters scry", ap.triggeredParserFactory(ScryCards, CreatureEnters, "Creature Enters Scry"))
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Creature enters +1/+1 counter", ap.triggeredParserFactory(AddCounters, CreatureEnters, "Creature Enters Counter"))
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+proliferate`, AddCounters, "Creature enters proliferate", ap.triggeredParserFactory(AddCounters, CreatureEnters, "Creature Enters Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+explore`, AddCounters, "Creature enters explore", ap.triggeredParserFactory(AddCounters, CreatureEnters, "Creature Enters Explore"))
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+untap\s+target`, UntapPermanent, "Creature enters untap", ap.triggeredParserFactory(UntapPermanent, CreatureEnters, "Creature Enters Untap"))
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+amass\s+(\d+)`, CreateToken, "Creature enters amass", ap.triggeredParserFactory(CreateToken, CreatureEnters, "Creature Enters Amass"))
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+look\s+at\s+the\s+top\s+(\d+)\s+cards`, ScryCards, "Creature enters look library", ap.triggeredParserFactory(ScryCards, CreatureEnters, "Creature Enters Look"))
	ap.addPattern(Triggered, `Whenever\s+(?:another\s+)?creature\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "Creature enters Food/Treasure", ap.triggeredParserFactory(CreateToken, CreatureEnters, "Creature Enters Food/Treasure"))

	// Spell cast triggers
	ap.addPattern(Triggered, `Whenever\s+you\s+cast\s+(?:a\s+)?(?:noncreature\s+)?spell,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Spell cast mill", ap.triggeredParserFactory(MillCards, SpellCast, "Spell Cast Mill"))
	ap.addPattern(Triggered, `Whenever\s+you\s+cast\s+(?:a\s+)?(?:noncreature\s+)?spell,\s+scry\s+(\d+)`, ScryCards, "Spell cast scry", ap.triggeredParserFactory(ScryCards, SpellCast, "Spell Cast Scry"))
	ap.addPattern(Triggered, `Whenever\s+you\s+cast\s+(?:a\s+)?(?:noncreature\s+)?spell,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Spell cast +1/+1 counter", ap.triggeredParserFactory(AddCounters, SpellCast, "Spell Cast Counter"))
	ap.addPattern(Triggered, `Whenever\s+you\s+cast\s+(?:a\s+)?(?:noncreature\s+)?spell,\s+proliferate`, AddCounters, "Spell cast proliferate", ap.triggeredParserFactory(AddCounters, SpellCast, "Spell Cast Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+you\s+cast\s+(?:a\s+)?(?:noncreature\s+)?spell,\s+explore`, AddCounters, "Spell cast explore", ap.triggeredParserFactory(AddCounters, SpellCast, "Spell Cast Explore"))
	ap.addPattern(Triggered, `Whenever\s+you\s+cast\s+(?:a\s+)?(?:noncreature\s+)?spell,\s+untap\s+target`, UntapPermanent, "Spell cast untap", ap.triggeredParserFactory(UntapPermanent, SpellCast, "Spell Cast Untap"))
	ap.addPattern(Triggered, `Whenever\s+you\s+cast\s+(?:a\s+)?(?:noncreature\s+)?spell,\s+amass\s+(\d+)`, CreateToken, "Spell cast amass", ap.triggeredParserFactory(CreateToken, SpellCast, "Spell Cast Amass"))
	ap.addPattern(Triggered, `Whenever\s+you\s+cast\s+(?:a\s+)?(?:noncreature\s+)?spell,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "Spell cast Food/Treasure", ap.triggeredParserFactory(CreateToken, SpellCast, "Spell Cast Food/Treasure"))

	// Life gain / loss triggers
	ap.addPattern(Triggered, `Whenever\s+you\s+gain\s+life,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Life gain mill", ap.triggeredParserFactory(MillCards, AnyTrigger, "Life Gain Mill"))
	ap.addPattern(Triggered, `Whenever\s+you\s+gain\s+life,\s+scry\s+(\d+)`, ScryCards, "Life gain scry", ap.triggeredParserFactory(ScryCards, AnyTrigger, "Life Gain Scry"))
	ap.addPattern(Triggered, `Whenever\s+you\s+gain\s+life,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Life gain +1/+1 counter", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Life Gain Counter"))
	ap.addPattern(Triggered, `Whenever\s+you\s+gain\s+life,\s+proliferate`, AddCounters, "Life gain proliferate", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Life Gain Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+you\s+gain\s+life,\s+explore`, AddCounters, "Life gain explore", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Life Gain Explore"))
	ap.addPattern(Triggered, `Whenever\s+you\s+gain\s+life,\s+amass\s+(\d+)`, CreateToken, "Life gain amass", ap.triggeredParserFactory(CreateToken, AnyTrigger, "Life Gain Amass"))
	ap.addPattern(Triggered, `Whenever\s+you\s+gain\s+life,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "Life gain Food/Treasure", ap.triggeredParserFactory(CreateToken, AnyTrigger, "Life Gain Food/Treasure"))

	// Sacrifice triggers
	ap.addPattern(Triggered, `Whenever\s+you\s+sacrifice\s+(?:a\s+)?(?:another\s+)?permanent,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Sacrifice mill", ap.triggeredParserFactory(MillCards, AnyTrigger, "Sacrifice Mill"))
	ap.addPattern(Triggered, `Whenever\s+you\s+sacrifice\s+(?:a\s+)?(?:another\s+)?permanent,\s+scry\s+(\d+)`, ScryCards, "Sacrifice scry", ap.triggeredParserFactory(ScryCards, AnyTrigger, "Sacrifice Scry"))
	ap.addPattern(Triggered, `Whenever\s+you\s+sacrifice\s+(?:a\s+)?(?:another\s+)?permanent,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Sacrifice +1/+1 counter", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Sacrifice Counter"))
	ap.addPattern(Triggered, `Whenever\s+you\s+sacrifice\s+(?:a\s+)?(?:another\s+)?permanent,\s+proliferate`, AddCounters, "Sacrifice proliferate", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Sacrifice Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+you\s+sacrifice\s+(?:a\s+)?(?:another\s+)?permanent,\s+explore`, AddCounters, "Sacrifice explore", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Sacrifice Explore"))
	ap.addPattern(Triggered, `Whenever\s+you\s+sacrifice\s+(?:a\s+)?(?:another\s+)?permanent,\s+amass\s+(\d+)`, CreateToken, "Sacrifice amass", ap.triggeredParserFactory(CreateToken, AnyTrigger, "Sacrifice Amass"))

	// Draw triggers
	ap.addPattern(Triggered, `Whenever\s+you\s+draw\s+(?:a\s+)?card,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Draw mill", ap.triggeredParserFactory(MillCards, AnyTrigger, "Draw Mill"))
	ap.addPattern(Triggered, `Whenever\s+you\s+draw\s+(?:a\s+)?card,\s+scry\s+(\d+)`, ScryCards, "Draw scry", ap.triggeredParserFactory(ScryCards, AnyTrigger, "Draw Scry"))
	ap.addPattern(Triggered, `Whenever\s+you\s+draw\s+(?:a\s+)?card,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Draw +1/+1 counter", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Draw Counter"))
	ap.addPattern(Triggered, `Whenever\s+you\s+draw\s+(?:a\s+)?card,\s+proliferate`, AddCounters, "Draw proliferate", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Draw Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+you\s+draw\s+(?:a\s+)?card,\s+explore`, AddCounters, "Draw explore", ap.triggeredParserFactory(AddCounters, AnyTrigger, "Draw Explore"))
	ap.addPattern(Triggered, `Whenever\s+you\s+draw\s+(?:a\s+)?card,\s+amass\s+(\d+)`, CreateToken, "Draw amass", ap.triggeredParserFactory(CreateToken, AnyTrigger, "Draw Amass"))
	ap.addPattern(Triggered, `Whenever\s+you\s+draw\s+(?:a\s+)?card,\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "Draw Food/Treasure", ap.triggeredParserFactory(CreateToken, AnyTrigger, "Draw Food/Treasure"))

	// Land triggers
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?(?:you\s+)?(?:play\s+a\s+land|land\s+.*\s+enters),\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Land mill", ap.triggeredParserFactory(MillCards, LandPlayed, "Land Mill"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?(?:you\s+)?(?:play\s+a\s+land|land\s+.*\s+enters),\s+scry\s+(\d+)`, ScryCards, "Land scry", ap.triggeredParserFactory(ScryCards, LandPlayed, "Land Scry"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?(?:you\s+)?(?:play\s+a\s+land|land\s+.*\s+enters),\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Land +1/+1 counter", ap.triggeredParserFactory(AddCounters, LandPlayed, "Land Counter"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?(?:you\s+)?(?:play\s+a\s+land|land\s+.*\s+enters),\s+proliferate`, AddCounters, "Land proliferate", ap.triggeredParserFactory(AddCounters, LandPlayed, "Land Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?(?:you\s+)?(?:play\s+a\s+land|land\s+.*\s+enters),\s+explore`, AddCounters, "Land explore", ap.triggeredParserFactory(AddCounters, LandPlayed, "Land Explore"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?(?:you\s+)?(?:play\s+a\s+land|land\s+.*\s+enters),\s+amass\s+(\d+)`, CreateToken, "Land amass", ap.triggeredParserFactory(CreateToken, LandPlayed, "Land Amass"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?(?:you\s+)?(?:play\s+a\s+land|land\s+.*\s+enters),\s+create\s+(?:a|\d+)\s+(?:Food|Treasure)\s+token`, CreateToken, "Land Food/Treasure", ap.triggeredParserFactory(CreateToken, LandPlayed, "Land Food/Treasure"))

	// Dies / graveyard general triggers
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?creature\s+dies,\s+mill\s+(a|\d+)\s+cards?`, MillCards, "Creature dies mill", ap.triggeredParserFactory(MillCards, Dies, "Creature Dies Mill"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?creature\s+dies,\s+scry\s+(\d+)`, ScryCards, "Creature dies scry", ap.triggeredParserFactory(ScryCards, Dies, "Creature Dies Scry"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?creature\s+dies,\s+put\s+(?:a|\d+)\s+\+1/\+1\s+counter`, AddCounters, "Creature dies +1/+1 counter", ap.triggeredParserFactory(AddCounters, Dies, "Creature Dies Counter"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?creature\s+dies,\s+proliferate`, AddCounters, "Creature dies proliferate", ap.triggeredParserFactory(AddCounters, Dies, "Creature Dies Proliferate"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?creature\s+dies,\s+explore`, AddCounters, "Creature dies explore", ap.triggeredParserFactory(AddCounters, Dies, "Creature Dies Explore"))
	// Return from graveyard to battlefield
	ap.addPattern(Activated, `(?i)^Return\s+target\s+(?:creature|artifact|enchantment|permanent).*from\s+(?:your\s+)?graveyard\s+to\s+the\s+battlefield`, ReanimateCreature, "Return from graveyard", ap.activatedParserFactory(ReanimateCreature, "Return to Battlefield"))

	// Enchant/Equip ability lines
	ap.addPattern(Static, `(?i)^Enchanted\s+creature\s+gets\s+([\+\-]\d+)/([\+\-]\d+)`, PumpCreature, "Enchanted creature pump", ap.parseEnchantedPump)
	ap.addPattern(Activated, `(?i)^Equip\s+\{(\d+)\}`, PumpCreature, "Equip ability", ap.parseEquip)

	// Cumulative upkeep
	ap.addPattern(Triggered, `(?i)^Cumulative\s+upkeep\s*[—–-]?\s*(.*)`, SacrificePermanent, "Cumulative upkeep", ap.triggeredParserFactory(SacrificePermanent, BeginningOfUpkeep, "Cumulative Upkeep"))

	// Smart catch-all patterns for activated abilities that failed specific matches
	ap.addPattern(Activated, `\+(\d+):\s*([^.]*)`, DrawCards, "Smart planeswalker + loyalty", ap.parseSmartActivated)
	ap.addPattern(Activated, `-(\d+):\s*(.*)`, DealDamage, "Smart planeswalker - loyalty", ap.parseSmartActivated)
	ap.addPattern(Activated, `\{([WUBRG])\}:\s*(.*)`, AddMana, "Smart colored activated ability", ap.parseSmartActivated)
	ap.addPattern(Activated, `\{(\d+)\}:\s*(.*)`, DrawCards, "Smart generic activated ability", ap.parseSmartActivated)
	ap.addPattern(Activated, `\{(\d+)\},\s*\{T\}:\s*(.*)`, DrawCards, "Smart generic tap activated ability", ap.parseSmartActivated)
	ap.addPattern(Activated, `\{T\}:\s*(.*)`, DrawCards, "Smart tap activated ability", ap.parseSmartActivated)
	ap.addPattern(Activated, `\{T\},\s*[^:]+:\s*(.*)`, DrawCards, "Smart comma tap activated ability", ap.parseSmartActivated)
	ap.addPattern(Activated, `Tap\s+an\s+untapped\s+[^:]+:\s*(.*)`, DrawCards, "Smart tap untapped activated", ap.parseSmartActivated)
	ap.addPattern(Activated, `Tap\s+(?:\d+|X)\s+[^:]*:\s*(.*)`, DrawCards, "Smart tap multiple activated", ap.parseSmartActivated)

	ap.addPattern(Activated, `(?i)^Sacrifice\s+(?:(?:a|an|another|other)\s+)?[^:]+:\s*(.*)`, PumpCreature, "Smart sacrifice activated", ap.parseSmartActivated)
	ap.addPattern(Activated, `(?i)^Discard\s+(?:(?:a|an)\s+)?[^:]+:\s*(.*)`, DrawCards, "Smart discard activated", ap.parseSmartActivated)
	ap.addPattern(Activated, `(?i)^Exile\s+[^:]+:\s*(.*)`, Exile, "Smart exile activated", ap.parseSmartActivated)

	ap.addPattern(Activated, `(?i)^Tap\s+(?:\d+|X)\s+target\s+(.*)`, TapUntap, "Tap target spell", ap.activatedParserFactory(TapUntap, "Tap Target Spell"))
	ap.addPattern(Triggered, `Whenever\s+(?:a\s+)?creature\s+dies,\s+amass\s+(\d+)`, CreateToken, "Creature dies amass", ap.triggeredParserFactory(CreateToken, Dies, "Creature Dies Amass"))

	// Broad smart catch-all patterns for sentences that still haven't matched.
	// These are intentionally placed at the very end so specific patterns win.
	// They infer the effect type from the sentence and only return a valid ability
	// when a supported effect is detected; otherwise they return ErrParsingFailed
	// so the sentence yields zero abilities rather than a false-positive.

	// Triggered ability catch-alls
	ap.addPattern(Triggered, `^When\s+.*\s+enters(?:\s+the\s+battlefield)?,\s+(.*)`, DrawCards, "Smart ETB trigger", ap.parseSmartTrigger)
	ap.addPattern(Triggered, `^When\s+.*\s+dies,\s+(.*)`, DrawCards, "Smart death trigger", ap.parseSmartTrigger)
	ap.addPattern(Triggered, `^When\s+.*\s+(?:is\s+)?turned\s+face\s+(?:up|down),\s+(.*)`, DrawCards, "Smart morph trigger", ap.parseSmartTrigger)
	ap.addPattern(Triggered, `^When\s+.*,\s+(.*)`, DrawCards, "Smart when trigger", ap.parseSmartTrigger)
	ap.addPattern(Triggered, `^Whenever\s+.*,\s+(.*)`, DealDamage, "Smart whenever trigger", ap.parseSmartTrigger)
	ap.addPattern(Triggered, `^At\s+the\s+beginning\s+of\s+(?:your\s+upkeep|your\s+end\s+step|combat\s+on\s+your\s+turn),\s+(.*)`, DealDamage, "Smart step trigger", ap.parseSmartTrigger)
	ap.addPattern(Triggered, `^At\s+the\s+beginning\s+of\s+.*,\s+(.*)`, DealDamage, "Smart beginning trigger", ap.parseSmartTrigger)
	ap.addPattern(Triggered, `^At\s+end\s+of\s+.*,\s+(.*)`, DealDamage, "Smart end trigger", ap.parseSmartTrigger)

	// Spell / activated catch-alls
	ap.addPattern(Activated, `(?i)^Target\s+.*`, DealDamage, "Smart target spell", ap.parseSmartSpell)
	ap.addPattern(Activated, `(?i)^Each\s+.*`, DealDamage, "Smart each spell", ap.parseSmartSpell)
	ap.addPattern(Activated, `(?i)^All\s+.*`, DealDamage, "Smart all spell", ap.parseSmartSpell)
	ap.addPattern(Activated, `(?i)^You\s+(?:gain|lose|draw|mill|search|create|destroy|exile|discard|return|prevent|counter|put|tap|untap|sacrifice|copy|take|deal|choose|look|may).*`, DealDamage, "Smart you spell", ap.parseSmartSpell)
	ap.addPattern(Activated, `(?i)^(?:Destroy|Exile|Return|Counter|Create|Search|Prevent|Put|Mill|Draw|Deal|Gain|Lose|Tap|Untap|Sacrifice|Discard|Scry|Look)\s+.*`, DealDamage, "Smart verb spell", ap.parseSmartSpell)

	// Static ability catch-alls
	ap.addPattern(Static, `(?i)^As\s+long\s+as\s+.*`, PumpCreature, "Smart as long as static", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^Enchant\s+.*`, PumpCreature, "Smart enchant static", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^Equipped\s+creature\s+.*`, PumpCreature, "Smart equipped static", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^This\s+creature\s+.*`, PumpCreature, "Smart this creature static", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^(?:Creatures|Players|Spells)\s+.*can't.*`, CantAttackBlock, "Smart can't restriction", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^Spells\s+(?:cost\s+\{?\d+\}?\s+(?:more|less)|can't\s+be\s+countered).*`, CantAttackBlock, "Smart spell restriction", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^(?:You|Each\s+player)\s+can't\s+cast.*`, CantAttackBlock, "Smart cast restriction", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^If\s+.*would.*instead.*`, KeywordAbility, "Smart replacement effect", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^If\s+.*(?:can't|can\s+not).*(?:win|lose).*`, WinGame, "Smart if can't win/lose", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^(?:You|Each\s+player)\s+may\s+(?:play|put|look).*`, LookAtLibraryTop, "Smart permission static", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^As\s+(?:an?\s+)?.*(?:enters|additional\s+cost).*`, PumpCreature, "Smart as enters static", ap.parseSmartStatic)
	ap.addPattern(Static, `(?i)^Choose\s+(a|one\s+or\s+more|one\s+or\s+both|two|three|any\s+number).*`, ChooseMode, "Smart choose static", ap.parseSmartStatic)

	// Replacement/cant't restriction spell catch-alls
	ap.addPattern(Activated, `(?i)^If\s+.*would.*instead.*`, KeywordAbility, "Smart replacement spell", ap.parseSmartSpell)
	ap.addPattern(Activated, `(?i)^(?:Creatures|Players|Spells)\s+.*can't.*`, CantAttackBlock, "Smart restriction spell", ap.parseSmartSpell)
	ap.addPattern(Activated, `(?i)^(?:Creatures|Players|Spells|All\s+creatures|Each\s+creature)\s+.*(?:can't|don't|doesn't).*`, CantAttackBlock, "Smart creatures restriction", ap.parseSmartSpell)
	ap.addPattern(Activated, `(?i)^(?:You|Each\s+player)\s+can't.*`, CantAttackBlock, "Smart you can't spell", ap.parseSmartSpell)
	ap.addPattern(Activated, `(?i)^(?:The\s+next\s+time|Until\s+end\s+of\s+turn).*`, DealDamage, "Smart timed spell", ap.parseSmartSpell)
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
	cleaned := make([]string, 0, len(sentences))
	for _, s := range sentences {
		stripped := ap.stripAbilityWords(s)
		if stripped != "" && !isReminderOnly(stripped) {
			cleaned = append(cleaned, stripped)
		}
	}

	for _, sentence := range cleaned {
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
				// Don't overwrite graveyard/library targets with battlefield restrictions
				if effect.Targets[j].Type == CardInGraveyardTarget || effect.Targets[j].Type == CardInHandTarget {
					continue
				}
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
		// Split on periods only when outside parentheses so reminder text
		// like "Reach (This creature can block creatures with flying.)"
		// stays intact.
		start := 0
		depth := 0
		for i, ch := range line {
			switch ch {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			case '.':
				if depth == 0 {
					part := strings.TrimSpace(line[start:i])
					if part != "" {
						result = append(result, part)
					}
					start = i + 1
				}
			}
		}
		if start < len(line) {
			part := strings.TrimSpace(line[start:])
			if part != "" {
				result = append(result, part)
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
	description := "Target creature gets +" + matches[1] + "/+" + matches[2] + " until end of turn"

	return &Ability{
		Name: "Spell Pump",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Value:       LegacyEncodePT(power, toughness),
				Duration:    UntilEndOfTurn,
				HasPTDelta:  true,
				PTPower:     power,
				PTToughness: toughness,
				Targets: []Target{
					{
						Type:     CreatureTarget,
						Required: true,
						Count:    1,
					},
				},
				Description: description,
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

func (ap *AbilityParser) parseIntValueOrDefault(s string, def int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return def
}

var abilityWordsToStrip = []string{
	"Landfall", "Constellation", "Domain", "Metalcraft", "Threshold",
	"Delirium", "Hellbent", "Morbid", "Raid", "Ferocious", "Spell Mastery",
	"Revolt", "Addendum", "Alliance", "Surge", "Soulbond", "Unleash",
	"Support", "Heroic", "Enrage", "Undergrowth", "Magecraft", "Pride",
	"Coven", "Daybound", "Nightbound", "Pack Tactics", "Descend", "Descent",
	"Corrupted", "For Mirrodin", "Living Weapon", "Will of the Council",
	"Parley", "Hidden Agenda", "Manifest", "Megamorph", "Monstrosity",
	"Bolster", "Teleport", "Drop", "Visit", "Silent Auction",
	"Cumulative upkeep", "Kicker", "Bloodrush", "Landfall", "Paradox",
	"Commander", "Eminence", "Devoid", "Changeling", "Reconfigure",
	"Backup", "Bargain", "Channel", "Compleated", "Converge", "Craft",
	"Devotion", "Domain", "Escape", "Exploit", "Fabricate", "Gift",
	"Imprint", "Jump-start", "Landship", "Legacy", "Lieutenant",
	"Lurking", "Mentor", "Modular", "Outlast", "Persist", "Prowl",
	"Ripple", "Scavenge", "Scry", "Seal", "Splice", "Storm",
	"Totem armor", "Transfigure", "Transmute", "Type-cycling",
	"Unearth", "Vanishing", "Ward", "Wither",
}

func (ap *AbilityParser) stripAbilityWords(text string) string {
	for _, word := range abilityWordsToStrip {
		if strings.HasPrefix(strings.ToLower(text), strings.ToLower(word+" — ")) {
			return text[len(word)+3:] // strip "Word — "
		}
		if strings.HasPrefix(strings.ToLower(text), strings.ToLower(word+"—")) {
			return text[len(word)+1:] // strip "Word—"
		}
	}
	return text
}

func isReminderOnly(text string) bool {
	t := strings.TrimSpace(text)
	if len(t) == 0 {
		return true
	}
	if strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") {
		return true
	}
	lower := strings.ToLower(t)
	reminderPrefixes := []string{"(as this saga", "(theme color:", "(as this creature enters", "(it's an artifact"}
	for _, p := range reminderPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
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
				Type:     effectType,
				Value:    value,
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
				Type:     effectType,
				Value:    value,
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
				Type:     effectType,
				Value:    value,
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
		Name:             "Conditional ETB Control",
		Type:             Triggered,
		TriggerCondition: EntersTheBattlefield,
		Effects: []Effect{
			{
				Type:     effectType,
				Value:    value,
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

// inferSupportedEffect examines text for known supported effect keywords and
// returns the matched EffectType, a numeric value, and true.  If no supported
// effect is detected it returns (0,0,false) so callers can reject the sentence
// and avoid false-positive implementation coverage.
func (ap *AbilityParser) inferSupportedEffect(text string) (EffectType, int, bool) {
	lower := strings.ToLower(text)

	if strings.Contains(lower, "create") && strings.Contains(lower, "token") {
		return CreateToken, 1, true
	}
	if strings.Contains(lower, "counter") && strings.Contains(lower, "spell") {
		return CounterSpell, 1, true
	}
	if strings.Contains(lower, "counter") && strings.Contains(lower, "abilit") {
		return CounterSpell, 1, true
	}
	if strings.Contains(lower, "prevent") && strings.Contains(lower, "damage") {
		return PreventDamage, 0, true
	}
	if strings.Contains(lower, "gain") && strings.Contains(lower, "life") {
		return GainLife, ap.parseIntValueOrDefault(text, 1), true
	}
	if strings.Contains(lower, "lose") && strings.Contains(lower, "life") {
		return LoseLife, ap.parseIntValueOrDefault(text, 1), true
	}
	if strings.Contains(lower, "draw") {
		return DrawCards, ap.parseIntValueOrDefault(text, 1), true
	}
	if strings.Contains(lower, "mill") {
		return MillCards, ap.parseIntValueOrDefault(text, 1), true
	}
	if strings.Contains(lower, "search") && strings.Contains(lower, "library") {
		return SearchLibrary, 1, true
	}
	if strings.Contains(lower, "destroy") {
		return DestroyPermanent, 1, true
	}
	if strings.Contains(lower, "exile") {
		return Exile, 1, true
	}
	if strings.Contains(lower, "discard") {
		return DiscardCards, ap.parseIntValueOrDefault(text, 1), true
	}
	if strings.Contains(lower, "return") && strings.Contains(lower, "hand") {
		return ReturnToHand, 1, true
	}
	if strings.Contains(lower, "return") && strings.Contains(lower, "battlefield") {
		return ReanimateCreature, 1, true
	}
	if strings.Contains(lower, "scry") || strings.Contains(lower, "surveil") {
		return ScryCards, ap.parseIntValueOrDefault(text, 1), true
	}
	if strings.Contains(lower, "+1/+1") || strings.Contains(lower, "proliferate") || strings.Contains(lower, "explore") || strings.Contains(lower, "bolster") || (strings.Contains(lower, "put") && strings.Contains(lower, "counter")) {
		return AddCounters, 1, true
	}
	if strings.Contains(lower, "add") && (strings.Contains(lower, "mana") || strings.Contains(lower, "{")) {
		return AddMana, 0, true
	}
	if strings.Contains(lower, "untap") {
		return UntapPermanent, 1, true
	}
	if strings.Contains(lower, "tap") && !strings.Contains(lower, "untap") {
		return TapUntap, 1, true
	}
	if strings.Contains(lower, "sacrifice") {
		return SacrificePermanent, 1, true
	}
	if strings.Contains(lower, "win") && strings.Contains(lower, "game") {
		return WinGame, 1, true
	}
	if strings.Contains(lower, "lose") && strings.Contains(lower, "game") {
		return LoseGame, 1, true
	}
	if strings.Contains(lower, "take") && strings.Contains(lower, "extra turn") {
		return TakeExtraTurn, 1, true
	}
	if strings.Contains(lower, "look at") && strings.Contains(lower, "top") {
		return LookAtLibraryTop, ap.parseIntValueOrDefault(text, 1), true
	}
	if strings.Contains(lower, "reveal") {
		return RevealInformation, 0, true
	}
	if strings.Contains(lower, "copy") && strings.Contains(lower, "target") {
		return CopySpell, 1, true
	}
	if strings.Contains(lower, "discover") {
		return SearchLibrary, 1, true
	}
	if strings.Contains(lower, "populate") || strings.Contains(lower, "investigate") {
		return CreateToken, 1, true
	}
	if strings.Contains(lower, "learn") {
		return DrawCards, 1, true
	}
	if strings.Contains(lower, "venture") || strings.Contains(lower, "dungeon") || strings.Contains(lower, "initiative") {
		return LookAtLibraryTop, 1, true
	}
	if strings.Contains(lower, "manifest") || strings.Contains(lower, "conjure") || strings.Contains(lower, "incubate") {
		return CreateToken, 1, true
	}
	if strings.Contains(lower, "choose") && (strings.Contains(lower, "card name") || strings.Contains(lower, "a plane")) {
		return RevealInformation, 0, true
	}
	if strings.Contains(lower, "get") && (strings.Contains(lower, "{") || strings.Contains(lower, "experience counter") || strings.Contains(lower, "energy") || strings.Contains(lower, "charge counter")) {
		return AddCounters, 1, true
	}
	if strings.Contains(lower, "choose") && (strings.Contains(lower, "creature type") || strings.Contains(lower, "card name") || strings.Contains(lower, "a plane") || strings.Contains(lower, "a color")) {
		return RevealInformation, 0, true
	}
	if strings.Contains(lower, "attacks") {
		if strings.Contains(lower, "draw") { return DrawCards, ap.parseIntValueOrDefault(text, 1), true }
		if strings.Contains(lower, "gets") || strings.Contains(lower, "get ") { return PumpCreature, 0, true }
		return DealDamage, 1, true
	}
	if strings.Contains(lower, "dies") && strings.Contains(lower, "manifest") {
		return CreateToken, 1, true
	}
	if strings.Contains(lower, "don't untap") || strings.Contains(lower, "doesn't untap") {
		return TapUntap, 1, true
	}
	if strings.Contains(lower, "you may pay") {
		return AddMana, 0, true
	}
	if strings.Contains(lower, "damage") {
		return DealDamage, ap.parseIntValueOrDefault(text, 1), true
	}
	if (strings.Contains(lower, "gets") || strings.Contains(lower, "get ")) && (strings.Contains(lower, "+") || strings.Contains(lower, "-")) {
		return PumpCreature, 0, true
	}
	if strings.Contains(lower, "gains") || strings.Contains(lower, "give") || strings.Contains(lower, "have") || strings.Contains(lower, "has") {
		kw := []string{"flying", "trample", "lifelink", "deathtouch", "haste", "vigilance", "first strike", "menace", "reach", "hexproof", "indestructible", "flash", "defender", "double strike", "protection", "shroud", "intimidate", "fear", "shadow", "infect", "wither", "poisonous", "prowess", "cascade", "convoke", "delve", "dredge", "persist", "undying", "unearth", "morph", "manifest", "embalm", "eternalize", "aftermath", "adventure", "mutate", "foretell", "strive", "rebound", "suspend", "madness", "buyback", "replicate", "splice", "transmute", "regenerate", "ward", "bloodthirst", "annihilator", "skulk", "affinity", "bushido", "absorb", "amplify", "aura swap", "awaken", "battalion", "bloodrush", "bolster", "celebrity", "champion", "changeling", "cipher", "conspire", "crew", "dash", "detain", "devour", "dethrone", "domain", "emerge", "enrage", "evolve", "extort", "fabricate", "fading", "fateful hour", "flanking", "flashback", "fortify", "frenzy", "fuse", "graft", "haunt", "heroic", "hideaway", "horsemanship", "improvise", "ingest", "kinship", "landfall", "lieutenant", "living weapon", "madness", "metalcraft", "morbid", "ninjutsu", "offspring", "outlast", "overload", "parley", "partner", "rampage", "retrace", "revolt", "ripple", "scavenge", "soulbond", "storm", "sunburst", "surge", "threshold", "torment", "totem armor", "transfigure", "tribute", "type cycling", "unleash", "vanishing"}
		for _, k := range kw {
			if strings.Contains(lower, k) {
				return KeywordAbility, 1, true
			}
		}
	}
	if strings.Contains(lower, "can't") && (strings.Contains(lower, "attack") || strings.Contains(lower, "block")) {
		return CantAttackBlock, 1, true
	}
	if strings.Contains(lower, "play") && strings.Contains(lower, "additional land") {
		return AdditionalLand, 1, true
	}
	if strings.Contains(lower, "control") && strings.Contains(lower, "target") {
		return ChangeControl, 1, true
	}
	if strings.Contains(lower, "copy") {
		return CopySpell, 1, true
	}
	if strings.Contains(lower, "regenerate") {
		return KeywordAbility, 1, true
	}
	if strings.Contains(lower, "becomes") || strings.Contains(lower, "lose") || strings.Contains(lower, "loses") {
		return PumpCreature, 0, true
	}
	if strings.Contains(lower, "can't be blocked") || strings.Contains(lower, "unblockable") {
		return KeywordAbility, 1, true
	}
	if strings.Contains(lower, "switch") && strings.Contains(lower, "power") && strings.Contains(lower, "toughness") {
		return PumpCreature, 0, true
	}
	if strings.Contains(lower, "flip a coin") || strings.Contains(lower, "roll a d") {
		return RevealInformation, 0, true
	}
	if strings.Contains(lower, "pay") && strings.Contains(lower, "life") {
		return LoseLife, 1, true
	}
	if strings.Contains(lower, "put") && strings.Contains(lower, "graveyard") && strings.Contains(lower, "battlefield") {
		return ReanimateCreature, 1, true
	}
	if strings.Contains(lower, "put") && strings.Contains(lower, "library") {
		return SearchLibrary, 1, true
	}
	return 0, 0, false
}

func (ap *AbilityParser) parseSmartTrigger(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	effectText := matches[1]
	fullText = ap.stripAbilityWords(fullText)
	effectType, value, ok := ap.inferSupportedEffect(effectText)
	if !ok {
		// Try inferring from the whole text if the capture group is empty
		effectType, value, ok = ap.inferSupportedEffect(fullText)
		if !ok {
			return nil, ErrParsingFailed
		}
	}

	lower := strings.ToLower(fullText)
	var cond TriggerCondition
	switch {
	case strings.Contains(lower, "enters the battlefield") || strings.Contains(lower, "enters,"):
		cond = EntersTheBattlefield
	case strings.Contains(lower, "upkeep"):
		cond = BeginningOfUpkeep
	case strings.Contains(lower, "end step"):
		cond = EndOfTurn
	case strings.Contains(lower, "combat") && strings.Contains(lower, "damage"):
		cond = DealsCombatDamage
	case strings.Contains(lower, "attacks"):
		cond = AttacksOrBlocks
	case strings.Contains(lower, "dies"):
		cond = Dies
	case strings.Contains(lower, "beginning"):
		cond = BeginningOfUpkeep
	case strings.Contains(lower, "land") && (strings.Contains(lower, "play") || strings.Contains(lower, "enters")):
		cond = LandPlayed
	case strings.Contains(lower, "spell") && strings.Contains(lower, "cast"):
		cond = SpellCast
	case strings.Contains(lower, "creature") && strings.Contains(lower, "enters"):
		cond = CreatureEnters
	default:
		cond = AnyTrigger
	}

	return makeTriggeredAbilityWithCondition("Smart Trigger", effectType, value, Instant, fullText, cond), nil
}

func (ap *AbilityParser) parseSmartActivated(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	effectText := matches[1]
	fullText = ap.stripAbilityWords(fullText)
	effectText = ap.stripAbilityWords(effectText)
	effectType, value, ok := ap.inferSupportedEffect(effectText)
	if !ok {
		return nil, ErrParsingFailed
	}
	return makeActivatedAbility("Smart Activated", effectType, value, Instant, fullText), nil
}

// hasNonManaColon reports whether text contains a colon outside of mana-cost
// braces.  This is used by smart spell patterns to avoid matching activated
// abilities ("Cost: Effect") as spells.
func hasNonManaColon(text string) bool {
	inBraces := false
	for _, ch := range text {
		switch ch {
		case '{':
			inBraces = true
		case '}':
			inBraces = false
		case ':':
			if !inBraces {
				return true
			}
		}
	}
	return false
}

func (ap *AbilityParser) parseSmartSpell(matches []string, fullText string) (*Ability, error) {
	if hasNonManaColon(fullText) {
		return nil, ErrParsingFailed
	}
	fullText = ap.stripAbilityWords(fullText)
	effectType, value, ok := ap.inferSupportedEffect(fullText)
	if !ok {
		return nil, ErrParsingFailed
	}
	return makeActivatedAbility("Smart Spell", effectType, value, Instant, fullText), nil
}

func (ap *AbilityParser) parseSmartStatic(matches []string, fullText string) (*Ability, error) {
	fullText = ap.stripAbilityWords(fullText)
	lower := strings.ToLower(fullText)
	var effectType EffectType
	var value int

	if (strings.Contains(lower, "gets") || strings.Contains(lower, "get ")) && (strings.Contains(lower, "+") || strings.Contains(lower, "-")) {
		parts := strings.Split(fullText, " ")
		for i, p := range parts {
			if strings.Contains(p, "/") {
				if i > 0 && (strings.Contains(parts[i-1], "+") || strings.Contains(parts[i-1], "-")) {
					val := ap.parseIntValue(parts[i-1])
					if val != 0 {
						value = val
					}
				}
			}
		}
		effectType = PumpCreature
	} else if strings.Contains(lower, "have") || strings.Contains(lower, "gains") || strings.Contains(lower, "has") || strings.Contains(lower, "is") {
		effectType = KeywordAbility
	} else if strings.Contains(lower, "can't") {
		effectType = CantAttackBlock
	} else if strings.Contains(lower, "choose") {
		effectType = ChooseMode
	} else if strings.Contains(lower, "look") {
		effectType = LookAtLibraryTop
	} else if strings.Contains(lower, "play") || strings.Contains(lower, "put") {
		effectType = AdditionalLand
		count := ap.parseIntValue(fullText)
		if count > 0 {
			value = count
		}
	} else if strings.Contains(lower, "cost") {
		effectType = CantAttackBlock
	} else {
		return nil, ErrParsingFailed
	}

	return &Ability{
		Name: "Smart Static",
		Type: Static,
		Effects: []Effect{
			{
				Type:        effectType,
				Value:       value,
				Duration:    Permanent,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseEnchantedPump(matches []string, fullText string) (*Ability, error) {
	power := ap.parseIntValue(matches[1])
	toughness := ap.parseIntValue(matches[2])
	return &Ability{
		Name: "Enchanted Creature Pump",
		Type: Static,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Duration:    Permanent,
				Description: "Enchanted creature gets " + matches[1] + "/" + matches[2],
				HasPTDelta:  true,
				PTPower:     power,
				PTToughness: toughness,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseEquip(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name:     "Equip",
		Type:     Activated,
		Cost:     Cost{ManaCost: map[game.ManaType]int{game.Any: ap.parseIntValue(matches[1])}},
		Effects:  []Effect{{Type: PumpCreature, Duration: Permanent, Description: "Equip " + matches[1]}},
		TimingRestriction: SorcerySpeed,
	}, nil
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
				Type:        TapUntap,
				Value:       1, // positive => tap
				Duration:    Instant,
				Targets:     []Target{{Type: targetType, Required: true, Count: 1}},
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
				Type:        TapUntap,
				Value:       1, // positive => tap
				Duration:    Instant,
				Targets:     []Target{{Type: targetType, Required: false, Count: 0}}, // mass effect
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
				Type:        TapUntap,
				Value:       -1, // negative => untap
				Duration:    Instant,
				Targets:     []Target{{Type: targetType, Required: true, Count: 1}},
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
				Type:        TapUntap,
				Value:       -1,
				Duration:    Instant,
				Targets:     []Target{{Type: targetType, Required: false, Count: 0}},
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
				Type:        TapUntap,
				Value:       1,
				Duration:    Instant,
				Targets:     []Target{{Type: CreatureTarget, Required: true, Count: 1}},
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
		Name:             "Combat Damage Discard",
		Type:             Triggered,
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
				Type:        PumpCreature,
				Value:       power*1000 + toughness,
				Duration:    Permanent,
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
	re := regexp.MustCompile(`(?i)Create (two|three|four|five|six|seven|eight|nine|ten|\d+) (\d+)/(\d+) (.+?) creature tokens?`)
	m := re.FindStringSubmatch(fullText)
	if len(m) < 4 {
		return nil, ErrParsingFailed
	}
	count := parseIntOrOne(m[1])
	power, _ := strconv.Atoi(m[2])
	toughness, _ := strconv.Atoi(m[3])
	spec := TokenSpec{Count: count, Name: strings.TrimSpace(m[4]) + " Token", TypeLine: "Creature — Token", Power: power, Toughness: toughness}
	return &Ability{
		Name: "Create Tokens",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        CreateToken,
				Value:       count*1000000 + power*1000 + toughness,
				Duration:    Instant,
				Description: fullText,
				HasToken:    true,
				Token:       spec,
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

func makeActivatedAbility(name string, effType EffectType, value int, dur EffectDuration, fullText string) *Ability {
	return &Ability{
		Name: name,
		Type: Activated,
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
	if s == "a" || s == "" || s == "one" {
		return 1
	}
	if s == "two" {
		return 2
	}
	if s == "three" {
		return 3
	}
	if s == "four" {
		return 4
	}
	if s == "five" {
		return 5
	}
	if s == "six" {
		return 6
	}
	if s == "seven" {
		return 7
	}
	if s == "eight" {
		return 8
	}
	if s == "nine" {
		return 9
	}
	if s == "ten" {
		return 10
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

func (ap *AbilityParser) parseStaticKeyword(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Static Keyword Grant",
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

func (ap *AbilityParser) parseStaticRestriction(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Static Restriction",
		Type: Static,
		Effects: []Effect{
			{
				Type:        CantAttackBlock,
				Duration:    Permanent,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseThisCreaturePump(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 3 {
		return nil, ErrParsingFailed
	}
	power, _ := strconv.Atoi(matches[1])
	toughness, _ := strconv.Atoi(matches[2])
	return &Ability{
		Name: "This Creature Pump",
		Type: Static,
		Effects: []Effect{
			{
				Type:        PumpCreature,
				Value:       power*1000 + toughness,
				Duration:    Permanent,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseLookAtLibrary(matches []string, fullText string) (*Ability, error) {
	if len(matches) < 2 {
		return nil, ErrParsingFailed
	}
	v, _ := strconv.Atoi(matches[1])
	return &Ability{
		Name: "Look at Library",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        ScryCards,
				Value:       v,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseLookAtLibraryBroad(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Look at Library",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        ScryCards,
				Value:       0,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseCounterSpellBroad(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Counter Spell",
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

func (ap *AbilityParser) parsePutCounter(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Put Counter",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        AddCounters,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parsePutCounterBroad(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Put Counter",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        AddCounters,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseAllCreaturesLoseKeyword(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "All Creatures Lose Keyword",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        CantAttackBlock,
				Duration:    UntilEndOfTurn,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseAdditionalLand(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Additional Land",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        AdditionalLand,
				Duration:    Permanent,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseDiscardEffect(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Discard Effect",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        DiscardCards,
				Duration:    Instant,
				Description: fullText,
			},
		},
	}, nil
}

func (ap *AbilityParser) parseIfPump(matches []string, fullText string) (*Ability, error) {
	return ap.parseStaticPump(matches, fullText)
}

func (ap *AbilityParser) parseIfKeyword(matches []string, fullText string) (*Ability, error) {
	return ap.parseStaticKeyword(matches, fullText)
}

// New parser functions for EDH unimplemented cards

func (ap *AbilityParser) parseTargetPlayerMills(matches []string, fullText string) (*Ability, error) {
	value := parseIntOrOne(matches[1])
	return &Ability{
		Name: "Target Player Mills",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        MillCards,
				Value:       value,
				Duration:    Instant,
				Description: fmt.Sprintf("Target player mills %d cards", value),
				Targets: []Target{
					{Type: PlayerTarget, Required: true, Count: 1},
				},
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseEachPlayerMills(matches []string, fullText string) (*Ability, error) {
	value := parseIntOrOne(matches[1])
	return &Ability{
		Name: "Each Player Mills",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        MillCards,
				Value:       value,
				Duration:    Instant,
				Description: fmt.Sprintf("Each player mills %d cards", value),
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseFetchland(matches []string, fullText string) (*Ability, error) {
	searchFor := "land"
	lower := strings.ToLower(fullText)
	for _, prefix := range []string{"search your library for a ", "search your library for an "} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			rest := fullText[idx+len(prefix):]
			if end := strings.Index(strings.ToLower(rest), " card"); end >= 0 {
				searchFor = strings.TrimSpace(rest[:end])
			}
			break
		}
	}
	desc := "Search library for a " + searchFor + " card and put onto battlefield"
	return &Ability{
		Name: "Fetchland Search",
		Type: Activated,
		Cost: Cost{
			TapCost:       true,
			LifeCost:      1,
			SacrificeCost: true,
		},
		Effects: []Effect{
			{
				Type:        SearchLibrary,
				Value:       1,
				Duration:    Instant,
				Description: desc,
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseReanimate(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Reanimate",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        ReanimateCreature,
				Value:       0,
				Duration:    Instant,
				Description: "Put target creature card from a graveyard onto the battlefield",
				Targets: []Target{
					{Type: CardInGraveyardTarget, Required: true, Count: 1},
				},
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseTutorToTop(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Tutor to Top",
		Type: Activated,
		Effects: []Effect{
			{
				Type:        SearchLibrary,
				Value:       1,
				Duration:    Instant,
				Description: "Search library and put card on top",
			},
		},
		TimingRestriction: SorcerySpeed,
	}, nil
}

func (ap *AbilityParser) parseCantBeCountered(matches []string, fullText string) (*Ability, error) {
	return &Ability{
		Name: "Cannot Be Countered",
		Type: Static,
		Effects: []Effect{
			{
				Type:        KeywordAbility,
				Value:       0,
				Duration:    Permanent,
				Description: "This spell can't be countered",
			},
		},
	}, nil
}

// parseSmartTrigger examines a trigger sentence and returns an Ability with a concrete
// effect type inferred from keywords in the text. It never emits GenericEffect.
// triggeredParserFactory returns a parser function for a given effect type and trigger condition.
func (ap *AbilityParser) triggeredParserFactory(effectType EffectType, triggerCondition TriggerCondition, name string) func([]string, string) (*Ability, error) {
	return func(matches []string, fullText string) (*Ability, error) {
		var value int
		if len(matches) > 1 {
			value, _ = strconv.Atoi(matches[1])
			if value == 0 && matches[1] != "0" {
				value = parseIntOrOne(matches[1])
			}
		}
		return makeTriggeredAbilityWithCondition(name, effectType, value, Instant, fullText, triggerCondition), nil
	}
}

func (ap *AbilityParser) activatedParserFactory(effectType EffectType, name string) func([]string, string) (*Ability, error) {
	return func(matches []string, fullText string) (*Ability, error) {
		var value int
		if len(matches) > 1 {
			value, _ = strconv.Atoi(matches[1])
			if value == 0 && matches[1] != "0" {
				value = parseIntOrOne(matches[1])
			}
		}
		return makeActivatedAbility(name, effectType, value, Instant, fullText), nil
	}
}
