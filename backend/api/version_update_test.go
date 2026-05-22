package api

import "testing"

func TestCompareSemanticVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    int
	}{
		{name: "same with v prefix", current: "v0.0.6", latest: "0.0.6", want: 0},
		{name: "patch update", current: "0.0.6", latest: "0.0.7", want: -1},
		{name: "numeric compare", current: "0.0.10", latest: "0.0.9", want: 1},
		{name: "minor update", current: "0.1.0", latest: "0.2.0", want: -1},
		{name: "dev current is treated as older", current: "dev", latest: "0.0.1", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareSemanticVersions(tt.current, tt.latest)
			if got < 0 {
				got = -1
			} else if got > 0 {
				got = 1
			}
			if got != tt.want {
				t.Fatalf("compareSemanticVersions(%q, %q) = %d, want %d", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	if got := normalizeVersion("refs/tags/v0.0.6"); got != "0.0.6" {
		t.Fatalf("normalizeVersion() = %q, want 0.0.6", got)
	}
}
