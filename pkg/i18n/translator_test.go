package i18n

import (
	"context"
	"testing"
)

// CONST-050(A): mocks/fakes permitted in unit tests only.

func TestNoopTranslator_T_ReturnsMessageIDVerbatim(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(
		context.Background(),
		"some.message.id",
		map[string]any{"x": 1},
	)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "some.message.id" {
		t.Fatalf(
			"NoopTranslator.T = %q, want %q",
			got, "some.message.id",
		)
	}
}

func TestNoopTranslator_TPlural_ReturnsMessageIDVerbatim(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(
		context.Background(),
		"plural.message.id",
		7,
		map[string]any{"y": "z"},
	)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "plural.message.id" {
		t.Fatalf(
			"NoopTranslator.TPlural = %q, want %q",
			got, "plural.message.id",
		)
	}
}

func TestSetPkgTranslator_AndPkg(t *testing.T) {
	defer SetPkgTranslator(nil) // restore default
	// Default is Noop
	if _, ok := Pkg().(NoopTranslator); !ok {
		t.Fatalf("default Pkg() not NoopTranslator")
	}
	// Install a fake and verify
	fake := &fakeTranslator{out: "FAKE_OUT"}
	SetPkgTranslator(fake)
	got, _ := Pkg().T(
		context.Background(), "anything", nil,
	)
	if got != "FAKE_OUT" {
		t.Fatalf("Pkg().T = %q, want FAKE_OUT", got)
	}
	// Nil resets to Noop
	SetPkgTranslator(nil)
	if _, ok := Pkg().(NoopTranslator); !ok {
		t.Fatalf("Pkg() not reset to NoopTranslator on nil")
	}
}

// fakeTranslator is a CONST-050(A)-permitted unit-test mock.
type fakeTranslator struct {
	out string
}

func (f *fakeTranslator) T(
	_ context.Context,
	_ string,
	_ map[string]any,
) (string, error) {
	return f.out, nil
}

func (f *fakeTranslator) TPlural(
	_ context.Context,
	_ string,
	_ int,
	_ map[string]any,
) (string, error) {
	return f.out, nil
}
