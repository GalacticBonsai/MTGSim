package game

import "github.com/google/uuid"

// AsThoughEffectType identifies the kind of "as though" effect (CR 609.4).
type AsThoughEffectType int

const (
	AsThoughTypeCardCanExistAsThough AsThoughEffectType = iota
	AsThoughTypeLandCanTap
	AsThoughTypePermanentCanTap
	AsThoughTypeCreatureCanAttack
	AsThoughTypeBlockerCanBlock
	AsThoughTypeShroud
	AsThoughTypeHexproof
	AsThoughTypeProtection
	AsThoughTypeCardCanCast
	AsThoughTypeAbilityCanActivate
	AsThoughTypeTriggerCanFire
	AsThoughTypeDamageCanPrevent
	AsThoughTypeCanRegenerate
	AsThoughTypeCanSacrifice
	AsThoughTypeCanDestroy
	AsThoughTypeCanExile
	AsThoughTypeCanReturn
	AsThoughTypeCanTarget
	AsThoughTypeCanUntap
	AsThoughTypeCanCopy
	AsThoughTypeCanMoveZone
)

// AsThoughEffect represents a continuous effect that allows something to
// happen "as though" it weren't the case (CR 609.4). These effects
// modify what is possible without changing the game state directly.
type AsThoughEffect struct {
	ID       uuid.UUID
	Type     AsThoughEffectType
	Subject  uuid.UUID // ID of the affected object/player (0 = game-wide)
	Priority int       // higher wins conflicts
	Applicable func(g *Game, sourceID uuid.UUID, targetID uuid.UUID) bool
	Duration Duration
}

// asThoughEngine holds all active as-though effects.
type asThoughEngine struct {
	effects []*AsThoughEffect
}

func (g *Game) ensureAsThoughEngine() {
	if g.asThoughEngine == nil {
		g.asThoughEngine = &asThoughEngine{}
	}
}

// RegisterAsThoughEffect adds an as-though effect to the engine.
func (g *Game) RegisterAsThoughEffect(ate *AsThoughEffect) {
	g.ensureAsThoughEngine()
	ate.ID = uuid.New()
	g.asThoughEngine.effects = append(g.asThoughEngine.effects, ate)
}

// RemoveAsThoughEffect removes an as-though effect by ID.
func (g *Game) RemoveAsThoughEffect(id uuid.UUID) {
	if g.asThoughEngine == nil {
		return
	}
	for i, ate := range g.asThoughEngine.effects {
		if ate.ID == id {
			g.asThoughEngine.effects = append(
				g.asThoughEngine.effects[:i],
				g.asThoughEngine.effects[i+1:]...,
			)
			return
		}
	}
}

// CanTargetWithAsThough checks if a source can target a permanent using
// as-though effects (e.g. "can target hexproof creatures").
func (g *Game) CanTargetWithAsThough(sourceID, targetID uuid.UUID) bool {
	if g.asThoughEngine == nil {
		return false
	}
	for _, ate := range g.asThoughEngine.effects {
		if ate == nil {
			continue
		}
		if ate.Type != AsThoughTypeCanTarget {
			continue
		}
		if ate.Applicable != nil && ate.Applicable(g, sourceID, targetID) {
			return true
		}
	}
	return false
}

// CanAttackAsThough checks if a creature can attack despite an effect
// that would prevent it (e.g. "creatures can attack as though they didn't
// have defender").
func (g *Game) CanAttackAsThrough(creatureID uuid.UUID) bool {
	if g.asThoughEngine == nil {
		return false
	}
	for _, ate := range g.asThoughEngine.effects {
		if ate == nil {
			continue
		}
		if ate.Type != AsThoughTypeCreatureCanAttack {
			continue
		}
		if ate.Applicable != nil && ate.Applicable(g, creatureID, uuid.Nil) {
			return true
		}
	}
	return false
}

// CanBlockAsThough checks if a creature can block despite an effect
// that would prevent it.
func (g *Game) CanBlockAsThrough(blockerID uuid.UUID) bool {
	if g.asThoughEngine == nil {
		return false
	}
	for _, ate := range g.asThoughEngine.effects {
		if ate == nil {
			continue
		}
		if ate.Type != AsThoughTypeBlockerCanBlock {
			continue
		}
		if ate.Applicable != nil && ate.Applicable(g, blockerID, uuid.Nil) {
			return true
		}
	}
	return false
}

// CanCastAsThough checks if a card can be cast despite restrictions
// (e.g. "cast this card from your graveyard as though it were in your hand").
func (g *Game) CanCastAsThrough(cardID uuid.UUID) bool {
	if g.asThoughEngine == nil {
		return false
	}
	for _, ate := range g.asThoughEngine.effects {
		if ate == nil {
			continue
		}
		if ate.Type != AsThoughTypeCardCanCast {
			continue
		}
		if ate.Applicable != nil && ate.Applicable(g, cardID, uuid.Nil) {
			return true
		}
	}
	return false
}

// clearAsThoughEffectsEOT removes all end-of-turn duration effects.
func (g *Game) clearAsThoughEffectsEOT() {
	if g.asThoughEngine == nil {
		return
	}
	var kept []*AsThoughEffect
	for _, ate := range g.asThoughEngine.effects {
		if ate != nil && ate.Duration != DurationEndOfTurn {
			kept = append(kept, ate)
		}
	}
	g.asThoughEngine.effects = kept
}

// --- Convenience constructors ---

// AddFlashAsThough grants flash to a player (e.g. "you may cast creature
// spells as though they had flash").
func (g *Game) AddFlashAsThough(controller *Player) {
	g.RegisterAsThoughEffect(&AsThoughEffect{
		Type:     AsThoughTypeCardCanCast,
		Priority: 5,
		Duration: DurationPermanent,
		Applicable: func(g *Game, sourceID, targetID uuid.UUID) bool {
			return true // all cards can be cast at instant speed
		},
	})
}

// AddControlledPermanentsCanAttackAsThough grants attacking ability to
// creatures that normally can't (e.g. "you may have creatures you control
// attack as though they weren't defender").
func (g *Game) AddControlledPermanentsCanAttackAsThough(controller *Player) {
	g.RegisterAsThoughEffect(&AsThoughEffect{
		Type:     AsThoughTypeCreatureCanAttack,
		Priority: 5,
		Duration: DurationEndOfTurn,
		Applicable: func(g *Game, creatureID, _ uuid.UUID) bool {
			// Check if the creature is controlled by the player
			for _, pl := range g.players {
				for _, perm := range pl.Battlefield {
					if perm.GetID() == creatureID && perm.GetController() == controller {
						return true
					}
				}
			}
			return false
		},
	})
}
