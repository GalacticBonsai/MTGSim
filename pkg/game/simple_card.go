package game

// SimpleCard is a minimal in-engine card representation to avoid package cycles.
type SimpleCard struct {
	Name       string
	TypeLine   string
	Power      string
	Toughness  string
	OracleText string
	Colors     []string
}

func (c SimpleCard) IsLand() bool     { return contains(c.TypeLine, "Land") }
func (c SimpleCard) IsCreature() bool { return contains(c.TypeLine, "Creature") }
func (c SimpleCard) IsAura() bool     { return contains(c.TypeLine, "Aura") }

// contains is a simple substring checker (ASCII)
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
