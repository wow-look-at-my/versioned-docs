package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestNoDeadLinks(t *testing.T) {
	// Set up test directories
	tmp := t.TempDir()
	outputDir := filepath.Join(tmp, "site")

	// Use the example content
	contentDir := "example/content"
	configPath := "example/config.yaml"

	// Check if example exists (skip if running in isolation)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("example config not found, skipping integration test")
	}

	// Load config and generate
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	documentedVersions, err := DiscoverDocumentedVersions(contentDir, cfg.SoftwareVersions)
	if err != nil {
		t.Fatalf("DiscoverDocumentedVersions: %v", err)
	}

	vmap := ResolveVersionMap(cfg.SoftwareVersions, documentedVersions)

	gen := &Generator{
		Config:             cfg,
		VersionMap:         vmap,
		DocumentedVersions: documentedVersions,
		ContentDir:         contentDir,
		TemplateDir:        "templates",
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Find all HTML files
	var htmlFiles []string
	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			htmlFiles = append(htmlFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking output: %v", err)
	}

	// Regex to find href attributes
	hrefRe := regexp.MustCompile(`href="([^"]+)"`)

	// Check each HTML file for dead links
	for _, htmlFile := range htmlFiles {
		data, err := os.ReadFile(htmlFile)
		if err != nil {
			t.Fatalf("reading %s: %v", htmlFile, err)
		}

		matches := hrefRe.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			href := match[1]

			// Skip external links
			if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
				continue
			}

			// Skip anchor-only links
			if strings.HasPrefix(href, "#") {
				continue
			}

			// Resolve the link relative to the HTML file's directory
			var targetPath string
			if strings.HasPrefix(href, "/") {
				// Absolute path from output root
				targetPath = filepath.Join(outputDir, href)
			} else {
				// Relative path from the HTML file's directory
				targetPath = filepath.Join(filepath.Dir(htmlFile), href)
			}

			// Check if target exists
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				relHTML, _ := filepath.Rel(outputDir, htmlFile)
				t.Errorf("dead link in %s: %s -> %s", relHTML, href, targetPath)
			}
		}
	}
}
