package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDocChains(t *testing.T) {
	software := []string{"v1", "v2", "v3", "v4"}
	pageVersions := map[string]map[string]bool{
		"multi.md":  {"v4": true, "v1": true, "v3": true}, // insertion order must not matter
		"single.md": {"v2": true},
	}

	chains := buildDocChains(software, pageVersions)

	assert.Equal(t, []string{"v1", "v3", "v4"}, chains["multi.md"], "chain must follow software version order, oldest first")
	assert.Equal(t, []string{"v2"}, chains["single.md"])
}

func TestSortDocPaths(t *testing.T) {
	paths := []string{"zebra.md", "alpha.md", "index.md", "beta.md"}
	sortDocPaths(paths)
	assert.Equal(t, []string{"index.md", "alpha.md", "beta.md", "zebra.md"}, paths)
}

func TestBuildDocEntries(t *testing.T) {
	software := []string{"v1", "v2", "v3"}
	pageVersions := map[string]map[string]bool{
		"guide.md": {"v1": true, "v3": true},
		"old.md":   {"v1": true},
		"edge.md":  {"v1": true},
	}
	docContent := map[string]map[string]DocPage{
		"v1": {
			"guide.md": {Filename: "guide.md", Title: "Guide v1"},
			"old.md":   {Filename: "old.md", Title: "Old"},
			"edge.md":  {Filename: "edge.md", Title: "Edge"},
		},
		"v3": {
			"guide.md": {Filename: "guide.md", Title: "Guide v3"},
		},
	}
	chains := buildDocChains(software, pageVersions)

	// old.md is removed before current (T=v1 < v3); edge.md is terminated AT
	// current (T=v3) and must stay in the default listing.
	term, err := newTermination(software, map[string]string{"old.md": "v1", "edge.md": "v3"}, pageVersions)
	require.NoError(t, err)

	docs, removed := buildDocEntries(chains, docContent, term, "v3")

	require.Len(t, docs, 2)
	assert.Equal(t, "edge", docs[0].Path)
	assert.Equal(t, "guide", docs[1].Path)
	assert.Equal(t, "Guide v3", docs[1].Title, "title must come from the newest authored version")
	assert.Equal(t, "v3", docs[1].NewestVersion)
	assert.Equal(t, 2, docs[1].NumVersions)
	assert.Equal(t, "v3", docs[0].TerminatedAt)

	require.Len(t, removed, 1)
	assert.Equal(t, "old", removed[0].Path)
	assert.Equal(t, "v1", removed[0].TerminatedAt)
}

func TestBuildVersionRanges(t *testing.T) {
	software := []string{"v1", "v2", "v3", "v4", "v5"}
	vmap := map[string]string{
		"v1": "v2", "v2": "v2", "v3": "v2", "v4": "v4", "v5": "v4",
	}

	ranges := buildVersionRanges(software, vmap)

	require.Len(t, ranges, 2)
	assert.Equal(t, VersionRange{Docs: "v2", First: "v1", Last: "v3"}, ranges[0])
	assert.Equal(t, VersionRange{Docs: "v4", First: "v4", Last: "v5"}, ranges[1])
}

func TestEscapePath(t *testing.T) {
	assert.Equal(t, "plain-name", escapePath("plain-name"))
	assert.Equal(t, "with%20space", escapePath("with space"))
	assert.Equal(t, "sub%20dir/nested%20doc", escapePath("sub dir/nested doc"))
	assert.Equal(t, "%C3%BCnicode-d%C3%A4sh", escapePath("ünicode-däsh"))
}

// writeTestDoc writes a markdown doc into contentDir/version/name.
func writeTestDoc(t *testing.T, contentDir, version, name, content string) {
	t.Helper()
	path := filepath.Join(contentDir, version, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// terminationFixture builds a site with a multi-version chain, a terminated
// doc, and an undocumented current version, then generates it.
//
//	software: v1 v2 v3 v4 v5 (current = v5, undocumented)
//	guide.md:     authored v1, v2, v4
//	old-stuff.md: authored v1, terminated at v2
//	new-doc.md:   authored v4
func terminationFixture(t *testing.T) (outputDir string) {
	t.Helper()
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir = filepath.Join(tmp, "site")

	writeTestDoc(t, contentDir, "v1", "guide.md", "# Guide\n\nv1 content")
	writeTestDoc(t, contentDir, "v1", "old-stuff.md", "# Old Stuff\n\nlegacy words")
	writeTestDoc(t, contentDir, "v2", "guide.md", "# Guide\n\nv2 content")
	writeTestDoc(t, contentDir, "v4", "guide.md", "# Guide\n\nv4 content")
	writeTestDoc(t, contentDir, "v4", "new-doc.md", "# New Doc\n\nshiny")

	software := []string{"v1", "v2", "v3", "v4", "v5"}
	documented := []string{"v1", "v2", "v4"}

	g := &Generator{
		Config: &Config{
			Project:    "testproj",
			Repo:       "https://github.com/test/test",
			Terminated: map[string]string{"old-stuff.md": "v2"},
		},
		SoftwareVersions:   software,
		VersionMap:         ResolveVersionMap(software, documented),
		DocumentedVersions: documented,
		ContentDir:         contentDir,
		TemplateDir:        "templates",
		OutputDir:          outputDir,
		BaseURL:            "",
	}
	require.NoError(t, g.Generate())
	return outputDir
}

func readOutput(t *testing.T, outputDir string, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(outputDir, filepath.Join(parts...)))
	require.NoError(t, err)
	return string(data)
}

func TestGenerate_DocCentricIndex(t *testing.T) {
	outputDir := terminationFixture(t)

	index := readOutput(t, outputDir, "index.html")

	// Default listing: every live doc links to its logical doc page
	assert.Contains(t, index, `href="/docs/guide.html"`)
	assert.Contains(t, index, `href="/docs/new-doc.html"`)
	assert.Contains(t, index, "current version v5")

	// The terminated doc is dropped from the default listing...
	defaultSection := index[:strings.Index(index, "Removed documents")]
	assert.NotContains(t, defaultSection, "Old Stuff")

	// ...but shows up in the Removed section with its last version
	assert.Contains(t, index, "Removed documents")
	removedSection := index[strings.Index(index, "Removed documents"):]
	assert.Contains(t, removedSection, `href="/docs/old-stuff.html"`)
	assert.Contains(t, removedSection, "Old Stuff")
	assert.Contains(t, removedSection, "v2")
}

func TestGenerate_DocPageNewestContentAndPicker(t *testing.T) {
	outputDir := terminationFixture(t)

	doc := readOutput(t, outputDir, "docs", "guide.html")

	// Newest authored content is shown by default
	assert.Contains(t, doc, "v4 content")
	assert.NotContains(t, doc, "v1 content")
	assert.Contains(t, doc, "authored for version <strong>v4</strong>")

	// The picker lists ONLY authored versions, newest first
	assert.Contains(t, doc, "doc-picker")
	v4Pos := strings.Index(doc, ">\n          v4 (newest)")
	v2Pos := strings.Index(doc, ">\n          v2\n")
	v1Pos := strings.Index(doc, ">\n          v1\n")
	assert.True(t, v4Pos >= 0 && v2Pos >= 0 && v1Pos >= 0, "picker must list v4, v2, v1")
	assert.True(t, v4Pos < v2Pos && v2Pos < v1Pos, "picker must be newest first")
	assert.NotContains(t, doc, `/v3/guide.html`, "undocumented versions must not appear in the picker")
	assert.NotContains(t, doc, `/v5/guide.html`)

	// Older picker entries link into the per-version history pages
	assert.Contains(t, doc, `/v2/guide.html`)
	assert.Contains(t, doc, `/v1/guide.html`)

	// No removed banner on a live doc
	assert.NotContains(t, doc, "removed-banner")
}

func TestGenerate_TerminatedDocPage(t *testing.T) {
	outputDir := terminationFixture(t)

	doc := readOutput(t, outputDir, "docs", "old-stuff.html")

	// Removed banner names the last applicable version
	assert.Contains(t, doc, "removed-banner")
	assert.Contains(t, doc, "last applies to version <strong>v2</strong>")
	assert.Contains(t, doc, "legacy words")
}

func TestGenerate_TerminationBoundaryInHistory(t *testing.T) {
	outputDir := terminationFixture(t)

	// old-stuff terminated at v2: visible at v1 (authored) and v2 (V == T)...
	_, err := os.Stat(filepath.Join(outputDir, "v1", "old-stuff.html"))
	assert.NoError(t, err, "terminated doc must exist at its authored version")
	_, err = os.Stat(filepath.Join(outputDir, "v2", "old-stuff.html"))
	assert.NoError(t, err, "terminated doc must still be visible AT the termination version")

	// ...and hidden for V > T
	_, err = os.Stat(filepath.Join(outputDir, "v4", "old-stuff.html"))
	assert.True(t, os.IsNotExist(err), "terminated doc must be hidden after the termination version")

	// The visible historical pages carry the removed banner
	v2Page := readOutput(t, outputDir, "v2", "old-stuff.html")
	assert.Contains(t, v2Page, "removed-banner")
	assert.Contains(t, v2Page, "last applies to version <strong>v2</strong>")

	// The v4 page list must not link the terminated doc
	v4Guide := readOutput(t, outputDir, "v4", "guide.html")
	assert.NotContains(t, v4Guide, "old-stuff.html")

	// Live docs don't carry the removed banner in history pages
	v4NewDoc := readOutput(t, outputDir, "v4", "new-doc.html")
	assert.NotContains(t, v4NewDoc, "removed-banner")
}

func TestGenerate_HistoryPagePicker(t *testing.T) {
	outputDir := terminationFixture(t)

	page := readOutput(t, outputDir, "v2", "guide.html")

	// The per-doc picker is present with the viewed version selected
	assert.Contains(t, page, "doc-picker")
	assert.Contains(t, page, `/v1/guide.html`)
	assert.Contains(t, page, `/docs/guide.html`, "newest picker entry must link to the logical doc page")
	assert.NotContains(t, page, `/v3/guide.html`, "picker lists authored versions only")

	// The site version select navigates page-preserving across documented versions
	assert.Contains(t, page, `/v4/guide.html`)
}

func TestGenerate_VersionsMatrix(t *testing.T) {
	outputDir := terminationFixture(t)

	versions := readOutput(t, outputDir, "versions.html")

	// Columns for documented versions only
	assert.Contains(t, versions, "<th>v1</th>")
	assert.Contains(t, versions, "<th>v2</th>")
	assert.Contains(t, versions, "<th>v4</th>")
	assert.NotContains(t, versions, "<th>v3</th>")
	assert.NotContains(t, versions, "<th>v5</th>")

	// Authored and inherited cells link into history; terminated cells are empty
	assert.Contains(t, versions, `class="cell-authored"><a href="/v4/guide.html"`)
	assert.Contains(t, versions, `class="cell-inherited"><a href="/v2/old-stuff.html"`)
	assert.NotContains(t, versions, `href="/v4/old-stuff.html"`, "terminated doc must have no cell link past T")

	// Version ranges summarize the whole software axis
	assert.Contains(t, versions, "v4 – v5")
}

func TestGenerate_LLMSOutputsWithTermination(t *testing.T) {
	outputDir := terminationFixture(t)

	llms := readOutput(t, outputDir, "llms.txt")

	// Live docs in the default listing
	assert.Contains(t, llms, "(/docs/guide.html)")
	assert.Contains(t, llms, "(/docs/new-doc.html)")

	// Terminated doc only in the Removed section
	docsSection := llms[:strings.Index(llms, "## Removed documents")]
	assert.NotContains(t, docsSection, "old-stuff")
	assert.Contains(t, llms, "last applies to version v2")

	// Per-version bundles listed for documented versions only
	assert.Contains(t, llms, "(/v4/llms-full.md)")
	assert.NotContains(t, llms, "(/v5/llms-full.md)")

	// Root bundle: newest content of live docs, no removed docs
	full := readOutput(t, outputDir, "llms-full.md")
	assert.Contains(t, full, "v4 content")
	assert.Contains(t, full, "shiny")
	assert.NotContains(t, full, "legacy words")

	// Per-version bundle past T excludes the terminated doc
	v4Full := readOutput(t, outputDir, "v4", "llms-full.md")
	assert.NotContains(t, v4Full, "legacy words")
	v2Full := readOutput(t, outputDir, "v2", "llms-full.md")
	assert.Contains(t, v2Full, "legacy words")
}

func TestGenerate_NoDeadLinksWithTermination(t *testing.T) {
	outputDir := terminationFixture(t)
	assertNoDeadLinks(t, outputDir)
}

func TestGenerate_TerminationValidationFailsBuild(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	writeTestDoc(t, contentDir, "v1", "guide.md", "# Guide")

	g := &Generator{
		Config: &Config{
			Project:    "testproj",
			Repo:       "r",
			Terminated: map[string]string{"guide.md": "v9"},
		},
		SoftwareVersions:   []string{"v1"},
		VersionMap:         map[string]string{"v1": "v1"},
		DocumentedVersions: []string{"v1"},
		ContentDir:         contentDir,
		TemplateDir:        filepath.Join(tmp, "templates"),
		OutputDir:          filepath.Join(tmp, "site"),
	}

	err := g.Generate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in the software versions list")
}

func TestGenerate_WeirdDocNames(t *testing.T) {
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	writeTestDoc(t, contentDir, "v1", "with space.md", "# With Space")
	writeTestDoc(t, contentDir, "v1", "ünicode-däsh.md", "# Unicode Dash")
	writeTestDoc(t, contentDir, "v1", "sub dir/nested doc.md", "# Nested Doc")
	writeTestDoc(t, contentDir, "v2", "with space.md", "# With Space v2")

	software := []string{"v1", "v2"}
	documented := []string{"v1", "v2"}

	g := &Generator{
		Config:             &Config{Project: "weird", Repo: "https://example.com/weird"},
		SoftwareVersions:   software,
		VersionMap:         ResolveVersionMap(software, documented),
		DocumentedVersions: documented,
		ContentDir:         contentDir,
		TemplateDir:        "templates",
		OutputDir:          outputDir,
	}
	require.NoError(t, g.Generate())

	// Files land at their raw names
	for _, p := range []string{
		filepath.Join("docs", "with space.html"),
		filepath.Join("docs", "ünicode-däsh.html"),
		filepath.Join("docs", "sub dir", "nested doc.html"),
		filepath.Join("v1", "sub dir", "nested doc.html"),
		filepath.Join("v2", "with space.html"),
	} {
		_, err := os.Stat(filepath.Join(outputDir, p))
		assert.NoError(t, err, "expected output file %q", p)
	}

	// Hrefs are percent-escaped so they are valid URLs
	index := readOutput(t, outputDir, "index.html")
	assert.Contains(t, index, `href="/docs/with%20space.html"`)
	assert.Contains(t, index, `href="/docs/sub%20dir/nested%20doc.html"`)

	// llms.txt links are escaped too
	llms := readOutput(t, outputDir, "llms.txt")
	assert.Contains(t, llms, "(/docs/with%20space.html)")

	// And every link still resolves to a real file
	assertNoDeadLinks(t, outputDir)
}

func TestGenerate_SparseVersionAxis(t *testing.T) {
	// A wide software axis (many undocumented versions, semver-ish names with
	// multi-digit components in sort -V order) with few documented versions:
	// only documented dirs are emitted, and the doc page resolves newest.
	tmp := t.TempDir()
	contentDir := filepath.Join(tmp, "content")
	outputDir := filepath.Join(tmp, "site")

	var software []string
	for _, v := range []string{"2.1.9", "2.1.50", "2.1.98", "2.1.104", "2.1.150", "2.1.207"} {
		software = append(software, v)
	}
	writeTestDoc(t, contentDir, "2.1.50", "topic.md", "# Topic\n\nold")
	writeTestDoc(t, contentDir, "2.1.104", "topic.md", "# Topic\n\nnewest words")
	documented := []string{"2.1.50", "2.1.104"}

	g := &Generator{
		Config:             &Config{Project: "sparse", Repo: "https://example.com/sparse"},
		SoftwareVersions:   software,
		VersionMap:         ResolveVersionMap(software, documented),
		DocumentedVersions: documented,
		ContentDir:         contentDir,
		TemplateDir:        "templates",
		OutputDir:          outputDir,
	}
	require.NoError(t, g.Generate())

	// Only documented version dirs exist
	entries, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "static" && e.Name() != "docs" {
			dirs = append(dirs, e.Name())
		}
	}
	assert.ElementsMatch(t, []string{"2.1.50", "2.1.104"}, dirs)

	// Doc page carries the newest authored content (2.1.104 > 2.1.98 positionally)
	doc := readOutput(t, outputDir, "docs", "topic.html")
	assert.Contains(t, doc, "newest words")
	assert.Contains(t, doc, "2.1.104")

	// Current version is the end of the axis even though it is undocumented
	index := readOutput(t, outputDir, "index.html")
	assert.Contains(t, index, "current version 2.1.207")

	assertNoDeadLinks(t, outputDir)
}
