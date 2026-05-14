package ability

func targetsForEffect(effect Effect, allTargets []any, cursor *int) []any {
	needed := effectTargetSlots(effect)
	if needed == 0 || cursor == nil || *cursor >= len(allTargets) {
		return nil
	}
	end := *cursor + needed
	if end > len(allTargets) {
		end = len(allTargets)
	}
	out := allTargets[*cursor:end]
	*cursor = end
	return out
}

func effectTargetSlots(effect Effect) int {
	total := 0
	for _, target := range effect.Targets {
		count := target.Count
		if count <= 0 && target.Required {
			count = 1
		}
		total += count
	}
	if total == 0 {
		return legacyEffectTargetSlots(effect.Type)
	}
	return total
}

func legacyEffectTargetSlots(effectType EffectType) int {
	switch effectType {
	case SourcePowerDamage:
		return 2
	case DealDamage, PumpCreature, DestroyPermanent, CounterSpell, ReturnToHand,
		TapUntap, ChangeControl, Exile, AddCounters, UntapPermanent, SacrificePermanent:
		return 1
	default:
		return 0
	}
}
