package slogx

import "testing"

func TestCanonicalLogNameIsTaple(t *testing.T) {
	if LogFileName != "taple.log" {
		t.Fatalf("unexpected log filename: %s", LogFileName)
	}
}
