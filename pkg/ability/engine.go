// Package ability provides the execution engine for MTG abilities.
package ability

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/internal/logger"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// GameState represents the current state of the game for ability resolution.
type GameState interface {
	GetPlayer(name string) AbilityPlayer
	GetAllPlayers() []AbilityPlayer
	GetCurrentPlayer() AbilityPlayer
	GetActivePlayer() AbilityPlayer
	IsMainPhase() bool
	IsCombatPhase() bool
	CanActivateAbilities() bool
	AddManaToPool(player AbilityPlayer, manaType game.ManaType, amount int)
	DealDamage(source any, target any, amount int)
	DrawCards(player AbilityPlayer, count int)
	GainLife(player AbilityPlayer, amount int)
	LoseLife(player AbilityPlayer, amount int)
	DiscardCards(player AbilityPlayer, count int)
	SearchLibrary(player AbilityPlayer, count int)
	CreateToken(controller AbilityPlayer, token game.SimpleCard)
	PreventDamage(target any, amount int)
	MillCards(player AbilityPlayer, count int)
	ReanimateCreature(player AbilityPlayer, card game.SimpleCard)
	ScryLibrary(player AbilityPlayer, count int)
	TakeExtraTurn()
}

// AbilityPlayer represents a player in the game for ability purposes.
type AbilityPlayer interface {
	GetName() string
	GetLifeTotal() int
	SetLifeTotal(life int)
	GetHand() []any
	AddCardToHand(card any)
	GetCreatures() []any
	GetLands() []any
	CanPayCost(cost Cost) bool
	PayCost(cost Cost) error
	GetManaPool() map[game.ManaType]int
}

// AbilityPermanent represents a permanent for ability purposes.
type AbilityPermanent interface {
	GetID() uuid.UUID
	GetName() string
	GetOwner() AbilityPlayer
	GetController() AbilityPlayer
	IsTapped() bool
	Tap()
	Untap()
	GetAbilities() []*Ability
	AddAbility(ability *Ability)
	RemoveAbility(abilityID uuid.UUID)
}

// SummoningSickness interface for checking if a permanent has summoning sickness
type SummoningSickness interface {
	HasSummoningSickness() bool
}

// Track cards that failed to implement correctly (parsed but not supported, or no effects for spell types)
var unimplementedCards = map[string]string{}

func markUnimplementedCard(name, reason string) {
	if _, exists := unimplementedCards[name]; !exists {
		unimplementedCards[name] = reason
		logger.LogMeta("Unimplemented card: %s (%s)", name, reason)
	}
}

// GetUnimplementedCards returns a map[name]reason of unimplemented cards encountered.
func GetUnimplementedCards() map[string]string { return unimplementedCards }

// ExecutionEngine handles the execution of abilities and their effects.
type ExecutionEngine struct {
	gameState       GameState
	parser          *AbilityParser
	targetValidator *TargetValidator
	targetParser    *TargetParser
}

// NewExecutionEngine creates a new execution engine.
func NewExecutionEngine(gameState GameState) *ExecutionEngine {
	return &ExecutionEngine{
		gameState:       gameState,
		parser:          NewAbilityParser(),
		targetValidator: NewTargetValidator(gameState),
		targetParser:    NewTargetParser(),
	}
}

// ExecuteAbility executes an ability with the given targets.
func (ee *ExecutionEngine) ExecuteAbility(ability *Ability, controller AbilityPlayer, targets []any) error {
	logger.LogCard("Executing ability: %s", ability.Name)

	// Check if ability can be activated
	if !ee.canActivateAbility(ability, controller) {
		return ErrCannotActivate
	}

	// Validate targets before execution
	if err := ee.validateTargets(ability, controller, targets); err != nil {
		return err
	}

	// Pay costs
	if err := ee.payCosts(ability, controller, ability.Source); err != nil {
		return err
	}

	// For mana abilities, resolve immediately
	if ability.Type == Mana {
		return ee.resolveManaAbility(ability, controller)
	}

	// For other abilities, they would go on the stack
	// This is now handled by the spell casting engine
	logger.LogCard("Ability %s would be added to stack", ability.Name)
	return ee.resolveAbility(ability, controller, targets)
}

// canActivateAbility checks if an ability can be activated.
func (ee *ExecutionEngine) canActivateAbility(ability *Ability, controller AbilityPlayer) bool {
	// Check timing restrictions
	switch ability.TimingRestriction {
	case SorcerySpeed:
		if !ee.gameState.IsMainPhase() {
			return false
		}
	case OnlyDuringCombat:
		if !ee.gameState.IsCombatPhase() {
			return false
		}
	case OnlyOnYourTurn:
		if ee.gameState.GetCurrentPlayer().GetName() != controller.GetName() {
			return false
		}
	}

	// Check tap cost requirements
	if ability.Cost.TapCost {
		if tapper, ok := ability.Source.(interface{ IsTapped() bool }); ok {
			if tapper.IsTapped() {
				return false // Cannot activate tap abilities when source is tapped
			}

			// Check summoning sickness for non-mana abilities
			if ability.Type != Mana {
				// Check if this permanent has summoning sickness
				if summoningSickPerm, ok := ability.Source.(SummoningSickness); ok {
					if summoningSickPerm.HasSummoningSickness() {
						return false // Creatures with summoning sickness cannot use tap abilities (except mana abilities)
					}
				}
			}
		}
	}

	// Check usage limits
	if !ability.CanActivate(controller) {
		return false
	}

	// Check if player can pay costs
	if !controller.CanPayCost(ability.Cost) {
		return false
	}

	// Check if valid targets exist using the new target validation system
	if ee.requiresTargets(ability) && !ee.hasValidTargets(ability, controller) {
		return false
	}

	return true
}

// payCosts pays the costs for an ability.
func (ee *ExecutionEngine) payCosts(ability *Ability, controller AbilityPlayer, source any) error {
	if ability.Cost.TapCost {
		if tapper, ok := source.(interface{ Tap() }); ok {
			tapper.Tap()
		}
	}
	if ability.Cost.SacrificeCost && source != nil {
		if s, ok := ee.gameState.(interface{ SacrificeSource(object any) }); ok {
			s.SacrificeSource(source)
		}
	}
	return controller.PayCost(ability.Cost)
}

// resolveManaAbility resolves a mana ability immediately.
func (ee *ExecutionEngine) resolveManaAbility(ability *Ability, controller AbilityPlayer) error {
	oracleText := strings.ToLower(ability.OracleText)

	// Bloom Tender / Vivid: {T}: For each color among permanents you control, add one mana of that color.
	if strings.Contains(oracleText, "for each color among permanents you control") {
		colors := ee.collectDistinctPermanentColors(controller)
		for _, mt := range colors {
			ee.gameState.AddManaToPool(controller, mt, 1)
		}
		logger.LogCard("%s adds vivid mana for %d colors: %v", controller.GetName(), len(colors), colors)
		return nil
	}

	for _, effect := range ability.Effects {
		if effect.Type == AddMana {
			manaType := ee.determineManaType(ability, effect)

			// Chrome Mox imprint: "any of the exiled card's colors" → produce any-color mana
			if strings.Contains(oracleText, "exiled card") && strings.Contains(oracleText, "colors") {
				manaType = game.Any
			}

			ee.gameState.AddManaToPool(controller, manaType, effect.Value)
			logger.LogCard("%s adds %d %s mana", controller.GetName(), effect.Value, string(manaType))
		}
	}
	return nil
}

// collectDistinctPermanentColors returns the set of distinct colors among the
// controller's permanents, used by vivid mana abilities like Bloom Tender.
func (ee *ExecutionEngine) collectDistinctPermanentColors(controller AbilityPlayer) []game.ManaType {
	distinct := make(map[game.ManaType]bool)

	// Duck-type interface for accessing card source info
	type cardColorSource interface {
		GetSource() game.SimpleCard
	}

	check := func(perms []any) {
		for _, p := range perms {
			if src, ok := p.(cardColorSource); ok {
				for _, color := range src.GetSource().Colors {
					mt := game.ManaType(color)
					if mt == game.White || mt == game.Blue || mt == game.Black || mt == game.Red || mt == game.Green {
						distinct[mt] = true
					}
				}
			}
		}
	}

	check(controller.GetCreatures())
	check(controller.GetLands())

	var result []game.ManaType
	for mt := range distinct {
		result = append(result, mt)
	}
	if len(result) == 0 {
		result = append(result, game.Colorless)
	}
	return result
}

// resolveAbility resolves a non-mana ability.
func (ee *ExecutionEngine) resolveAbility(ability *Ability, controller AbilityPlayer, targets []any) error {
	for _, effect := range ability.Effects {
		if err := ee.applyEffect(effect, controller, targets); err != nil {
			// Mark card as unimplemented if we can identify the source card
			if ability != nil && ability.Source != nil {
				if cc, ok := ability.Source.(card.Card); ok {
					markUnimplementedCard(cc.Name, fmt.Sprintf("unimplemented effect: %v", effect.Type))
				}
			}
			logger.LogCard("Unimplemented effect during resolution for %s: %v", ability.Name, err)
			return err
		}
	}
	return nil
}

// checkConditions returns true if all conditions on an effect are met.
func (ee *ExecutionEngine) checkConditions(effect Effect, controller AbilityPlayer) bool {
	for _, cond := range effect.Conditions {
		switch cond.Type {
		case NoCondition:
			continue
		case ControlPermanentType:
			if !playerControlsMatching(controller, cond.Value) {
				return false
			}
		case HaveMoreLifeThanOpponent:
			if !ee.hasMoreLifeThanAnOpponent(controller) {
				return false
			}
		case OpponentHasMoreCreatures:
			if !ee.opponentHasMoreCreatures(controller) {
				return false
			}
		case NoCardsInHand:
			if len(controller.GetHand()) != 0 {
				return false
			}
		case KickerPaid:
			return true
		case UnlessPaysMana:
			return true
		case HaveMoreLandsThanOpponent:
			if !ee.hasMoreLandsThanAnOpponent(controller) {
				return false
			}
		case HaveMoreCardsInHandThanOpponent:
			if !ee.hasMoreCardsInHandThanAnOpponent(controller) {
				return false
			}
		case ControlCreatureWithPowerGreater:
			if !playerControlsCreatureWithPowerGreater(controller, cond.Value) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// applyEffect applies a specific effect.
func (ee *ExecutionEngine) applyEffect(effect Effect, controller AbilityPlayer, targets []any) error {
	if !ee.checkConditions(effect, controller) {
		logger.LogCard("Effect skipped: conditions not met")
		return nil
	}

	switch effect.Type {
	case DrawCards:
		ee.gameState.DrawCards(controller, effect.Value)
		logger.LogCard("%s draws %d cards", controller.GetName(), effect.Value)

	case DealDamage:
		if len(targets) > 0 {
			ee.gameState.DealDamage(controller, targets[0], effect.Value)
			logger.LogCard("Deal %d damage to target", effect.Value)
		}

	case SourcePowerDamage:
		// Expect two targets: [0] source creature, [1] target creature
		if len(targets) >= 2 {
			if src, ok := targets[0].(*game.Permanent); ok {
				amount := src.GetPower()
				ee.gameState.DealDamage(src, targets[1], amount)
				logger.LogCard("%s deals %d damage equal to its power", src.GetName(), amount)
			}
		}

	case GainLife:
		ee.gameState.GainLife(controller, effect.Value)
		logger.LogCard("%s gains %d life", controller.GetName(), effect.Value)

	case LoseLife:
		ee.gameState.LoseLife(controller, effect.Value)
		logger.LogCard("%s loses %d life", controller.GetName(), effect.Value)

	case WinGame:
		ee.eliminateOpponents(controller, "effect")
		logger.LogCard("%s wins the game", controller.GetName())

	case LoseGame:
		if len(targets) > 0 {
			for _, target := range targets {
				ee.losePlayerTarget(target, "effect")
			}
		} else {
			// Most parser-generated no-target loss text is either "each opponent loses"
			// or an alternate-win clause. Treat it as the controller winning.
			ee.eliminateOpponents(controller, "effect")
		}
		logger.LogCard("Lose game effect resolves: %s", effect.Description)

	case PumpCreature:
		if len(targets) > 0 {
			power, toughness := effectPTDelta(effect)
			ee.applyPumpEffect(targets[0], power, toughness, effect.Duration)
			logger.LogCard("Target creature gets %+d/%+d", power, toughness)
		}

	case TapUntap:
		if len(targets) > 0 {
			ee.applyTapEffect(targets[0], effect.Value > 0)
		}

	case ChangeControl:
		// Simplified: permanently change controller to the activating player
		if len(targets) > 0 {
			if perm, ok := targets[0].(*game.Permanent); ok {
				// Find the actual player object in the underlying game if available
				for _, p := range ee.gameState.GetAllPlayers() {
					if p.GetName() == controller.GetName() {
						// Best-effort: try to downcast to *game.Player via known adapter shapes
						// This depends on the bridge returning *game.Permanent targets.
						if gp, ok2 := any(p).(interface{ Underlying() *game.Player }); ok2 {
							perm.SetController(gp.Underlying())
							break
						}
					}
				}
				logger.LogCard("Change control of %s", perm.GetName())
			}
		}

	case ReturnToHand:
		if len(targets) > 0 {
			if perm, ok := targets[0].(*game.Permanent); ok {
				owner := perm.GetOwner()
				if owner != nil {
					owner.ReturnPermanentToHand(perm)
					logger.LogCard("Returned %s to its owner's hand", perm.GetName())
				}
			}
		}

	case CounterSpell:
		// Counter spell effect - mark the target spell as countered
		if len(targets) > 0 {
			if stackItem, ok := targets[0].(*StackItem); ok {
				stackItem.Countered = true
				logger.LogCard("Counterspell effect resolves - %s is countered", stackItem.Description)
			} else {
				logger.LogCard("Counterspell effect resolves")
			}
		} else {
			logger.LogCard("Counterspell effect resolves")
		}

	case DestroyPermanent:
		if len(targets) > 0 {
			if perm, ok := targets[0].(*game.Permanent); ok {
				// Destroy the permanent (put into graveyard)
				owner := perm.GetOwner()
				if owner != nil {
					owner.DestroyPermanent(perm)
					logger.LogCard("Destroyed %s", perm.GetName())
				}
			}
		}

	case AddMana:
		// Add mana to player's mana pool
		// For simplicity, assume it's generic mana
		ee.gameState.AddManaToPool(controller, game.Colorless, effect.Value)
		logger.LogCard("%s adds %d mana", controller.GetName(), effect.Value)

	case DiscardCards:
		if len(targets) > 0 {
			if player, ok := targets[0].(AbilityPlayer); ok {
				ee.gameState.DiscardCards(player, effect.Value)
				logger.LogCard("%s discards %d cards", player.GetName(), effect.Value)
			}
		} else {
			ee.gameState.DiscardCards(controller, effect.Value)
			logger.LogCard("%s discards %d cards", controller.GetName(), effect.Value)
		}

	case SearchLibrary:
		if adv, ok := ee.gameState.(interface{ SearchLibraryAdvanced(AbilityPlayer, int, string) }); ok {
			adv.SearchLibraryAdvanced(controller, effect.Value, effect.Description)
		} else {
			ee.gameState.SearchLibrary(controller, effect.Value)
		}
		logger.LogCard("%s searches library: %s", controller.GetName(), effect.Description)

	case CreateToken:
		tokenSpec := effectTokenSpec(effect)
		for i := 0; i < tokenSpec.Count; i++ {
			token := game.SimpleCard{
				Name:      tokenSpec.Name,
				TypeLine:  tokenSpec.TypeLine,
				Power:     strconv.Itoa(tokenSpec.Power),
				Toughness: strconv.Itoa(tokenSpec.Toughness),
			}
			ee.gameState.CreateToken(controller, token)
		}
		logger.LogCard("%s creates %d %d/%d tokens", controller.GetName(), tokenSpec.Count, tokenSpec.Power, tokenSpec.Toughness)

	case PreventDamage:
		if effect.Value == 0 {
			// Fog-style: prevent all damage to all players and permanents this turn
			for _, player := range ee.gameState.GetAllPlayers() {
				ee.gameState.PreventDamage(player, 9999)
			}
			logger.LogCard("Prevent all combat damage this turn")
		} else if len(targets) > 0 {
			for _, target := range targets {
				ee.gameState.PreventDamage(target, effect.Value)
				logger.LogCard("Prevent %d damage to target", effect.Value)
			}
		}

	case KeywordAbility:
		// Keyword abilities are static and don't resolve as one-shot effects.
		// They are parsed for coverage but require no runtime action.
		logger.LogCard("Keyword ability: %s", effect.Description)

	case ChooseMode:
		if len(targets) > 0 {
			effectIdx := 0
			if mode, ok := targets[0].(int); ok && mode >= 0 {
				effectIdx = mode
			}
			if effs, ok := targets[0].([]Effect); ok {
				if effectIdx < len(effs) {
					ee.applyEffect(effs[effectIdx], controller, targets[1:])
				}
			}
		}
		logger.LogCard("Choose mode: %s", effect.Description)

	case TakeExtraTurn:
		// Queue an extra turn via the game state.
		ee.gameState.TakeExtraTurn()
		logger.LogCard("Take extra turn: %s", effect.Description)

	case LookAtLibraryTop:
		// Informational: no game-state change.
		logger.LogCard("Look at top of library: %s", effect.Description)

	case RevealInformation:
		// Informational: no game-state change.
		logger.LogCard("Reveal information: %s", effect.Description)

	case Exile:
		if len(targets) > 0 {
			if perm, ok := targets[0].(*game.Permanent); ok {
				owner := perm.GetOwner()
				if owner != nil {
					owner.DestroyPermanentToExile(perm)
					logger.LogCard("Exiled %s", perm.GetName())
				}
			}
		}

	case MillCards:
		ee.gameState.MillCards(controller, effect.Value)
		logger.LogCard("%s mills %d cards", controller.GetName(), effect.Value)

	case ScryCards:
		ee.gameState.ScryLibrary(controller, effect.Value)
		logger.LogCard("%s scries %d", controller.GetName(), effect.Value)

	case AddCounters:
		if len(targets) > 0 {
			if perm, ok := targets[0].(*game.Permanent); ok {
				counterType := "+1/+1"
				if effect.Description != "" && strings.Contains(strings.ToLower(effect.Description), "loyalty") {
					counterType = "loyalty"
				}
				perm.AddCounters(counterType, effect.Value)
				logger.LogCard("Added %d %s counters to %s", effect.Value, counterType, perm.GetName())
			}
		}

	case UntapPermanent:
		if len(targets) > 0 {
			if perm, ok := targets[0].(*game.Permanent); ok {
				perm.Untap()
				logger.LogCard("Untapped %s", perm.GetName())
			}
		} else {
			// Untap all permanents controlled by player (e.g., Seedborn Muse)
			for _, c := range controller.GetCreatures() {
				if p, ok := c.(*game.Permanent); ok {
					p.Untap()
				}
			}
			for _, l := range controller.GetLands() {
				if p, ok := l.(*game.Permanent); ok {
					p.Untap()
				}
			}
			logger.LogCard("Untapped all permanents for %s", controller.GetName())
		}

	case CopySpell:
		if s, ok := ee.gameState.(interface{ GetStack() *Stack }); ok {
			stack := s.GetStack()
			if stack.Size() > 0 {
				top := stack.Peek()
				if top != nil {
					stack.AddSpell(top.Spell, controller, top.Targets)
					logger.LogCard("Copied spell: %s", top.Description)
				}
			}
		}

	case CantAttackBlock:
		if len(targets) > 0 {
			if perm, ok := targets[0].(*game.Permanent); ok {
				perm.GrantKeyword(game.KWDefender)
				perm.SetCantBlock(true)
				logger.LogCard("Applied attack/block restriction to %s", perm.GetName())
			}
		}

	case AdditionalLand:
		for _, p := range ee.gameState.GetAllPlayers() {
			if p.GetName() == controller.GetName() {
				if gplayer, ok := any(p).(interface{ Underlying() *game.Player }); ok {
					gplayer.Underlying().AddLandPlay(1)
				} else if gplayer, ok := any(p).(interface{ AddLandPlay(int) }); ok {
					gplayer.AddLandPlay(1)
				}
				logger.LogCard("%s may play additional land", p.GetName())
				break
			}
		}

	case SacrificePermanent:
		if len(targets) > 0 {
			if perm, ok := targets[0].(*game.Permanent); ok {
				owner := perm.GetOwner()
				if owner != nil {
					owner.DestroyPermanent(perm)
					logger.LogCard("Sacrificed %s", perm.GetName())
				}
			}
		}

	case ImprintCards:
		logger.LogCard("Imprint effect resolves (card exiled from hand)")

	case ReanimateCreature:
		var reanimatedCard game.SimpleCard
		found := false
		if len(targets) > 0 {
			if sc, ok := targets[0].(game.SimpleCard); ok {
				reanimatedCard = sc
				found = true
			}
		}
		if !found {
			for _, p := range ee.gameState.GetAllPlayers() {
				if gp, ok := p.(interface{ GetGraveyard() []game.SimpleCard }); ok {
					for _, c := range gp.GetGraveyard() {
						if c.IsCreature() {
							reanimatedCard = c
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
		}
		if !found {
			reanimatedCard = game.SimpleCard{
				Name:      "Reanimated Creature",
				TypeLine:  "Creature",
				Power:     "2",
				Toughness: "2",
			}
		}
		ee.gameState.ReanimateCreature(controller, reanimatedCard)
		logger.LogCard("Reanimated %s from graveyard", reanimatedCard.Name)

	default:
		return fmt.Errorf("unimplemented effect type: %v", effect.Type)
	}

	return nil
}

func (ee *ExecutionEngine) eliminateOpponents(controller AbilityPlayer, reason string) {
	if controller == nil || ee.gameState == nil {
		return
	}
	for _, player := range ee.gameState.GetAllPlayers() {
		if player == nil || player.GetName() == controller.GetName() {
			continue
		}
		ee.losePlayerTarget(player, reason)
	}
}

func (ee *ExecutionEngine) losePlayerTarget(target any, reason string) {
	switch p := target.(type) {
	case *game.Player:
		p.Lose(reason)
	case interface{ Lose(string) }:
		p.Lose(reason)
	case AbilityPlayer:
		p.SetLifeTotal(0)
	}
}

// CanExecuteEffect returns true if the execution engine has a concrete
// implementation for the given effect type.
func CanExecuteEffect(effectType EffectType) bool {
	switch effectType {
	case DrawCards, DealDamage, GainLife, LoseLife, AddMana,
		PumpCreature, DestroyPermanent, CounterSpell,
		TapUntap, ChangeControl, ReturnToHand, SourcePowerDamage,
		DiscardCards, SearchLibrary, CreateToken, PreventDamage,
		KeywordAbility, ChooseMode, TakeExtraTurn, Exile,
		MillCards, ScryCards, AddCounters, UntapPermanent, CopySpell,
		CantAttackBlock, AdditionalLand, SacrificePermanent, ReanimateCreature,
		WinGame, LoseGame, LookAtLibraryTop, RevealInformation, ImprintCards:
		return true
	default:
		return false
	}
}

// CanExecuteCondition returns true if the execution engine can evaluate the
// given condition type during ability resolution.
func CanExecuteCondition(conditionType ConditionType) bool {
	switch conditionType {
	case NoCondition, ControlPermanentType, HaveMoreLifeThanOpponent,
		OpponentHasMoreCreatures, NoCardsInHand, KickerPaid, UnlessPaysMana,
		HaveMoreLandsThanOpponent, HaveMoreCardsInHandThanOpponent,
		ControlCreatureWithPowerGreater:
		return true
	default:
		return false
	}
}

// determineManaType determines what type of mana an ability produces.
func (ee *ExecutionEngine) determineManaType(ability *Ability, _ Effect) game.ManaType {
	// Parse from ability description or oracle text
	text := ability.OracleText

	if contains(text, "{W}") {
		return game.White
	}
	if contains(text, "{U}") {
		return game.Blue
	}
	if contains(text, "{B}") {
		return game.Black
	}
	if contains(text, "{R}") {
		return game.Red
	}
	if contains(text, "{G}") {
		return game.Green
	}
	if contains(text, "{C}") {
		return game.Colorless
	}
	if contains(text, "any color") {
		return game.Any
	}

	return game.Colorless // Default
}

// requiresTargets checks if an ability requires targets.
func (ee *ExecutionEngine) requiresTargets(ability *Ability) bool {
	for _, effect := range ability.Effects {
		if len(effect.Targets) > 0 {
			for _, target := range effect.Targets {
				if target.Required {
					return true
				}
			}
		}
	}
	return false
}

// hasValidTargets checks if valid targets exist for an ability.
func (ee *ExecutionEngine) hasValidTargets(ability *Ability, controller AbilityPlayer) bool {
	for _, effect := range ability.Effects {
		for _, target := range effect.Targets {
			if target.Required {
				// Use enhanced targeting if available
				if target.Enhanced != nil {
					if !ee.hasValidTargetsForEnhanced(*target.Enhanced, controller) {
						return false
					}
				} else {
					// Fallback to basic target validation
					if !ee.hasValidTargetsBasic(target, controller) {
						return false
					}
				}
			}
		}
	}
	return true // If no required targets or all have valid targets, it's valid
}

// hasValidTargetsForEnhanced checks if valid targets exist for an enhanced target.
func (ee *ExecutionEngine) hasValidTargetsForEnhanced(enhancedTarget EnhancedTarget, controller AbilityPlayer) bool {
	// Get all potential targets based on type
	potentialTargets := ee.GetPotentialTargets(enhancedTarget.Type, controller)

	// Check if any potential target passes validation
	for _, target := range potentialTargets {
		legality := ee.targetValidator.ValidateTarget(target, enhancedTarget, controller)
		if legality.IsLegal {
			return true
		}
	}

	return false
}

// hasValidTargetsBasic provides fallback validation for basic targets.
func (ee *ExecutionEngine) hasValidTargetsBasic(target Target, _ AbilityPlayer) bool {
	switch target.Type {
	case CreatureTarget:
		// Check if any creatures exist
		for _, player := range ee.gameState.GetAllPlayers() {
			if len(player.GetCreatures()) > 0 {
				return true
			}
		}
	case PlayerTarget:
		// Players always exist
		return true
	case CardInGraveyardTarget:
		for _, player := range ee.gameState.GetAllPlayers() {
			if gp, ok := player.(interface{ GetGraveyard() []any }); ok {
				if len(gp.GetGraveyard()) > 0 {
					return true
				}
			}
		}
		return false
	case AnyTarget:
		// Check if any valid targets exist
		for _, player := range ee.gameState.GetAllPlayers() {
			if len(player.GetCreatures()) > 0 {
				return true
			}
		}
		return true // Players are always valid targets
	case SpellTarget:
		// Spells on the stack are dynamic targets; assume possible
		return true
	}
	return false
}

// GetPotentialTargets returns all potential targets of a given type.
func (ee *ExecutionEngine) GetPotentialTargets(targetType TargetType, controller any) []any {
	_ = controller
	var targets []any

	switch targetType {
	case CreatureTarget:
		for _, player := range ee.gameState.GetAllPlayers() {
			targets = append(targets, player.GetCreatures()...)
		}
	case PlayerTarget:
		for _, player := range ee.gameState.GetAllPlayers() {
			targets = append(targets, player)
		}
	case PermanentTarget:
		// Get all permanents (creatures, lands, artifacts, etc.)
		for _, player := range ee.gameState.GetAllPlayers() {
			targets = append(targets, player.GetCreatures()...)
			targets = append(targets, player.GetLands()...)
		}
	case AnyTarget:
		// Get all players and permanents
		for _, player := range ee.gameState.GetAllPlayers() {
			targets = append(targets, player)
			targets = append(targets, player.GetCreatures()...)
			targets = append(targets, player.GetLands()...)
		}
	case CardInGraveyardTarget:
		for _, player := range ee.gameState.GetAllPlayers() {
			if gp, ok := player.(interface{ GetGraveyard() []any }); ok {
				targets = append(targets, gp.GetGraveyard()...)
			}
		}
	case SpellTarget:
		// Stack items are dynamic; potential targets are queried from the stack
		if s, ok := ee.gameState.(interface{ GetStack() *Stack }); ok {
			for _, item := range s.GetStack().GetItems() {
				targets = append(targets, item)
			}
		}
	}

	return targets
}

// validateTargets validates that the provided targets are legal for the ability.
func (ee *ExecutionEngine) validateTargets(ability *Ability, controller AbilityPlayer, targets []any) error {
	targetIndex := 0

	for _, effect := range ability.Effects {
		for _, targetReq := range effect.Targets {
			if targetReq.Required {
				// Check if we have enough targets
				if targetIndex >= len(targets) {
					return ErrNoValidTargets
				}

				target := targets[targetIndex]

				// Use enhanced targeting if available
				if targetReq.Enhanced != nil {
					legality := ee.targetValidator.ValidateTarget(target, *targetReq.Enhanced, controller)
					if !legality.IsLegal {
						return ErrInvalidTarget
					}
				} else {
					// Fallback to basic validation
					if !ee.isValidBasicTarget(target, targetReq) {
						return ErrInvalidTarget
					}
				}

				targetIndex++
			}
		}
	}

	return nil
}

// isValidBasicTarget provides basic target validation for legacy targets.
func (ee *ExecutionEngine) isValidBasicTarget(target any, targetReq Target) bool {
	switch targetReq.Type {
	case CreatureTarget:
		return ee.targetValidator.isCreature(target)
	case PlayerTarget:
		return ee.targetValidator.isPlayer(target)
	case PermanentTarget:
		return ee.targetValidator.isPermanent(target)
	case AnyTarget:
		return ee.targetValidator.isPlayer(target) || ee.targetValidator.isPermanent(target)
	case CardInGraveyardTarget:
		return target != nil
	case SpellTarget:
		_, ok := target.(*StackItem)
		return ok
	default:
		return false
	}
}

// applyPumpEffect applies a pump effect to a creature.
func (ee *ExecutionEngine) applyPumpEffect(target any, power, toughness int, duration EffectDuration) {
	// Apply temporary P/T changes until end of turn where applicable
	if gp, ok := target.(*game.Permanent); ok {
		if duration == UntilEndOfTurn || duration == Instant || duration == UntilEndOfCombat {
			gp.AddTempBuff(power, toughness)
			logger.LogCard("Applying temporary +%d/+%d to %s (duration: %v)", power, toughness, gp.GetName(), duration)
			// Also register as a layered effect when the game state supports it
			if lgs, ok := ee.gameState.(interface{ AddLayeredEffect(*game.LayeredEffect) uint64 }); ok {
				lgs.AddLayeredEffect(&game.LayeredEffect{
					Layer:    game.Layer7PT,
					Sublayer: game.Sublayer7C,
					Source:   gp,
					Affects:  func(p *game.Permanent) bool { return p == gp },
					Apply: func(_ *game.Permanent, v *game.PermanentView) {
						v.Power += power
						v.Toughness += toughness
					},
					ExpiresEOT: true,
				})
			}
			return
		}
		// For permanent duration, adjust base stats (simplified)
		if duration == Permanent {
			gp.SetPower(gp.GetPower() + power)
			gp.SetToughness(gp.GetToughness() + toughness)
			logger.LogCard("Applying permanent +%d/+%d to %s", power, toughness, gp.GetName())
			return
		}
	}
	logger.LogCard("Applying +%d/+%d effect (duration: %v)", power, toughness, duration)
}

// applyTapEffect applies a tap/untap effect.
func (ee *ExecutionEngine) applyTapEffect(target any, shouldTap bool) {
	if tapper, ok := target.(interface {
		Tap()
		Untap()
		GetName() string
	}); ok {
		if shouldTap {
			tapper.Tap()
			logger.LogCard("Tapping %s", tapper.GetName())
		} else {
			tapper.Untap()
			logger.LogCard("Untapping %s", tapper.GetName())
		}
	}
}

// ParseAndRegisterAbilities parses abilities from oracle text and registers them.
func (ee *ExecutionEngine) ParseAndRegisterAbilities(oracleText string, source any) ([]*Ability, error) {
	abilities, err := ee.parser.ParseAbilities(oracleText, source)
	if err != nil {
		if cc, ok := source.(card.Card); ok {
			markUnimplementedCard(cc.Name, fmt.Sprintf("parse error: %v", err))
		}
		return nil, err
	}

	// A card with oracle text inherently has abilities. If the parser returns
	// zero abilities, that is a parser failure, not a missing-ability card.
	if len(abilities) == 0 {
		if cc, ok := source.(card.Card); ok {
			if strings.TrimSpace(cc.OracleText) != "" && !isBasicLand(cc.TypeLine) {
				markUnimplementedCard(cc.Name, "parser failed to extract abilities from oracle text")
			}
		}
	}

	logger.LogCard("Parsed %d abilities from oracle text", len(abilities))
	for _, ability := range abilities {
		logger.LogCard("  - %s: %s", ability.Name, ability.Effects[0].Description)
	}

	return abilities, nil
}

// applyEffect applies a specific effect (exposed for stack resolution)
func (ee *ExecutionEngine) ApplyEffect(effect Effect, controller AbilityPlayer, targets []any) error {
	return ee.applyEffect(effect, controller, targets)
}

// abilitiesFrom extracts abilities from an object, trying AbilityPermanent first
// then falling back to the duck-typed []any getter used by game.Permanent.
func abilitiesFrom(v any) []*Ability {
	if perm, ok := v.(AbilityPermanent); ok {
		return perm.GetAbilities()
	}
	if carrier, ok := v.(interface{ GetAbilities() []any }); ok {
		var out []*Ability
		for _, a := range carrier.GetAbilities() {
			if ab, ok := a.(*Ability); ok {
				out = append(out, ab)
			}
		}
		return out
	}
	return nil
}

func isTapped(v any) bool {
	if tapper, ok := v.(interface{ IsTapped() bool }); ok {
		return tapper.IsTapped()
	}
	return false
}

// ActivateManaAbilities activates all available mana abilities for a player.
func (ee *ExecutionEngine) ActivateManaAbilities(player AbilityPlayer) int {
	totalManaAdded := 0

	// Get all lands controlled by the player
	for _, land := range player.GetLands() {
		if isTapped(land) {
			continue
		}
		for _, ability := range abilitiesFrom(land) {
			if ability.Type == Mana && ee.canActivateAbility(ability, player) {
				if err := ee.ExecuteAbility(ability, player, nil); err == nil {
					totalManaAdded++
					break // Only activate one mana ability per land
				}
			}
		}
	}

	return totalManaAdded
}

// GetActivatableAbilities returns all abilities that can be activated by a player.
func (ee *ExecutionEngine) GetActivatableAbilities(player AbilityPlayer) []*Ability {
	var activatable []*Ability

	// Check creatures for activated abilities
	for _, creature := range player.GetCreatures() {
		for _, ability := range abilitiesFrom(creature) {
			if ability.Type == Activated && ee.canActivateAbility(ability, player) {
				activatable = append(activatable, ability)
			}
		}
	}

	// Check lands for non-mana activated abilities
	for _, land := range player.GetLands() {
		for _, ability := range abilitiesFrom(land) {
			if ability.Type == Activated && ability.Type != Mana && ee.canActivateAbility(ability, player) {
				activatable = append(activatable, ability)
			}
		}
	}

	return activatable
}

// Helper function to check if a string contains a substring (case-insensitive).
func contains(text, substr string) bool {
	return len(text) >= len(substr) &&
		(text == substr ||
			(len(text) > len(substr) &&
				(text[:len(substr)] == substr ||
					text[len(text)-len(substr):] == substr ||
					containsSubstring(text, substr))))
}

func containsSubstring(text, substr string) bool {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
