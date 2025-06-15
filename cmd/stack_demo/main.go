// Package main provides a demonstration of the MTGSim stack system.
package main

import (
	"fmt"
	"log"

	"github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/card"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// DemoPlayer implements AbilityPlayer for demonstration
type DemoPlayer struct {
	name      string
	lifeTotal int
	manaPool  map[game.ManaType]int
}

func (p *DemoPlayer) GetName() string                                    { return p.name }
func (p *DemoPlayer) PayCost(cost ability.Cost) error                    { return nil }
func (p *DemoPlayer) AddCardToHand(card any)                             {}
func (p *DemoPlayer) RemoveCardFromHand(card any) bool                   { return true }
func (p *DemoPlayer) GetHandSize() int                                   { return 7 }
func (p *DemoPlayer) GetLifeTotal() int                                  { return p.lifeTotal }
func (p *DemoPlayer) SetLifeTotal(life int)                              { p.lifeTotal = life }
func (p *DemoPlayer) GetManaPool() map[game.ManaType]int                 { return p.manaPool }
func (p *DemoPlayer) AddMana(manaType game.ManaType, amount int)         {}
func (p *DemoPlayer) SpendMana(manaType game.ManaType, amount int) bool  { return true }
func (p *DemoPlayer) CanPayCost(cost ability.Cost) bool                  { return true }
func (p *DemoPlayer) GetCreatures() []any                                { return []any{} }
func (p *DemoPlayer) GetPermanents() []any                               { return []any{} }
func (p *DemoPlayer) AddPermanent(permanent any)                         {}
func (p *DemoPlayer) RemovePermanent(permanent any) bool                 { return true }
func (p *DemoPlayer) GetHand() []any                                     { return []any{} }
func (p *DemoPlayer) GetLands() []any                                    { return []any{} }

// DemoGameState implements GameState for demonstration
type DemoGameState struct {
	players       []ability.AbilityPlayer
	activePlayer  ability.AbilityPlayer
	currentPlayer ability.AbilityPlayer
	isMainPhase   bool
	isCombatPhase bool
}

func (gs *DemoGameState) GetPlayer(name string) ability.AbilityPlayer {
	for _, player := range gs.players {
		if player.GetName() == name {
			return player
		}
	}
	return nil
}

func (gs *DemoGameState) GetAllPlayers() []ability.AbilityPlayer { return gs.players }
func (gs *DemoGameState) GetCurrentPlayer() ability.AbilityPlayer { return gs.currentPlayer }
func (gs *DemoGameState) GetActivePlayer() ability.AbilityPlayer  { return gs.activePlayer }
func (gs *DemoGameState) IsMainPhase() bool                       { return gs.isMainPhase }
func (gs *DemoGameState) IsCombatPhase() bool                     { return gs.isCombatPhase }
func (gs *DemoGameState) CanActivateAbilities() bool             { return true }

func (gs *DemoGameState) AddManaToPool(player ability.AbilityPlayer, manaType game.ManaType, amount int) {
	fmt.Printf("  → %s adds %d %v mana to their pool\n", player.GetName(), amount, manaType)
}

func (gs *DemoGameState) DealDamage(source any, target any, amount int) {
	if player, ok := target.(ability.AbilityPlayer); ok {
		fmt.Printf("  → %d damage dealt to %s\n", amount, player.GetName())
		player.SetLifeTotal(player.GetLifeTotal() - amount)
	}
}

func (gs *DemoGameState) DrawCards(player ability.AbilityPlayer, count int) {
	fmt.Printf("  → %s draws %d cards\n", player.GetName(), count)
}

func (gs *DemoGameState) GainLife(player ability.AbilityPlayer, amount int) {
	fmt.Printf("  → %s gains %d life\n", player.GetName(), amount)
	player.SetLifeTotal(player.GetLifeTotal() + amount)
}

func (gs *DemoGameState) LoseLife(player ability.AbilityPlayer, amount int) {
	fmt.Printf("  → %s loses %d life\n", player.GetName(), amount)
	player.SetLifeTotal(player.GetLifeTotal() - amount)
}

func main() {
	fmt.Println("=== MTGSim Stack System Demonstration ===\n")

	// Create players
	alice := &DemoPlayer{name: "Alice", lifeTotal: 20, manaPool: make(map[game.ManaType]int)}
	bob := &DemoPlayer{name: "Bob", lifeTotal: 20, manaPool: make(map[game.ManaType]int)}

	// Create game state
	gameState := &DemoGameState{
		players:       []ability.AbilityPlayer{alice, bob},
		activePlayer:  alice,
		currentPlayer: alice,
		isMainPhase:   true,
		isCombatPhase: false,
	}

	// Create spell casting engine
	executionEngine := ability.NewExecutionEngine(gameState)
	spellCastingEngine := ability.NewSpellCastingEngine(gameState, executionEngine)

	// Set up the engine
	spellCastingEngine.SetPlayers([]ability.AbilityPlayer{alice, bob})
	spellCastingEngine.SetActivePlayer(alice)
	spellCastingEngine.SetPhase("Main Phase")

	// Demonstrate Lightning Bolt vs Counterspell scenario
	fmt.Println("🎯 Scenario: Lightning Bolt vs Counterspell")
	fmt.Printf("Initial state: Alice (%d life), Bob (%d life)\n\n", alice.GetLifeTotal(), bob.GetLifeTotal())

	// Create cards
	lightningBolt := card.Card{
		Name:       "Lightning Bolt",
		ManaCost:   "{R}",
		CMC:        1,
		TypeLine:   "Instant",
		OracleText: "Lightning Bolt deals 3 damage to any target.",
	}

	counterspell := card.Card{
		Name:       "Counterspell",
		ManaCost:   "{U}{U}",
		CMC:        2,
		TypeLine:   "Instant",
		OracleText: "Counter target spell.",
	}

	// Step 1: Alice casts Lightning Bolt
	fmt.Println("📋 Step 1: Alice casts Lightning Bolt targeting Bob")
	err := spellCastingEngine.CastSpell(lightningBolt, alice, []interface{}{bob})
	if err != nil {
		log.Fatalf("Failed to cast Lightning Bolt: %v", err)
	}

	fmt.Printf("Stack size: %d\n", spellCastingEngine.GetStack().Size())
	fmt.Printf("Priority: %s\n\n", spellCastingEngine.GetPriorityPlayer().GetName())

	// Step 2: Alice passes priority
	fmt.Println("📋 Step 2: Alice passes priority")
	err = spellCastingEngine.GetPriorityManager().PassPriority(alice)
	if err != nil {
		log.Fatalf("Failed to pass priority: %v", err)
	}

	fmt.Printf("Priority: %s\n\n", spellCastingEngine.GetPriorityPlayer().GetName())

	// Step 3: Bob casts Counterspell
	fmt.Println("📋 Step 3: Bob casts Counterspell targeting Lightning Bolt")
	
	// Get Lightning Bolt from stack
	stackItems := spellCastingEngine.GetStack().GetItems()
	var lightningBoltItem *ability.StackItem
	for _, item := range stackItems {
		if item.Spell != nil && item.Spell.Name == "Lightning Bolt" {
			lightningBoltItem = item
			break
		}
	}

	if lightningBoltItem == nil {
		log.Fatal("Lightning Bolt not found on stack")
	}

	err = spellCastingEngine.CounterSpell(counterspell, bob, lightningBoltItem)
	if err != nil {
		log.Fatalf("Failed to cast Counterspell: %v", err)
	}

	fmt.Printf("Stack size: %d\n", spellCastingEngine.GetStack().Size())
	fmt.Printf("Lightning Bolt countered: %v\n", lightningBoltItem.Countered)
	fmt.Printf("Priority: %s\n\n", spellCastingEngine.GetPriorityPlayer().GetName())

	// Step 4: Show stack state
	fmt.Println("📋 Step 4: Current stack state (top to bottom):")
	stackState := spellCastingEngine.GetStackState()
	for i := len(stackState) - 1; i >= 0; i-- {
		fmt.Printf("  %d. %s\n", len(stackState)-i, stackState[i])
	}
	fmt.Println()

	// Step 5: Resolve stack
	fmt.Println("📋 Step 5: Both players pass priority, resolving stack")
	
	// Simulate both players passing priority
	currentPriority := spellCastingEngine.GetPriorityPlayer()
	fmt.Printf("Current priority: %s\n", currentPriority.GetName())
	
	// Resolve the stack manually for demonstration
	fmt.Println("\nResolving stack items:")
	
	for !spellCastingEngine.IsStackEmpty() {
		topItem := spellCastingEngine.GetStack().Peek()
		fmt.Printf("Resolving: %s\n", topItem.Description)
		
		err = spellCastingEngine.GetStack().ResolveTop()
		if err != nil {
			log.Fatalf("Failed to resolve stack item: %v", err)
		}
	}

	// Final state
	fmt.Printf("\n🎯 Final state: Alice (%d life), Bob (%d life)\n", alice.GetLifeTotal(), bob.GetLifeTotal())
	fmt.Printf("Stack empty: %v\n\n", spellCastingEngine.IsStackEmpty())

	// Demonstrate sorcery timing
	fmt.Println("🎯 Scenario: Sorcery Timing Restrictions")
	
	divination := card.Card{
		Name:       "Divination",
		ManaCost:   "{2}{U}",
		CMC:        3,
		TypeLine:   "Sorcery",
		OracleText: "Draw two cards.",
	}

	// Try to cast sorcery during main phase (should work)
	fmt.Println("📋 Attempting to cast Divination during main phase...")
	err = spellCastingEngine.CastSorcerySpell(divination, alice, []interface{}{})
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Success: Divination cast successfully")
		spellCastingEngine.ResolveStack()
	}

	// Try to cast sorcery during combat (should fail)
	fmt.Println("\n📋 Attempting to cast Divination during combat phase...")
	spellCastingEngine.SetPhase("Combat Phase")
	gameState.isMainPhase = false
	gameState.isCombatPhase = true

	err = spellCastingEngine.CastSorcerySpell(divination, alice, []interface{}{})
	if err != nil {
		fmt.Printf("❌ Failed as expected: %v\n", err)
	} else {
		fmt.Println("✅ Unexpected success")
	}

	fmt.Println("\n=== Stack System Demonstration Complete ===")
}
