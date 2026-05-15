package ability

var approximateRuntimeReasons = map[EffectType]string{
	ChooseMode:        "modal choices are parsed but not executed",
	TakeExtraTurn:     "extra turns are parsed but not queued",
	CopySpell:         "copy effects are parsed but not created",
	CantAttackBlock:   "attack/block restrictions are parsed but not enforced",
	AdditionalLand:    "additional land allowances are parsed but not tracked",
	ReanimateCreature: "reanimation uses an approximate placeholder creature",
}
