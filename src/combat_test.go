package main

import "testing"

func TestCanBlock(t *testing.T) {
	// Helper to make a minimal Card and Permanant
	makePerm := func(name string, keywords []string, colors []string, typeline string) *Permanant {
		return &Permanant{
			source: Card{
				Name:     name,
				Keywords: keywords,
				Colors:   colors,
				TypeLine: typeline,
			},
		}
	}

	t.Run("Flying blocked by Flying", func(t *testing.T) {
		attacker := makePerm("A", []string{"Flying"}, nil, "Creature")
		blocker := makePerm("B", []string{"Flying"}, nil, "Creature")
		if !CanBlock(attacker, blocker) {
			t.Error("Flying should be blocked by Flying")
		}
	})
	t.Run("Flying blocked by Reach", func(t *testing.T) {
		attacker := makePerm("A", []string{"Flying"}, nil, "Creature")
		blocker := makePerm("B", []string{"Reach"}, nil, "Creature")
		if !CanBlock(attacker, blocker) {
			t.Error("Flying should be blocked by Reach")
		}
	})
	t.Run("Flying blocked by no ability", func(t *testing.T) {
		attacker := makePerm("A", []string{"Flying"}, nil, "Creature")
		blocker := makePerm("B", nil, nil, "Creature")
		if CanBlock(attacker, blocker) {
			t.Error("Flying should not be blocked by a creature with no ability")
		}
	})
	t.Run("Intimidate blocked by artifact", func(t *testing.T) {
		attacker := makePerm("A", []string{"Intimidate"}, []string{"R"}, "Creature")
		blocker := makePerm("B", nil, nil, "Artifact Creature")
		if !CanBlock(attacker, blocker) {
			t.Error("Intimidate should be blocked by artifact creature")
		}
	})
	t.Run("Intimidate blocked by same color", func(t *testing.T) {
		attacker := makePerm("A", []string{"Intimidate"}, []string{"R"}, "Creature")
		blocker := makePerm("B", nil, []string{"R"}, "Creature")
		if !CanBlock(attacker, blocker) {
			t.Error("Intimidate should be blocked by same color creature")
		}
	})
	t.Run("Intimidate not blocked by off color", func(t *testing.T) {
		attacker := makePerm("A", []string{"Intimidate"}, []string{"R"}, "Creature")
		blocker := makePerm("B", nil, []string{"G"}, "Creature")
		if CanBlock(attacker, blocker) {
			t.Error("Intimidate should not be blocked by off color non-artifact creature")
		}
	})
	t.Run("Shadow blocked by Shadow", func(t *testing.T) {
		attacker := makePerm("A", []string{"Shadow"}, nil, "Creature")
		blocker := makePerm("B", []string{"Shadow"}, nil, "Creature")
		if !CanBlock(attacker, blocker) {
			t.Error("Shadow should be blocked by Shadow")
		}
	})
	t.Run("Shadow not blocked by non-Shadow", func(t *testing.T) {
		attacker := makePerm("A", []string{"Shadow"}, nil, "Creature")
		blocker := makePerm("B", nil, nil, "Creature")
		if CanBlock(attacker, blocker) {
			t.Error("Shadow should not be blocked by non-Shadow")
		}
	})
	t.Run("Fear blocked by artifact", func(t *testing.T) {
		attacker := makePerm("A", []string{"Fear"}, nil, "Creature")
		blocker := makePerm("B", nil, nil, "Artifact Creature")
		if !CanBlock(attacker, blocker) {
			t.Error("Fear should be blocked by artifact creature")
		}
	})
	t.Run("Fear blocked by black", func(t *testing.T) {
		attacker := makePerm("A", []string{"Fear"}, nil, "Creature")
		blocker := makePerm("B", nil, []string{"B"}, "Creature")
		if !CanBlock(attacker, blocker) {
			t.Error("Fear should be blocked by black creature")
		}
	})
	t.Run("Fear not blocked by non-black, non-artifact", func(t *testing.T) {
		attacker := makePerm("A", []string{"Fear"}, nil, "Creature")
		blocker := makePerm("B", nil, []string{"G"}, "Creature")
		if CanBlock(attacker, blocker) {
			t.Error("Fear should not be blocked by non-black, non-artifact creature")
		}
	})
	t.Run("Protection from Black blocks black creature", func(t *testing.T) {
		attacker := makePerm("A", []string{"Protection", "Protection from Black"}, nil, "Creature")
		blocker := makePerm("B", nil, []string{"B"}, "Creature")
		if CanBlock(attacker, blocker) {
			t.Error("Protection from Black should not be blocked by black creature")
		}
	})
	// Default: can be blocked
	t.Run("No ability blocks", func(t *testing.T) {
		attacker := makePerm("A", nil, nil, "Creature")
		blocker := makePerm("B", nil, nil, "Creature")
		if !CanBlock(attacker, blocker) {
			t.Error("Normal creatures should be able to block each other")
		}
	})
}

func TestDoubleStrikeDealsDoubleDamage(t *testing.T) {
	attacker := Permanant{
		source: Card{
			Name:     "Fencing Ace",
			Keywords: []string{"Double Strike"},
		},
		power:     1,
		toughness: 10,
	}
	defender := Permanant{
		source: Card{
			Name:     "Grizzly Bears",
			Keywords: []string{},
		},
		power:     3,
		toughness: 4,
	}
	player := Player{Creatures: []*Permanant{&attacker}, LifeTotal: 20}
	opp := Player{Creatures: []*Permanant{&defender}, LifeTotal: 20}
	attacker.owner = &player
	defender.owner = &opp
	attacker.attacking = &opp
	player.Opponents = []*Player{&opp}
	opp.Opponents = []*Player{&player}

	// Simulate combat: attacker attacks, defender blocks
	AssignBlockers(&attacker, []*Permanant{&defender})

	// Use the full combat damage function
	player.DealDamage()

	doubleStrikeDamage := defender.damage_counters
	expected := attacker.power * 2
	if doubleStrikeDamage != expected {
		t.Errorf("Double Strike attacker should deal %d total damage, got %d", expected, doubleStrikeDamage)
	}

	regularDamage := attacker.damage_counters
	if regularDamage != defender.power {
		t.Errorf("Regular damage should be dealt, got %d", regularDamage)
	}

	// No damage went through to the player
	if player.LifeTotal != 20 {
		t.Errorf("Player should start at 20 life, got %d", player.LifeTotal)
	}
	if opp.LifeTotal != 20 {
		t.Errorf("Opponent should start at 20 life, got %d", opp.LifeTotal)
	}
}
