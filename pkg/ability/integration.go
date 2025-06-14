// Package ability provides integration with the existing MTGSim game engine.
package ability

import (
	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// GameAdapter adapts the existing game structures to work with the ability system.
type GameAdapter struct {
	game           GameInterface
	abilityEngine  *AbilityEngine
	executionEngine *ExecutionEngine
	aiDecisionMaker *AIDecisionMaker
	parser         *AbilityParser
	currentPhase   string
}

// GameInterface represents the interface to the existing game engine.
type GameInterface interface {
	GetPlayers() []PlayerInterface
	GetCurrentPlayer() PlayerInterface
	GetActivePlayer() PlayerInterface
	GetCurrentPhase() string
	IsMainPhase() bool
	IsCombatPhase() bool
}

// PlayerInterface represents the interface to existing player structures.
type PlayerInterface interface {
	GetName() string
	GetLifeTotal() int
	SetLifeTotal(life int)
	GetHand() []card.Card
	AddCardToHand(card card.Card)
	GetCreatures() []PermanentInterface
	GetLands() []PermanentInterface
	GetArtifacts() []PermanentInterface
	GetEnchantments() []PermanentInterface
	GetPlaneswalkers() []PermanentInterface
	GetManaPool() map[game.ManaType]int
	AddManaToPool(manaType game.ManaType, amount int)
	CanPayManaCost(cost map[game.ManaType]int) bool
	PayManaCost(cost map[game.ManaType]int) error
}

// PermanentInterface represents the interface to existing permanent structures.
type PermanentInterface interface {
	GetID() uuid.UUID
	GetName() string
	GetSource() card.Card
	GetOwner() PlayerInterface
	GetController() PlayerInterface
	IsTapped() bool
	Tap()
	Untap()
	GetPower() int
	GetToughness() int
	SetPower(power int)
	SetToughness(toughness int)
	GetDamageCounters() int
	AddDamage(damage int)
	ClearDamage()
}

// NewGameAdapter creates a new game adapter.
func NewGameAdapter(game GameInterface) *GameAdapter {
	abilityEngine := NewAbilityEngine()
	
	adapter := &GameAdapter{
		game:          game,
		abilityEngine: abilityEngine,
		parser:        NewAbilityParser(),
		currentPhase:  "Main",
	}
	
	// Create execution engine with adapter as game state
	adapter.executionEngine = NewExecutionEngine(adapter)
	
	// Create AI decision maker
	adapter.aiDecisionMaker = NewAIDecisionMaker(adapter.executionEngine)
	
	return adapter
}

// Implement GameState interface for ExecutionEngine

func (ga *GameAdapter) GetPlayer(name string) AbilityPlayer {
	for _, player := range ga.game.GetPlayers() {
		if player.GetName() == name {
			return &PlayerAdapter{player: player, adapter: ga}
		}
	}
	return nil
}

func (ga *GameAdapter) GetAllPlayers() []AbilityPlayer {
	var players []AbilityPlayer
	for _, player := range ga.game.GetPlayers() {
		players = append(players, &PlayerAdapter{player: player, adapter: ga})
	}
	return players
}

func (ga *GameAdapter) GetCurrentPlayer() AbilityPlayer {
	player := ga.game.GetCurrentPlayer()
	if player != nil {
		return &PlayerAdapter{player: player, adapter: ga}
	}
	return nil
}

func (ga *GameAdapter) GetActivePlayer() AbilityPlayer {
	player := ga.game.GetActivePlayer()
	if player != nil {
		return &PlayerAdapter{player: player, adapter: ga}
	}
	return nil
}

func (ga *GameAdapter) IsMainPhase() bool {
	return ga.game.IsMainPhase()
}

func (ga *GameAdapter) IsCombatPhase() bool {
	return ga.game.IsCombatPhase()
}

func (ga *GameAdapter) CanActivateAbilities() bool {
	return true // Simplified - in a real game this would check priority
}

func (ga *GameAdapter) AddManaToPool(player AbilityPlayer, manaType game.ManaType, amount int) {
	if playerAdapter, ok := player.(*PlayerAdapter); ok {
		playerAdapter.player.AddManaToPool(manaType, amount)
	}
}

func (ga *GameAdapter) DealDamage(source interface{}, target interface{}, amount int) {
	// Handle different target types
	switch t := target.(type) {
	case AbilityPlayer:
		if playerAdapter, ok := t.(*PlayerAdapter); ok {
			currentLife := playerAdapter.player.GetLifeTotal()
			playerAdapter.player.SetLifeTotal(currentLife - amount)
		}
	case AbilityPermanent:
		if permAdapter, ok := t.(*PermanentAdapter); ok {
			permAdapter.permanent.AddDamage(amount)
		}
	case string:
		// Handle string targets like "opponent"
		if t == "opponent" {
			// Deal damage to first opponent
			players := ga.GetAllPlayers()
			if len(players) > 1 {
				currentLife := players[1].GetLifeTotal()
				players[1].SetLifeTotal(currentLife - amount)
			}
		}
	}
}

func (ga *GameAdapter) DrawCards(player AbilityPlayer, count int) {
	// This would need to be implemented by calling the game's draw card function
	// For now, we'll just log it
	// logger.LogCard("%s draws %d cards", player.GetName(), count)
}

func (ga *GameAdapter) GainLife(player AbilityPlayer, amount int) {
	currentLife := player.GetLifeTotal()
	player.SetLifeTotal(currentLife + amount)
}

func (ga *GameAdapter) LoseLife(player AbilityPlayer, amount int) {
	currentLife := player.GetLifeTotal()
	player.SetLifeTotal(currentLife - amount)
}

// PlayerAdapter adapts existing player structures to the ability system.
type PlayerAdapter struct {
	player  PlayerInterface
	adapter *GameAdapter
}

func (pa *PlayerAdapter) GetName() string {
	return pa.player.GetName()
}

func (pa *PlayerAdapter) GetLifeTotal() int {
	return pa.player.GetLifeTotal()
}

func (pa *PlayerAdapter) SetLifeTotal(life int) {
	pa.player.SetLifeTotal(life)
}

func (pa *PlayerAdapter) GetHand() []interface{} {
	hand := pa.player.GetHand()
	result := make([]interface{}, len(hand))
	for i, card := range hand {
		result[i] = card
	}
	return result
}

func (pa *PlayerAdapter) AddCardToHand(cardInterface interface{}) {
	if mtgCard, ok := cardInterface.(card.Card); ok {
		pa.player.AddCardToHand(mtgCard)
	}
}

func (pa *PlayerAdapter) GetCreatures() []interface{} {
	creatures := pa.player.GetCreatures()
	result := make([]interface{}, len(creatures))
	for i, creature := range creatures {
		result[i] = &PermanentAdapter{permanent: creature, adapter: pa.adapter}
	}
	return result
}

func (pa *PlayerAdapter) GetLands() []interface{} {
	lands := pa.player.GetLands()
	result := make([]interface{}, len(lands))
	for i, land := range lands {
		result[i] = &PermanentAdapter{permanent: land, adapter: pa.adapter}
	}
	return result
}

func (pa *PlayerAdapter) CanPayCost(cost Cost) bool {
	return pa.player.CanPayManaCost(cost.ManaCost)
}

func (pa *PlayerAdapter) PayCost(cost Cost) error {
	// Handle tap costs, sacrifice costs, etc.
	// For now, just handle mana costs
	return pa.player.PayManaCost(cost.ManaCost)
}

func (pa *PlayerAdapter) GetManaPool() map[game.ManaType]int {
	return pa.player.GetManaPool()
}

// PermanentAdapter adapts existing permanent structures to the ability system.
type PermanentAdapter struct {
	permanent PermanentInterface
	adapter   *GameAdapter
	abilities []*Ability
}

func (pa *PermanentAdapter) GetID() uuid.UUID {
	return pa.permanent.GetID()
}

func (pa *PermanentAdapter) GetName() string {
	return pa.permanent.GetName()
}

func (pa *PermanentAdapter) GetOwner() AbilityPlayer {
	owner := pa.permanent.GetOwner()
	return &PlayerAdapter{player: owner, adapter: pa.adapter}
}

func (pa *PermanentAdapter) GetController() AbilityPlayer {
	controller := pa.permanent.GetController()
	return &PlayerAdapter{player: controller, adapter: pa.adapter}
}

func (pa *PermanentAdapter) IsTapped() bool {
	return pa.permanent.IsTapped()
}

func (pa *PermanentAdapter) Tap() {
	pa.permanent.Tap()
}

func (pa *PermanentAdapter) Untap() {
	pa.permanent.Untap()
}

func (pa *PermanentAdapter) GetAbilities() []*Ability {
	return pa.abilities
}

func (pa *PermanentAdapter) AddAbility(ability *Ability) {
	pa.abilities = append(pa.abilities, ability)
	pa.adapter.abilityEngine.RegisterAbility(ability)
}

func (pa *PermanentAdapter) RemoveAbility(abilityID uuid.UUID) {
	for i, ability := range pa.abilities {
		if ability.ID == abilityID {
			pa.abilities = append(pa.abilities[:i], pa.abilities[i+1:]...)
			pa.adapter.abilityEngine.UnregisterAbility(abilityID)
			break
		}
	}
}

// Public methods for game integration

// ParseAndAddAbilities parses abilities from a card's oracle text and adds them to a permanent.
func (ga *GameAdapter) ParseAndAddAbilities(permanent PermanentInterface, oracleText string) error {
	permAdapter := &PermanentAdapter{permanent: permanent, adapter: ga}
	
	abilities, err := ga.parser.ParseAbilities(oracleText, permAdapter)
	if err != nil {
		return err
	}
	
	for _, ability := range abilities {
		permAdapter.AddAbility(ability)
	}
	
	return nil
}

// TriggerAbilities triggers abilities based on game events.
func (ga *GameAdapter) TriggerAbilities(condition TriggerCondition, source interface{}) {
	ga.abilityEngine.TriggerAbilities(condition, source)
	ga.abilityEngine.ProcessTriggeredAbilities()
}

// ActivateAbilitiesForPlayer uses AI to activate abilities for a player.
func (ga *GameAdapter) ActivateAbilitiesForPlayer(player PlayerInterface, phase string) {
	playerAdapter := &PlayerAdapter{player: player, adapter: ga}
	ga.aiDecisionMaker.ActivateAbilitiesForPlayer(playerAdapter, phase)
}

// ResetTurnCounters resets all ability usage counters at the start of a turn.
func (ga *GameAdapter) ResetTurnCounters() {
	ga.abilityEngine.ResetTurnCounters()
}

// SetCurrentPhase sets the current game phase for timing restrictions.
func (ga *GameAdapter) SetCurrentPhase(phase string) {
	ga.currentPhase = phase
}
