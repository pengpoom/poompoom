package cliproxy

import "testing"

func TestConfiguredRequiresEnabledFlag(t *testing.T) {
	client := New(false, "http://127.0.0.1:8317", "secret", "codex", 0)
	if client.Configured() {
		t.Fatal("expected disabled sync client to be unconfigured")
	}

	client = New(true, "http://127.0.0.1:8317", "secret", "codex", 0)
	if !client.Configured() {
		t.Fatal("expected enabled sync client with credentials to be configured")
	}
}

func TestConfiguredTreatsExampleManagementKeyAsUnset(t *testing.T) {
	client := New(true, "http://127.0.0.1:8317", "your-cliproxy-management-key", "codex", 0)
	if client.Configured() {
		t.Fatal("expected example placeholder key to be treated as unconfigured")
	}
}
