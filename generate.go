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
	return template.New("page").Funcs(funcMap).Parse(defaultPageTemplate)
}

func (g *Generator) loadIndexTemplate() (*template.Template, error) {
	path := filepath.Join(g.TemplateDir, "index.html")
	if data, err := os.ReadFile(path); err == nil {
		return template.New("index").Funcs(funcMap).Parse(string(data))
	}
	return template.New("index").Funcs(funcMap).Parse(defaultIndexTemplate)
}

var defaultPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Project}} {{.SoftwareVersion}} — {{.CurrentPage.Title}}</title>
<style>
  :root { --bg: #1a1a2e; --surface: #16213e; --text: #e0e0e0; --accent: #0f3460; --link: #53a8b6; --warn: #e2b93b; }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: var(--text); line-height: 1.6; }
  .layout { display: grid; grid-template-columns: 240px 1fr; min-height: 100vh; }
  nav { background: var(--surface); padding: 1.5rem; border-right: 1px solid rgba(255,255,255,0.05); }
  nav h2 { font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.1em; color: #888; margin-bottom: 0.75rem; }
  nav a { display: block; color: var(--link); text-decoration: none; padding: 0.3rem 0; font-size: 0.95rem; }
  nav a:hover { text-decoration: underline; }
  nav a.active { font-weight: bold; color: #fff; }
  .version-select { margin-bottom: 1.5rem; }
  .version-select select { width: 100%; padding: 0.4rem; background: var(--accent); color: var(--text); border: 1px solid rgba(255,255,255,0.1); border-radius: 4px; }
  main { padding: 2rem 3rem; max-width: 900px; }
  .inherited-banner { background: rgba(226,185,59,0.15); border: 1px solid var(--warn); border-radius: 6px; padding: 0.75rem 1rem; margin-bottom: 1.5rem; font-size: 0.9rem; color: var(--warn); }
  main h1 { font-size: 1.8rem; margin-bottom: 1rem; }
  main h2 { font-size: 1.4rem; margin-top: 2rem; margin-bottom: 0.5rem; }
  main h3 { font-size: 1.15rem; margin-top: 1.5rem; margin-bottom: 0.5rem; }
  main p { margin-bottom: 1rem; }
  main pre { background: var(--surface); padding: 1rem; border-radius: 6px; overflow-x: auto; margin-bottom: 1rem; }
  main code { font-family: "JetBrains Mono", "Fira Code", monospace; font-size: 0.9em; }
  main :not(pre) > code { background: var(--surface); padding: 0.15rem 0.4rem; border-radius: 3px; }
  main a { color: var(--link); }
  main table { border-collapse: collapse; margin-bottom: 1rem; width: 100%; }
  main th, main td { border: 1px solid rgba(255,255,255,0.1); padding: 0.5rem 0.75rem; text-align: left; }
  main th { background: var(--surface); }
  .llm-link { margin-top: 2rem; padding-top: 1rem; border-top: 1px solid rgba(255,255,255,0.05); font-size: 0.85rem; color: #888; }
  .llm-link a { color: var(--link); }
</style>
</head>
<body>
<div class="layout">
  <nav>
    <div class="version-select">
      <h2>Version</h2>
      <select onchange="if(this.value)window.location.href=this.value">
        {{range .AllVersions -}}
        <option value="{{$.BaseURL}}/{{.}}/index.html" {{if eq . $.SoftwareVersion}}selected{{end}}>
          {{.}}{{if ne . (index $.VersionMap .)}} (docs: {{index $.VersionMap .}}){{end}}
        </option>
        {{end -}}
      </select>
    </div>
    <h2>Pages</h2>
    {{range .Pages -}}
    <a href="{{$.BaseURL}}/{{$.SoftwareVersion}}/{{.Filename | trimmd}}.html"
       {{if eq .Filename $.CurrentPage.Filename}}class="active"{{end}}>
      {{.Title}}
    </a>
    {{end -}}
  </nav>
  <main>
    {{if .IsInherited -}}
    <div class="inherited-banner">
      📋 These docs were written for version <strong>{{.DocsVersion}}</strong>.
      No specific documentation exists for {{.SoftwareVersion}} yet.
    </div>
    {{end -}}
    {{.CurrentPage.HTML}}
    <div class="llm-link">
      🤖 <a href="{{.BaseURL}}/{{.SoftwareVersion}}/llms-full.md">LLM-friendly version</a>
      · <a href="{{.BaseURL}}/llms.txt">llms.txt</a>
    </div>
  </main>
</div>
<script>
// Handle relative page links within the same version
document.querySelectorAll('main a[href$=".md"]').forEach(a => {
  const href = a.getAttribute('href');
  if (!href.startsWith('http')) {
    a.href = href.replace(/\.md$/, '.html');
  }
});
</script>
</body>
</html>`

var defaultIndexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Project}} — Documentation</title>
<style>
  :root { --bg: #1a1a2e; --surface: #16213e; --text: #e0e0e0; --link: #53a8b6; --ok: #4caf50; --inherit: #e2b93b; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: var(--text); padding: 3rem; line-height: 1.6; }
  .container { max-width: 100%; margin: 0 auto; }
  h1 { margin-bottom: 0.5rem; }
  h2 { margin-top: 2rem; margin-bottom: 1rem; font-size: 1.2rem; color: #888; }
  .subtitle { color: #888; margin-bottom: 2rem; }
  table { border-collapse: collapse; margin-bottom: 2rem; }
  th, td { text-align: center; padding: 0.5rem 0.75rem; border: 1px solid rgba(255,255,255,0.1); }
  th { background: var(--surface); color: #888; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.05em; }
  td.page-name { text-align: left; font-weight: 500; }
  a { color: var(--link); text-decoration: none; }
  a:hover { text-decoration: underline; }
  .cell-authored { background: rgba(76,175,80,0.2); }
  .cell-authored a { color: var(--ok); }
  .cell-inherited { background: rgba(226,185,59,0.1); }
  .cell-inherited a { color: var(--inherit); }
  .cell-empty { background: rgba(255,255,255,0.02); color: #444; }
  .legend { display: flex; gap: 1.5rem; margin-bottom: 1rem; font-size: 0.85rem; }
  .legend-item { display: flex; align-items: center; gap: 0.4rem; }
  .legend-box { width: 1rem; height: 1rem; border-radius: 2px; }
  .legend-authored { background: rgba(76,175,80,0.4); }
  .legend-inherited { background: rgba(226,185,59,0.3); }
  .llm-section { margin-top: 2rem; padding-top: 1rem; border-top: 1px solid rgba(255,255,255,0.1); font-size: 0.9rem; color: #888; }
</style>
</head>
<body>
<div class="container">
  <h1>{{.Project}}</h1>
  <p class="subtitle">Versioned documentation matrix</p>

  <div class="legend">
    <div class="legend-item"><div class="legend-box legend-authored"></div> Authored</div>
    <div class="legend-item"><div class="legend-box legend-inherited"></div> Inherited</div>
  </div>

  <table>
    <thead>
      <tr>
        <th>Page</th>
        {{range .Versions -}}
        <th>{{.Software}}</th>
        {{end -}}
      </tr>
    </thead>
    <tbody>
    {{range $page := .AllPages -}}
    <tr>
      <td class="page-name">{{$page}}</td>
      {{range $v := $.Versions -}}
      {{$isAuthored := index (index $.PageMatrix $page) $v.Software -}}
      {{if eq $isAuthored true -}}
      <td class="cell-authored"><a href="{{$.BaseURL}}/{{$v.Software}}/{{$page}}.html">✓</a></td>
      {{else if eq $isAuthored false -}}
      <td class="cell-inherited"><a href="{{$.BaseURL}}/{{$v.Software}}/{{$page}}.html">↓</a></td>
      {{else -}}
      <td class="cell-empty">—</td>
      {{end -}}
      {{end -}}
    </tr>
    {{end -}}
    </tbody>
  </table>

  <h2>Version Sources</h2>
  <table>
    <thead><tr><th>Version</th><th>Docs from</th></tr></thead>
    <tbody>
    {{range .Versions -}}
    <tr>
      <td><a href="{{$.BaseURL}}/{{.Software}}/index.html">{{.Software}}</a></td>
      <td>{{if .IsAuthored}}authored{{else}}{{.Docs}}{{end}}</td>
    </tr>
    {{end -}}
    </tbody>
  </table>

  <div class="llm-section">
    🤖 For LLM consumption: <a href="{{.BaseURL}}/llms.txt">llms.txt</a>
  </div>
</div>
</body>
</html>`
