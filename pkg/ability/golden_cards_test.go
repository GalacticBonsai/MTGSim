package ability

import (
	"testing"

	"github.com/mtgsim/mtgsim/pkg/game"
)

func TestGoldenRealCardParsing(t *testing.T) {
	parser := NewAbilityParser()
	tests := []struct {
		name       string
		oracle     string
		effectType EffectType
		value      int
		power      int
		toughness  int
		tokens     int
	}{
		{name: "Lightning Bolt", oracle: "Lightning Bolt deals 3 damage to any target.", effectType: DealDamage, value: 3},
		{name: "Giant Growth", oracle: "Target creature gets +3/+3 until end of turn.", effectType: PumpCreature, power: 3, toughness: 3},
		{name: "Raise the Alarm", oracle: "Create two 1/1 white Soldier creature tokens.", effectType: CreateToken, tokens: 2, power: 1, toughness: 1},
		{name: "Counterspell", oracle: "Counter target spell.", effectType: CounterSpell},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tt.oracle, nil)
			if err != nil || len(abilities) != 1 || len(abilities[0].Effects) != 1 {
				t.Fatalf("parse failed: abilities=%#v err=%v", abilities, err)
			}
			effect := abilities[0].Effects[0]
			if effect.Type != tt.effectType {
				t.Fatalf("effect type = %v, want %v", effect.Type, tt.effectType)
			}
			if tt.value != 0 && effect.Value != tt.value {
				t.Fatalf("value = %d, want %d", effect.Value, tt.value)
			}
			if tt.effectType == PumpCreature {
				power, toughness := effectPTDelta(effect)
				if power != tt.power || toughness != tt.toughness {
					t.Fatalf("pump = %+d/%+d, want %+d/%+d", power, toughness, tt.power, tt.toughness)
				}
			}
			if tt.effectType == CreateToken {
				spec := effectTokenSpec(effect)
				if spec.Count != tt.tokens || spec.Power != tt.power || spec.Toughness != tt.toughness {
					t.Fatalf("token spec = %+v", spec)
				}
			}
		})
	}
}

func TestGoldenLightningBoltExecution(t *testing.T) {
	caster := &mockPlayer{name: "Alice", life: 20, manaPool: map[game.ManaType]int{}}
	target := &mockPlayer{name: "Bob", life: 20, manaPool: map[game.ManaType]int{}}
	gs := &mockGameState{players: []AbilityPlayer{caster, target}, currentPlayer: caster, isMainPhase: true}
	ee := NewExecutionEngine(gs)

	if err := ee.ApplyEffect(Effect{Type: DealDamage, Value: 3}, caster, []any{target}); err != nil {
		t.Fatalf("ApplyEffect failed: %v", err)
	}
	if target.life != 17 {
		t.Fatalf("target life = %d, want 17", target.life)
	}
}
