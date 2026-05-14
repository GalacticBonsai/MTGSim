package ability

import "strings"

func isHeuristicPattern(description string) bool {
	lower := strings.ToLower(description)
	for _, marker := range []string{"broad", "generic", "catch-all", "simplified", "unknown"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func markAbilityApproximate(ability *Ability, reason string) {
	if ability == nil {
		return
	}
	ability.Approximate = true
	ability.ApproximationReason = reason
	for i := range ability.Effects {
		ability.Effects[i].Approximate = true
		ability.Effects[i].ApproximationReason = reason
	}
}