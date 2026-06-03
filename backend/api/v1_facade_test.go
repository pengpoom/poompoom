package api

import "testing"

func TestToPublicStatus(t *testing.T) {
	cases := map[string]string{
		"queued":           "queued",
		"running":          "running",
		"cancel_requested": "running",
		"cancelled":        "cancelled",
		"succeeded":        "succeeded",
		"failed":           "failed",
		"weird":            "running",
	}
	for in, want := range cases {
		if got := toPublicStatus(in); got != want {
			t.Errorf("toPublicStatus(%q)=%q want %q", in, got, want)
		}
	}
}
