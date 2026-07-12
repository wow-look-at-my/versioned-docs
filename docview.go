package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DocIndexEntry describes one logical document (a doc path grouped across all
// versions that author it) for doc-centric listings.
type DocIndexEntry struct {
	Path          string // content-relative path without .md, forward slashes
	Title         string
	NewestVersion string // newest version with authored content
	NumVersions   int    // number of versions with authored content
	TerminatedAt  string // termination version, "" when not terminated
}

// URLPath returns the site-relative path of the logical doc page, with each
// segment percent-escaped so weird filenames survive as valid URLs.
func (e DocIndexEntry) URLPath() string {
	return "docs/" + escapePath(e.Path) + ".html"
}

// DocPageData is the template payload for a logical (doc-centric) page.
type DocPageData struct {
	Project        string
	Path           string // without .md
	Title          string
	HTML           template.HTML
	NewestVersion  string   // the authored version whose content is shown
	Chain          []string // authored versions, newest first
	TerminatedAt   string   // "" when not terminated
	IsRemoved      bool     // terminated before the current version
	CurrentVersion string
	Docs           []DocIndexEntry
	RemovedDocs    []DocIndexEntry
	BaseURL        string
}

// IndexPageData is the template payload for the doc-centric root index.
type IndexPageData struct {
	Project        string
	Repo           string
	CurrentVersion string
	Docs           []DocIndexEntry
	RemovedDocs    []DocIndexEntry
	BaseURL        string
}

// VersionsPageData is the template payload for the version-history matrix.
type VersionsPageData struct {
	Project        string
	CurrentVersion string
	Columns        []string // documented versions
	Rows           []MatrixRow
	Ranges         []VersionRange
	BaseURL        string
}

type MatrixRow struct {
	Path  string // without .md
	Title string
	Cells []MatrixCell
}

type MatrixCell struct {
	Version string
	State   string // "authored", "inherited", or "" (absent/terminated)
}

// VersionRange summarizes which consecutive software versions display docs
// from the same documented version.
type VersionRange struct {
	Docs  string // the documented version providing the docs
	First string // first software version in the range
	Last  string // last software version in the range
}

// escapePath percent-escapes a forward-slash path for use in URLs while
// keeping the slashes.
func escapePath(p string) string {
	return (&url.URL{Path: p}).EscapedPath()
}

// buildDocChains returns, for each doc path, the ordered (oldest to newest,
// following softwareVersions order) list of versions that author it.
func buildDocChains(softwareVersions []string, pageVersions map[string]map[string]bool) map[string][]string {
	chains := make(map[string][]string, len(pageVersions))
	for p, set := range pageVersions {
		for _, v := range softwareVersions {
			if set[v] {
				chains[p] = append(chains[p], v)
			}
		}
	}
	return chains
}

// sortDocPaths sorts doc paths with index.md first, then lexicographically.
func sortDocPaths(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		if paths[i] == "index.md" {
			return true
		}
		if paths[j] == "index.md" {
			return false
		}
		return paths[i] < paths[j]
	})
}

// buildDocEntries splits all logical docs into the default listing and the
// removed listing. A doc is "removed" when it is terminated strictly before
// the current version; a doc terminated AT the current version still applies
// and stays in the default listing.
func buildDocEntries(chains map[string][]string, docContent map[string]map[string]DocPage, term *termination, current string) (docs, removed []DocIndexEntry) {
	paths := make([]string, 0, len(chains))
	for p := range chains {
		paths = append(paths, p)
	}
	sortDocPaths(paths)

	for _, p := range paths {
		chain := chains[p]
		newest := chain[len(chain)-1]
		entry := DocIndexEntry{
			Path:          strings.TrimSuffix(p, ".md"),
			Title:         docContent[newest][p].Title,
			NewestVersion: newest,
			NumVersions:   len(chain),
			TerminatedAt:  term.terminatedAt(p),
		}
		if term.removedBefore(p, current) {
			removed = append(removed, entry)
		} else {
			docs = append(docs, entry)
		}
	}
	return docs, removed
}

// generateDocPages writes the logical doc page for every doc chain to
// docs/<path>.html. The content shown is the newest authored version.
func (g *Generator) generateDocPages(tmpl *template.Template, docContent map[string]map[string]DocPage, chains map[string][]string, term *termination, current string, docs, removed []DocIndexEntry) error {
	for p, chain := range chains {
		newest := chain[len(chain)-1]
		page := docContent[newest][p]

		chainDesc := make([]string, len(chain))
		for i, v := range chain {
			chainDesc[len(chain)-1-i] = v
		}

		data := DocPageData{
			Project:        g.Config.Project,
			Path:           strings.TrimSuffix(p, ".md"),
			Title:          page.Title,
			HTML:           page.HTML,
			NewestVersion:  newest,
			Chain:          chainDesc,
			TerminatedAt:   term.terminatedAt(p),
			IsRemoved:      term.removedBefore(p, current),
			CurrentVersion: current,
			Docs:           docs,
			RemovedDocs:    removed,
			BaseURL:        g.BaseURL,
		}

		outPath := filepath.Join(g.OutputDir, "docs", strings.TrimSuffix(p, ".md")+".html")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("rendering doc page %s: %w", p, err)
		}
		if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// buildVersionsPage assembles the version-history matrix over documented
// versions plus the version-range summary.
func (g *Generator) buildVersionsPage(allPageNames []string, pageVersions map[string]map[string]bool, docContent map[string]map[string]DocPage, chains map[string][]string, term *termination, current string) VersionsPageData {
	var rows []MatrixRow
	for _, p := range allPageNames {
		chain := chains[p]
		newest := chain[len(chain)-1]
		row := MatrixRow{
			Path:  strings.TrimSuffix(p, ".md"),
			Title: docContent[newest][p].Title,
		}
		for _, dv := range g.DocumentedVersions {
			cell := MatrixCell{Version: dv}
			if term.visibleAt(p, dv) {
				source := ResolvePageVersion(dv, p, g.SoftwareVersions, pageVersions[p])
				switch source {
				case "":
					// unreachable: every page resolves somewhere
				case dv:
					cell.State = "authored"
				default:
					cell.State = "inherited"
				}
			}
			row.Cells = append(row.Cells, cell)
		}
		rows = append(rows, row)
	}

	return VersionsPageData{
		Project:        g.Config.Project,
		CurrentVersion: current,
		Columns:        g.DocumentedVersions,
		Rows:           rows,
		Ranges:         buildVersionRanges(g.SoftwareVersions, g.VersionMap),
		BaseURL:        g.BaseURL,
	}
}

// buildVersionRanges groups consecutive software versions that display docs
// from the same documented version.
func buildVersionRanges(softwareVersions []string, vmap map[string]string) []VersionRange {
	var ranges []VersionRange
	for _, sv := range softwareVersions {
		docs := vmap[sv]
		if n := len(ranges); n > 0 && ranges[n-1].Docs == docs {
			ranges[n-1].Last = sv
			continue
		}
		ranges = append(ranges, VersionRange{Docs: docs, First: sv, Last: sv})
	}
	return ranges
}

// generateLLMSFull writes the root llms-full.md: the newest authored content
// of every doc in the default listing (removed docs are excluded, consistent
// with the doc-centric index).
func (g *Generator) generateLLMSFull(docs []DocIndexEntry, docContent map[string]map[string]DocPage, current string) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s — Documentation (current version %s)\n\n", g.Config.Project, current))
	b.WriteString(fmt.Sprintf("Source: %s\n\n", g.Config.Repo))
	b.WriteString("Each document below is its newest authored revision.\n\n---\n\n")

	for _, e := range docs {
		page := docContent[e.NewestVersion][e.Path+".md"]
		if e.NewestVersion != current {
			b.WriteString(fmt.Sprintf("> Note: This page was authored for version %s.\n\n", e.NewestVersion))
		}
		b.WriteString(page.Markdown)
		b.WriteString("\n\n---\n\n")
	}

	return os.WriteFile(filepath.Join(g.OutputDir, "llms-full.md"), []byte(b.String()), 0o644)
}
