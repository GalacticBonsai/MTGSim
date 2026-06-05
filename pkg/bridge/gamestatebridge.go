package bridge

import (
	"strings"

	abil "github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/google/uuid"
	"github.com/mtgsim/mtgsim/pkg/game"
)

// AbilityGameState bridges ability.GameState to a concrete *game.Game.
type AbilityGameState struct {
	G              *game.Game
	players        []abil.AbilityPlayer
	OnActivate     func(cardName, detail string)
	OnSearchResult func(foundCardName string)
}

// NewAbilityGameState creates the bridge for a given game.
func NewAbilityGameState(g *game.Game) *AbilityGameState {
	return &AbilityGameState{G: g}
}

// --- AbilityPlayer adapter ---

type playerAdapter struct {
	P    *game.Player
	Game *game.Game
}

func (p *playerAdapter) GetName() string    { return p.P.GetName() }
func (p *playerAdapter) GetLifeTotal() int  { return p.P.GetLifeTotal() }
func (p *playerAdapter) SetLifeTotal(l int) { p.P.SetLifeTotal(l) }
func (p *playerAdapter) Lose(reason string) { p.P.Lose(reason) }
func (p *playerAdapter) GetHand() []any     { return sliceToAny(p.P.GetHand()) }
func (p *playerAdapter) AddCardToHand(c any) {
	if sc, ok := c.(game.SimpleCard); ok {
		p.P.AddCardToHand(sc)
	}
}
func (p *playerAdapter) GetCreatures() []any       { return wrapPerms(p.P.GetCreatures(), p.Game) }
func (p *playerAdapter) GetLands() []any           { return wrapPerms(p.P.GetLands(), p.Game) }
func (p *playerAdapter) AddLandPlay(n int)         { p.P.AddLandPlay(n) }

func wrapPerms(perms []*game.Permanent, g *game.Game) []any {
	out := make([]any, len(perms))
	for i, perm := range perms {
		out[i] = &permAdapter{P: perm, Game: g}
	}
	return out
}

type permAdapter struct {
	P         *game.Permanent
	Game      *game.Game
	abilities []*abil.Ability
	parsed     bool
}

func (pa *permAdapter) GetAbilities() []*abil.Ability {
	if !pa.parsed {
		pa.parsed = true
		src := pa.P.GetSource()
		if src.OracleText != "" {
			engine := abil.NewExecutionEngine(NewAbilityGameState(pa.Game))
			abs, err := engine.ParseAndRegisterAbilities(src.OracleText, src)
			if err == nil {
				pa.abilities = abs
			}
		}
	}
	return pa.abilities
}
func (pa *permAdapter) AddAbility(a *abil.Ability)     { pa.abilities = append(pa.abilities, a) }
func (pa *permAdapter) RemoveAbility(id uuid.UUID) {
	for i, a := range pa.abilities {
		if a.ID == id {
			pa.abilities = append(pa.abilities[:i], pa.abilities[i+1:]...)
			return
		}
	}
}
func (pa *permAdapter) GetSource() game.SimpleCard { return pa.P.GetSource() }
func (pa *permAdapter) Tap()            { pa.P.Tap() }
func (pa *permAdapter) Untap()          { pa.P.Untap() }
func (pa *permAdapter) IsTapped() bool  { return pa.P.IsTapped() }
func (pa *permAdapter) GetName() string { return pa.P.GetName() }
func (pa *permAdapter) GetID() uuid.UUID { return pa.P.GetID() }
func (pa *permAdapter) GetOwner() abil.AbilityPlayer {
	return &playerAdapter{P: pa.P.GetOwner(), Game: pa.Game}
}
func (pa *permAdapter) GetController() abil.AbilityPlayer {
	return &playerAdapter{P: pa.P.GetController(), Game: pa.Game}
}
func (p *playerAdapter) GetManaPool() map[game.ManaType]int {
	return p.P.GetManaPool()
}

func (p *playerAdapter) CanPayCost(c abil.Cost) bool {
	if c.LifeCost > 0 && p.P.GetLifeTotal() < c.LifeCost {
		return false
	}
	if c.DiscardCost > 0 && len(p.P.Hand) < c.DiscardCost {
		return false
	}
	if c.SacrificeCost && len(p.P.Battlefield) == 0 {
		return false
	}
	return true
}

func (p *playerAdapter) PayCost(c abil.Cost) error {
	if !p.CanPayCost(c) {
		return nil
	}
	if c.LifeCost > 0 {
		p.P.SetLifeTotal(p.P.GetLifeTotal() - c.LifeCost)
	}
	if c.DiscardCost > 0 {
		p.P.Discard(c.DiscardCost)
	}
	return nil
}

func sliceToAny[T any](in []T) []any {
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
		b.players = append(b.players, &playerAdapter{P: rp, Game: b.G})
	}
	return b.players
}

func (b *AbilityGameState) GetCurrentPlayer() abil.AbilityPlayer {
	cur := b.G.GetCurrentPlayerRaw()
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

func (b *AbilityGameState) DiscardCards(player abil.AbilityPlayer, count int) {
	if pa, ok := player.(*playerAdapter); ok {
		pa.P.Discard(count)
	}
}

func (b *AbilityGameState) SearchLibrary(player abil.AbilityPlayer, count int) {
	if pa, ok := player.(*playerAdapter); ok {
		pa.P.SearchLibraryToHand(count)
	}
}

func (b *AbilityGameState) SearchLibraryAdvanced(player abil.AbilityPlayer, count int, description string) {
	if pa, ok := player.(*playerAdapter); ok {
		searchLibrarySmart(pa.P, count, description, b.OnSearchResult)
	}
}

func searchLibrarySmart(p *game.Player, count int, description string, onResult func(string)) {
	desc := strings.ToLower(description)
	targetBattlefield := strings.Contains(desc, "onto the battlefield") || strings.Contains(desc, "put it onto")

	var filter func(game.SimpleCard) bool
	switch {
	case strings.Contains(desc, "creature card") || strings.Contains(desc, "creature spell"):
		filter = func(c game.SimpleCard) bool { return c.IsCreature() }
	case strings.Contains(desc, "artifact card"):
		filter = func(c game.SimpleCard) bool { return c.IsArtifact() }
	case strings.Contains(desc, "enchantment card"):
		filter = func(c game.SimpleCard) bool { return c.IsEnchantment() }
	case strings.Contains(desc, "instant card") || strings.Contains(desc, "instant spell"):
		filter = func(c game.SimpleCard) bool { return c.IsInstant() }
	case strings.Contains(desc, "sorcery card") || strings.Contains(desc, "sorcery spell"):
		filter = func(c game.SimpleCard) bool { return c.IsSorcery() }
	case strings.Contains(desc, "planeswalker card"):
		filter = func(c game.SimpleCard) bool { return c.IsPlaneswalker() }
	case strings.Contains(desc, "basic land"):
		filter = func(c game.SimpleCard) bool {
			return c.IsLand() && strings.Contains(strings.ToLower(c.TypeLine), "basic")
		}
	case strings.Contains(desc, "land card") || strings.Contains(desc, "island") || strings.Contains(desc, "swamp") ||
		strings.Contains(desc, "mountain") || strings.Contains(desc, "forest") || strings.Contains(desc, "plains"):
		filter = func(c game.SimpleCard) bool {
			if !c.IsLand() {
				return false
			}
			ct := strings.ToLower(c.TypeLine)
			matchedAny := (strings.Contains(desc, "island") && strings.Contains(ct, "island")) ||
				(strings.Contains(desc, "swamp") && strings.Contains(ct, "swamp")) ||
				(strings.Contains(desc, "mountain") && strings.Contains(ct, "mountain")) ||
				(strings.Contains(desc, "forest") && strings.Contains(ct, "forest"))
			if strings.Contains(desc, "plains") && strings.Contains(ct, "plains") {
				matchedAny = true
			}
			if !matchedAny && (strings.Contains(desc, "island") || strings.Contains(desc, "swamp") ||
				strings.Contains(desc, "mountain") || strings.Contains(desc, "forest") || strings.Contains(desc, "plains")) {
				return false
			}
			return true
		}
	case strings.Contains(desc, "card") && (strings.Contains(desc, "search") || strings.Contains(desc, "tutor") || strings.Contains(desc, "find")):
		filter = func(c game.SimpleCard) bool { return true }
	default:
		searchLibrarySmartFallback(p, count, targetBattlefield)
		return
	}

	found := 0
	var remaining []game.SimpleCard
	for _, c := range p.Library {
		if found < count && filter(c) {
			found++
			if onResult != nil {
				onResult(c.Name)
			}
			if targetBattlefield {
				perm := game.NewPermanent(c, p, p)
				p.Battlefield = append(p.Battlefield, perm)
			} else {
				p.Hand = append(p.Hand, c)
			}
		} else {
			remaining = append(remaining, c)
		}
	}
	p.Library = remaining
}

func searchLibrarySmartFallback(p *game.Player, count int, toBattlefield bool) {
	if count > len(p.Library) {
		count = len(p.Library)
	}
	found := make([]game.SimpleCard, count)
	copy(found, p.Library[:count])
	p.Library = p.Library[count:]
	if toBattlefield {
		for _, c := range found {
			perm := game.NewPermanent(c, p, p)
			p.Battlefield = append(p.Battlefield, perm)
		}
	} else {
		p.Hand = append(p.Hand, found...)
	}
}

func (b *AbilityGameState) CreateToken(controller abil.AbilityPlayer, token game.SimpleCard) {
	if pa, ok := controller.(*playerAdapter); ok {
		pa.P.PutTokenOnBattlefield(token)
	}
}

func (b *AbilityGameState) PreventDamage(target any, amount int) {
	b.G.AddDamagePrevention(target, amount)
}

func (b *AbilityGameState) MillCards(player abil.AbilityPlayer, count int) {
	if pa, ok := player.(*playerAdapter); ok {
		for i := 0; i < count && len(pa.P.Library) > 0; i++ {
			top := pa.P.Library[0]
			pa.P.Library = pa.P.Library[1:]
			pa.P.Graveyard = append(pa.P.Graveyard, top)
		}
	}
}

func (b *AbilityGameState) ReanimateCreature(player abil.AbilityPlayer, card game.SimpleCard) {
	if pa, ok := player.(*playerAdapter); ok {
		pa.P.PutTokenOnBattlefield(card)
		if b.OnActivate != nil {
			b.OnActivate(card.Name, "reanimated")
		}
	}
}

func (b *AbilityGameState) ScryLibrary(player abil.AbilityPlayer, count int) {
	if pa, ok := player.(*playerAdapter); ok {
		if count > len(pa.P.Library) {
			count = len(pa.P.Library)
		}
		if count == 0 {
			return
		}
		var keep, bottom []game.SimpleCard
		for i := 0; i < count; i++ {
			c := pa.P.Library[i]
			if c.IsLand() || c.IsCreature() {
				keep = append(keep, c)
			} else {
				bottom = append(bottom, c)
			}
		}
		rest := pa.P.Library[count:]
		pa.P.Library = append(append(keep, bottom...), rest...)
	}
}

func (b *AbilityGameState) TakeExtraTurn() {
	if b.G != nil {
		b.G.TakeExtraTurn()
	}
}

func (b *AbilityGameState) SacrificeSource(source any) {
	if srcCard, ok := source.(game.SimpleCard); ok {
		for _, p := range b.G.GetPlayersRaw() {
			for i, perm := range p.Battlefield {
				if perm.GetSource().Name == srcCard.Name {
					p.Battlefield = append(p.Battlefield[:i], p.Battlefield[i+1:]...)
					p.Graveyard = append(p.Graveyard, srcCard)
					if b.OnActivate != nil {
						b.OnActivate(srcCard.Name, "sacrificed")
					}
					return
				}
			}
		}
	}
}
