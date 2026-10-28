package fuzz

import "testing"

func TestFuzzEntrypoint(t *testing.T) {
	if got := Fuzz([]byte("div.item")); got != 1 {
		t.Fatalf("valid selector returned %d", got)
	}
	if got := Fuzz([]byte("div[")); got != 0 {
		t.Fatalf("invalid selector returned %d", got)
	}
}
