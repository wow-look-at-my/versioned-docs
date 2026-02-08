package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscoverDocumentedVersions(t *testing.T) {
	tmp := t.TempDir()

	// Create directories for v2, v3, v5 (not v1, v4)
	for _, v := range []string{"v2", "v3", "v5"} {
		if err := os.Mkdir(filepath.Join(tmp, v), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	software := []string{"v1", "v2", "v3", "v4", "v5"}
	got, err := DiscoverDocumentedVersions(tmp, software)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"v2", "v3", "v5"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiscoverDocumentedVersions_None(t *testing.T) {
	tmp := t.TempDir()

	software := []string{"v1", "v2", "v3"}
	_, err := DiscoverDocumentedVersions(tmp, software)
	if err == nil {
		t.Fatal("expected error when no documented versions found")
	}
}

func TestDiscoverDocumentedVersions_All(t *testing.T) {
	tmp := t.TempDir()

	software := []string{"v1", "v2", "v3"}
	for _, v := range software {
		if err := os.Mkdir(filepath.Join(tmp, v), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := DiscoverDocumentedVersions(tmp, software)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != len(software) {
		t.Fatalf("got %v, want %v", got, software)
	}
	for i := range software {
		if got[i] != software[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], software[i])
		}
	}
}

func TestLoadConfig(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	yaml := `project: testproject
repo: https://github.com/test/test
version_command: "echo v1"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Project != "testproject" {
		t.Errorf("Project = %q, want %q", cfg.Project, "testproject")
	}
	if cfg.Repo != "https://github.com/test/test" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "https://github.com/test/test")
	}
	if cfg.VersionCommand != "echo v1" {
		t.Errorf("VersionCommand = %q, want %q", cfg.VersionCommand, "echo v1")
	}
}

func TestLoadConfig_MissingVersionCommand(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	yaml := `project: testproject
repo: https://github.com/test/test
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Fatal("expected error for missing version_command")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	if err := os.WriteFile(configPath, []byte("not: valid: yaml: ["), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestLoadSoftwareVersions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		VersionCommand: "printf 'v1\nv2\nv3'",
	}
	versions, err := LoadSoftwareVersions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"v1", "v2", "v3"}
	if len(versions) != len(want) {
		t.Fatalf("got %v, want %v", versions, want)
	}
	for i := range want {
		if versions[i] != want[i] {
			t.Errorf("versions[%d] = %q, want %q", i, versions[i], want[i])
		}
	}
}

func TestLoadSoftwareVersions_SkipsEmptyLines(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		VersionCommand: "printf 'v1\n\nv2\n\n\nv3\n'",
	}
	versions, err := LoadSoftwareVersions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"v1", "v2", "v3"}
	if len(versions) != len(want) {
		t.Fatalf("got %v, want %v", versions, want)
	}
}

func TestLoadSoftwareVersions_PreservesOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		VersionCommand: "printf 'v3\nv1\nv2'",
	}
	versions, err := LoadSoftwareVersions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"v3", "v1", "v2"}
	if len(versions) != len(want) {
		t.Fatalf("got %v, want %v", versions, want)
	}
	for i := range want {
		if versions[i] != want[i] {
			t.Errorf("versions[%d] = %q, want %q", i, versions[i], want[i])
		}
	}
}

func TestLoadSoftwareVersions_CommandFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		VersionCommand: "exit 1",
	}
	_, err := LoadSoftwareVersions(cfg)
	if err == nil {
		t.Fatal("expected error for command failure")
	}
}

func TestLoadSoftwareVersions_NoOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		VersionCommand: "echo ''",
	}
	_, err := LoadSoftwareVersions(cfg)
	if err == nil {
		t.Fatal("expected error when command produces no versions")
	}
}

func TestDiscoverDocumentedVersions_FileNotDir(t *testing.T) {
	tmp := t.TempDir()

	// Create a file (not directory) named v1
	if err := os.WriteFile(filepath.Join(tmp, "v1"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a proper directory for v2
	if err := os.Mkdir(filepath.Join(tmp, "v2"), 0o755); err != nil {
		t.Fatal(err)
	}

	software := []string{"v1", "v2"}
	got, err := DiscoverDocumentedVersions(tmp, software)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only find v2 since v1 is a file not a directory
	if len(got) != 1 || got[0] != "v2" {
		t.Errorf("got %v, want [v2]", got)
	}
}

func TestDiscoverDocumentedVersions_InvalidContentDir(t *testing.T) {
	tmp := t.TempDir()

	// Create directories for v1 and v2, but v3 is not in software list
	for _, v := range []string{"v1", "v2", "v3"} {
		if err := os.Mkdir(filepath.Join(tmp, v), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Software list only has v1 and v2, not v3
	software := []string{"v1", "v2"}
	_, err := DiscoverDocumentedVersions(tmp, software)
	if err == nil {
		t.Fatal("expected error when content dir not in software versions list")
	}
}
