package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWritePermissionPreservesUnknownKeys(t *testing.T) {
	projectPath := t.TempDir()
	settingsPath := filepath.Join(projectPath, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	initial := []byte(`{
  "model": "claude-opus-4-5",
  "permissions": {
    "allow": ["Read"],
    "deny": []
  },
  "hooks": {
    "pre_tool_use": ["echo hi"]
  }
}`)
	if err := os.WriteFile(settingsPath, initial, 0644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	if _, err := WritePermissionToProjectSettings(projectPath, "Bash(git:*)"); err != nil {
		t.Fatalf("write permission: %v", err)
	}

	updated, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read updated settings: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(updated, &root); err != nil {
		t.Fatalf("unmarshal updated settings: %v", err)
	}

	if got, ok := root["model"].(string); !ok || got != "claude-opus-4-5" {
		t.Fatalf("model key was not preserved: %#v", root["model"])
	}

	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks key was not preserved: %#v", root["hooks"])
	}
	if _, ok := hooks["pre_tool_use"]; !ok {
		t.Fatalf("hooks.pre_tool_use was not preserved: %#v", hooks)
	}

	perms, ok := root["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions object missing: %#v", root["permissions"])
	}
	allow, ok := perms["allow"].([]any)
	if !ok {
		t.Fatalf("permissions.allow missing: %#v", perms["allow"])
	}

	hasRead := false
	hasGit := false
	for _, raw := range allow {
		s, _ := raw.(string)
		if s == "Read" {
			hasRead = true
		}
		if s == "Bash(git:*)" {
			hasGit = true
		}
	}
	if !hasRead || !hasGit {
		t.Fatalf("permissions.allow missing expected values: %#v", allow)
	}
}

func TestConcurrentWritePermissionToProjectSettings(t *testing.T) {
	projectPath := t.TempDir()
	settingsPath := filepath.Join(projectPath, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	for i := 0; i < 50; i++ {
		if err := os.WriteFile(settingsPath, []byte(`{"permissions":{"allow":[],"deny":[]}}`), 0644); err != nil {
			t.Fatalf("reset settings: %v", err)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan error, 2)

		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, err := WritePermissionToProjectSettings(projectPath, "Bash(git:*)")
			errs <- err
		}()
		go func() {
			defer wg.Done()
			<-start
			_, err := WritePermissionToProjectSettings(projectPath, "Read")
			errs <- err
		}()

		close(start)
		wg.Wait()
		close(errs)

		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent write failed: %v", err)
			}
		}

		data, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("read settings: %v", err)
		}
		doc, err := parseSettingsDocument(data)
		if err != nil {
			t.Fatalf("settings became invalid json on iteration %d: %v", i, err)
		}

		hasRead := doc.hasPermission("Read")
		hasGit := doc.hasPermission("Bash(git:*)")
		if !hasRead || !hasGit {
			t.Fatalf("lost update on iteration %d: allow=%v", i, doc.allow)
		}
	}
}

func TestPreviewProjectDiffReturnsParseError(t *testing.T) {
	projectPath := t.TempDir()
	settingsPath := filepath.Join(projectPath, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"permissions":`), 0644); err != nil {
		t.Fatalf("write invalid settings: %v", err)
	}

	_, _, _, err := PreviewProjectDiff(projectPath, []string{"Read"})
	if err == nil {
		t.Fatal("expected preview to return parse error")
	}
}
