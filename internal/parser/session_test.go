package parser

import (
	"testing"
)

func TestLoadAllPermissionStatsFrom(t *testing.T) {
	stats, err := LoadAllPermissionStatsFrom("../../testdata/projects")
	if err != nil {
		t.Fatalf("Failed to load stats: %v", err)
	}

	if len(stats) == 0 {
		t.Fatal("Expected some permissions, got none")
	}

	// Check we found Bash permissions (now scoped as Bash(ls:*), Bash(pwd:*), etc.)
	var bashCount int
	var bashPermCount int
	for _, s := range stats {
		if s.Permission.Type == "Bash" {
			bashPermCount++
			bashCount += s.Count
		}
	}

	if bashPermCount == 0 {
		t.Error("Expected to find Bash permissions")
	}

	// Total Bash usage across all scoped permissions should be 4
	if bashCount != 4 {
		t.Errorf("Expected total Bash count of 4, got %d", bashCount)
	}

	// Should have 4 distinct scoped Bash permissions (ls, pwd, curl, jq)
	if bashPermCount != 4 {
		t.Errorf("Expected 4 scoped Bash permissions, got %d", bashPermCount)
	}

	// Check Read
	var readFound bool
	for _, s := range stats {
		if s.Permission.Type == "Read" {
			readFound = true
			if s.Count != 1 {
				t.Errorf("Expected Read count of 1, got %d", s.Count)
			}
			break
		}
	}

	if !readFound {
		t.Error("Expected to find Read permission")
	}

	// Check project decoding
	for _, s := range stats {
		if len(s.Projects) == 0 {
			t.Errorf("Permission %s has no projects", s.Permission.Raw)
		}
		for _, proj := range s.Projects {
			if proj != "/test/project" {
				t.Errorf("Expected project /test/project, got %s", proj)
			}
		}
	}
}

func TestDecodeProjectPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"-Users-satchmo-code-myproject", "/Users/satchmo/code/myproject"},
		{"-test-project", "/test/project"},
		{"local-project", "local/project"},
	}

	for _, tc := range tests {
		result := decodeProjectPath(tc.input)
		if result != tc.expected {
			t.Errorf("decodeProjectPath(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}
