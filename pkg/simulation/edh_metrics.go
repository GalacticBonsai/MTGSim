package simulation

import "github.com/mtgsim/mtgsim/pkg/game"

type edhMetrics struct {
	players    []EDHPlayerRecord
	turnSpells []int
	eliminated []bool
	game       EDHGameRecord
}

func newEDHMetrics(n int) *edhMetrics {
	return &edhMetrics{players: make([]EDHPlayerRecord, n), turnSpells: make([]int, n), eliminated: make([]bool, n)}
}

func (m *edhMetrics) resetTurn() {
	if m == nil {
		return
	}
	for i := range m.turnSpells {
		m.turnSpells[i] = 0
	}
}

func (m *edhMetrics) recordLand(player int, cardName string) {
	if !m.valid(player) {
		return
	}
	p := &m.players[player]
	p.LandsPlayed++
	p.CardsPlayed++
	m.game.TotalCardsPlayed++
	if p.CardStats == nil {
		p.CardStats = map[string]CardPerformance{}
	}
	cp := p.CardStats[cardName]
	cp.Casts++
	p.CardStats[cardName] = cp
}

func (m *edhMetrics) recordSpell(player int, manaSpent int, creature bool, cardName string) int {
	if !m.valid(player) {
		return 0
	}
	p := &m.players[player]
	p.SpellsCast++
	p.CardsPlayed++
	p.ManaSpent += manaSpent
	m.turnSpells[player]++
	if m.turnSpells[player] > p.MaxStormCount {
		p.MaxStormCount = m.turnSpells[player]
	}
	if p.MaxStormCount > m.game.MaxStormCount {
		m.game.MaxStormCount = p.MaxStormCount
	}
	if creature {
		p.CreaturesCast++
	}
	m.game.TotalCardsPlayed++
	m.game.TotalManaSpent += manaSpent
	if p.CardStats == nil {
		p.CardStats = map[string]CardPerformance{}
	}
	cp := p.CardStats[cardName]
	cp.Casts++
	p.CardStats[cardName] = cp
	return m.turnSpells[player]
}

func (m *edhMetrics) recordManaProduced(player int, amount int) {
	if !m.valid(player) || amount <= 0 {
		return
	}
	if amount > m.players[player].ManaProduced {
		m.players[player].ManaProduced = amount
	}
}

func (m *edhMetrics) recordCombatDamage(player int, damage int) {
	if !m.valid(player) || damage <= 0 {
		return
	}
	m.players[player].CombatDamage += damage
	m.game.TotalCombatDamage += damage
}

func (m *edhMetrics) recordElimination(player int) {
	if !m.valid(player) {
		return
	}
	m.players[player].Eliminations++
	m.game.TotalEliminations++
}

func (m *edhMetrics) recordPlayerLost(player int) bool {
	if !m.valid(player) || m.eliminated[player] {
		return false
	}
	m.eliminated[player] = true
	return true
}

func (m *edhMetrics) applyToPlayerRecord(player int, rec *EDHPlayerRecord) {
	if !m.valid(player) || rec == nil {
		return
	}
	stats := m.players[player]
	stats.DeckName = rec.DeckName
	stats.CommanderName = rec.CommanderName
	stats.Mulligans = rec.Mulligans
	stats.FinalLife = rec.FinalLife
	stats.CommanderCasts = rec.CommanderCasts
	stats.Eliminated = rec.Eliminated
	stats.KillSource = rec.KillSource
	*rec = stats
}

func (m *edhMetrics) applyToGameRecord(rec *EDHGameRecord) {
	if m == nil || rec == nil {
		return
	}
	rec.MaxStormCount = m.game.MaxStormCount
	rec.TotalManaSpent = m.game.TotalManaSpent
	rec.TotalManaProduced = 0
	for _, p := range m.players {
		rec.TotalManaProduced += p.ManaProduced
	}
	rec.TotalCardsPlayed = m.game.TotalCardsPlayed
	rec.TotalCombatDamage = m.game.TotalCombatDamage
	rec.TotalEliminations = m.game.TotalEliminations
}

func (m *edhMetrics) valid(player int) bool {
	return m != nil && player >= 0 && player < len(m.players)
}

func manaSpentForCard(c game.SimpleCard) int {
	return manaSpentForCost(c.GetManaCost())
}

func manaSpentForCommander(p *game.Player, c game.SimpleCard) int {
	return manaSpentForCard(c) + p.CommanderTax(c.Name)
}

func manaSpentForCost(cost game.Mana) int {
	total := 0
	for mt, n := range cost {
		if n <= 0 || mt == game.X {
			continue
		}
		total += n
	}
	return total
}
