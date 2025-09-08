package game

import (
	"strconv"
)

// Permanent is a battlefield object created from a card.
type Permanent struct {
	id         string
	source     SimpleCard
	owner      *Player
	controller *Player
	tapped     bool

	// attachment (for Auras etc.)
	attachedTo *Permanent

	// Base combat stats
	power     int
	toughness int
	damage    int

	// Temporary stat modifiers (Until end of turn)
	tempPowerMod     int
	tempToughnessMod int

	// Minimal keyword flags (subset for Task 10)
	firstStrike  bool
	doubleStrike bool
}

func NewPermanent(c SimpleCard, owner *Player, controller *Player) *Permanent {
	p := &Permanent{
		id:               "",
		source:           c,
		owner:            owner,
		controller:       controller,
		tapped:           false,
		attachedTo:       nil,
		power:            parseIntSafe(c.Power),
		toughness:        parseIntSafe(c.Toughness),
		damage:           0,
		tempPowerMod:     0,
		tempToughnessMod: 0,
		firstStrike:      false,
		doubleStrike:     false,
	}
	return p
}

func parseIntSafe(s string) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return 0
}

// Accessors compatible with future adapters
func (p *Permanent) GetID() string          { return p.id }
func (p *Permanent) GetName() string        { return p.source.Name }
func (p *Permanent) GetSource() SimpleCard  { return p.source }
func (p *Permanent) GetOwner() *Player      { return p.owner }
func (p *Permanent) GetController() *Player { return p.controller }
func (p *Permanent) IsTapped() bool         { return p.tapped }
func (p *Permanent) Tap()                   { p.tapped = true }
func (p *Permanent) Untap()                 { p.tapped = false }
func (p *Permanent) GetPower() int          { return p.power + p.tempPowerMod }
func (p *Permanent) GetToughness() int      { return p.toughness + p.tempToughnessMod }
func (p *Permanent) SetPower(v int)         { p.power = v }
func (p *Permanent) SetToughness(v int)     { p.toughness = v }
func (p *Permanent) GetDamageCounters() int { return p.damage }
func (p *Permanent) AddDamage(d int)        { p.damage += d }
func (p *Permanent) ClearDamage()           { p.damage = 0 }

// Temporary pump helpers
func (p *Permanent) addTempPump(dp, dt int) { p.tempPowerMod += dp; p.tempToughnessMod += dt }
func (p *Permanent) clearTempPump()         { p.tempPowerMod = 0; p.tempToughnessMod = 0 }

// Minimal keyword setters/getters
func (p *Permanent) SetFirstStrike(v bool)  { p.firstStrike = v }
func (p *Permanent) HasFirstStrike() bool   { return p.firstStrike }
func (p *Permanent) SetDoubleStrike(v bool) { p.doubleStrike = v }
func (p *Permanent) HasDoubleStrike() bool  { return p.doubleStrike }

// Attachment helpers
func (p *Permanent) AttachTo(target *Permanent) { p.attachedTo = target }
func (p *Permanent) Detach()                    { p.attachedTo = nil }
func (p *Permanent) GetAttachedTo() *Permanent  { return p.attachedTo }

// Helpers
func (p *Permanent) IsCreature() bool { return p.source.IsCreature() }
func (p *Permanent) IsLand() bool     { return p.source.IsLand() }
func (p *Permanent) IsAura() bool     { return p.source.IsAura() }
