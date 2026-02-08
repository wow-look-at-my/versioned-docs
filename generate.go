package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

type Generator struct {
	Config             *Config
	VersionMap         map[string]string // software version -> docs version
	DocumentedVersions []string          // versions that have authored docs
	ContentDir         string
	TemplateDir        string
	OutputDir          string
	BaseURL            string
}

type DocPage struct {
	Filename string // e.g. "api.md"
	Title    string // derived from first H1 or filename
	Markdown string // raw markdown
	HTML     template.HTML
}

type VersionPageData struct {
	Project          string
	SoftwareVersion  string
	DocsVersion      string
	IsInherited      bool // true if docs come from a different version
	Pages            []DocPage
	CurrentPage      DocPage
	AllVersions      []string
	BaseURL          string
	VersionMap       map[string]string
}

type IndexPageData struct {
	Project    string
	Versions   []VersionEntry
	AllPages   []string                    // all unique page filenames (without .md)
	PageMatrix map[string]map[string]bool  // page -> version -> isAuthored
	BaseURL    string
}

type VersionEntry struct {
	Software   string
	Docs       string
	IsAuthored bool
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

	// Clean output directory before generating
	if err := os.RemoveAll(g.OutputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return err
	}

	// Write static assets (CSS, JS)
	if err := g.writeStaticAssets(); err != nil {
		return fmt.Errorf("writing static assets: %w", err)
	}

	// Load all documented version content
	docContent := make(map[string][]DocPage) // docs version -> pages
	for _, dv := range g.DocumentedVersions {
		pages, err := g.loadVersionContent(dv, md)
		if err != nil {
			return fmt.Errorf("loading content for %s: %w", dv, err)
		}
		docContent[dv] = pages
		fmt.Printf("  Loaded %d pages for %s\n", len(pages), dv)
	}

	// Load templates (use defaults if not present)
	pageTmpl, err := g.loadPageTemplate()
	if err != nil {
		return fmt.Errorf("loading page template: %w", err)
	}

	indexTmpl, err := g.loadIndexTemplate()
	if err != nil {
		return fmt.Errorf("loading index template: %w", err)
	}

	// Generate per-version output
	for _, sv := range g.Config.SoftwareVersions {
		dv := g.VersionMap[sv]
		pages := docContent[dv]
		if len(pages) == 0 {
			continue
		}

		versionDir := filepath.Join(g.OutputDir, sv)
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			return err
		}

		isInherited := sv != dv

		// Generate HTML for each page
		for _, page := range pages {
			data := VersionPageData{
				Project:         g.Config.Project,
				SoftwareVersion: sv,
				DocsVersion:     dv,
				IsInherited:     isInherited,
				Pages:           pages,
				CurrentPage:     page,
				AllVersions:     g.Config.SoftwareVersions,
				BaseURL:         g.BaseURL,
				VersionMap:      g.VersionMap,
			}

			outName := strings.TrimSuffix(page.Filename, ".md") + ".html"
			outPath := filepath.Join(versionDir, outName)

			var buf bytes.Buffer
			if err := pageTmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("rendering %s/%s: %w", sv, outName, err)
			}
			if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
				return err
			}
		}

		// Generate per-version LLM-friendly concatenated markdown
		g.generateVersionLLMDoc(versionDir, sv, dv, isInherited, pages)
	}

	// Generate root index
	var entries []VersionEntry
	for _, sv := range g.Config.SoftwareVersions {
		dv := g.VersionMap[sv]
		entries = append(entries, VersionEntry{
			Software:   sv,
			Docs:       dv,
			IsAuthored: sv == dv,
		})
	}

	// Build page matrix: collect all unique pages and track authored vs inherited
	pageSet := make(map[string]bool)
	for _, pages := range docContent {
		for _, p := range pages {
			pageName := strings.TrimSuffix(p.Filename, ".md")
			pageSet[pageName] = true
		}
	}
	var allPages []string
	for p := range pageSet {
		allPages = append(allPages, p)
	}
	sort.Strings(allPages)

	// For each page, for each version: is it authored (green) or inherited (gray)?
	pageMatrix := make(map[string]map[string]bool) // page -> version -> isAuthored
	for _, pageName := range allPages {
		pageMatrix[pageName] = make(map[string]bool)
		for _, sv := range g.Config.SoftwareVersions {
			dv := g.VersionMap[sv]
			// Check if this page exists for this version's doc source
			pages := docContent[dv]
			hasPage := false
			for _, p := range pages {
				if strings.TrimSuffix(p.Filename, ".md") == pageName {
					hasPage = true
					break
				}
			}
			if hasPage {
				pageMatrix[pageName][sv] = (sv == dv) // true if authored, false if inherited
			}
		}
	}

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, IndexPageData{
		Project:    g.Config.Project,
		Versions:   entries,
		AllPages:   allPages,
		PageMatrix: pageMatrix,
		BaseURL:    g.BaseURL,
	}); err != nil {
		return fmt.Errorf("rendering index: %w", err)
	}
	if err := os.WriteFile(filepath.Join(g.OutputDir, "index.html"), buf.Bytes(), 0o644); err != nil {
		return err
	}

	// Generate root llms.txt
	return g.generateLLMSIndex()
}

func (g *Generator) loadVersionContent(version string, md goldmark.Markdown) ([]DocPage, error) {
	dir := filepath.Join(g.ContentDir, version)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var pages []DocPage
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}

		var htmlBuf bytes.Buffer
		if err := md.Convert(data, &htmlBuf); err != nil {
			return nil, fmt.Errorf("converting %s: %w", e.Name(), err)
		}

		title := extractTitle(string(data), e.Name())

		pages = append(pages, DocPage{
			Filename: e.Name(),
			Title:    title,
			Markdown: string(data),
			HTML:     template.HTML(htmlBuf.String()),
		})
	}

	// Sort: index.md first, then alphabetical
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

func (g *Generator) generateVersionLLMDoc(dir, sv, dv string, inherited bool, pages []DocPage) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s — Documentation for version %s\n\n", g.Config.Project, sv))
	if inherited {
		b.WriteString(fmt.Sprintf("> Note: These docs were authored for version %s and may not reflect changes in %s.\n\n", dv, sv))
	}
	b.WriteString(fmt.Sprintf("Source: %s\n\n", g.Config.Repo))
	b.WriteString("---\n\n")

	for _, page := range pages {
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

func (g *Generator) generateLLMSIndex() error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", g.Config.Project))
	b.WriteString(fmt.Sprintf("> %s\n\n", g.Config.Repo))
	b.WriteString("Reverse-engineered documentation.\n\n")
	b.WriteString("## Versions\n\n")

	for _, sv := range g.Config.SoftwareVersions {
		dv := g.VersionMap[sv]
		marker := ""
		if sv != dv {
			marker = fmt.Sprintf(" (using docs from %s)", dv)
		}
		url := fmt.Sprintf("%s/%s/llms-full.md", g.BaseURL, sv)
		b.WriteString(fmt.Sprintf("- [%s](%s)%s\n", sv, url, marker))
	}

	return os.WriteFile(filepath.Join(g.OutputDir, "llms.txt"), []byte(b.String()), 0o644)
}

var funcMap = template.FuncMap{
	"trimmd": func(s string) string { return strings.TrimSuffix(s, ".md") },
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
