package ability

func effectPTDelta(effect Effect) (int, int) {
	if effect.HasPTDelta {
		return effect.PTPower, effect.PTToughness
	}
	v := effect.Value
	if v == 0 {
		return 0, 0
	}
	if absInt(v) >= 1000 {
		return v / 1000, v % 1000
	}
	return v / 100, v % 100
}

func effectTokenSpec(effect Effect) TokenSpec {
	if effect.HasToken {
		return normalizeEffectTokenSpec(effect.Token)
	}
	if effect.Value >= 1000000 {
		return normalizeEffectTokenSpec(TokenSpec{
			Count:     effect.Value / 1000000,
			Name:      "Token",
			TypeLine:  "Creature — Token",
			Power:     (effect.Value % 1000000) / 1000,
			Toughness: effect.Value % 1000,
		})
	}
	if effect.Value > 0 {
		return normalizeEffectTokenSpec(TokenSpec{Name: "Token", TypeLine: "Creature — Token", Count: effect.Value, Power: 1, Toughness: 1})
	}
	return normalizeEffectTokenSpec(TokenSpec{Name: "Token", TypeLine: "Creature — Token", Count: 1, Power: 1, Toughness: 1})
}

func normalizeEffectTokenSpec(spec TokenSpec) TokenSpec {
	if spec.Count <= 0 {
		spec.Count = 1
	}
	if spec.Name == "" {
		spec.Name = "Token"
	}
	if spec.TypeLine == "" {
		spec.TypeLine = "Creature — Token"
	}
	return spec
}
