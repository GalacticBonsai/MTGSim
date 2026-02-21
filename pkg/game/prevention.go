package game

import "reflect"

type prevention struct {
	pool map[uintptr]int // remaining prevention by target pointer
}

func (g *Game) ensurePrevention() {
	if g.prevention == nil {
		g.prevention = &prevention{pool: map[uintptr]int{}}
	}
}

func keyFor(target any) uintptr {
	if target == nil {
		return 0
	}
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr {
		return 0
	}
	return v.Pointer()
}

// AddDamagePrevention adds a prevention shield for the target until EOT.
func (g *Game) AddDamagePrevention(target any, amount int) {
	if amount <= 0 || target == nil {
		return
	}
	g.ensurePrevention()
	k := keyFor(target)
	g.prevention.pool[k] = g.prevention.pool[k] + amount
}

// clearPreventionEOT resets all damage prevention at end of turn.
func (g *Game) clearPreventionEOT() {
	if g.prevention != nil {
		g.prevention.pool = map[uintptr]int{}
	}
}

// ApplyDamageToPlayer applies damage to a player after prevention.
func (g *Game) ApplyDamageToPlayer(p *Player, amount int) {
	if p == nil || amount <= 0 {
		return
	}
	rem := g.consumePrevention(p, amount)
	if rem > 0 {
		p.SetLifeTotal(p.GetLifeTotal() - rem)
	}
}

// ApplyDamageToPermanent applies damage to a permanent after prevention.
func (g *Game) ApplyDamageToPermanent(per *Permanent, amount int) {
	if per == nil || amount <= 0 {
		return
	}
	rem := g.consumePrevention(per, amount)
	if rem > 0 {
		per.AddDamage(rem)
	}
}

func (g *Game) consumePrevention(target any, amount int) int {
	if g.prevention == nil || amount <= 0 {
		return amount
	}
	k := keyFor(target)
	shield := g.prevention.pool[k]
	if shield <= 0 {
		return amount
	}
	if shield >= amount {
		g.prevention.pool[k] = shield - amount
		return 0
	}
	// consume all shield, return leftover
	g.prevention.pool[k] = 0
	return amount - shield
}
