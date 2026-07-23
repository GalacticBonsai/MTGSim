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
	KWFlash
	KWShroud
	KWWard
	KWProtection
	KWPhasing
	KWProwess
	KWIntimidate
	KWShadow

	// --- Non-evergreen keywords (CR 702) ---
	KWKicker         // 702.33
	KWStorm          // 702.36
	KWCascade        // 702.41
	KWSuspend        // 702.42
	KWFlashback      // 702.45
	KWMadness        // 702.47
	KWDredge         // 702.50
	KWEscape         // 702.52
	KWEcho           // 702.5
	KWAffinity       // 702.40
	KWBanding        // 702.42 (removed from modern, but keeps legacy
	KWFear           // 702.37 (deprecated, replaced by intimidate/menace)
	KWFlanking       // 702.43
	KWThreshold      // 702.51
	KWDelve          // 702.52
	KWImprovise      // 702.53
	KWAftermath      // 702.54
	KWEmbalm         // 702.55
	KWEternalize     // 702.56
	KWPrototype      // 702.57
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
	case KWFlash:
		return "flash"
	case KWShroud:
		return "shroud"
	case KWWard:
		return "ward"
	case KWProtection:
		return "protection"
	case KWPhasing:
		return "phasing"
	case KWProwess:
		return "prowess"
	case KWIntimidate:
		return "intimidate"
	case KWShadow:
		return "shadow"
	case KWKicker:
		return "kicker"
	case KWStorm:
		return "storm"
	case KWCascade:
		return "cascade"
	case KWSuspend:
		return "suspend"
	case KWFlashback:
		return "flashback"
	case KWMadness:
		return "madness"
	case KWDredge:
		return "dredge"
	case KWEscape:
		return "escape"
	case KWEcho:
		return "echo"
	case KWAffinity:
		return "affinity"
	case KWBanding:
		return "banding"
	case KWFear:
		return "fear"
	case KWFlanking:
		return "flanking"
	case KWThreshold:
		return "threshold"
	case KWDelve:
		return "delve"
	case KWImprovise:
		return "improvise"
	case KWAftermath:
		return "aftermath"
	case KWEmbalm:
		return "embalm"
	case KWEternalize:
		return "eternalize"
	case KWPrototype:
		return "prototype"
	default:
		return "unknown"
	}
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

// HasKeywordByName allows checking keywords by string name, used by the
// ability package via the KeywordChecker interface to avoid import cycles.
func (p *Permanent) HasKeywordByName(name string) bool {
	if p == nil {
		return false
	}
	lower := strings.ToLower(name)
	// Check printed keywords
	for k, v := range p.printedKeywords {
		if v && strings.ToLower(keywordName(k)) == lower {
			return true
		}
	}
	// Check granted keywords
	for k, v := range p.grantedKeywords {
		if v && strings.ToLower(keywordName(k)) == lower {
			return true
		}
	}
	return false
}

func keywordName(k Keyword) string {
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
	case KWFlash:
		return "flash"
	case KWShroud:
		return "shroud"
	case KWWard:
		return "ward"
	case KWProtection:
		return "protection"
	case KWPhasing:
		return "phasing"
	case KWProwess:
		return "prowess"
	case KWIntimidate:
		return "intimidate"
	case KWShadow:
		return "shadow"
	case KWKicker:
		return "kicker"
	case KWStorm:
		return "storm"
	case KWCascade:
		return "cascade"
	case KWSuspend:
		return "suspend"
	case KWFlashback:
		return "flashback"
	case KWMadness:
		return "madness"
	case KWDredge:
		return "dredge"
	case KWEscape:
		return "escape"
	case KWEcho:
		return "echo"
	case KWAffinity:
		return "affinity"
	case KWBanding:
		return "banding"
	case KWFear:
		return "fear"
	case KWFlanking:
		return "flanking"
	case KWThreshold:
		return "threshold"
	case KWDelve:
		return "delve"
	case KWImprovise:
		return "improvise"
	case KWAftermath:
		return "aftermath"
	case KWEmbalm:
		return "embalm"
	case KWEternalize:
		return "eternalize"
	case KWPrototype:
		return "prototype"
	default:
		return "unknown"
	}
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
		// Split on commas — each token is checked independently for keyword
		// matching. This handles lines like "Trample, hexproof, can't be
		// countered" where only some tokens are evergreen keywords.
		parts := strings.Split(line, ",")
		for _, raw := range parts {
			tok := strings.TrimSpace(raw)
			k, ok := keywordFromToken(tok)
			if ok {
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
	case "flash":
		return KWFlash, true
	case "shroud":
		return KWShroud, true
	case "prowess":
		return KWProwess, true
	case "intimidate":
		return KWIntimidate, true
	case "shadow":
		return KWShadow, true
	case "phasing":
		return KWPhasing, true
	case "kicker":
		return KWKicker, true
	case "storm":
		return KWStorm, true
	case "cascade":
		return KWCascade, true
	case "suspend":
		return KWSuspend, true
	case "flashback":
		return KWFlashback, true
	case "madness":
		return KWMadness, true
	case "dredge":
		return KWDredge, true
	case "escape":
		return KWEscape, true
	case "echo":
		return KWEcho, true
	case "affinity":
		return KWAffinity, true
	case "banding":
		return KWBanding, true
	case "fear":
		return KWFear, true
	case "flanking":
		return KWFlanking, true
	case "threshold":
		return KWThreshold, true
	case "delve":
		return KWDelve, true
	case "improvise":
		return KWImprovise, true
	case "aftermath":
		return KWAftermath, true
	case "embalm":
		return KWEmbalm, true
	case "eternalize":
		return KWEternalize, true
	case "prototype":
		return KWPrototype, true
	}
	// Ward has a cost suffix: "ward {2}", "ward {U}", etc.
	if strings.HasPrefix(tok, "ward") {
		return KWWard, true
	}
	// Protection from X
	if strings.HasPrefix(tok, "protection from") {
		return KWProtection, true
	}
	return 0, false
}
