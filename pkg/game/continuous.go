package game

// continuous holds simple continuous effects state for the game.
type continuous struct {
	// base set effects until EOT: store original base stats for restore
	setBase map[*Permanent]struct{ power, toughness int }
}

func (g *Game) ensureContinuous() {
	if g.continuous == nil {
		g.continuous = &continuous{setBase: map[*Permanent]struct{ power, toughness int }{}}
	}
}

// ApplySetPTUntilEOT sets a creature's base power/toughness until end of turn.
// Later calls overwrite earlier ones (last-wins). On EOT, original base is restored.
func (g *Game) ApplySetPTUntilEOT(p *Permanent, power, toughness int) {
	if p == nil {
		return
	}
	g.ensureContinuous()
	// Save original base if first time
	if _, ok := g.continuous.setBase[p]; !ok {
		g.continuous.setBase[p] = struct{ power, toughness int }{power: p.power, toughness: p.toughness}
	}
	// Overwrite base for the duration
	p.power = power
	p.toughness = toughness
}
