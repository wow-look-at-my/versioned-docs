package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project          string   `yaml:"project"`
	Repo             string   `yaml:"repo"`
	SoftwareVersions []string `yaml:"software_versions"`
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

	if len(cfg.SoftwareVersions) == 0 {
		return nil, fmt.Errorf("software_versions is empty")
	}

	return &cfg, nil
}

// DiscoverDocumentedVersions scans the content directory to find which
// software versions have authored documentation (i.e., have a subdirectory).
func DiscoverDocumentedVersions(contentDir string, softwareVersions []string) ([]string, error) {
	var documented []string
	for _, v := range softwareVersions {
		versionDir := contentDir + "/" + v
		info, err := os.Stat(versionDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // no docs for this version
			}
			return nil, fmt.Errorf("checking %s: %w", versionDir, err)
		}
		if info.IsDir() {
			documented = append(documented, v)
		}
	}
	if len(documented) == 0 {
		return nil, fmt.Errorf("no documented versions found in %s", contentDir)
	}
	return documented, nil
}
