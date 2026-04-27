package ability

import "testing"

// Compile-time / behavioural checks for the new effect interfaces in
// types.go. These do not exercise the full game loop; they verify the
// interface shapes match the XMage-style contract so future card
// implementations can rely on them.

type fakeOneShot struct{ desc string }

func (f *fakeOneShot) Kind() EffectKind  { return EffectKindOneShot }
func (f *fakeOneShot) Description() string { return f.desc }
func (f *fakeOneShot) Apply(g any, source any, targets []any) error {
	return nil
}

type fakeContinuous struct {
	layer, sub int
	done       bool
}

func (c *fakeContinuous) Kind() EffectKind   { return EffectKindContinuous }
func (c *fakeContinuous) Description() string { return "anthem" }
func (c *fakeContinuous) Layer() int           { return c.layer }
func (c *fakeContinuous) Sublayer() int        { return c.sub }
func (c *fakeContinuous) ApplyView(_ any, _ any) {}
func (c *fakeContinuous) Affects(_ any) bool   { return true }
func (c *fakeContinuous) Discard() bool         { return c.done }

type fakeReplacement struct{}

func (r *fakeReplacement) Kind() EffectKind   { return EffectKindReplacement }
func (r *fakeReplacement) Description() string { return "would die -> exile" }
func (r *fakeReplacement) Replaces(_ any) bool { return true }
func (r *fakeReplacement) Replace(e any) []any { return []any{e} }

type fakeTrigger struct{ fired bool }

func (t *fakeTrigger) Triggers(_ any) bool { return true }
func (t *fakeTrigger) Resolve(_ any, _ any, _ []any) error {
	t.fired = true
	return nil
}
func (t *fakeTrigger) Description() string { return "ETB draw" }

func TestEffectInterfaces_OneShot(t *testing.T) {
	var e OneShotEffect = &fakeOneShot{desc: "draw a card"}
	if e.Kind() != EffectKindOneShot {
		t.Fatalf("expected one-shot kind")
	}
	if err := e.Apply(nil, nil, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if e.Description() != "draw a card" {
		t.Fatalf("desc mismatch: %q", e.Description())
	}
}

func TestEffectInterfaces_Continuous(t *testing.T) {
	c := &fakeContinuous{layer: 7, sub: 3}
	var e ContinuousEffect = c
	if e.Kind() != EffectKindContinuous || e.Layer() != 7 || e.Sublayer() != 3 {
		t.Fatalf("continuous interface wiring wrong")
	}
	if e.Discard() {
		t.Fatalf("should not be discarded yet")
	}
	c.done = true
	if !e.Discard() {
		t.Fatalf("should be discarded after flip")
	}
}

func TestEffectInterfaces_Replacement(t *testing.T) {
	var e ReplacementEffect = &fakeReplacement{}
	if e.Kind() != EffectKindReplacement {
		t.Fatalf("expected replacement kind")
	}
	if !e.Replaces("anything") {
		t.Fatalf("expected replaces=true")
	}
	out := e.Replace("evt")
	if len(out) != 1 || out[0].(string) != "evt" {
		t.Fatalf("unexpected replace result: %#v", out)
	}
}

func TestEffectInterfaces_Triggered(t *testing.T) {
	tr := &fakeTrigger{}
	var ta TriggeredAbility = tr
	if !ta.Triggers(nil) {
		t.Fatalf("expected to trigger")
	}
	_ = ta.Resolve(nil, nil, nil)
	if !tr.fired {
		t.Fatalf("resolve did not fire")
	}
}
