package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project        string `yaml:"project"`
	Repo           string `yaml:"repo"`
	VersionCommand string `yaml:"version_command"`

	// ConfigDir is the directory containing the config file.
	// Used as working directory for version_command.
	ConfigDir string `yaml:"-"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if cfg.VersionCommand == "" {
		return nil, fmt.Errorf("version_command is required")
	}

	cfg.ConfigDir = filepath.Dir(path)

	return &cfg, nil
}

// LoadSoftwareVersions runs the version command and parses its output.
// Returns one version per line from stdout, skipping empty lines.
// The command runs from the config file's directory.
func LoadSoftwareVersions(cfg *Config) ([]string, error) {
	cmd := exec.Command("sh", "-c", cfg.VersionCommand)
	cmd.Dir = cfg.ConfigDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running version_command: %w", err)
	}

	var versions []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			versions = append(versions, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parsing version_command output: %w", err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("version_command produced no versions")
	}

	return versions, nil
}

// DiscoverDocumentedVersions scans the content directory to find which
// software versions have authored documentation (i.e., have a subdirectory).
// It validates that all content subdirectories are in the software versions list.
func DiscoverDocumentedVersions(contentDir string, softwareVersions []string) ([]string, error) {
	// Build a set of valid software versions for validation
	validVersions := make(map[string]bool)
	for _, v := range softwareVersions {
		validVersions[v] = true
	}

	// Scan content directory for all subdirectories
	entries, err := os.ReadDir(contentDir)
	if err != nil {
		return nil, fmt.Errorf("reading content directory %s: %w", contentDir, err)
	}

	// Check that all content subdirectories are valid versions
	contentVersions := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !validVersions[name] {
			return nil, fmt.Errorf("content directory %q is not in software versions list", name)
		}
		contentVersions[name] = true
	}

	// Build documented list in software versions order
	var documented []string
	for _, v := range softwareVersions {
		if contentVersions[v] {
			documented = append(documented, v)
		}
	}

	if len(documented) == 0 {
		return nil, fmt.Errorf("no documented versions found in %s", contentDir)
	}
	return documented, nil
}
