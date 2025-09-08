package bridge

import (
    "testing"

    abil "github.com/mtgsim/mtgsim/pkg/ability"
    "github.com/mtgsim/mtgsim/pkg/game"
)

// --- Minimal test doubles to satisfy ability interfaces ---

type tdPlayer struct {
    name string
    life int
}

func (p *tdPlayer) GetName() string                { return p.name }
func (p *tdPlayer) GetLifeTotal() int              { return p.life }
func (p *tdPlayer) SetLifeTotal(l int)             { p.life = l }
func (p *tdPlayer) GetHand() []any                 { return nil }
func (p *tdPlayer) AddCardToHand(any)              {}
func (p *tdPlayer) GetCreatures() []any            { return nil }
func (p *tdPlayer) GetLands() []any                { return nil }
func (p *tdPlayer) CanPayCost(c abil.Cost) bool    { return true }
func (p *tdPlayer) PayCost(c abil.Cost) error      { return nil }
func (p *tdPlayer) GetManaPool() map[game.ManaType]int { return map[game.ManaType]int{} }

type tdGameState struct{
    players []abil.AbilityPlayer
    active  abil.AbilityPlayer
}

func (gs *tdGameState) GetPlayer(name string) abil.AbilityPlayer {
    for _, p := range gs.players { if p.GetName()==name { return p } }
    return nil
}
func (gs *tdGameState) GetAllPlayers() []abil.AbilityPlayer { return gs.players }
func (gs *tdGameState) GetCurrentPlayer() abil.AbilityPlayer { return gs.active }
func (gs *tdGameState) GetActivePlayer() abil.AbilityPlayer  { return gs.active }
func (gs *tdGameState) IsMainPhase() bool { return true }
func (gs *tdGameState) IsCombatPhase() bool { return false }
func (gs *tdGameState) CanActivateAbilities() bool { return true }
func (gs *tdGameState) AddManaToPool(abil.AbilityPlayer, game.ManaType, int) {}
func (gs *tdGameState) DealDamage(source any, target any, amount int) {}
func (gs *tdGameState) DrawCards(abil.AbilityPlayer, int) {}
func (gs *tdGameState) GainLife(abil.AbilityPlayer, int) {}
func (gs *tdGameState) LoseLife(abil.AbilityPlayer, int) {}

// --- Tests ---

func TestStackBridge_PushPopSpell(t *testing.T) {
    p := &tdPlayer{name:"Alice", life:20}
    gs := &tdGameState{players: []abil.AbilityPlayer{p}, active: p}
    b := NewStackBridge(gs)

    // Create a trivial spell
    sp := &abil.Spell{Name: "Shock", ManaCost: "{R}", CMC:1, TypeLine:"Instant"}

    b.Stack().AddSpell(sp, p, nil)
    if b.Size() != 1 {
        t.Fatalf("expected size 1, got %d", b.Size())
    }

    // Resolve top (should not error with our doubles)
    if err := b.Stack().ResolveTop(); err != nil {
        t.Fatalf("resolve error: %v", err)
    }
    if !b.IsEmpty() {
        t.Fatalf("expected stack empty after resolve")
    }
}

