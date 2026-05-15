package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mtgsim/mtgsim/pkg/ability"
	"github.com/mtgsim/mtgsim/pkg/card"
)

func main() {
	paths := []string{card.CardDBFile, "../../" + card.CardDBFile}
	var dbPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			dbPath = p
			break
		}
	}
	if dbPath == "" {
		fmt.Println("cardDB.json not found")
		return
	}

	data, _ := os.ReadFile(dbPath)
	var cards []card.Card
	json.Unmarshal(data, &cards)
	db := card.NewCardDB(cards)

	parser := ability.NewAbilityParser()

	triggers := make(map[string]int)
	var samples []string

	for _, c := range db.ListAll() {
		abilities, _ := parser.ParseAbilities(c.OracleText, c)
		if len(abilities) == 0 {
			continue
		}
		impl := true
		for _, ab := range abilities {
			if ab.Approximate {
				impl = false
				break
			}
			for _, eff := range ab.Effects {
				if !ability.CanExecuteEffect(eff.Type) {
					impl = false
					break
				}
				if eff.Approximate {
					impl = false
					break
				}
				if eff.Type == ability.ChooseMode || eff.Type == ability.CopySpell ||
					eff.Type == ability.CantAttackBlock || eff.Type == ability.AdditionalLand ||
					eff.Type == ability.ReanimateCreature {
					impl = false
					break
				}
				for _, cond := range eff.Conditions {
					if !ability.CanExecuteCondition(cond.Type) {
						impl = false
						break
					}
				}
				for _, tgt := range eff.Targets {
					if tgt.Enhanced != nil {
						for _, r := range tgt.Enhanced.Restrictions {
							if !ability.CanExecuteTargetRestriction(r.Type) {
								impl = false
								break
							}
						}
					}
				}
			}
		}
		if !impl {
			continue
		}

		hasAny := false
		for _, ab := range abilities {
			if ab.TriggerCondition == ability.AnyTrigger {
				hasAny = true
				cond := extractTriggerText(c.OracleText)
				triggers[cond]++
				if len(samples) < 15 {
					samples = append(samples, fmt.Sprintf("%s: %s", c.Name, cond))
				}
			}
		}
		if hasAny {
			// count as implemented card with AnyTrigger
		}
	}

	fmt.Printf("Total AnyTrigger cards: ~%d\n", len(samples))
	fmt.Println("\nTrigger text frequencies:")
	for k, v := range triggers {
		if v >= 5 {
			fmt.Printf("  %s: %d\n", k, v)
		}
	}
	fmt.Println("\nSample cards:")
	for _, s := range samples {
		fmt.Println("  -", s)
	}
}

func extractTriggerText(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "whenever") || strings.Contains(lower, "when") || strings.Contains(lower, "at ") {
			idx := strings.Index(lower, "whenever")
			if idx == -1 {
				idx = strings.Index(lower, "when")
			}
			if idx == -1 {
				idx = strings.Index(lower, "at ")
			}
			if idx >= 0 {
				comma := strings.Index(lower[idx:], ",")
				if comma > 0 {
					return line[idx : idx+comma]
				}
				return line[idx:]
			}
		}
	}
	return "unknown"
}
