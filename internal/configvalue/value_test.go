package configvalue

import (
	"encoding/json"
	"fmt"
	"testing"
)

type stringerValue struct {
	value string
}

func (v stringerValue) String() string {
	return v.value
}

func TestMapAcceptsMapsAndStructs(t *testing.T) {
	if got := Map(map[string]any{"name": "direct"})["name"]; got != "direct" {
		t.Fatalf("direct map value = %v, want direct", got)
	}

	converted := Map(struct {
		Name string `json:"name"`
	}{Name: "encoded"})
	if got := converted["name"]; got != "encoded" {
		t.Fatalf("encoded map value = %v, want encoded", got)
	}
}

func TestStringPreservesDynamicValues(t *testing.T) {
	if got := String(stringerValue{"named"}, "fallback"); got != "named" {
		t.Fatalf("String(Stringer) = %q, want named", got)
	}
	if got := String(42, "fallback"); got != "42" {
		t.Fatalf("String(number) = %q, want 42", got)
	}
	if got := String(nil, "fallback"); got != "fallback" {
		t.Fatalf("String(nil) = %q, want fallback", got)
	}
}

func TestNumericAndBoolValues(t *testing.T) {
	if got := Bool("y", false); !got {
		t.Fatal("Bool(y) = false, want true")
	}
	if got := Bool(json.Number("0"), true); got {
		t.Fatal("Bool(json.Number(0)) = true, want false")
	}
	if got := Int(" 123 ", 0); got != 123 {
		t.Fatalf("Int(string) = %d, want 123", got)
	}
	if got := Float(fmt.Stringer(stringerValue{"not-used"}), 7); got != 7 {
		t.Fatalf("Float(unsupported) = %f, want fallback 7", got)
	}
	if got := Float("1.25", 0); got != 1.25 {
		t.Fatalf("Float(string) = %f, want 1.25", got)
	}
}
