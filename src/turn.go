package main

type step struct {
	name string
}

type phase struct {
	name  string
	steps []step
}

type turn struct {
	phases      []phase
	currentPhase int
	landPerTurn int
}

func newTurn() *turn {
	return &turn{
		phases:      turnOrder,
		currentPhase: 0,
		landPerTurn: 1,
	}
}

var turnOrder = []phase{
	{
		name: "Beginning Phase",
		steps: []step{
			{name: "Untap Step"},
			{name: "Upkeep Step"},
			{name: "Draw Step"},
		},
	},
	{
		name: "Main Phase 1",
		steps: []step{
			{name: "Play Land"},
			{name: "Cast Spells"},
		},
	},
	{
		name: "Combat Phase",
		steps: []step{
			{name: "Beginning of Combat Step"},
			{name: "Declare Attackers Step"},
			{name: "Declare Blockers Step"},
			{name: "Combat Damage Step"},
			{name: "End of Combat Step"},
		},
	},
	{
		name: "Main Phase 2",
		steps: []step{
			{name: "Play Land"},
			{name: "Cast Spells"},
		},
	},
	{
		name: "End Phase",
		steps: []step{
			{name: "End Step"},
			{name: "Cleanup Step"},
		},
	},
}

func (s *step) next(p *phase) *step {
	for i, st := range p.steps {
		if st.name == s.name && i < len(p.steps)-1 {
			return &p.steps[i+1]
		}
	}
	return nil
}

func (p *phase) next() *phase {
	for i, ph := range turnOrder {
		if ph.name == p.name && i < len(turnOrder)-1 {
			return &turnOrder[i+1]
		}
	}
	return nil
}
