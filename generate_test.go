package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		filename string
		want     string
	}{
		{
			name:     "h1 title",
			markdown: "# My Title\n\nSome content",
			filename: "page.md",
			want:     "My Title",
		},
		{
			name:     "h1 with leading whitespace",
			markdown: "  # Spaced Title\n\nContent",
			filename: "page.md",
			want:     "Spaced Title",
		},
		{
			name:     "no h1 uses filename",
			markdown: "Some content without heading",
			filename: "api.md",
			want:     "api",
		},
		{
			name:     "h2 ignored uses filename",
			markdown: "## Not H1\n\nContent",
			filename: "guide.md",
			want:     "guide",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.markdown, tt.filename)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerator_loadVersionContent(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	if err := os.Mkdir(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(versionDir, "index.md"), []byte("# Index\n\nWelcome"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "api.md"), []byte("# API Reference\n\nDocs here"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	if err != nil {
		t.Fatalf("loadVersionContent: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}

	if pages[0].Filename != "index.md" {
		t.Errorf("first page = %q, want index.md", pages[0].Filename)
	}
	if pages[0].Title != "Index" {
		t.Errorf("first title = %q, want Index", pages[0].Title)
	}

	if pages[1].Filename != "api.md" {
		t.Errorf("second page = %q, want api.md", pages[1].Filename)
	}
}

func TestGenerator_loadVersionContent_SkipsNonMD(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	if err := os.Mkdir(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(versionDir, "index.md"), []byte("# Index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "image.png"), []byte("fake image"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	if err != nil {
		t.Fatalf("loadVersionContent: %v", err)
	}

	if len(pages) != 1 {
		t.Errorf("got %d pages, want 1 (only .md files)", len(pages))
	}
}

func TestGenerator_loadVersionContent_Subdirectories(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	subDir := filepath.Join(versionDir, "rendering")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(versionDir, "index.md"), []byte("# Index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "shaders.md"), []byte("# Shaders"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "lighting.md"), []byte("# Lighting"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-md file in subdir should be skipped
	if err := os.WriteFile(filepath.Join(subDir, "diagram.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	if err != nil {
		t.Fatalf("loadVersionContent: %v", err)
	}

	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3", len(pages))
	}

	// index.md should be first
	if pages[0].Filename != "index.md" {
		t.Errorf("first page = %q, want index.md", pages[0].Filename)
	}

	// Check subdirectory pages use forward-slash relative paths
	names := make(map[string]bool)
	for _, p := range pages {
		names[p.Filename] = true
	}
	if !names["rendering/lighting.md"] {
		t.Error("missing rendering/lighting.md")
	}
	if !names["rendering/shaders.md"] {
		t.Error("missing rendering/shaders.md")
	}
}

func TestGenerator_loadVersionContent_DeepNesting(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	deepDir := filepath.Join(versionDir, "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(deepDir, "deep.md"), []byte("# Deep"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	if err != nil {
		t.Fatalf("loadVersionContent: %v", err)
	}

	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	if pages[0].Filename != "a/b/c/deep.md" {
		t.Errorf("filename = %q, want a/b/c/deep.md", pages[0].Filename)
	}
}

func TestGenerator_loadVersionContent_DirNotFound(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	_, err := g.loadVersionContent("nonexistent", md)
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestGenerator_Generate_PageLevelInheritance(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// v2 has index.md and api.md
	v2Dir := filepath.Join(contentDir, "v2")
	if err := os.MkdirAll(v2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v2Dir, "index.md"), []byte("# v2 Index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v2Dir, "api.md"), []byte("# v2 API"), 0o644); err != nil {
		t.Fatal(err)
	}

	// v3 has only index.md
	v3Dir := filepath.Join(contentDir, "v3")
	if err := os.MkdirAll(v3Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v3Dir, "index.md"), []byte("# v3 Index"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
		SoftwareVersions: []string{"v1", "v2", "v3"},
		VersionMap: map[string]string{
			"v1": "v2",
			"v2": "v2",
			"v3": "v3",
		},
		DocumentedVersions: []string{"v2", "v3"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// v3 should have BOTH index.html AND api.html
	if _, err := os.Stat(filepath.Join(outputDir, "v3", "index.html")); err != nil {
		t.Error("missing v3/index.html")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "v3", "api.html")); err != nil {
		t.Error("missing v3/api.html - page-level inheritance failed")
	}

	// v3/index.html should NOT have inherited banner (authored in v3)
	v3Index, _ := os.ReadFile(filepath.Join(outputDir, "v3", "index.html"))
	if strings.Contains(string(v3Index), "inherited-banner") {
		t.Error("v3/index.html should not have inherited banner")
	}

	// v3/api.html SHOULD have inherited banner (from v2)
	v3Api, _ := os.ReadFile(filepath.Join(outputDir, "v3", "api.html"))
	if !strings.Contains(string(v3Api), "inherited-banner") {
		t.Error("v3/api.html should have inherited banner")
	}
	if !strings.Contains(string(v3Api), "v2") {
		t.Error("v3/api.html banner should mention v2")
	}
}

func TestGenerator_Generate_BasicOutput(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	v1Dir := filepath.Join(contentDir, "v1")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "index.md"), []byte("# Docs"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
		SoftwareVersions: []string{"v1"},
		VersionMap:         map[string]string{"v1": "v1"},
		DocumentedVersions: []string{"v1"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Check output files exist
	if _, err := os.Stat(filepath.Join(outputDir, "index.html")); err != nil {
		t.Error("missing root index.html")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "llms.txt")); err != nil {
		t.Error("missing llms.txt")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "v1", "index.html")); err != nil {
		t.Error("missing v1/index.html")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "v1", "llms-full.md")); err != nil {
		t.Error("missing v1/llms-full.md")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "static", "style.css")); err != nil {
		t.Error("missing static/style.css")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "static", "script.js")); err != nil {
		t.Error("missing static/script.js")
	}
}

func TestGenerator_Generate_ContentLoadError(t *testing.T) {
	tmp := t.TempDir()
	outputDir := filepath.Join(tmp, "site")

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
		SoftwareVersions: []string{"v1"},
		VersionMap:         map[string]string{"v1": "v1"},
		DocumentedVersions: []string{"v1"},
		ContentDir:         "/nonexistent/path",
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	err := g.Generate()
	if err == nil {
		t.Fatal("expected error for missing content directory")
	}
}

func TestGenerator_loadPageTemplate(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{TemplateDir: tmp}

	// Test default template
	tmpl, err := g.loadPageTemplate()
	if err != nil {
		t.Fatalf("loadPageTemplate with default: %v", err)
	}
	if tmpl == nil {
		t.Error("expected non-nil template")
	}

	// Test custom template
	customTmpl := `<html><body>{{.Project}}</body></html>`
	if err := os.WriteFile(filepath.Join(tmp, "page.html"), []byte(customTmpl), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpl, err = g.loadPageTemplate()
	if err != nil {
		t.Fatalf("loadPageTemplate with custom: %v", err)
	}
	if tmpl == nil {
		t.Error("expected non-nil template")
	}
}

func TestGenerator_loadIndexTemplate(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{TemplateDir: tmp}

	// Test default template
	tmpl, err := g.loadIndexTemplate()
	if err != nil {
		t.Fatalf("loadIndexTemplate with default: %v", err)
	}
	if tmpl == nil {
		t.Error("expected non-nil template")
	}

	// Test custom template
	customTmpl := `<html><body>{{.Project}} index</body></html>`
	if err := os.WriteFile(filepath.Join(tmp, "index.html"), []byte(customTmpl), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpl, err = g.loadIndexTemplate()
	if err != nil {
		t.Fatalf("loadIndexTemplate with custom: %v", err)
	}
	if tmpl == nil {
		t.Error("expected non-nil template")
	}
}

func TestGenerator_generateLLMSIndex(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
		SoftwareVersions: []string{"v1", "v2"},
		VersionMap: map[string]string{
			"v1": "v2",
			"v2": "v2",
		},
		OutputDir: tmp,
		BaseURL:   "/docs",
	}

	if err := g.generateLLMSIndex(); err != nil {
		t.Fatalf("generateLLMSIndex: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "llms.txt"))
	if err != nil {
		t.Fatalf("reading llms.txt: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "testproj") {
		t.Error("missing project name")
	}
	if !strings.Contains(content, "/docs/v1/llms-full.md") {
		t.Error("missing v1 link")
	}
	if !strings.Contains(content, "(using docs from v2)") {
		t.Error("missing inheritance marker for v1")
	}
}

func TestGenerator_generateVersionLLMDoc(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
	}

	pages := []DocPage{
		{Filename: "index.md", Title: "Index", Markdown: "# Index\n\nHello", SourceVersion: "v2"},
		{Filename: "api.md", Title: "API", Markdown: "# API\n\nReference", SourceVersion: "v1"},
	}

	g.generateVersionLLMDoc(tmp, "v2", pages)

	data, err := os.ReadFile(filepath.Join(tmp, "llms-full.md"))
	if err != nil {
		t.Fatalf("reading llms-full.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "testproj") {
		t.Error("missing project name")
	}
	if !strings.Contains(content, "version v2") {
		t.Error("missing version")
	}
	// api.md has SourceVersion v1 != sv v2, should show note
	if !strings.Contains(content, "authored for version v1") {
		t.Error("missing per-page inheritance note for api")
	}
	if !strings.Contains(content, "# Index") {
		t.Error("missing index content")
	}
	if !strings.Contains(content, "# API") {
		t.Error("missing api content")
	}
}

func TestGenerator_Generate_EmptyDocVersion(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// Create empty version directory
	v1Dir := filepath.Join(contentDir, "v1")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
		SoftwareVersions: []string{"v1"},
		VersionMap:         map[string]string{"v1": "v1"},
		DocumentedVersions: []string{"v1"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	// Should succeed with no pages
	if err := g.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}
}

func TestGenerator_writeStaticAssets(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{OutputDir: tmp}

	if err := g.writeStaticAssets(); err != nil {
		t.Fatalf("writeStaticAssets: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, "static", "style.css")); err != nil {
		t.Error("missing style.css")
	}
	if _, err := os.Stat(filepath.Join(tmp, "static", "script.js")); err != nil {
		t.Error("missing script.js")
	}
}

func TestGenerator_Generate_SubdirectoryOutput(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// v1 has root index.md and rendering/shaders.md
	v1Dir := filepath.Join(contentDir, "v1")
	v1Sub := filepath.Join(v1Dir, "rendering")
	if err := os.MkdirAll(v1Sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "index.md"), []byte("# v1 Index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Sub, "shaders.md"), []byte("# Shaders Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
		SoftwareVersions:   []string{"v1", "v2"},
		VersionMap:         map[string]string{"v1": "v1", "v2": "v1"},
		DocumentedVersions: []string{"v1"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// v1 should have rendering/shaders.html in a subdirectory
	if _, err := os.Stat(filepath.Join(outputDir, "v1", "rendering", "shaders.html")); err != nil {
		t.Error("missing v1/rendering/shaders.html")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "v1", "index.html")); err != nil {
		t.Error("missing v1/index.html")
	}

	// v2 should inherit subdirectory pages from v1
	if _, err := os.Stat(filepath.Join(outputDir, "v2", "rendering", "shaders.html")); err != nil {
		t.Error("missing v2/rendering/shaders.html - subdirectory inheritance failed")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "v2", "index.html")); err != nil {
		t.Error("missing v2/index.html")
	}
}

func TestGenerator_Generate_MultipleVersionsMultiplePages(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// v1: only api.md
	v1Dir := filepath.Join(contentDir, "v1")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "api.md"), []byte("# v1 API"), 0o644); err != nil {
		t.Fatal(err)
	}

	// v3: only index.md
	v3Dir := filepath.Join(contentDir, "v3")
	if err := os.MkdirAll(v3Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v3Dir, "index.md"), []byte("# v3 Index"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
		SoftwareVersions: []string{"v1", "v2", "v3"},
		VersionMap: map[string]string{
			"v1": "v1",
			"v2": "v1",
			"v3": "v3",
		},
		DocumentedVersions: []string{"v1", "v3"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "/docs",
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// v1 should have api.html (authored) and index.html (from v3, forward inheritance)
	if _, err := os.Stat(filepath.Join(outputDir, "v1", "api.html")); err != nil {
		t.Error("missing v1/api.html")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "v1", "index.html")); err != nil {
		t.Error("missing v1/index.html")
	}

	// v3 should have index.html (authored) and api.html (from v1, backward inheritance)
	if _, err := os.Stat(filepath.Join(outputDir, "v3", "index.html")); err != nil {
		t.Error("missing v3/index.html")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "v3", "api.html")); err != nil {
		t.Error("missing v3/api.html")
	}
}
