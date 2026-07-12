package main

import (
	"archive/tar"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// fetch-aggregate adapts a docs-aggregate style corpus (a git branch laid out
// as <version>/docs/<file>.md, as produced by claude-docs-gaps' aggregate-docs
// tool) into this tool's input model:
//
//   - <content>/<version>/<file>.md   (the docs/ path level is stripped)
//   - <versions-out>                  (all semver version branches of the
//     remote, ascending — the software versions list)
//
// The remote may be a network URL or a local path. Version branches are
// discovered from refs/heads/<X.Y.Z> and refs/remotes/*/<X.Y.Z> so a local
// clone with only remote-tracking refs works without network access. Any
// version present in the corpus tree is unioned into the version list even if
// its branch has disappeared, so content validation never trips on a deleted
// branch.

type fetchOptions struct {
	Remote      string
	Branch      string
	ContentDir  string
	VersionsOut string
}

func runFetchAggregate(args []string) error {
	fs := flag.NewFlagSet("fetch-aggregate", flag.ContinueOnError)
	remote := fs.String("remote", "", "git remote URL or local path of the corpus repository (required)")
	branch := fs.String("branch", "docs-aggregate", "branch containing the aggregated docs tree")
	contentDir := fs.String("content", "content", "output content directory (wiped and recreated)")
	versionsOut := fs.String("versions-out", "versions.txt", "output file for the ascending software version list")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *remote == "" {
		return fmt.Errorf("fetch-aggregate: -remote is required")
	}
	return fetchAggregate(fetchOptions{
		Remote:      *remote,
		Branch:      *branch,
		ContentDir:  *contentDir,
		VersionsOut: *versionsOut,
	})
}

func fetchAggregate(opts fetchOptions) error {
	branchVersions, err := listSemverVersions(opts.Remote)
	if err != nil {
		return err
	}

	contentVersions, docCount, err := extractAggregateTree(opts)
	if err != nil {
		return err
	}
	if docCount == 0 {
		return fmt.Errorf("fetch-aggregate: branch %q of %s contains no <version>/docs/*.md files", opts.Branch, sanitizeCredentials(opts.Remote))
	}

	// Union: every version present in the corpus stays in the list even if
	// its branch is gone; branch-only versions extend the version axis past
	// the newest documented version (they anchor "current").
	union := make(map[string]bool, len(branchVersions)+len(contentVersions))
	for _, v := range branchVersions {
		union[v] = true
	}
	for _, v := range contentVersions {
		union[v] = true
	}
	versions := make([]string, 0, len(union))
	for v := range union {
		versions = append(versions, v)
	}
	if len(versions) == 0 {
		return fmt.Errorf("fetch-aggregate: no semver versions found at %s", sanitizeCredentials(opts.Remote))
	}
	sortSemverAsc(versions)

	var b strings.Builder
	for _, v := range versions {
		b.WriteString(v)
		b.WriteString("\n")
	}
	if err := os.WriteFile(opts.VersionsOut, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("fetch-aggregate: writing %s: %w", opts.VersionsOut, err)
	}

	fmt.Printf("fetch-aggregate: %d docs across %d documented versions; %d software versions -> %s\n",
		docCount, len(contentVersions), len(versions), opts.VersionsOut)
	return nil
}

// listSemverVersions lists X.Y.Z version names advertised by the remote,
// matching both local branches (refs/heads/) and remote-tracking refs
// (refs/remotes/<name>/) so local clones work offline.
func listSemverVersions(remote string) ([]string, error) {
	out, err := runGit("", "ls-remote", remote)
	if err != nil {
		return nil, fmt.Errorf("fetch-aggregate: listing versions of %s failed: %w\nhint: if the repository is private the remote URL needs a credential that can read it (in CI, secret PRIVATE_ORG_REPO_READ; locally, point -remote at a local clone)", sanitizeCredentials(remote), err)
	}

	refRe := regexp.MustCompile(`refs/(?:heads|remotes/[^/]+)/(\d+\.\d+\.\d+)$`)
	seen := make(map[string]bool)
	var versions []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		m := refRe.FindStringSubmatch(fields[1])
		if m == nil || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		versions = append(versions, m[1])
	}
	return versions, nil
}

// extractAggregateTree fetches the aggregate branch and extracts every
// <version>/docs/<file>.md into <content>/<version>/<file>.md. Returns the
// versions that have docs and the number of doc files written.
func extractAggregateTree(opts fetchOptions) ([]string, int, error) {
	tmp, err := os.MkdirTemp("", "fetch-aggregate-*")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(tmp)

	if _, err := runGit(tmp, "init", "-q"); err != nil {
		return nil, 0, err
	}

	// Try the branch name first (network remotes), then the remote-tracking
	// ref (local clones that only fetched, never checked out, the branch).
	var fetchErr error
	fetched := false
	for _, ref := range []string{"refs/heads/" + opts.Branch, "refs/remotes/origin/" + opts.Branch, opts.Branch} {
		if _, fetchErr = runGit(tmp, "fetch", "-q", "--depth=1", opts.Remote, ref); fetchErr == nil {
			fetched = true
			break
		}
	}
	if !fetched {
		return nil, 0, fmt.Errorf("fetch-aggregate: fetching branch %q from %s failed: %w\nhint: if the repository is private the remote URL needs a credential that can read it (in CI, secret PRIVATE_ORG_REPO_READ; locally, run `git fetch origin %s` in the clone first)", opts.Branch, sanitizeCredentials(opts.Remote), fetchErr, opts.Branch)
	}

	tarBytes, err := runGit(tmp, "archive", "--format=tar", "FETCH_HEAD")
	if err != nil {
		return nil, 0, err
	}

	if err := os.RemoveAll(opts.ContentDir); err != nil {
		return nil, 0, err
	}
	if err := os.MkdirAll(opts.ContentDir, 0o755); err != nil {
		return nil, 0, err
	}

	versionSet := make(map[string]bool)
	docCount := 0
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("fetch-aggregate: reading archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		rel, ok := aggregateDocPath(hdr.Name)
		if !ok {
			continue
		}
		dest := filepath.Join(opts.ContentDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, 0, err
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, 0, fmt.Errorf("fetch-aggregate: reading %s from archive: %w", hdr.Name, err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return nil, 0, err
		}
		versionSet[strings.SplitN(rel, "/", 2)[0]] = true
		docCount++
	}

	versions := make([]string, 0, len(versionSet))
	for v := range versionSet {
		versions = append(versions, v)
	}
	sortSemverAsc(versions)
	return versions, docCount, nil
}

// aggregateDocPath maps an archive entry name <version>/docs/<subpath>.md to
// the content-relative path <version>/<subpath>.md. Returns ok=false for
// entries that are not version docs (INDEX.md, summary.json, non-md files)
// or contain unsafe path elements.
func aggregateDocPath(name string) (string, bool) {
	parts := strings.Split(name, "/")
	if len(parts) < 3 || !isSemver(parts[0]) || parts[1] != "docs" || !strings.HasSuffix(name, ".md") {
		return "", false
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", false
		}
	}
	return parts[0] + "/" + strings.Join(parts[2:], "/"), true
}

// runGit executes git with the given args (in dir when non-empty), returning
// stdout. Error messages have credentials scrubbed so tokens embedded in
// remote URLs never reach logs.
func runGit(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %v: %s",
			sanitizeCredentials(strings.Join(args, " ")), err,
			sanitizeCredentials(strings.TrimSpace(errb.String())))
	}
	return out.Bytes(), nil
}

var credentialRe = regexp.MustCompile(`(https?://)[^/\s@]+@`)

// sanitizeCredentials masks userinfo (tokens, passwords) in URLs.
func sanitizeCredentials(s string) string {
	return credentialRe.ReplaceAllString(s, "$1***@")
}

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// isSemver reports whether s is a strict X.Y.Z version triple.
func isSemver(s string) bool {
	return semverRe.MatchString(s)
}

// compareSemver compares two X.Y.Z versions numerically (so 2.1.98 < 2.1.104).
func compareSemver(a, b string) int {
	pa := strings.SplitN(a, ".", 3)
	pb := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			if na < nb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// sortSemverAsc sorts versions ascending (oldest first) — the order the
// generator treats as release order.
func sortSemverAsc(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) < 0
	})
}
