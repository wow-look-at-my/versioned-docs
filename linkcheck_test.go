package main

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertNoDeadLinks walks every HTML file under outputDir and asserts that
// every non-external href resolves to a file on disk. Hrefs are
// percent-unescaped first, mirroring what a web server does when mapping a
// request path to a static file.
func assertNoDeadLinks(t *testing.T, outputDir string) {
	t.Helper()

	var htmlFiles []string
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			htmlFiles = append(htmlFiles, path)
		}
		return nil
	})
	require.Nil(t, err)
	require.NotEmpty(t, htmlFiles)

	hrefRe := regexp.MustCompile(`href="([^"]+)"`)

	for _, htmlFile := range htmlFiles {
		data, err := os.ReadFile(htmlFile)
		require.Nil(t, err)

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

			// Undo URL escaping to get the on-disk name
			unescaped, err := url.PathUnescape(href)
			require.NoError(t, err, "invalid escaping in href %q (%s)", href, htmlFile)

			// Resolve the link relative to the HTML file's directory
			var targetPath string
			if strings.HasPrefix(unescaped, "/") {
				// Absolute path from output root
				targetPath = filepath.Join(outputDir, unescaped)
			} else {
				// Relative path from the HTML file's directory
				targetPath = filepath.Join(filepath.Dir(htmlFile), unescaped)
			}

			// Check if target exists
			_, err = os.Stat(targetPath)
			assert.False(t, os.IsNotExist(err), "dead link %q in %s", href, htmlFile)
		}
	}
}

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
	require.Nil(t, err)

	softwareVersions, err := LoadSoftwareVersions(cfg)
	require.Nil(t, err)

	documentedVersions, err := DiscoverDocumentedVersions(contentDir, softwareVersions)
	require.Nil(t, err)

	vmap := ResolveVersionMap(softwareVersions, documentedVersions)

	gen := &Generator{
		Config:             cfg,
		SoftwareVersions:   softwareVersions,
		VersionMap:         vmap,
		DocumentedVersions: documentedVersions,
		ContentDir:         contentDir,
		TemplateDir:        "templates",
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	require.NoError(t, gen.Generate())

	assertNoDeadLinks(t, outputDir)
}
