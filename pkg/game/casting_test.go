package game_test

import (
	"testing"

	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/bridge"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// --- Minimal test doubles to satisfy ability interfaces ---

type tdPlayer struct {
	name string
	life int
}

func (p *tdPlayer) GetName() string                    { return p.name }
func (p *tdPlayer) GetLifeTotal() int                  { return p.life }
func (p *tdPlayer) SetLifeTotal(l int)                 { p.life = l }
func (p *tdPlayer) GetHand() []any                     { return nil }
func (p *tdPlayer) AddCardToHand(any)                  {}
func (p *tdPlayer) GetCreatures() []any                { return nil }
func (p *tdPlayer) GetLands() []any                    { return nil }
func (p *tdPlayer) CanPayCost(c abil.Cost) bool        { return true }
func (p *tdPlayer) PayCost(c abil.Cost) error          { return nil }
func (p *tdPlayer) GetManaPool() map[game.ManaType]int { return map[game.ManaType]int{} }

type tdGameState struct {
	players []abil.AbilityPlayer
	active  abil.AbilityPlayer
}

func (gs *tdGameState) GetPlayer(name string) abil.AbilityPlayer {
	for _, p := range gs.players {
		if p.GetName() == name {
			return p
		}
	}
	return nil
}
func (gs *tdGameState) GetAllPlayers() []abil.AbilityPlayer                  { return gs.players }
func (gs *tdGameState) GetCurrentPlayer() abil.AbilityPlayer                 { return gs.active }
func (gs *tdGameState) GetActivePlayer() abil.AbilityPlayer                  { return gs.active }
func (gs *tdGameState) IsMainPhase() bool                                    { return true }
func (gs *tdGameState) IsCombatPhase() bool                                  { return false }
func (gs *tdGameState) CanActivateAbilities() bool                           { return true }
func (gs *tdGameState) AddManaToPool(abil.AbilityPlayer, game.ManaType, int) {}
func (gs *tdGameState) DealDamage(source any, target any, amount int)        {}
func (gs *tdGameState) DrawCards(abil.AbilityPlayer, int)                    {}
func (gs *tdGameState) GainLife(abil.AbilityPlayer, int)                     {}
func (gs *tdGameState) LoseLife(abil.AbilityPlayer, int)                     {}

// Bridge adapter implementing game.SimpleStack

type bridgeAdapter struct{ b *bridge.StackBridge }

func (a *bridgeAdapter) EnqueueSpell(name string, cmc int, manaCost string, typeLine string, controller any, targets []any) error {
	sp := &abil.Spell{Name: name, ManaCost: manaCost, CMC: cmc, TypeLine: typeLine}
	c := controller.(abil.AbilityPlayer)
	a.b.Stack().AddSpell(sp, c, targets)
	return nil
}
func (a *bridgeAdapter) Size() int { return a.b.Size() }

func TestGame_CastSimpleSpell_DelegatesToStack(t *testing.T) {
	p1 := &tdPlayer{name: "Alice", life: 20}
	p2 := &tdPlayer{name: "Bob", life: 20}
	gs := &tdGameState{players: []abil.AbilityPlayer{p1, p2}, active: p1}

	b := bridge.NewStackBridge(gs)
	adapter := &bridgeAdapter{b: b}

	g := game.NewGame(game.NewPlayer("G1P1", 20), game.NewPlayer("G1P2", 20))
	g.SetStack(adapter)

	if err := g.CastSimpleSpell("Shock", 1, "{R}", "Instant", p1, []any{p2}); err != nil {
		t.Fatalf("cast error: %v", err)
	}
	if adapter.Size() != 1 {
		t.Fatalf("expected stack size 1, got %d", adapter.Size())
	}
}
