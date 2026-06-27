package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	semverRE  = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	versionRE = regexp.MustCompile(`\d+\.\d+\.\d+`)
)

func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "metadata.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("project root not found (metadata.yaml missing in any parent)")
		}
		dir = parent
	}
}

func wantVersion(t *testing.T) string {
	t.Helper()
	root := findProjectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "metadata.yaml"))
	if err != nil {
		t.Fatalf("read metadata.yaml: %v", err)
	}
	m := regexp.MustCompile(`(?m)^version:\s*(.+)$`).FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("version field not found in metadata.yaml")
	}
	v := strings.TrimSpace(m[1])
	if !semverRE.MatchString(v) {
		t.Fatalf("version %q in metadata.yaml is not valid semver (X.Y.Z)", v)
	}
	return v
}

func hasSuffix(path string, exts ...string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func isExempt(rel string) bool {
	switch rel {
	case "metadata.yaml",
		"go.sum",
		"cmd/orion/version_consistency_test.go":
		return true
	}
	return false
}

func isTrackedExt(rel string) bool {
	if hasSuffix(rel, ".go", ".sh", ".md", ".yaml", ".yml") {
		return true
	}
	return rel == "Makefile" || strings.HasPrefix(rel, "Makefile")
}

func checkShellForVersion(rel, content, ver string) []string {
	out := make([]string, 0, 1)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for lineNum := 1; scanner.Scan(); lineNum++ {
		line := scanner.Text()
		if !strings.Contains(line, ver) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}
		out = append(out, fmt.Sprintf(
			"%s:%d: non-comment line has version %q: %s",
			rel, lineNum, ver, trimmed,
		))
	}
	return out
}

func readProjectFile(t *testing.T, parts ...string) string {
	t.Helper()
	root := findProjectRoot(t)
	data, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(parts...), err)
	}
	return string(data)
}

// checkMarkdownVersion parses a .md file and reports versions found that differ
// from the metadata.yaml source of truth. Hardcoded versions in docs are accepted
// as long as they match — only stale/mismatched versions are flagged.
// IP addresses like 1.1.1.1 are intentionally ignored (they match \d+.\d+.\d+
// but have a fourth octet).
func checkMarkdownVersion(rel, content, wantVer string) []string {
	out := make([]string, 0, 1)
	seen := make(map[string]bool)
	for _, loc := range versionRE.FindAllStringIndex(content, -1) {
		match := content[loc[0]:loc[1]]
		if seen[match] {
			continue
		}
		seen[match] = true
		if match == wantVer {
			continue
		}
		// skip if this is the first three octets of an IP address (e.g. 1.1.1 in 1.1.1.1)
		if loc[1] < len(content) && content[loc[1]] == '.' {
			end := loc[1] + 1
			for end < len(content) && content[end] >= '0' && content[end] <= '9' {
				end++
			}
			if end > loc[1]+1 {
				continue
			}
		}
		out = append(out, fmt.Sprintf(
			"%s: references version %q but metadata.yaml is %q (stale documentation)",
			rel, match, wantVer,
		))
	}
	return out
}

func skipUnwantedDirs(info os.FileInfo) error {
	if !info.IsDir() {
		return nil
	}
	switch info.Name() {
	case "vendor", ".git", "build":
		return filepath.SkipDir
	}
	return nil
}

// Tests.

func TestVersionConsistency_MetadataYamlIsValid(t *testing.T) {
	t.Parallel()
	v := wantVersion(t)
	t.Logf("metadata.yaml version: %s", v)
}

func TestVersionConsistency_MainGoUsesDevPlaceholder(t *testing.T) {
	t.Parallel()
	ver := wantVersion(t)
	if ver == "dev" {
		t.Fatal("metadata.yaml version is 'dev' — must be a real semver")
	}
	if version == ver {
		t.Errorf(
			"cmd/orion/main.go variable hardcodes %q — must be \"dev\" "+
				"(ldflags injects at build via Makefile)", ver,
		)
	}
}

func TestVersionConsistency_MakefileReadsVersionFromMetadata(t *testing.T) {
	t.Parallel()
	content := readProjectFile(t, "Makefile")
	ver := wantVersion(t)

	if !strings.Contains(content, "metadata.yaml") {
		t.Error("Makefile must reference metadata.yaml as the version source of truth")
	}
	if !strings.Contains(content, "grep '^version:' metadata.yaml") &&
		!strings.Contains(content, `grep "^version:" metadata.yaml`) {
		t.Error("Makefile must dynamically extract version from metadata.yaml via grep")
	}
	if regexp.MustCompile(
		`^VERSION\s*\?*=\s*` + regexp.QuoteMeta(ver) + `$`,
	).MatchString(content) {
		t.Errorf("Makefile VERSION appears hardcoded to %q instead of dynamic", ver)
	}
}

func TestVersionConsistency_MakefileLdflagsInjectVersion(t *testing.T) {
	t.Parallel()
	content := readProjectFile(t, "Makefile")
	ver := wantVersion(t)

	for _, c := range []struct {
		flag string
		desc string
	}{
		{"-X main.version=$(VERSION)", "main.version"},
		{"-X main.programName=$(PROGRAM_NAME)", "main.programName"},
	} {
		if !strings.Contains(content, c.flag) {
			t.Errorf("Makefile ldflags missing %s", c.desc)
		}
	}
	if strings.Contains(content, "-X main.version="+ver) {
		t.Errorf("Makefile ldflags must not inject hardcoded version %q directly", ver)
	}
}

func TestVersionConsistency_ReleaseScriptReadsMetadata(t *testing.T) {
	t.Parallel()
	content := readProjectFile(t, "scripts/release.sh")

	if !strings.Contains(content, "METADATA=") &&
		!strings.Contains(content, "metadata.yaml") {
		t.Error("release.sh must read project metadata from metadata.yaml")
	}
	if !strings.Contains(content, "project_name:") {
		t.Error("release.sh must read project_name from metadata.yaml")
	}
	if !strings.Contains(content, "owner:") &&
		!strings.Contains(content, "repo:") {
		t.Error("release.sh must read owner/repo from metadata.yaml")
	}
}

func TestVersionConsistency_NoSourceFileHardcodesVersion(t *testing.T) {
	t.Parallel()
	ver := wantVersion(t)
	root := findProjectRoot(t)

	checked := 0
	var violations []string

	_ = filepath.Walk(root, func(path string, info os.FileInfo, _ error) error {
		if info == nil {
			return nil
		}
		if info.IsDir() {
			return skipUnwantedDirs(info)
		}
		rel, _ := filepath.Rel(root, path)
		if isExempt(rel) || !isTrackedExt(rel) {
			return nil
		}
		checked++
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), ver) {
			return nil
		}
		ext := filepath.Ext(path)
		switch {
		case ext == ".sh":
			violations = append(violations,
				checkShellForVersion(rel, string(data), ver)...)
		case ext == ".go" && !hasSuffix(rel, "_test.go"):
			violations = append(violations,
				fmt.Sprintf("%s: contains literal version %q (must be dynamic)", rel, ver))
		case ext == ".yaml" || ext == ".yml":
			violations = append(violations,
				fmt.Sprintf("%s: contains literal version %q (only metadata.yaml defines version)", rel, ver))
		case ext == ".md":
			violations = append(violations,
				checkMarkdownVersion(rel, string(data), ver)...)
		case !hasSuffix(rel, "_test.go") && ext == ".go":
			violations = append(violations,
				fmt.Sprintf("%s: contains literal version %q", rel, ver))
		}
		return nil
	})

	t.Logf("Checked %d source files", checked)
	for _, v := range violations {
		t.Error(v)
	}
}

// Golden-version integrity.
//
// These smoke tests verify that the entire version chain is wired correctly:
//
//	metadata.yaml ──► Makefile VERSION ──► ldflags -X main.version=$(VERSION)
//	                                          │
//	                                          └─► main.go var version = "dev" (placeholder)
//
// If any link breaks — e.g. metadata.yaml is renamed, the grep in Makefile stops
// matching, ldflags drift from the variable names in main.go — these tests
// catch it before a release goes out.
