package mix

import (
	"fmt"
	"testing"
)

type testStringer struct{ v string }

func (s testStringer) String() string { return "str:" + s.v }

func TestSafeStringer(t *testing.T) {
	if safeStringer(nil) != "" {
		t.Fatalf("nil -> %q", safeStringer(nil))
	}
	if got := safeStringer(testStringer{v: "x"}); got != "str:x" {
		t.Fatalf("Stringer -> %q", got)
	}
	if got := safeStringer(123); got != "123" {
		t.Fatalf("int -> %q", got)
	}
	if got := safeStringer(map[string]int{"a": 1}); got != fmt.Sprintf("%v", map[string]int{"a": 1}) {
		t.Fatalf("map -> %q", got)
	}
}
