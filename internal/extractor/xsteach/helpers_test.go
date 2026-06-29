package xsteach

import "testing"

func TestNormalizeURLInvalidRelativeReturnsOriginal(t *testing.T) {
	got := normalizeURL("/%zz")
	if got != "/%zz" {
		t.Fatalf("normalizeURL(%q) = %q, want %q", "/%zz", got, "/%zz")
	}
}
