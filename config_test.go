package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

func TestDiscoverDocumentedVersions(t *testing.T) {
	tmp := t.TempDir()

	// Create directories for v2, v3, v5 (not v1, v4)
	for _, v := range []string{"v2", "v3", "v5"} {
		require.NoError(t, os.Mkdir(filepath.Join(tmp, v), 0o755))

	}

	software := []string{"v1", "v2", "v3", "v4", "v5"}
	got, err := DiscoverDocumentedVersions(tmp, software)
	require.Nil(t, err)

	want := []string{"v2", "v3", "v5"}
	require.Equal(t, len(want), len(got))

	for i := range want {
		assert.Equal(t, want[i], got[i])

	}
}

func TestDiscoverDocumentedVersions_None(t *testing.T) {
	tmp := t.TempDir()

	software := []string{"v1", "v2", "v3"}
	_, err := DiscoverDocumentedVersions(tmp, software)
	require.NotNil(t, err)

}

func TestDiscoverDocumentedVersions_All(t *testing.T) {
	tmp := t.TempDir()

	software := []string{"v1", "v2", "v3"}
	for _, v := range software {
		require.NoError(t, os.Mkdir(filepath.Join(tmp, v), 0o755))

	}

	got, err := DiscoverDocumentedVersions(tmp, software)
	require.Nil(t, err)

	require.Equal(t, len(software), len(got))

	for i := range software {
		assert.Equal(t, software[i], got[i])

	}
}

func TestLoadConfig(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	yaml := `project: testproject
repo: https://github.com/test/test
version_command: "echo v1"
`
	require.NoError(t, os.WriteFile(configPath, []byte(yaml), 0o644))

	cfg, err := LoadConfig(configPath)
	require.Nil(t, err)

	assert.Equal(t, "testproject", cfg.Project)

	assert.Equal(t, "https://github.com/test/test", cfg.Repo)

	assert.Equal(t, "echo v1", cfg.VersionCommand)

}

func TestLoadConfig_MissingVersionCommand(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	yaml := `project: testproject
repo: https://github.com/test/test
`
	require.NoError(t, os.WriteFile(configPath, []byte(yaml), 0o644))

	_, err := LoadConfig(configPath)
	require.NotNil(t, err)

}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	require.NotNil(t, err)

}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	require.NoError(t, os.WriteFile(configPath, []byte("not: valid: yaml: ["), 0o644))

	_, err := LoadConfig(configPath)
	require.NotNil(t, err)

}

func TestLoadSoftwareVersions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		VersionCommand: "printf 'v1\nv2\nv3'",
	}
	versions, err := LoadSoftwareVersions(cfg)
	require.Nil(t, err)

	want := []string{"v1", "v2", "v3"}
	require.Equal(t, len(want), len(versions))

	for i := range want {
		assert.Equal(t, want[i], versions[i])

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
	require.Nil(t, err)

	want := []string{"v1", "v2", "v3"}
	require.Equal(t, len(want), len(versions))

}

func TestLoadSoftwareVersions_PreservesOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		VersionCommand: "printf 'v3\nv1\nv2'",
	}
	versions, err := LoadSoftwareVersions(cfg)
	require.Nil(t, err)

	want := []string{"v3", "v1", "v2"}
	require.Equal(t, len(want), len(versions))

	for i := range want {
		assert.Equal(t, want[i], versions[i])

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
	require.NotNil(t, err)

}

func TestLoadSoftwareVersions_NoOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		VersionCommand: "echo ''",
	}
	_, err := LoadSoftwareVersions(cfg)
	require.NotNil(t, err)

}

func TestDiscoverDocumentedVersions_FileNotDir(t *testing.T) {
	tmp := t.TempDir()

	// Create a file (not directory) named v1
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "v1"), []byte("not a dir"), 0o644))

	// Create a proper directory for v2
	require.NoError(t, os.Mkdir(filepath.Join(tmp, "v2"), 0o755))

	software := []string{"v1", "v2"}
	got, err := DiscoverDocumentedVersions(tmp, software)
	require.Nil(t, err)

	// Should only find v2 since v1 is a file not a directory
	assert.False(t, len(got) != 1 || got[0] != "v2")

}

func TestLoadConfig_WithContentCommand(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")

	yaml := `project: testproject
repo: https://github.com/test/test
version_command: "echo v1"
content_command: "mkdir -p content/v1"
`
	require.NoError(t, os.WriteFile(configPath, []byte(yaml), 0o644))

	cfg, err := LoadConfig(configPath)
	require.Nil(t, err)

	assert.Equal(t, "mkdir -p content/v1", cfg.ContentCommand)

}

func TestRunContentCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	tmp := t.TempDir()
	cfg := &Config{
		ContentCommand:	"mkdir -p content/v1 && echo hello > content/v1/index.md",
		ConfigDir:	tmp,
	}

	require.NoError(t, RunContentCommand(cfg))

	// Verify the command created the expected files
	data, err := os.ReadFile(filepath.Join(tmp, "content", "v1", "index.md"))
	require.Nil(t, err)

	assert.Contains(t, string(data), "hello")

}

func TestRunContentCommand_Empty(t *testing.T) {
	cfg := &Config{ContentCommand: ""}
	require.NoError(t, RunContentCommand(cfg))

}

func TestRunContentCommand_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	cfg := &Config{
		ContentCommand:	"exit 1",
		ConfigDir:	t.TempDir(),
	}

	err := RunContentCommand(cfg)
	require.NotNil(t, err)

}

func TestDiscoverDocumentedVersions_InvalidContentDir(t *testing.T) {
	tmp := t.TempDir()

	// Create directories for v1 and v2, but v3 is not in software list
	for _, v := range []string{"v1", "v2", "v3"} {
		require.NoError(t, os.Mkdir(filepath.Join(tmp, v), 0o755))

	}

	// Software list only has v1 and v2, not v3
	software := []string{"v1", "v2"}
	_, err := DiscoverDocumentedVersions(tmp, software)
	require.NotNil(t, err)

}
