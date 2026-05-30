package main

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	input := "Authorization: Bearer sk-testsecret123456 api_key=plain password: hunter2 ghp_abcdefghijklmnopqrstuvwxyz123456"
	output := redactSecrets(input)
	for _, forbidden := range []string{"sk-testsecret123456", "plain", "hunter2", "ghp_abcdefghijklmnopqrstuvwxyz123456"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("redactSecrets leaked %q in %q", forbidden, output)
		}
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("redactSecrets output missing marker: %q", output)
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote([]string{"go", "test", "./...", "name with spaces"})
	want := "go test ./... 'name with spaces'"
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

func TestRenderMarkdownIncludesReleaseEvidence(t *testing.T) {
	doc := renderMarkdown(handoffData{
		Git: gitInfo{
			Branch:      "codex/p10-release-handoff",
			Commit:      "abc1234",
			CommitTitle: "feat(release): add handoff",
			Clean:       true,
		},
		LatestMigration: "000010_p7_media_forwarding.up.sql",
		Checks: []checkResult{{
			Name:    "unit and integration tests",
			Command: "go test ./...",
			Passed:  true,
		}},
	})
	for _, required := range []string{
		"# Release Handoff",
		"codex/p10-release-handoff",
		"000010_p7_media_forwarding.up.sql",
		"rc_smoke=portal_customer_acceptance",
		"Portal smoke result",
		"/configd/snapshots/rollback",
	} {
		if !strings.Contains(doc, required) {
			t.Fatalf("rendered handoff missing %q:\n%s", required, doc)
		}
	}
}
