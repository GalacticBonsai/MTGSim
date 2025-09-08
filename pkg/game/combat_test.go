package game

import "testing"

func TestCombat_AttackUnblockedDealsDamageToDefendingPlayer(t *testing.T) {
    p1 := NewPlayer("P1", 20)
    p2 := NewPlayer("P2", 20)
    g := NewGame(p1, p2)

    // Create a 3/2 attacker for P1
    card := SimpleCard{Name: "Attacker", TypeLine: "Creature", Power: "3", Toughness: "2"}
    att := NewPermanent(card, p1, p1)
    p1.Battlefield = append(p1.Battlefield, att)

    g.BeginCombat()
    if err := g.DeclareAttacker(att, p2); err != nil { t.Fatalf("declare attacker: %v", err) }
    g.ResolveCombatDamage()

    if p2.GetLifeTotal() != 17 {
        t.Fatalf("expected defender at 17 life, got %d", p2.GetLifeTotal())
    }
    if !att.IsTapped() {
        t.Fatalf("attacker should be tapped after attacking")
    }
}

func TestCombat_BlockerTradesWithAttacker(t *testing.T) {
    p1 := NewPlayer("P1", 20)
    p2 := NewPlayer("P2", 20)
    g := NewGame(p1, p2)

    attCard := SimpleCard{Name: "BearA", TypeLine: "Creature", Power: "2", Toughness: "2"}
    blkCard := SimpleCard{Name: "BearB", TypeLine: "Creature", Power: "2", Toughness: "2"}
    att := NewPermanent(attCard, p1, p1)
    blk := NewPermanent(blkCard, p2, p2)
    p1.Battlefield = append(p1.Battlefield, att)
    p2.Battlefield = append(p2.Battlefield, blk)

    g.BeginCombat()
    if err := g.DeclareAttacker(att, p2); err != nil { t.Fatalf("declare attacker: %v", err) }
    if err := g.DeclareBlocker(blk, att); err != nil { t.Fatalf("declare blocker: %v", err) }
    g.ResolveCombatDamage()

    if len(p1.Battlefield) != 0 || len(p2.Battlefield) != 0 {
        t.Fatalf("expected no creatures on battlefield after trade")
    }
    if len(p1.Graveyard) != 1 || len(p2.Graveyard) != 1 {
        t.Fatalf("expected both creatures in respective graveyards")
    }
}

func TestCombat_FirstStrikeAttackerSurvives(t *testing.T) {
    p1 := NewPlayer("P1", 20)
    p2 := NewPlayer("P2", 20)
    g := NewGame(p1, p2)

    attCard := SimpleCard{Name: "FS Attacker", TypeLine: "Creature", Power: "2", Toughness: "2"}
    blkCard := SimpleCard{Name: "Normal Blocker", TypeLine: "Creature", Power: "2", Toughness: "2"}
    att := NewPermanent(attCard, p1, p1)
    att.SetFirstStrike(true)
    blk := NewPermanent(blkCard, p2, p2)

    p1.Battlefield = append(p1.Battlefield, att)
    p2.Battlefield = append(p2.Battlefield, blk)

    g.BeginCombat()
    if err := g.DeclareAttacker(att, p2); err != nil { t.Fatalf("declare attacker: %v", err) }
    if err := g.DeclareBlocker(blk, att); err != nil { t.Fatalf("declare blocker: %v", err) }
    g.ResolveCombatDamage()

    if len(p2.Battlefield) != 0 || len(p2.Graveyard) != 1 {
        t.Fatalf("expected blocker dead from first strike")
    }
    if len(p1.Battlefield) != 1 || len(p1.Graveyard) != 0 {
        t.Fatalf("expected attacker to survive first strike combat")
    }
}

