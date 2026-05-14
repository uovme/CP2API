package admin

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestParseCLIProxyAuthImportEntriesSupportsObjectArrayAndJSONLines(t *testing.T) {
	req := CLIProxyAuthImportRequest{
		Content: strings.Join([]string{
			`{"type":"claude","email":"claude@example.com","access_token":"claude-at","refresh_token":"claude-rt","expired":"2026-08-05T13:40:42Z"}`,
			`[{"type":"gemini","email":"gemini@example.com","project_id":"proj-1","token":{"access_token":"gemini-at","refresh_token":"gemini-rt","expiry":"2026-08-05T13:40:42Z"}}]`,
		}, "\n"),
	}

	entries, err := parseCLIProxyAuthImportEntries(req)
	if err != nil {
		t.Fatalf("parseCLIProxyAuthImportEntries error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	first, err := normalizeCLIProxyAuthImportEntry(entries[0])
	if err != nil {
		t.Fatalf("normalize first entry error = %v", err)
	}
	if first.Platform != service.PlatformAnthropic {
		t.Fatalf("first platform = %q, want %q", first.Platform, service.PlatformAnthropic)
	}
	if first.Credentials["access_token"] != "claude-at" || first.Credentials["refresh_token"] != "claude-rt" {
		t.Fatalf("first credentials = %#v", first.Credentials)
	}
	if first.Credentials["expires_at"] != "2026-08-05T13:40:42Z" {
		t.Fatalf("first expires_at = %v", first.Credentials["expires_at"])
	}

	second, err := normalizeCLIProxyAuthImportEntry(entries[1])
	if err != nil {
		t.Fatalf("normalize second entry error = %v", err)
	}
	if second.Platform != service.PlatformGemini {
		t.Fatalf("second platform = %q, want %q", second.Platform, service.PlatformGemini)
	}
	if second.Credentials["project_id"] != "proj-1" {
		t.Fatalf("project_id = %v, want proj-1", second.Credentials["project_id"])
	}
	if second.Credentials["oauth_type"] != "code_assist" {
		t.Fatalf("oauth_type = %v, want code_assist", second.Credentials["oauth_type"])
	}
	if second.Credentials["access_token"] != "gemini-at" || second.Credentials["refresh_token"] != "gemini-rt" {
		t.Fatalf("second credentials = %#v", second.Credentials)
	}
}

func TestNormalizeCLIProxyAuthImportEntrySupportsSub2APIAccountShape(t *testing.T) {
	entry := cliproxyAuthImportEntry{
		Index: 1,
		Value: map[string]any{
			"name":     "Existing shape",
			"platform": service.PlatformOpenAI,
			"type":     service.AccountTypeOAuth,
			"credentials": map[string]any{
				"access_token":  "sub-at",
				"refresh_token": "sub-rt",
			},
			"extra": map[string]any{"team": "core"},
		},
	}

	item, err := normalizeCLIProxyAuthImportEntry(entry)
	if err != nil {
		t.Fatalf("normalizeCLIProxyAuthImportEntry error = %v", err)
	}
	if item.Name != "Existing shape" {
		t.Fatalf("name = %q, want Existing shape", item.Name)
	}
	if item.Platform != service.PlatformOpenAI || item.Type != service.AccountTypeOAuth {
		t.Fatalf("platform/type = %q/%q", item.Platform, item.Type)
	}
	if item.Credentials["access_token"] != "sub-at" || item.Credentials["refresh_token"] != "sub-rt" {
		t.Fatalf("credentials = %#v", item.Credentials)
	}
	if item.Extra["team"] != "core" {
		t.Fatalf("extra = %#v", item.Extra)
	}
}

func TestNormalizeCLIProxyAuthImportEntryMapsCodexToOpenAIOAuth(t *testing.T) {
	entry := cliproxyAuthImportEntry{
		Index: 1,
		Value: map[string]any{
			"type":          "codex",
			"email":         "codex@example.com",
			"account_id":    "acct-1",
			"access_token":  "codex-at",
			"refresh_token": "codex-rt",
			"id_token":      "codex-id",
		},
	}

	item, err := normalizeCLIProxyAuthImportEntry(entry)
	if err != nil {
		t.Fatalf("normalizeCLIProxyAuthImportEntry error = %v", err)
	}
	if item.Platform != service.PlatformOpenAI {
		t.Fatalf("platform = %q, want openai", item.Platform)
	}
	if item.Type != service.AccountTypeOAuth {
		t.Fatalf("type = %q, want oauth", item.Type)
	}
	if item.Credentials["chatgpt_account_id"] != "acct-1" {
		t.Fatalf("chatgpt_account_id = %v, want acct-1", item.Credentials["chatgpt_account_id"])
	}
	if item.Extra["cliproxy_provider"] != "codex" {
		t.Fatalf("cliproxy_provider = %v", item.Extra["cliproxy_provider"])
	}
	if item.IdentityKeys[0] != "openai:account:acct-1" {
		t.Fatalf("identity keys = %#v", item.IdentityKeys)
	}
}

func TestNormalizeCLIProxyAuthImportEntryRejectsUnsupportedProvider(t *testing.T) {
	_, err := normalizeCLIProxyAuthImportEntry(cliproxyAuthImportEntry{
		Index: 1,
		Value: map[string]any{"type": "unknown", "access_token": "token"},
	})
	if err == nil {
		t.Fatal("normalizeCLIProxyAuthImportEntry error = nil, want unsupported provider error")
	}
	if !strings.Contains(err.Error(), "不支持") {
		t.Fatalf("error = %v, want unsupported provider message", err)
	}
}
