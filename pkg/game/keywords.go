package game

import "strings"

// Keyword identifies an evergreen keyword ability tracked on a Permanent.
// Only the subset relevant to combat/SBA evaluation is modelled here; more
// can be added without API churn since the storage is a bitmap.
type Keyword int

const (
	KWFlying Keyword = iota
	KWReach
	KWTrample
	KWLifelink
	KWDeathtouch
	KWHaste
	KWVigilance
	KWIndestructible
	KWHexproof
	KWMenace
	KWDefender
	KWFirstStrike
	KWDoubleStrike
)

func (k Keyword) String() string {
	switch k {
	case KWFlying:
		return "flying"
	case KWReach:
		return "reach"
	case KWTrample:
		return "trample"
	case KWLifelink:
		return "lifelink"
	case KWDeathtouch:
		return "deathtouch"
	case KWHaste:
		return "haste"
	case KWVigilance:
		return "vigilance"
	case KWIndestructible:
		return "indestructible"
	case KWHexproof:
		return "hexproof"
	case KWMenace:
		return "menace"
	case KWDefender:
		return "defender"
	case KWFirstStrike:
		return "first strike"
	case KWDoubleStrike:
		return "double strike"
	}
	return "unknown"
}

// HasKeyword reports whether the permanent currently has a keyword ability,
// considering both the printed flags (parsed from oracle text at creation)
// and any granted-by-effect flags.
func (p *Permanent) HasKeyword(k Keyword) bool {
	if p == nil {
		return false
	}
	if p.printedKeywords[k] {
		return true
	}
	return p.grantedKeywords[k]
}

// SetKeyword sets a printed keyword flag on the permanent. Used by ETB
// initialization and tests.
func (p *Permanent) SetKeyword(k Keyword, v bool) {
	if p == nil {
		return
	}
	if p.printedKeywords == nil {
		p.printedKeywords = map[Keyword]bool{}
	}
	p.printedKeywords[k] = v
	// Mirror legacy first/double-strike fields so existing combat logic stays consistent.
	switch k {
	case KWFirstStrike:
		p.firstStrike = v
	case KWDoubleStrike:
		p.doubleStrike = v
	}
}

// GrantKeyword adds a granted keyword (e.g. by an Aura or Layer 6 effect).
func (p *Permanent) GrantKeyword(k Keyword) {
	if p == nil {
		return
	}
	if p.grantedKeywords == nil {
		p.grantedKeywords = map[Keyword]bool{}
	}
	p.grantedKeywords[k] = true
}

// RevokeKeyword removes a granted keyword.
func (p *Permanent) RevokeKeyword(k Keyword) {
	if p == nil || p.grantedKeywords == nil {
		return
	}
	delete(p.grantedKeywords, k)
}

// parseKeywordsFromOracle inspects oracle text for the evergreen keyword
// list and returns the printed-keyword set. The matching is conservative:
// we only flag a keyword when the bare word appears as a simple ability
// (a comma-separated keyword line) so prose like "Whenever ~ deals damage,
// gain life" does not accidentally grant lifelink.
func parseKeywordsFromOracle(oracle string) map[Keyword]bool {
	out := map[Keyword]bool{}
	if oracle == "" {
		return out
	}
	low := strings.ToLower(oracle)
	for _, line := range strings.Split(low, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split on commas — a keyword line is a comma-separated list of
		// single keywords. Anything else (full sentence) is ignored.
		parts := strings.Split(line, ",")
		allKeywords := true
		matched := []Keyword{}
		for _, raw := range parts {
			tok := strings.TrimSpace(raw)
			k, ok := keywordFromToken(tok)
			if !ok {
				allKeywords = false
				break
			}
			matched = append(matched, k)
		}
		if allKeywords {
			for _, k := range matched {
				out[k] = true
			}
		}
	}
	return out
}

func keywordFromToken(tok string) (Keyword, bool) {
	switch tok {
	case "flying":
		return KWFlying, true
	case "reach":
		return KWReach, true
	case "trample":
		return KWTrample, true
	case "lifelink":
		return KWLifelink, true
	case "deathtouch":
		return KWDeathtouch, true
	case "haste":
		return KWHaste, true
	case "vigilance":
		return KWVigilance, true
	case "indestructible":
		return KWIndestructible, true
	case "hexproof":
		return KWHexproof, true
	case "menace":
		return KWMenace, true
	case "defender":
		return KWDefender, true
	case "first strike":
		return KWFirstStrike, true
	case "double strike":
		return KWDoubleStrike, true
	}
	return 0, false
}
