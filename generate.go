package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
)

type Generator struct {
	Config             *Config
	SoftwareVersions   []string
	VersionMap         map[string]string // software version -> default docs version (legacy, kept for display)
	DocumentedVersions []string
	ContentDir         string
	TemplateDir        string
	OutputDir          string
	BaseURL            string
}

type DocPage struct {
	Filename      string
	Title         string
	Markdown      string
	HTML          template.HTML
	SourceVersion string // which doc version this page came from
}

type VersionPageData struct {
	Project          string
	SoftwareVersion  string
	DocsVersion      string // source version for CurrentPage
	IsInherited      bool   // true if CurrentPage.SourceVersion != SoftwareVersion
	TerminatedAt     string // termination version of CurrentPage, "" when not terminated
	IsRemoved        bool   // CurrentPage is terminated before the current version
	Pages            []DocPage
	CurrentPage      DocPage
	CurrentPageChain []string // versions with authored content for CurrentPage, newest first
	PageVersions     []string // documented versions where CurrentPage is visible (select targets)
	CurrentVersion   string   // newest software version
	DocURL           string   // URL of CurrentPage's logical (doc-centric) page
	BaseURL          string
}

func (g *Generator) Generate() error {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
			),
		),
	)

	if err := os.RemoveAll(g.OutputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return err
	}

	if err := g.writeStaticAssets(); err != nil {
		return fmt.Errorf("writing static assets: %w", err)
	}

	// Load all pages from all documented versions
	// docContent[docVersion][pageName] = DocPage
	docContent := make(map[string]map[string]DocPage)
	for _, dv := range g.DocumentedVersions {
		pages, err := g.loadVersionContent(dv, md)
		if err != nil {
			return fmt.Errorf("loading content for %s: %w", dv, err)
		}
		docContent[dv] = make(map[string]DocPage)
		for _, p := range pages {
			docContent[dv][p.Filename] = p
		}
		fmt.Printf("  Loaded %d pages for %s\n", len(pages), dv)
	}

	// Build pageVersions: pageName -> set of doc versions that have it
	pageVersions := make(map[string]map[string]bool)
	for dv, pages := range docContent {
		for pageName := range pages {
			if pageVersions[pageName] == nil {
				pageVersions[pageName] = make(map[string]bool)
			}
			pageVersions[pageName][dv] = true
		}
	}

	// Validate the tombstone config against the version list and doc set.
	term, err := newTermination(g.SoftwareVersions, g.Config.Terminated, pageVersions)
	if err != nil {
		return err
	}

	chains := buildDocChains(g.SoftwareVersions, pageVersions)
	current := g.SoftwareVersions[len(g.SoftwareVersions)-1]

	// Collect all unique page names
	var allPageNames []string
	for pageName := range pageVersions {
		allPageNames = append(allPageNames, pageName)
	}
	sortDocPaths(allPageNames)

	pageTmpl, err := g.loadPageTemplate()
	if err != nil {
		return fmt.Errorf("loading page template: %w", err)
	}

	indexTmpl, err := g.loadIndexTemplate()
	if err != nil {
		return fmt.Errorf("loading index template: %w", err)
	}

	docTmpl, err := g.loadDocTemplate()
	if err != nil {
		return fmt.Errorf("loading doc template: %w", err)
	}

	versionsTmpl, err := g.loadVersionsTemplate()
	if err != nil {
		return fmt.Errorf("loading versions template: %w", err)
	}

	// Generate per-version (history) output with page-level inheritance.
	// Only documented versions get a directory: an undocumented version
	// renders byte-identical content to the nearest documented one, and the
	// version pickers only link authored versions.
	for _, sv := range g.DocumentedVersions {
		// Resolve each page independently
		var pages []DocPage
		for _, pageName := range allPageNames {
			if !term.visibleAt(pageName, sv) {
				continue // terminated before this version
			}
			sourceVersion := ResolvePageVersion(sv, pageName, g.SoftwareVersions, pageVersions[pageName])
			if sourceVersion == "" {
				continue // no version has this page (shouldn't happen)
			}
			page := docContent[sourceVersion][pageName]
			page.SourceVersion = sourceVersion
			pages = append(pages, page)
		}

		if len(pages) == 0 {
			continue
		}

		versionDir := filepath.Join(g.OutputDir, sv)
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			return err
		}

		for _, page := range pages {
			chain := chains[page.Filename]
			chainDesc := make([]string, len(chain))
			for i, v := range chain {
				chainDesc[len(chain)-1-i] = v
			}

			// The version select only offers documented versions where this
			// page is visible: the same page exists there, so switching
			// versions keeps the reader on the same document.
			var pageVersionsVisible []string
			for _, dv := range g.DocumentedVersions {
				if term.visibleAt(page.Filename, dv) {
					pageVersionsVisible = append(pageVersionsVisible, dv)
				}
			}

			data := VersionPageData{
				Project:          g.Config.Project,
				SoftwareVersion:  sv,
				DocsVersion:      page.SourceVersion,
				IsInherited:      page.SourceVersion != sv,
				TerminatedAt:     term.terminatedAt(page.Filename),
				IsRemoved:        term.removedBefore(page.Filename, current),
				Pages:            pages,
				CurrentPage:      page,
				CurrentPageChain: chainDesc,
				PageVersions:     pageVersionsVisible,
				CurrentVersion:   current,
				DocURL:           g.BaseURL + "/docs/" + strings.TrimSuffix(page.Filename, ".md") + ".html",
				BaseURL:          g.BaseURL,
			}

			outName := strings.TrimSuffix(page.Filename, ".md") + ".html"
			outPath := filepath.Join(versionDir, outName)

			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}

			var buf bytes.Buffer
			if err := pageTmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("rendering %s/%s: %w", sv, outName, err)
			}
			if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
				return err
			}
		}

		g.generateVersionLLMDoc(versionDir, sv, pages)
	}

	// Doc-centric outputs: the logical doc pages (newest authored content
	// per doc), the root index, and the version-history matrix.
	docs, removedDocs := buildDocEntries(chains, docContent, term, current)

	if err := g.generateDocPages(docTmpl, docContent, chains, term, current, docs, removedDocs); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, IndexPageData{
		Project:        g.Config.Project,
		Repo:           g.Config.Repo,
		CurrentVersion: current,
		Docs:           docs,
		RemovedDocs:    removedDocs,
		BaseURL:        g.BaseURL,
	}); err != nil {
		return fmt.Errorf("rendering index: %w", err)
	}
	if err := os.WriteFile(filepath.Join(g.OutputDir, "index.html"), buf.Bytes(), 0o644); err != nil {
		return err
	}

	var vbuf bytes.Buffer
	if err := versionsTmpl.Execute(&vbuf, g.buildVersionsPage(allPageNames, pageVersions, docContent, chains, term, current)); err != nil {
		return fmt.Errorf("rendering versions: %w", err)
	}
	if err := os.WriteFile(filepath.Join(g.OutputDir, "versions.html"), vbuf.Bytes(), 0o644); err != nil {
		return err
	}

	if err := g.generateLLMSFull(docs, docContent, current); err != nil {
		return err
	}

	return g.generateLLMSIndex(docs, removedDocs, current)
}

func (g *Generator) loadVersionContent(version string, md goldmark.Markdown) ([]DocPage, error) {
	dir := filepath.Join(g.ContentDir, version)

	// Check directory exists before walking
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	var pages []DocPage
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var htmlBuf bytes.Buffer
		if err := md.Convert(data, &htmlBuf); err != nil {
			return fmt.Errorf("converting %s: %w", relPath, err)
		}

		title := extractTitle(string(data), filepath.Base(relPath))

		pages = append(pages, DocPage{
			Filename: relPath,
			Title:    title,
			Markdown: string(data),
			HTML:     template.HTML(htmlBuf.String()),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Filename == "index.md" {
			return true
		}
		if pages[j].Filename == "index.md" {
			return false
		}
		return pages[i].Filename < pages[j].Filename
	})

	return pages, nil
}

func extractTitle(markdown, filename string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return strings.TrimSuffix(filename, ".md")
}

func (g *Generator) generateVersionLLMDoc(dir, sv string, pages []DocPage) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s — Documentation for version %s\n\n", g.Config.Project, sv))
	b.WriteString(fmt.Sprintf("Source: %s\n\n", g.Config.Repo))
	b.WriteString("---\n\n")

	for _, page := range pages {
		if page.SourceVersion != sv {
			b.WriteString(fmt.Sprintf("> Note: This page was authored for version %s.\n\n", page.SourceVersion))
		}
		b.WriteString(page.Markdown)
		b.WriteString("\n\n---\n\n")
	}

	os.WriteFile(filepath.Join(dir, "llms-full.md"), []byte(b.String()), 0o644)
}

func (g *Generator) writeStaticAssets() error {
	staticDir := filepath.Join(g.OutputDir, "static")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staticDir, "style.css"), defaultCSS, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staticDir, "script.js"), defaultJS, 0o644); err != nil {
		return err
	}
	return nil
}

// generateLLMSIndex writes the root llms.txt: the doc-centric listing (each
// doc's newest URL, terminated docs split into a Removed section) plus the
// per-version llms-full.md links for documented versions.
func (g *Generator) generateLLMSIndex(docs, removedDocs []DocIndexEntry, current string) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", g.Config.Project))
	b.WriteString(fmt.Sprintf("> %s\n\n", g.Config.Repo))
	b.WriteString(fmt.Sprintf("Reverse-engineered documentation. Current software version: %s.\n\n", current))

	b.WriteString("## Documents\n\n")
	b.WriteString(fmt.Sprintf("Newest authored revision of every current document. Full bundle: [llms-full.md](%s/llms-full.md)\n\n", g.BaseURL))
	for _, e := range docs {
		b.WriteString(fmt.Sprintf("- [%s](%s/%s) — newest content authored for %s\n", e.Title, g.BaseURL, e.URLPath(), e.NewestVersion))
	}

	if len(removedDocs) > 0 {
		b.WriteString("\n## Removed documents\n\n")
		for _, e := range removedDocs {
			b.WriteString(fmt.Sprintf("- [%s](%s/%s) — last applies to version %s\n", e.Title, g.BaseURL, e.URLPath(), e.TerminatedAt))
		}
	}

	b.WriteString("\n## Versions\n\n")
	for _, dv := range g.DocumentedVersions {
		b.WriteString(fmt.Sprintf("- [%s](%s/%s/llms-full.md)\n", dv, g.BaseURL, dv))
	}

	return os.WriteFile(filepath.Join(g.OutputDir, "llms.txt"), []byte(b.String()), 0o644)
}

var funcMap = template.FuncMap{
	"trimmd":   func(s string) string { return strings.TrimSuffix(s, ".md") },
	"basename": func(s string) string { return filepath.Base(s) },
	"dirname": func(s string) string {
		d := filepath.Dir(s)
		if d == "." {
			return ""
		}
		return d
	},
}

func (g *Generator) loadPageTemplate() (*template.Template, error) {
	path := filepath.Join(g.TemplateDir, "page.html")
	if data, err := os.ReadFile(path); err == nil {
		return template.New("page").Funcs(funcMap).Parse(string(data))
	}
	return template.New("page").Funcs(funcMap).Parse(string(defaultPageTemplateBytes))
}

func (g *Generator) loadIndexTemplate() (*template.Template, error) {
	path := filepath.Join(g.TemplateDir, "index.html")
	if data, err := os.ReadFile(path); err == nil {
		return template.New("index").Funcs(funcMap).Parse(string(data))
	}
	return template.New("index").Funcs(funcMap).Parse(string(defaultIndexTemplateBytes))
}

func (g *Generator) loadDocTemplate() (*template.Template, error) {
	path := filepath.Join(g.TemplateDir, "doc.html")
	if data, err := os.ReadFile(path); err == nil {
		return template.New("doc").Funcs(funcMap).Parse(string(data))
	}
	return template.New("doc").Funcs(funcMap).Parse(string(defaultDocTemplateBytes))
}

func (g *Generator) loadVersionsTemplate() (*template.Template, error) {
	path := filepath.Join(g.TemplateDir, "versions.html")
	if data, err := os.ReadFile(path); err == nil {
		return template.New("versions").Funcs(funcMap).Parse(string(data))
	}
	return template.New("versions").Funcs(funcMap).Parse(string(defaultVersionsTemplateBytes))
}
