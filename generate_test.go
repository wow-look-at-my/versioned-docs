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

	// Create test markdown files
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

	// index.md should be first
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

	// Create md and non-md files
	if err := os.WriteFile(filepath.Join(versionDir, "index.md"), []byte("# Index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "image.png"), []byte("fake image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(versionDir, "subdir"), 0o755); err != nil {
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

func TestGenerator_loadVersionContent_DirNotFound(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	_, err := g.loadVersionContent("nonexistent", md)
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestGenerator_loadVersionContent_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	if err := os.Mkdir(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	if err != nil {
		t.Fatalf("loadVersionContent: %v", err)
	}

	if len(pages) != 0 {
		t.Errorf("got %d pages, want 0", len(pages))
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
		{Filename: "index.md", Title: "Index", Markdown: "# Index\n\nHello"},
		{Filename: "api.md", Title: "API", Markdown: "# API\n\nReference"},
	}

	g.generateVersionLLMDoc(tmp, "v2", "v1", true, pages)

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
	if !strings.Contains(content, "authored for version v1") {
		t.Error("missing inheritance note")
	}
	if !strings.Contains(content, "# Index") {
		t.Error("missing index content")
	}
	if !strings.Contains(content, "# API") {
		t.Error("missing api content")
	}
}

func TestGenerator_generateLLMSIndex(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1", "v2", "v3"},
		},
		VersionMap: map[string]string{
			"v1": "v2",
			"v2": "v2",
			"v3": "v2",
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

func TestGenerator_Generate(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// Create content for v2
	v2Dir := filepath.Join(contentDir, "v2")
	if err := os.MkdirAll(v2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v2Dir, "index.md"), []byte("# v2 Docs\n\nHello v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1", "v2"},
		},
		VersionMap: map[string]string{
			"v1": "v2",
			"v2": "v2",
		},
		DocumentedVersions: []string{"v2"},
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
	if _, err := os.Stat(filepath.Join(outputDir, "v2", "index.html")); err != nil {
		t.Error("missing v2/index.html")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "v1", "llms-full.md")); err != nil {
		t.Error("missing v1/llms-full.md")
	}

	// Check inherited version has banner
	v1Html, err := os.ReadFile(filepath.Join(outputDir, "v1", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(v1Html), "These docs were written for version") {
		t.Error("v1 should have inherited banner text")
	}

	// Check authored version has no banner text
	v2Html, err := os.ReadFile(filepath.Join(outputDir, "v2", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(v2Html), "These docs were written for version") {
		t.Error("v2 should not have inherited banner text")
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

func TestGenerator_Generate_NoContent(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// v2 dir exists but is empty
	if err := os.Mkdir(filepath.Join(contentDir, "v2"), 0o755); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1", "v2"},
		},
		VersionMap: map[string]string{
			"v1": "v2",
			"v2": "v2",
		},
		DocumentedVersions: []string{"v2"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	// Should still succeed, just with empty pages
	if err := g.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}
}

func TestGenerator_Generate_ContentLoadError(t *testing.T) {
	tmp := t.TempDir()
	outputDir := filepath.Join(tmp, "site")

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1"},
		},
		VersionMap: map[string]string{
			"v1": "v1",
		},
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

func TestGenerator_Generate_OutputDirError(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")

	// Create content
	v1Dir := filepath.Join(contentDir, "v1")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "index.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a file where a parent directory should be - mkdir will fail
	parentFile := filepath.Join(tmp, "parent")
	if err := os.WriteFile(parentFile, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	badOutput := filepath.Join(parentFile, "subdir")

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1"},
		},
		VersionMap: map[string]string{
			"v1": "v1",
		},
		DocumentedVersions: []string{"v1"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          badOutput,
		BaseURL:            "",
	}

	err := g.Generate()
	if err == nil {
		t.Fatal("expected error for invalid output directory")
	}
}

func TestGenerator_generateVersionLLMDoc_NotInherited(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
	}

	pages := []DocPage{
		{Filename: "index.md", Title: "Index", Markdown: "# Index\n\nHello"},
	}

	// Not inherited (same version)
	g.generateVersionLLMDoc(tmp, "v2", "v2", false, pages)

	data, err := os.ReadFile(filepath.Join(tmp, "llms-full.md"))
	if err != nil {
		t.Fatalf("reading llms-full.md: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "authored for version") {
		t.Error("should not have inheritance note when not inherited")
	}
}

func TestGenerator_Generate_BadPageTemplate(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")
	templateDir := filepath.Join(tmp, "templates")

	// Create content
	v1Dir := filepath.Join(contentDir, "v1")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "index.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create invalid template
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "page.html"), []byte("{{.Invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1"},
		},
		VersionMap: map[string]string{
			"v1": "v1",
		},
		DocumentedVersions: []string{"v1"},
		ContentDir:         contentDir,
		TemplateDir:        templateDir,
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	err := g.Generate()
	if err == nil {
		t.Fatal("expected error for invalid page template")
	}
}

func TestGenerator_Generate_BadIndexTemplate(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")
	templateDir := filepath.Join(tmp, "templates")

	// Create content
	v1Dir := filepath.Join(contentDir, "v1")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "index.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create invalid index template (but valid page template)
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "index.html"), []byte("{{.Invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1"},
		},
		VersionMap: map[string]string{
			"v1": "v1",
		},
		DocumentedVersions: []string{"v1"},
		ContentDir:         contentDir,
		TemplateDir:        templateDir,
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	err := g.Generate()
	if err == nil {
		t.Fatal("expected error for invalid index template")
	}
}

func TestGenerator_Generate_MultiplePages(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// Create content with multiple pages
	v1Dir := filepath.Join(contentDir, "v1")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "index.md"), []byte("# Index\n\nWelcome"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "api.md"), []byte("# API\n\nReference"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "guide.md"), []byte("# Guide\n\nHow to"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1"},
		},
		VersionMap: map[string]string{
			"v1": "v1",
		},
		DocumentedVersions: []string{"v1"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "/docs",
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify all pages created
	for _, page := range []string{"index.html", "api.html", "guide.html"} {
		if _, err := os.Stat(filepath.Join(outputDir, "v1", page)); err != nil {
			t.Errorf("missing %s", page)
		}
	}
}

func TestGenerator_Generate_MultipleVersions(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// Create content for v1 and v3
	for _, v := range []string{"v1", "v3"} {
		vDir := filepath.Join(contentDir, v)
		if err := os.MkdirAll(vDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(vDir, "index.md"), []byte("# "+v+"\n\nDocs for "+v), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	g := &Generator{
		Config: &Config{
			Project:          "testproj",
			Repo:             "https://github.com/test/test",
			SoftwareVersions: []string{"v1", "v2", "v3"},
		},
		VersionMap: map[string]string{
			"v1": "v1",
			"v2": "v1", // inherits from v1
			"v3": "v3",
		},
		DocumentedVersions: []string{"v1", "v3"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          outputDir,
		BaseURL:            "",
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify version directories created
	for _, v := range []string{"v1", "v2", "v3"} {
		if _, err := os.Stat(filepath.Join(outputDir, v, "index.html")); err != nil {
			t.Errorf("missing %s/index.html", v)
		}
	}
}
