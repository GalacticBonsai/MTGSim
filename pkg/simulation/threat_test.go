package simulation

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

// makeTestPlayer builds a 40-life EDH-style player and seats them.
func makeTestPlayer(name string) *game.Player {
	return game.NewEDHPlayer(name)
}

// summonOnto fakes a creature on the battlefield with the given power.
func summonOnto(t *testing.T, g *game.Game, p *game.Player, power int) {
	t.Helper()
	c := game.SimpleCard{Name: "Bear", TypeLine: "Creature", Power: itoa(power), Toughness: "2"}
	p.AddCardToHand(c)
	if _, err := g.SummonCreature(p, "Bear"); err != nil {
		t.Fatalf("summon: %v", err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestChooseAttackTarget_PrefersBiggerBoard verifies a stacked opponent
// is attacked over a passive one even when seated further away.
func TestChooseAttackTarget_PrefersBiggerBoard(t *testing.T) {
	p1 := makeTestPlayer("Attacker")
	p2 := makeTestPlayer("Passive")
	p3 := makeTestPlayer("Threat")
	g := game.NewGame(p1, p2, p3)

	summonOnto(t, g, p3, 5)
	summonOnto(t, g, p3, 4)
	// p2 has no creatures.

	if got := chooseAttackTarget(g, p1); got != p3 {
		t.Fatalf("expected attack on bigger board (p3), got %v", got)
	}
}

// TestChooseAttackTarget_PreemptsCommanderDamage demonstrates the
// commander-damage kicker: the player that has already swung 14 with
// their commander becomes the priority target.
func TestChooseAttackTarget_PreemptsCommanderDamage(t *testing.T) {
	p1 := makeTestPlayer("Attacker")
	p2 := makeTestPlayer("CmdrAggressor")
	p3 := makeTestPlayer("BiggerBoard")
	g := game.NewGame(p1, p2, p3)

	// p3 has a slightly bigger board.
	summonOnto(t, g, p3, 4)
	summonOnto(t, g, p2, 1)

	// p2 has already dealt 14 commander damage to p1.
	cmdr := game.SimpleCard{Name: "Aggressor Cmdr", TypeLine: "Legendary Creature"}
	p2.RegisterCommander(cmdr)
	p1.AddCommanderDamage(p2, "Aggressor Cmdr", 14)

	if got := chooseAttackTarget(g, p1); got != p2 {
		t.Fatalf("expected attack on commander-damage threat (p2), got %v", got)
	}
}

// TestChooseAttackTarget_SkipsLost confirms eliminated players are not
// targetable.
func TestChooseAttackTarget_SkipsLost(t *testing.T) {
	p1 := makeTestPlayer("Attacker")
	p2 := makeTestPlayer("Dead")
	p3 := makeTestPlayer("Alive")
	g := game.NewGame(p1, p2, p3)

	p2.SetLifeTotal(0)
	g.ApplyStateBasedActions()
	if !p2.HasLost() {
		t.Fatalf("expected p2 to be lost")
	}

	if got := chooseAttackTarget(g, p1); got != p3 {
		t.Fatalf("expected attack on alive player, got %v", got)
	}
}

// TestChooseAttackTarget_TieBreakBySeatOrder verifies determinism: when
// scores tie, the next opponent in seat order wins.
func TestChooseAttackTarget_TieBreakBySeatOrder(t *testing.T) {
	p1 := makeTestPlayer("Attacker")
	p2 := makeTestPlayer("Left")
	p3 := makeTestPlayer("Right")
	g := game.NewGame(p1, p2, p3)
	// Both opponents identical (no creatures, full life).
	if got := chooseAttackTarget(g, p1); got != p2 {
		t.Fatalf("expected next-in-seat (p2) on tie, got %v", got)
	}
}
