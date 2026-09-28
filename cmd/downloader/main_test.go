package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseURLLine(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		defaultSHA   string
		expectedURL  string
		expectedSHA  string
		expectedAuth string
	}{
		{
			name:         "plain url",
			line:         "https://example.com/file.zip",
			defaultSHA:   "",
			expectedURL:  "https://example.com/file.zip",
			expectedSHA:  "",
			expectedAuth: "",
		},
		{
			name:         "url with auth tag",
			line:         "https://api.github.com/repo/tarball auth=github-prod",
			defaultSHA:   "",
			expectedURL:  "https://api.github.com/repo/tarball",
			expectedSHA:  "",
			expectedAuth: "github-prod",
		},
		{
			name:         "url with auth and sha256 tags",
			line:         "https://nexus.corp/pkg.tar.gz auth=corp-nexus sha256=2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			defaultSHA:   "",
			expectedURL:  "https://nexus.corp/pkg.tar.gz",
			expectedSHA:  "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			expectedAuth: "corp-nexus",
		},
		{
			name:         "url with bare sha256 (backwards compatibility)",
			line:         "https://example.com/data.bin 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			defaultSHA:   "",
			expectedURL:  "https://example.com/data.bin",
			expectedSHA:  "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			expectedAuth: "",
		},
		{
			name:         "tags in reversed order",
			line:         "https://s3.corp/data.iso sha256=abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890 auth=s3-prod",
			defaultSHA:   "",
			expectedURL:  "https://s3.corp/data.iso",
			expectedSHA:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expectedAuth: "s3-prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, sha, authProf := parseURLLine(tt.line, tt.defaultSHA)
			if u != tt.expectedURL {
				t.Errorf("URL mismatch: got %q, want %q", u, tt.expectedURL)
			}
			if sha != tt.expectedSHA {
				t.Errorf("SHA mismatch: got %q, want %q", sha, tt.expectedSHA)
			}
			if authProf != tt.expectedAuth {
				t.Errorf("Auth profile mismatch: got %q, want %q", authProf, tt.expectedAuth)
			}
		})
	}
}

func TestParseURLFile(t *testing.T) {
	tmpDir := t.TempDir()

	content := `# Sample URLs file
https://api.github.com/archive.zip auth=gh-work sha256=2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
# Public image
https://releases.ubuntu.com/22.04/mini.iso
`
	filePath := filepath.Join(tmpDir, "urls.txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed writing url file: %v", err)
	}

	jobs, err := parseURLFile(filePath, "")
	if err != nil {
		t.Fatalf("parseURLFile failed: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0].AuthProfile != "gh-work" {
		t.Fatalf("expected job 0 auth profile 'gh-work', got %q", jobs[0].AuthProfile)
	}
	if jobs[0].ExpectedSHA256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("expected job 0 SHA mismatch: %q", jobs[0].ExpectedSHA256)
	}

	if jobs[1].AuthProfile != "" {
		t.Fatalf("expected job 1 auth profile empty, got %q", jobs[1].AuthProfile)
	}
}
