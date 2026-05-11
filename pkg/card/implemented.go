package card

import (
	"sort"
	"strings"
)

// ImplementationEvaluator can test whether a single card is fully implemented.
type ImplementationEvaluator interface {
	EvaluateCard(c Card) (implemented bool, reason string)
}

// Bucket holds aggregate counts for one group (color, set, or type).
type Bucket struct {
	Name        string  `json:"name"`
	Total       int     `json:"total"`
	Implemented int     `json:"implemented"`
	Percentage  float64 `json:"percentage"`
}

// UnimplementedCard is a single unimplemented entry for the table view.
type UnimplementedCard struct {
	Name   string `json:"name"`
	Set    string `json:"set"`
	Colors string `json:"colors"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// ImplementationReport is the complete classification snapshot.
type ImplementationReport struct {
	TotalCards         int                 `json:"total_cards"`
	ImplementedCount   int                 `json:"implemented_count"`
	UnimplementedCount int                 `json:"unimplemented_count"`
	Percentage         float64             `json:"percentage"`
	ByColor            []Bucket            `json:"by_color"`
	BySet              []Bucket            `json:"by_set"`
	ByType             []Bucket            `json:"by_type"`
	UnimplementedCards []UnimplementedCard `json:"unimplemented_cards"`
}

// ComputeImplementationStatus evaluates every card in the database and
// produces a report with per-color, per-set, and per-type breakdowns.
func ComputeImplementationStatus(db *CardDB, evaluator ImplementationEvaluator) (*ImplementationReport, error) {
	all := db.ListAll()
	report := &ImplementationReport{}

	colorBuckets := map[string]*Bucket{}
	setBuckets := map[string]*Bucket{}
	setCounts := map[string]int{}
	typeBuckets := map[string]*Bucket{}

	for _, c := range all {
		impl, reason := evaluator.EvaluateCard(c)
		report.TotalCards++
		if impl {
			report.ImplementedCount++
		} else {
			report.UnimplementedCount++
			report.UnimplementedCards = append(report.UnimplementedCards, UnimplementedCard{
				Name:   c.Name,
				Set:    c.Set,
				Colors: colorString(c.ColorIdentity),
				Type:   simplifyType(c.TypeLine),
				Reason: reason,
			})
		}

		cb := classifyColor(c.ColorIdentity)
		if colorBuckets[cb] == nil {
			colorBuckets[cb] = &Bucket{Name: cb}
		}
		colorBuckets[cb].Total++
		if impl {
			colorBuckets[cb].Implemented++
		}

		if setBuckets[c.Set] == nil {
			setBuckets[c.Set] = &Bucket{Name: c.Set}
		}
		setBuckets[c.Set].Total++
		setCounts[c.Set]++
		if impl {
			setBuckets[c.Set].Implemented++
		}

		tb := simplifyType(c.TypeLine)
		if typeBuckets[tb] == nil {
			typeBuckets[tb] = &Bucket{Name: tb}
		}
		typeBuckets[tb].Total++
		if impl {
			typeBuckets[tb].Implemented++
		}
	}

	if report.TotalCards > 0 {
		report.Percentage = float64(report.ImplementedCount) / float64(report.TotalCards) * 100
	}

	sort.Slice(report.UnimplementedCards, func(i, j int) bool {
		return report.UnimplementedCards[i].Name < report.UnimplementedCards[j].Name
	})

	// Color buckets — fixed order.
	colorOrder := []string{"W", "U", "B", "R", "G", "Multicolor", "Colorless"}
	for _, name := range colorOrder {
		if b, ok := colorBuckets[name]; ok {
			b.Percentage = safePct(b.Implemented, b.Total)
			report.ByColor = append(report.ByColor, *b)
		}
	}

	// Set buckets — top 10 most-represented sets + Other.
	type setCount struct {
		name  string
		count int
	}
	var scs []setCount
	for name, count := range setCounts {
		scs = append(scs, setCount{name, count})
	}
	sort.Slice(scs, func(i, j int) bool {
		return scs[i].count > scs[j].count
	})

	topSetNames := map[string]bool{}
	for i := 0; i < 10 && i < len(scs); i++ {
		topSetNames[scs[i].name] = true
	}

	other := &Bucket{Name: "Other"}
	for name, b := range setBuckets {
		b.Percentage = safePct(b.Implemented, b.Total)
		if topSetNames[name] {
			report.BySet = append(report.BySet, *b)
		} else {
			other.Total += b.Total
			other.Implemented += b.Implemented
		}
	}
	if other.Total > 0 {
		other.Percentage = safePct(other.Implemented, other.Total)
		report.BySet = append(report.BySet, *other)
	}
	sort.Slice(report.BySet, func(i, j int) bool {
		if report.BySet[i].Name == "Other" {
			return false
		}
		if report.BySet[j].Name == "Other" {
			return true
		}
		return report.BySet[i].Total > report.BySet[j].Total
	})

	// Type buckets — fixed order.
	typeOrder := []string{"Land", "Creature", "Instant", "Sorcery", "Artifact", "Enchantment", "Planeswalker", "Other"}
	for _, name := range typeOrder {
		if b, ok := typeBuckets[name]; ok {
			b.Percentage = safePct(b.Implemented, b.Total)
			report.ByType = append(report.ByType, *b)
		}
	}

	return report, nil
}

func safePct(impl, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(impl) / float64(total) * 100
}

func classifyColor(colors []string) string {
	if len(colors) == 0 {
		return "Colorless"
	}
	if len(colors) > 1 {
		return "Multicolor"
	}
	switch colors[0] {
	case "W":
		return "W"
	case "U":
		return "U"
	case "B":
		return "B"
	case "R":
		return "R"
	case "G":
		return "G"
	default:
		return "Colorless"
	}
}

func colorString(colors []string) string {
	order := []string{"W", "U", "B", "R", "G"}
	present := map[string]bool{}
	for _, c := range colors {
		present[c] = true
	}
	var out []string
	for _, c := range order {
		if present[c] {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "C"
	}
	return strings.Join(out, "")
}

func simplifyType(typeLine string) string {
	switch {
	case strings.Contains(typeLine, "Creature"):
		return "Creature"
	case strings.Contains(typeLine, "Instant"):
		return "Instant"
	case strings.Contains(typeLine, "Sorcery"):
		return "Sorcery"
	case strings.Contains(typeLine, "Artifact"):
		return "Artifact"
	case strings.Contains(typeLine, "Enchantment"):
		return "Enchantment"
	case strings.Contains(typeLine, "Planeswalker"):
		return "Planeswalker"
	case strings.Contains(typeLine, "Land"):
		return "Land"
	default:
		return "Other"
	}
}
