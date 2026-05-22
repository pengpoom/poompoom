package buildinfo

import "testing"

func TestResolveVersionValuePrefersSemanticBuildVersion(t *testing.T) {
	got := resolveVersionValue("v1.2.10", "1.2.10", "v1.2.10-2-gabc123")
	if got != "v1.2.10" {
		t.Fatalf("resolveVersionValue() = %q, want %q", got, "v1.2.10")
	}
}

func TestResolveVersionValueFallsBackFromBranchBuildLabel(t *testing.T) {
	got := resolveVersionValue("main-f91d41d", "1.2.10", "")
	if got != "main-f91d41d" {
		t.Fatalf("resolveVersionValue() = %q, want %q", got, "main-f91d41d")
	}
}

func TestResolveVersionValueUsesGitDescribeForDevBuilds(t *testing.T) {
	got := resolveVersionValue("dev", "1.2.10", "v1.2.10-3-gabc123-dirty")
	if got != "v1.2.10-3-gabc123-dirty" {
		t.Fatalf("resolveVersionValue() = %q, want %q", got, "v1.2.10-3-gabc123-dirty")
	}
}

func TestResolveVersionValueReturnsRawVersionWhenNoBetterSourceExists(t *testing.T) {
	got := resolveVersionValue("main-f91d41d", "", "")
	if got != "main-f91d41d" {
		t.Fatalf("resolveVersionValue() = %q, want %q", got, "main-f91d41d")
	}
}
