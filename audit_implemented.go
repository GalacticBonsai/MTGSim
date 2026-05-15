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

	type bucket struct {
		name  string
		count int
		cards []string
	}

	onlyInfo := &bucket{name: "only_informational"}
	addManaTrigger := &bucket{name: "add_mana_trigger"}
	anyTrigger := &bucket{name: "any_trigger_condition"}
	unparsedRestrictions := &bucket{name: "unparsed_restriction_text"}
	clean := &bucket{name: "clean"}

	for _, c := range db.ListAll() {
		abilities, _ := parser.ParseAbilities(c.OracleText, c)
		if len(abilities) == 0 {
			continue
		}

		// Determine if implemented
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
				// runtime-approximate effect types
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

		// Categorize the implemented card
		allInfo := true
		hasAddManaTrigger := false
		hasAnyTrigger := false
		hasUnparsedRestriction := false

		lowerText := strings.ToLower(c.OracleText)

		for _, ab := range abilities {
			for _, eff := range ab.Effects {
				if eff.Type != ability.LookAtLibraryTop && eff.Type != ability.RevealInformation {
					allInfo = false
				}
				if ab.Type == ability.Triggered && eff.Type == ability.AddMana {
					hasAddManaTrigger = true
				}
			}
			if ab.TriggerCondition == ability.AnyTrigger {
				hasAnyTrigger = true
			}
			// Check for unparsed restriction keywords in oracle text
			for _, eff := range ab.Effects {
				for _, tgt := range eff.Targets {
					if tgt.Enhanced != nil && len(tgt.Enhanced.Restrictions) == 0 {
						// Target exists but has no parsed restrictions; check if oracle text has restriction phrases
						if strings.Contains(lowerText, "nonblack") || strings.Contains(lowerText, "nonblue") ||
							strings.Contains(lowerText, "nonwhite") || strings.Contains(lowerText, "nonred") ||
							strings.Contains(lowerText, "nongreen") || strings.Contains(lowerText, "nonartifact") ||
							strings.Contains(lowerText, "noncreature") || strings.Contains(lowerText, "nontoken") ||
							strings.Contains(lowerText, "without flying") || strings.Contains(lowerText, "without trample") ||
							strings.Contains(lowerText, "mana value") || strings.Contains(lowerText, "converted mana cost") {
							hasUnparsedRestriction = true
						}
					}
				}
			}
		}

		b := clean
		if allInfo && len(abilities) > 0 {
			b = onlyInfo
		} else if hasAddManaTrigger {
			b = addManaTrigger
		} else if hasAnyTrigger {
			b = anyTrigger
		} else if hasUnparsedRestriction {
			b = unparsedRestrictions
		}
		b.count++
		if len(b.cards) < 10 {
			b.cards = append(b.cards, c.Name)
		}
	}

	buckets := []*bucket{onlyInfo, addManaTrigger, anyTrigger, unparsedRestrictions, clean}
	for _, b := range buckets {
		fmt.Printf("%s: %d cards", b.name, b.count)
		if len(b.cards) > 0 {
			fmt.Printf(" (e.g. %s)", strings.Join(b.cards, ", "))
		}
		fmt.Println()
	}
}
