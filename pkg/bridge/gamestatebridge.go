package bridge

import (
	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// AbilityGameState bridges ability.GameState to a concrete *game.Game.
type AbilityGameState struct {
	G *game.Game
	// cached adapters for players
	players []abil.AbilityPlayer
}

// NewAbilityGameState creates the bridge for a given game.
func NewAbilityGameState(g *game.Game) *AbilityGameState {
	return &AbilityGameState{G: g}
}

// --- AbilityPlayer adapter ---

type playerAdapter struct{ P *game.Player }

func (p *playerAdapter) GetName() string    { return p.P.GetName() }
func (p *playerAdapter) GetLifeTotal() int  { return p.P.GetLifeTotal() }
func (p *playerAdapter) SetLifeTotal(l int) { p.P.SetLifeTotal(l) }
func (p *playerAdapter) GetHand() []any     { return sliceToAny(p.P.GetHand()) }
func (p *playerAdapter) AddCardToHand(c any) {
	if sc, ok := c.(game.SimpleCard); ok {
		p.P.AddCardToHand(sc)
	}
}
func (p *playerAdapter) GetCreatures() []any                { return permsToAny(p.P.GetCreatures()) }
func (p *playerAdapter) GetLands() []any                    { return permsToAny(p.P.GetLands()) }
func (p *playerAdapter) CanPayCost(c abil.Cost) bool        { return true }
func (p *playerAdapter) PayCost(c abil.Cost) error          { return nil }
func (p *playerAdapter) GetManaPool() map[game.ManaType]int { return p.P.GetManaPool() }

func sliceToAny[T any](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
func permsToAny(in []*game.Permanent) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// --- Ability.GameState methods ---

func (b *AbilityGameState) GetPlayer(name string) abil.AbilityPlayer {
	for _, p := range b.getAllPlayers() {
		if p.GetName() == name {
			return p
		}
	}
	return nil
}

func (b *AbilityGameState) GetAllPlayers() []abil.AbilityPlayer { return b.getAllPlayers() }

func (b *AbilityGameState) getAllPlayers() []abil.AbilityPlayer {
	if b.players != nil {
		return b.players
	}
	raw := b.G.GetPlayersRaw()
	b.players = make([]abil.AbilityPlayer, 0, len(raw))
	for _, rp := range raw {
		b.players = append(b.players, &playerAdapter{P: rp})
	}
	return b.players
}

func (b *AbilityGameState) GetCurrentPlayer() abil.AbilityPlayer {
	cur := b.G.GetCurrentPlayerRaw()
	// find matching adapter
	for i, rp := range b.G.GetPlayersRaw() {
		if rp == cur {
			return b.getAllPlayers()[i]
		}
	}
	return nil
}

func (b *AbilityGameState) GetActivePlayer() abil.AbilityPlayer {
	act := b.G.GetActivePlayerRaw()
	for i, rp := range b.G.GetPlayersRaw() {
		if rp == act {
			return b.getAllPlayers()[i]
		}
	}
	return nil
}

func (b *AbilityGameState) IsMainPhase() bool          { return b.G.IsMainPhase() }
func (b *AbilityGameState) IsCombatPhase() bool        { return b.G.IsCombatPhase() }
func (b *AbilityGameState) CanActivateAbilities() bool { return true }

func (b *AbilityGameState) AddManaToPool(player abil.AbilityPlayer, manaType game.ManaType, amount int) {
	if pa, ok := player.(*playerAdapter); ok {
		pa.P.AddManaToPool(manaType, amount)
	}
}

func (b *AbilityGameState) DealDamage(_ any, target any, amount int) {
	switch t := target.(type) {
	case *playerAdapter:
		b.G.ApplyDamageToPlayer(t.P, amount)
	case *game.Player:
		b.G.ApplyDamageToPlayer(t, amount)
	case *game.Permanent:
		b.G.ApplyDamageToPermanent(t, amount)
	}
	b.G.ApplyStateBasedActions()
}

func (b *AbilityGameState) DrawCards(player abil.AbilityPlayer, count int) {
	if pa, ok := player.(*playerAdapter); ok {
		pa.P.Draw(count)
	}
}

func (b *AbilityGameState) GainLife(player abil.AbilityPlayer, amount int) {
	if pa, ok := player.(*playerAdapter); ok {
		pa.P.SetLifeTotal(pa.P.GetLifeTotal() + amount)
	}
}

func (b *AbilityGameState) LoseLife(player abil.AbilityPlayer, amount int) {
	if pa, ok := player.(*playerAdapter); ok {
		pa.P.SetLifeTotal(pa.P.GetLifeTotal() - amount)
	}
	b.G.ApplyStateBasedActions()
}
