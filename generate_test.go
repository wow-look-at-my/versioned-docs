package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name		string
		markdown	string
		filename	string
		want		string
	}{
		{
			name:		"h1 title",
			markdown:	"# My Title\n\nSome content",
			filename:	"page.md",
			want:		"My Title",
		},
		{
			name:		"h1 with leading whitespace",
			markdown:	"  # Spaced Title\n\nContent",
			filename:	"page.md",
			want:		"Spaced Title",
		},
		{
			name:		"no h1 uses filename",
			markdown:	"Some content without heading",
			filename:	"api.md",
			want:		"api",
		},
		{
			name:		"h2 ignored uses filename",
			markdown:	"## Not H1\n\nContent",
			filename:	"guide.md",
			want:		"guide",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.markdown, tt.filename)
			assert.Equal(t, tt.want, got)

		})
	}
}

func TestGenerator_loadVersionContent(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	require.NoError(t, os.Mkdir(versionDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(versionDir, "index.md"), []byte("# Index\n\nWelcome"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(versionDir, "api.md"), []byte("# API Reference\n\nDocs here"), 0o644))

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	require.Nil(t, err)

	require.Equal(t, 2, len(pages))

	assert.Equal(t, "index.md", pages[0].Filename)

	assert.Equal(t, "Index", pages[0].Title)

	assert.Equal(t, "api.md", pages[1].Filename)

}

func TestGenerator_loadVersionContent_SkipsNonMD(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	require.NoError(t, os.Mkdir(versionDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(versionDir, "index.md"), []byte("# Index"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(versionDir, "image.png"), []byte("fake image"), 0o644))

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	require.Nil(t, err)

	assert.Equal(t, 1, len(pages))

}

func TestGenerator_loadVersionContent_Subdirectories(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	subDir := filepath.Join(versionDir, "rendering")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(versionDir, "index.md"), []byte("# Index"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(subDir, "shaders.md"), []byte("# Shaders"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(subDir, "lighting.md"), []byte("# Lighting"), 0o644))

	// Non-md file in subdir should be skipped
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "diagram.svg"), []byte("<svg/>"), 0o644))

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	require.Nil(t, err)

	require.Equal(t, 3, len(pages))

	// index.md should be first
	assert.Equal(t, "index.md", pages[0].Filename)

	// Check subdirectory pages use forward-slash relative paths
	names := make(map[string]bool)
	for _, p := range pages {
		names[p.Filename] = true
	}
	assert.True(t, names["rendering/lighting.md"])

	assert.True(t, names["rendering/shaders.md"])

}

func TestGenerator_loadVersionContent_DeepNesting(t *testing.T) {
	tmp := t.TempDir()
	versionDir := filepath.Join(tmp, "v1")
	deepDir := filepath.Join(versionDir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(deepDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(deepDir, "deep.md"), []byte("# Deep"), 0o644))

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	pages, err := g.loadVersionContent("v1", md)
	require.Nil(t, err)

	require.Equal(t, 1, len(pages))

	assert.Equal(t, "a/b/c/deep.md", pages[0].Filename)

}

func TestGenerator_loadVersionContent_DirNotFound(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{ContentDir: tmp}
	md := goldmark.New()
	_, err := g.loadVersionContent("nonexistent", md)
	require.NotNil(t, err)

}

func TestGenerator_Generate_PageLevelInheritance(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// v2 has index.md and api.md
	v2Dir := filepath.Join(contentDir, "v2")
	require.NoError(t, os.MkdirAll(v2Dir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(v2Dir, "index.md"), []byte("# v2 Index"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(v2Dir, "api.md"), []byte("# v2 API"), 0o644))

	// v3 has only index.md
	v3Dir := filepath.Join(contentDir, "v3")
	require.NoError(t, os.MkdirAll(v3Dir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(v3Dir, "index.md"), []byte("# v3 Index"), 0o644))

	g := &Generator{
		Config: &Config{
			Project:	"testproj",
			Repo:		"https://github.com/test/test",
		},
		SoftwareVersions:	[]string{"v1", "v2", "v3"},
		VersionMap: map[string]string{
			"v1":	"v2",
			"v2":	"v2",
			"v3":	"v3",
		},
		DocumentedVersions:	[]string{"v2", "v3"},
		ContentDir:		contentDir,
		TemplateDir:		filepath.Join(tmp, "templates"),
		OutputDir:		outputDir,
		BaseURL:		"",
	}

	require.NoError(t, g.Generate())

	// v3 should have BOTH index.html AND api.html
	_, err := os.Stat(filepath.Join(outputDir, "v3", "index.html"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "v3", "api.html"))
	assert.Nil(t, err)

	// v3/index.html should NOT have inherited banner (authored in v3)
	v3Index, _ := os.ReadFile(filepath.Join(outputDir, "v3", "index.html"))
	assert.NotContains(t, string(v3Index), "inherited-banner")

	// v3/api.html SHOULD have inherited banner (from v2)
	v3Api, _ := os.ReadFile(filepath.Join(outputDir, "v3", "api.html"))
	assert.Contains(t, string(v3Api), "inherited-banner")

	assert.Contains(t, string(v3Api), "v2")

}

func TestGenerator_Generate_BasicOutput(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	v1Dir := filepath.Join(contentDir, "v1")
	require.NoError(t, os.MkdirAll(v1Dir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(v1Dir, "index.md"), []byte("# Docs"), 0o644))

	g := &Generator{
		Config: &Config{
			Project:	"testproj",
			Repo:		"https://github.com/test/test",
		},
		SoftwareVersions:	[]string{"v1"},
		VersionMap:		map[string]string{"v1": "v1"},
		DocumentedVersions:	[]string{"v1"},
		ContentDir:		contentDir,
		TemplateDir:		filepath.Join(tmp, "templates"),
		OutputDir:		outputDir,
		BaseURL:		"",
	}

	require.NoError(t, g.Generate())

	// Check output files exist
	_, err := os.Stat(filepath.Join(outputDir, "index.html"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "llms.txt"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "v1", "index.html"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "v1", "llms-full.md"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "static", "style.css"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "static", "script.js"))
	assert.Nil(t, err)

}

func TestGenerator_Generate_ContentLoadError(t *testing.T) {
	tmp := t.TempDir()
	outputDir := filepath.Join(tmp, "site")

	g := &Generator{
		Config: &Config{
			Project:	"testproj",
			Repo:		"https://github.com/test/test",
		},
		SoftwareVersions:	[]string{"v1"},
		VersionMap:		map[string]string{"v1": "v1"},
		DocumentedVersions:	[]string{"v1"},
		ContentDir:		"/nonexistent/path",
		TemplateDir:		filepath.Join(tmp, "templates"),
		OutputDir:		outputDir,
		BaseURL:		"",
	}

	err := g.Generate()
	require.NotNil(t, err)

}

func TestGenerator_loadPageTemplate(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{TemplateDir: tmp}

	// Test default template
	tmpl, err := g.loadPageTemplate()
	require.Nil(t, err)

	assert.NotNil(t, tmpl)

	// Test custom template
	customTmpl := `<html><body>{{.Project}}</body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "page.html"), []byte(customTmpl), 0o644))

	tmpl, err = g.loadPageTemplate()
	require.Nil(t, err)

	assert.NotNil(t, tmpl)

}

func TestGenerator_loadIndexTemplate(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{TemplateDir: tmp}

	// Test default template
	tmpl, err := g.loadIndexTemplate()
	require.Nil(t, err)

	assert.NotNil(t, tmpl)

	// Test custom template
	customTmpl := `<html><body>{{.Project}} index</body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "index.html"), []byte(customTmpl), 0o644))

	tmpl, err = g.loadIndexTemplate()
	require.Nil(t, err)

	assert.NotNil(t, tmpl)

}

func TestGenerator_generateLLMSIndex(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{
		Config: &Config{
			Project:	"testproj",
			Repo:		"https://github.com/test/test",
		},
		SoftwareVersions:	[]string{"v1", "v2"},
		VersionMap: map[string]string{
			"v1":	"v2",
			"v2":	"v2",
		},
		OutputDir:	tmp,
		BaseURL:	"/docs",
	}

	require.NoError(t, g.generateLLMSIndex())

	data, err := os.ReadFile(filepath.Join(tmp, "llms.txt"))
	require.Nil(t, err)

	content := string(data)
	assert.Contains(t, content, "testproj")

	assert.Contains(t, content, "/docs/v1/llms-full.md")

	assert.Contains(t, content, "(using docs from v2)")

}

func TestGenerator_generateVersionLLMDoc(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{
		Config: &Config{
			Project:	"testproj",
			Repo:		"https://github.com/test/test",
		},
	}

	pages := []DocPage{
		{Filename: "index.md", Title: "Index", Markdown: "# Index\n\nHello", SourceVersion: "v2"},
		{Filename: "api.md", Title: "API", Markdown: "# API\n\nReference", SourceVersion: "v1"},
	}

	g.generateVersionLLMDoc(tmp, "v2", pages)

	data, err := os.ReadFile(filepath.Join(tmp, "llms-full.md"))
	require.Nil(t, err)

	content := string(data)
	assert.Contains(t, content, "testproj")

	assert.Contains(t, content, "version v2")

	// api.md has SourceVersion v1 != sv v2, should show note
	assert.Contains(t, content, "authored for version v1")

	assert.Contains(t, content, "# Index")

	assert.Contains(t, content, "# API")

}

func TestGenerator_Generate_EmptyDocVersion(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// Create empty version directory
	v1Dir := filepath.Join(contentDir, "v1")
	require.NoError(t, os.MkdirAll(v1Dir, 0o755))

	g := &Generator{
		Config: &Config{
			Project:	"testproj",
			Repo:		"https://github.com/test/test",
		},
		SoftwareVersions:	[]string{"v1"},
		VersionMap:		map[string]string{"v1": "v1"},
		DocumentedVersions:	[]string{"v1"},
		ContentDir:		contentDir,
		TemplateDir:		filepath.Join(tmp, "templates"),
		OutputDir:		outputDir,
		BaseURL:		"",
	}

	// Should succeed with no pages
	require.NoError(t, g.Generate())

}

func TestGenerator_writeStaticAssets(t *testing.T) {
	tmp := t.TempDir()

	g := &Generator{OutputDir: tmp}

	require.NoError(t, g.writeStaticAssets())

	css, err := os.ReadFile(filepath.Join(tmp, "static", "style.css"))
	require.Nil(t, err)
	assert.True(t, len(css) > 0)

	js, err := os.ReadFile(filepath.Join(tmp, "static", "script.js"))
	require.Nil(t, err)
	assert.True(t, len(js) > 0)
}

func TestGenerator_writeStaticAssets_BadDir(t *testing.T) {
	g := &Generator{OutputDir: "/dev/null/impossible"}
	err := g.writeStaticAssets()
	require.NotNil(t, err)
}

func TestGenerator_Generate_StaticAssetError(t *testing.T) {
	g := &Generator{
		Config: &Config{
			Project: "testproj",
			Repo:    "https://github.com/test/test",
		},
		SoftwareVersions:   []string{"v1"},
		VersionMap:         map[string]string{"v1": "v1"},
		DocumentedVersions: []string{"v1"},
		ContentDir:         "/nonexistent",
		TemplateDir:        "/nonexistent",
		OutputDir:          "/dev/null/impossible",
		BaseURL:            "",
	}
	err := g.Generate()
	require.NotNil(t, err)
}

func TestGenerator_Generate_SubdirectoryOutput(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// v1 has root index.md and rendering/shaders.md
	v1Dir := filepath.Join(contentDir, "v1")
	v1Sub := filepath.Join(v1Dir, "rendering")
	require.NoError(t, os.MkdirAll(v1Sub, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(v1Dir, "index.md"), []byte("# v1 Index"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(v1Sub, "shaders.md"), []byte("# Shaders Guide"), 0o644))

	g := &Generator{
		Config: &Config{
			Project:	"testproj",
			Repo:		"https://github.com/test/test",
		},
		SoftwareVersions:	[]string{"v1", "v2"},
		VersionMap:		map[string]string{"v1": "v1", "v2": "v1"},
		DocumentedVersions:	[]string{"v1"},
		ContentDir:		contentDir,
		TemplateDir:		filepath.Join(tmp, "templates"),
		OutputDir:		outputDir,
		BaseURL:		"",
	}

	require.NoError(t, g.Generate())

	// v1 should have rendering/shaders.html in a subdirectory
	_, err := os.Stat(filepath.Join(outputDir, "v1", "rendering", "shaders.html"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "v1", "index.html"))
	assert.Nil(t, err)

	// v2 should inherit subdirectory pages from v1
	_, err = os.Stat(filepath.Join(outputDir, "v2", "rendering", "shaders.html"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "v2", "index.html"))
	assert.Nil(t, err)

}

func TestGenerator_Generate_MultipleVersionsMultiplePages(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	// v1: only api.md
	v1Dir := filepath.Join(contentDir, "v1")
	require.NoError(t, os.MkdirAll(v1Dir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(v1Dir, "api.md"), []byte("# v1 API"), 0o644))

	// v3: only index.md
	v3Dir := filepath.Join(contentDir, "v3")
	require.NoError(t, os.MkdirAll(v3Dir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(v3Dir, "index.md"), []byte("# v3 Index"), 0o644))

	g := &Generator{
		Config: &Config{
			Project:	"testproj",
			Repo:		"https://github.com/test/test",
		},
		SoftwareVersions:	[]string{"v1", "v2", "v3"},
		VersionMap: map[string]string{
			"v1":	"v1",
			"v2":	"v1",
			"v3":	"v3",
		},
		DocumentedVersions:	[]string{"v1", "v3"},
		ContentDir:		contentDir,
		TemplateDir:		filepath.Join(tmp, "templates"),
		OutputDir:		outputDir,
		BaseURL:		"/docs",
	}

	require.NoError(t, g.Generate())

	// v1 should have api.html (authored) and index.html (from v3, forward inheritance)
	_, err := os.Stat(filepath.Join(outputDir, "v1", "api.html"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "v1", "index.html"))
	assert.Nil(t, err)

	// v3 should have index.html (authored) and api.html (from v1, backward inheritance)
	_, err = os.Stat(filepath.Join(outputDir, "v3", "index.html"))
	assert.Nil(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "v3", "api.html"))
	assert.Nil(t, err)

}
