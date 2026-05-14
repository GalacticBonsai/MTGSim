package simulation

import (
	"strings"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func attemptCEDHComboFinish(g *game.Game, ap *game.Player, log *EDHEventLog, metrics *edhMetrics) bool {
	if g == nil || ap == nil || ap.HasLost() {
		return false
	}
	tapManaSourcesForMainPhaseMana(g, ap)
	resolveCEDHVelocitySpells(g, ap, log, metrics)
	if tryOracleConsult(g, ap, log, metrics) {
		return true
	}
	if tryDoomsdayOracle(g, ap, log, metrics) {
		return true
	}
	if tryBreachBrainFreeze(g, ap, log, metrics) {
		return true
	}
	if tryGodoHelm(g, ap, log, metrics) {
		return true
	}
	if tryDualcasterTwinflame(g, ap, log, metrics) {
		return true
	}
	if tryFoodChain(g, ap, log, metrics) {
		return true
	}
	if tryAetherflux(g, ap, log, metrics) {
		return true
	}
	if tryOptimizedDeckCombo(g, ap, log) {
		return true
	}
	return false
}

func tryOptimizedDeckCombo(g *game.Game, p *game.Player, log *EDHEventLog) bool {
	if g.GetTurnNumber() < 4 || availableManaSources(p) < 3 {
		return false
	}
	if ownsAny(p, "Thassa's Oracle", "Laboratory Maniac", "Jace, Wielder of Mysteries") && ownsAny(p, "Demonic Consultation", "Tainted Pact") {
		comboWin(g, p, "optimized Oracle/Consultation line", log)
		return true
	}
	if g.GetTurnNumber() >= 5 && ownsAny(p, "Doomsday") && ownsAny(p, "Thassa's Oracle", "Laboratory Maniac", "Jace, Wielder of Mysteries") {
		comboWin(g, p, "optimized Doomsday pile", log)
		return true
	}
	if g.GetTurnNumber() >= 5 && ownsAny(p, "Underworld Breach") && ownsAny(p, "Brain Freeze") && ownsAny(p, "Lion's Eye Diamond", "Grinding Station") {
		comboWin(g, p, "optimized Breach combo", log)
		return true
	}
	if g.GetTurnNumber() >= 5 && ownsAny(p, "Dualcaster Mage") && ownsAny(p, "Twinflame", "Heat Shimmer", "Molten Duplication") {
		comboWin(g, p, "optimized Dualcaster combo", log)
		return true
	}
	if g.GetTurnNumber() >= 5 && ownsAny(p, "Food Chain") && ownsAny(p, "Squee, the Immortal", "Eternal Scourge", "Misthollow Griffin") {
		comboWin(g, p, "optimized Food Chain combo", log)
		return true
	}
	if g.GetTurnNumber() >= 6 && ownsAny(p, "Godo, Bandit Warlord") && ownsAny(p, "Helm of the Host") {
		comboWin(g, p, "optimized Godo / Helm line", log)
		return true
	}
	return false
}

func tryOracleConsult(g *game.Game, p *game.Player, log *EDHEventLog, metrics *edhMetrics) bool {
	payoffs := []string{"Thassa's Oracle", "Laboratory Maniac", "Jace, Wielder of Mysteries"}
	exilers := []string{"Demonic Consultation", "Tainted Pact"}
	for _, payoff := range payoffs {
		if !pieceAccessible(p, payoff) {
			continue
		}
		for _, exiler := range exilers {
			if !pieceAccessible(p, exiler) {
				continue
			}
			if ensurePiece(g, p, payoff, log, metrics) && castComboSpell(g, p, exiler, log, metrics) {
				p.Library = nil
				comboWin(g, p, "Oracle/Consultation combo", log)
				return true
			}
		}
	}
	return false
}

func tryDoomsdayOracle(g *game.Game, p *game.Player, log *EDHEventLog, metrics *edhMetrics) bool {
	if !pieceAccessible(p, "Doomsday") || !hasPayoffAnywhere(p) {
		return false
	}
	if castComboSpell(g, p, "Doomsday", log, metrics) {
		comboWin(g, p, "Doomsday pile", log)
		return true
	}
	return false
}

func tryBreachBrainFreeze(g *game.Game, p *game.Player, log *EDHEventLog, metrics *edhMetrics) bool {
	if !pieceAccessible(p, "Underworld Breach") || !pieceAccessible(p, "Brain Freeze") {
		return false
	}
	if !(pieceAccessible(p, "Lion's Eye Diamond") || pieceAccessible(p, "Grinding Station") || len(p.Graveyard) >= 8) {
		return false
	}
	if ensurePiece(g, p, "Underworld Breach", log, metrics) && castComboSpell(g, p, "Brain Freeze", log, metrics) {
		comboWin(g, p, "Underworld Breach / Brain Freeze", log)
		return true
	}
	return false
}

func tryGodoHelm(g *game.Game, p *game.Player, log *EDHEventLog, metrics *edhMetrics) bool {
	if !pieceAccessible(p, "Godo, Bandit Warlord") || !pieceAccessible(p, "Helm of the Host") {
		return false
	}
	if ensurePiece(g, p, "Godo, Bandit Warlord", log, metrics) && ensurePiece(g, p, "Helm of the Host", log, metrics) {
		comboWin(g, p, "Godo / Helm of the Host", log)
		return true
	}
	return false
}

func tryDualcasterTwinflame(g *game.Game, p *game.Player, log *EDHEventLog, metrics *edhMetrics) bool {
	if !pieceAccessible(p, "Dualcaster Mage") || !pieceAccessible(p, "Twinflame") {
		return false
	}
	if ensurePiece(g, p, "Dualcaster Mage", log, metrics) && castComboSpell(g, p, "Twinflame", log, metrics) {
		comboWin(g, p, "Dualcaster Mage / Twinflame", log)
		return true
	}
	return false
}

func tryFoodChain(g *game.Game, p *game.Player, log *EDHEventLog, metrics *edhMetrics) bool {
	if !pieceAccessible(p, "Food Chain") {
		return false
	}
	payoffs := []string{"Squee, the Immortal", "Eternal Scourge", "Misthollow Griffin", "Walking Ballista", "Goblin Cannon"}
	for _, payoff := range payoffs {
		if pieceAccessible(p, payoff) && ensurePiece(g, p, "Food Chain", log, metrics) {
			comboWin(g, p, "Food Chain combo", log)
			return true
		}
	}
	return false
}

func tryAetherflux(g *game.Game, p *game.Player, log *EDHEventLog, metrics *edhMetrics) bool {
	idx := indexOfPlayer(g, p)
	storm := 0
	if metrics != nil && idx >= 0 {
		storm = metrics.turnSpells[idx]
	}
	if pieceAccessible(p, "Aetherflux Reservoir") && (p.GetLifeTotal() >= 50 || storm >= 6) {
		if ensurePiece(g, p, "Aetherflux Reservoir", log, metrics) {
			comboWin(g, p, "Aetherflux Reservoir", log)
			return true
		}
	}
	return false
}

func resolveCEDHVelocitySpells(g *game.Game, p *game.Player, log *EDHEventLog, metrics *edhMetrics) {
	progress := true
	for progress && !p.HasLost() {
		progress = false
		tapManaSourcesForMainPhaseMana(g, p)
		if castDrawEngine(g, p, "Ad Nauseam", 20, 10, log, metrics) {
			progress = true
			continue
		}
		if castDrawEngine(g, p, "Peer into the Abyss", len(p.Library)/2, p.GetLifeTotal()/2, log, metrics) {
			progress = true
			continue
		}
		if castWheel(g, p, "Wheel of Fortune", log, metrics) || castWheel(g, p, "Windfall", log, metrics) || castWheel(g, p, "Timetwister", log, metrics) || castWheel(g, p, "Wheel of Misfortune", log, metrics) || castWheel(g, p, "Echo of Eons", log, metrics) {
			progress = true
			continue
		}
		for _, tutor := range []string{"Demonic Tutor", "Vampiric Tutor", "Imperial Seal", "Mystical Tutor", "Worldly Tutor", "Enlightened Tutor", "Gamble", "Finale of Devastation", "Eldritch Evolution"} {
			if castTutor(g, p, tutor, log, metrics) {
				progress = true
				break
			}
		}
	}
}

func castDrawEngine(g *game.Game, p *game.Player, name string, draw, lose int, log *EDHEventLog, metrics *edhMetrics) bool {
	if draw <= 0 || !castComboSpell(g, p, name, log, metrics) {
		return false
	}
	p.Draw(draw)
	if lose > 0 && p.GetLifeTotal() > lose+1 {
		p.SetLifeTotal(p.GetLifeTotal() - lose)
	}
	return true
}

func castWheel(g *game.Game, p *game.Player, name string, log *EDHEventLog, metrics *edhMetrics) bool {
	if !castComboSpell(g, p, name, log, metrics) {
		return false
	}
	p.Discard(len(p.Hand))
	p.Draw(7)
	return true
}

func castTutor(g *game.Game, p *game.Player, name string, log *EDHEventLog, metrics *edhMetrics) bool {
	if !castComboSpell(g, p, name, log, metrics) {
		return false
	}
	for _, target := range tutorPriority(p) {
		if idx := findZoneCard(p.Library, target); idx >= 0 {
			card := p.Library[idx]
			p.Library = append(p.Library[:idx], p.Library[idx+1:]...)
			p.Hand = append(p.Hand, card)
			return true
		}
	}
	return true
}

func tutorPriority(p *game.Player) []string {
	if pieceAccessible(p, "Thassa's Oracle") || pieceAccessible(p, "Laboratory Maniac") || pieceAccessible(p, "Jace, Wielder of Mysteries") {
		return []string{"Demonic Consultation", "Tainted Pact", "Doomsday"}
	}
	if pieceAccessible(p, "Demonic Consultation") || pieceAccessible(p, "Tainted Pact") {
		return []string{"Thassa's Oracle", "Laboratory Maniac", "Jace, Wielder of Mysteries"}
	}
	if pieceAccessible(p, "Underworld Breach") {
		return []string{"Brain Freeze", "Lion's Eye Diamond", "Grinding Station"}
	}
	return []string{"Thassa's Oracle", "Demonic Consultation", "Tainted Pact", "Underworld Breach", "Brain Freeze", "Doomsday", "Food Chain", "Aetherflux Reservoir"}
}

func comboWin(g *game.Game, winner *game.Player, detail string, log *EDHEventLog) {
	for _, opp := range g.GetPlayersRaw() {
		if opp != nil && opp != winner && !opp.HasLost() {
			opp.Lose("effect")
		}
	}
	if log != nil {
		log.Append(EDHEvent{Turn: g.GetTurnNumber(), Phase: phaseName(g.GetCurrentPhase()), Kind: EventGameEnd, Actor: winner.GetName(), Detail: detail})
	}
}

func pieceAccessible(p *game.Player, name string) bool {
	return findZoneCard(p.Hand, name) >= 0 || permanentNamed(p, name) || findZoneCard(p.CommandZone, name) >= 0 || findZoneCard(p.Graveyard, name) >= 0
}

func hasPayoffAnywhere(p *game.Player) bool {
	for _, name := range []string{"Thassa's Oracle", "Laboratory Maniac", "Jace, Wielder of Mysteries"} {
		if pieceAccessible(p, name) || findZoneCard(p.Library, name) >= 0 {
			return true
		}
	}
	return false
}

func ownsAny(p *game.Player, names ...string) bool {
	for _, name := range names {
		if findZoneCard(p.Hand, name) >= 0 || findZoneCard(p.Library, name) >= 0 || findZoneCard(p.Graveyard, name) >= 0 || findZoneCard(p.CommandZone, name) >= 0 || permanentNamed(p, name) {
			return true
		}
	}
	return false
}

func availableManaSources(p *game.Player) int {
	count := 0
	for _, perm := range p.Battlefield {
		if len(manaProductionOptions(perm.GetSource())) > 0 {
			count++
		}
	}
	return count
}

func permanentNamed(p *game.Player, name string) bool {
	for _, perm := range p.Battlefield {
		if sameCardName(perm.GetName(), name) {
			return true
		}
	}
	return false
}

func findZoneCard(zone []game.SimpleCard, name string) int {
	for i, c := range zone {
		if sameCardName(c.Name, name) {
			return i
		}
	}
	return -1
}

func ensurePiece(g *game.Game, p *game.Player, name string, log *EDHEventLog, metrics *edhMetrics) bool {
	if permanentNamed(p, name) {
		return true
	}
	if castCommanderPiece(g, p, name, log, metrics) {
		return true
	}
	idx := findZoneCard(p.Hand, name)
	if idx < 0 {
		return false
	}
	c := p.Hand[idx]
	if c.IsInstant() || c.IsSorcery() {
		return castComboSpell(g, p, name, log, metrics)
	}
	if !p.CanPayForCard(c) || !p.PayForCard(c) {
		return false
	}
	manaSpent := manaSpentForCard(c)
	perm, err := castPermanentCard(g, p, c)
	if err != nil || perm == nil {
		return false
	}
	perm.SetEnteredTurn(g.GetTurnNumber())
	recordComboCast(g, p, c, manaSpent, c.IsCreature(), log, metrics)
	return true
}

func castCommanderPiece(g *game.Game, p *game.Player, name string, log *EDHEventLog, metrics *edhMetrics) bool {
	idx := findZoneCard(p.CommandZone, name)
	if idx < 0 {
		return false
	}
	c := p.CommandZone[idx]
	if !p.CanPayForCommander(c) || !p.PayForCommander(c) {
		return false
	}
	perm := p.CastCommander(name)
	if perm == nil {
		return false
	}
	perm.SetEnteredTurn(g.GetTurnNumber())
	recordComboCast(g, p, c, manaSpentForCommander(p, c), c.IsCreature(), log, metrics)
	return true
}

func castComboSpell(g *game.Game, p *game.Player, name string, log *EDHEventLog, metrics *edhMetrics) bool {
	idx := findZoneCard(p.Hand, name)
	if idx < 0 {
		return false
	}
	c := p.Hand[idx]
	if !p.CanPayForCard(c) || !p.PayForCard(c) {
		return false
	}
	p.Hand = append(p.Hand[:idx], p.Hand[idx+1:]...)
	p.Graveyard = append(p.Graveyard, c)
	recordComboCast(g, p, c, manaSpentForCard(c), false, log, metrics)
	return true
}

func recordComboCast(g *game.Game, p *game.Player, c game.SimpleCard, manaSpent int, creature bool, log *EDHEventLog, metrics *edhMetrics) {
	idx := -1
	if g != nil {
		idx = indexOfPlayer(g, p)
	} else if metrics != nil && len(metrics.players) == 1 {
		idx = 0
	}
	storm := 0
	if metrics != nil && idx >= 0 {
		storm = metrics.recordSpell(idx, manaSpent, creature, c.Name)
	}
	if log != nil {
		turn := 0
		phase := "main1"
		if g != nil {
			turn = g.GetTurnNumber()
			phase = phaseName(g.GetCurrentPhase())
		}
		log.Append(EDHEvent{Turn: turn, Phase: phase, Kind: EventPermanentCast, Actor: p.GetName(), Detail: eventDetail(c.Name, manaSpent, storm)})
	}
}

func sameCardName(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }