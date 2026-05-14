package ability

var executableEffectTypes = map[EffectType]bool{
	DrawCards: true, DealDamage: true, GainLife: true, LoseLife: true, AddMana: true,
	PumpCreature: true, DestroyPermanent: true, CounterSpell: true, TapUntap: true,
	ChangeControl: true, ReturnToHand: true, SourcePowerDamage: true, DiscardCards: true,
	SearchLibrary: true, CreateToken: true, PreventDamage: true, KeywordAbility: true,
	ChooseMode: true, TakeExtraTurn: true, Exile: true, MillCards: true, ScryCards: true,
	AddCounters: true, UntapPermanent: true, CopySpell: true, CantAttackBlock: true,
	AdditionalLand: true, SacrificePermanent: true, ReanimateCreature: true,
	WinGame: true, LoseGame: true,
}

var approximateRuntimeReasons = map[EffectType]string{
	ChooseMode:        "modal choices are parsed but not executed",
	TakeExtraTurn:     "extra turns are parsed but not queued",
	CopySpell:         "copy effects are parsed but not created",
	CantAttackBlock:   "attack/block restrictions are parsed but not enforced",
	AdditionalLand:    "additional land allowances are parsed but not tracked",
	ReanimateCreature: "reanimation uses an approximate placeholder creature",
}
