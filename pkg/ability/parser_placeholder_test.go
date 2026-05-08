package ability

import "testing"

func TestAbilityParser_ModalAndExtraTurnUseExplicitEffectTypes(t *testing.T) {
	parser := NewAbilityParser()
	tests := []struct {
		name       string
		oracleText string
		want       EffectType
	}{
		{name: "modal", oracleText: "Choose one — Draw a card.", want: ChooseMode},
		{name: "extra turn", oracleText: "Take an extra turn after this one.", want: TakeExtraTurn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abilities, err := parser.ParseAbilities(tt.oracleText, nil)
			if err != nil {
				t.Fatalf("ParseAbilities: %v", err)
			}
			if len(abilities) != 1 || len(abilities[0].Effects) != 1 {
				t.Fatalf("expected one ability/effect, got %+v", abilities)
			}
			if got := abilities[0].Effects[0].Type; got != tt.want {
				t.Fatalf("effect type = %v, want %v", got, tt.want)
			}
		})
	}
}
