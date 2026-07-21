package main

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSemver(t *testing.T) {
	assert.True(t, isSemver("1.0.0"))
	assert.True(t, isSemver("2.1.207"))
	assert.False(t, isSemver("v1.0.0"))
	assert.False(t, isSemver("1.0"))
	assert.False(t, isSemver("1.0.0-rc1"))
	assert.False(t, isSemver("master"))
}

func TestCompareSemver(t *testing.T) {
	assert.Equal(t, 0, compareSemver("1.2.3", "1.2.3"))
	assert.Equal(t, -1, compareSemver("1.2.3", "1.2.4"))
	assert.Equal(t, 1, compareSemver("2.0.0", "1.99.99"))

	// Multi-digit components compare numerically, not lexically
	assert.Equal(t, -1, compareSemver("2.1.98", "2.1.104"))
	assert.Equal(t, -1, compareSemver("2.1.104", "2.1.207"))
	assert.Equal(t, -1, compareSemver("0.2.119", "2.1.32"))
}

func TestSortSemverAsc(t *testing.T) {
	versions := []string{"2.1.104", "0.2.9", "2.1.98", "1.0.0", "2.1.207", "0.2.119"}
	sortSemverAsc(versions)
	assert.Equal(t, []string{"0.2.9", "0.2.119", "1.0.0", "2.1.98", "2.1.104", "2.1.207"}, versions)
}

func TestSortSemverAsc_LargeList(t *testing.T) {
	// Build a large ordered version list, shuffle deterministically, sort,
	// and require the exact original order back.
	var ordered []string
	for major := 0; major < 3; major++ {
		for minor := 0; minor < 5; minor++ {
			for patch := 0; patch < 40; patch += 3 {
				ordered = append(ordered, versionString(major, minor, patch))
			}
		}
	}
	require.Greater(t, len(ordered), 150)

	shuffled := append([]string(nil), ordered...)
	rand.New(rand.NewSource(42)).Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	require.NotEqual(t, ordered, shuffled)

	sortSemverAsc(shuffled)
	assert.Equal(t, ordered, shuffled)
}

func versionString(major, minor, patch int) string {
	return strings.Join([]string{itoa(major), itoa(minor), itoa(patch)}, ".")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestSanitizeCredentials(t *testing.T) {
	assert.Equal(t,
		"https://***@github.com/a/b",
		sanitizeCredentials("https://x-access-token:secret123@github.com/a/b"))
	assert.Equal(t,
		"fatal: unable to access 'https://***@github.com/a/b'",
		sanitizeCredentials("fatal: unable to access 'https://user:pass@github.com/a/b'"))
	assert.Equal(t,
		"https://github.com/a/b",
		sanitizeCredentials("https://github.com/a/b"))
	assert.Equal(t, "/local/path", sanitizeCredentials("/local/path"))
}

func TestAggregateDocPath(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"flat doc", "2.1.160/docs/topic.md", "2.1.160/topic.md", true},
		{"nested doc", "2.1.160/docs/sub/deep.md", "2.1.160/sub/deep.md", true},
		{"weird name", "1.0.0/docs/with space.md", "1.0.0/with space.md", true},
		{"root index", "INDEX.md", "", false},
		{"root summary", "summary.json", "", false},
		{"non-md", "2.1.160/docs/data.json", "", false},
		{"not under docs", "2.1.160/other/topic.md", "", false},
		{"non-semver version", "master/docs/topic.md", "", false},
		{"path traversal", "2.1.160/docs/../evil.md", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := aggregateDocPath(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// gitT runs git in dir, failing the test on error.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{
		"-c", "user.name=test", "-c", "user.email=test@test", "-c", "init.defaultBranch=main",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return string(out)
}

// initCorpusRepo builds a local git repo shaped like claude-docs-gaps:
// a docs-aggregate branch whose tree is <version>/docs/<file>.md (+ root
// INDEX.md / summary.json), and semver branches for the version axis
// (1.10.0 has a branch but no docs).
func initCorpusRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitT(t, dir, "init", "-q")

	files := map[string]string{
		"1.0.0/docs/topic.md":       "# Topic\n\noldest words",
		"1.2.0/docs/topic.md":       "# Topic\n\nnewer words",
		"1.2.0/docs/nested/deep.md": "# Deep",
		"1.2.0/docs/notes.txt":      "not a doc",
		"INDEX.md":                  "# Index of docs",
		"summary.json":              "{}",
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "corpus")

	// The aggregate tree lives on its own branch; version branches only
	// contribute their names to the version axis.
	gitT(t, dir, "branch", "docs-aggregate")
	gitT(t, dir, "branch", "1.0.0")
	gitT(t, dir, "branch", "1.2.0")
	gitT(t, dir, "branch", "1.10.0")
	gitT(t, dir, "branch", "not-a-version")

	return dir
}

func TestFetchAggregate_EndToEnd(t *testing.T) {
	corpus := initCorpusRepo(t)
	work := t.TempDir()
	contentDir := filepath.Join(work, "content")
	versionsOut := filepath.Join(work, "versions.txt")

	require.NoError(t, runFetchAggregate([]string{
		"-remote", corpus,
		"-content", contentDir,
		"-versions-out", versionsOut,
	}))

	// Docs are reshaped to <content>/<version>/<file>.md (docs/ level stripped)
	data, err := os.ReadFile(filepath.Join(contentDir, "1.0.0", "topic.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "oldest words")

	_, err = os.Stat(filepath.Join(contentDir, "1.2.0", "nested", "deep.md"))
	assert.NoError(t, err, "nested docs must keep their subpath")

	// Non-docs are skipped
	for _, absent := range []string{"INDEX.md", "summary.json", filepath.Join("1.2.0", "notes.txt")} {
		_, err := os.Stat(filepath.Join(contentDir, absent))
		assert.True(t, os.IsNotExist(err), "%s must not be extracted", absent)
	}

	// Version list: semver branches + content versions, ascending semver,
	// multi-digit 1.10.0 sorting after 1.2.0
	versions, err := os.ReadFile(versionsOut)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0\n1.2.0\n1.10.0\n", string(versions))
}

func TestFetchAggregate_Idempotent(t *testing.T) {
	corpus := initCorpusRepo(t)
	work := t.TempDir()
	contentDir := filepath.Join(work, "content")
	versionsOut := filepath.Join(work, "versions.txt")

	opts := fetchOptions{Remote: corpus, Branch: "docs-aggregate", ContentDir: contentDir, VersionsOut: versionsOut}
	require.NoError(t, fetchAggregate(opts))

	// A stale file from a previous run must be wiped
	stale := filepath.Join(contentDir, "9.9.9", "stale.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(stale), 0o755))
	require.NoError(t, os.WriteFile(stale, []byte("stale"), 0o644))

	require.NoError(t, fetchAggregate(opts))
	_, err := os.Stat(stale)
	assert.True(t, os.IsNotExist(err), "content dir must be wiped between runs")
}

func TestFetchAggregate_RemoteTrackingRefsFallback(t *testing.T) {
	// Simulate a local clone that only has remote-tracking refs (no local
	// docs-aggregate branch, versions under refs/remotes/origin/).
	corpus := initCorpusRepo(t)
	sha := strings.TrimSpace(gitT(t, corpus, "rev-parse", "docs-aggregate"))
	gitT(t, corpus, "branch", "-D", "docs-aggregate")
	gitT(t, corpus, "branch", "-D", "1.10.0")
	gitT(t, corpus, "update-ref", "refs/remotes/origin/docs-aggregate", sha)
	gitT(t, corpus, "update-ref", "refs/remotes/origin/1.10.0", sha)
	gitT(t, corpus, "update-ref", "refs/remotes/origin/2.0.0", sha)

	work := t.TempDir()
	contentDir := filepath.Join(work, "content")
	versionsOut := filepath.Join(work, "versions.txt")

	require.NoError(t, fetchAggregate(fetchOptions{
		Remote: corpus, Branch: "docs-aggregate", ContentDir: contentDir, VersionsOut: versionsOut,
	}))

	_, err := os.Stat(filepath.Join(contentDir, "1.0.0", "topic.md"))
	assert.NoError(t, err)

	versions, err := os.ReadFile(versionsOut)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0\n1.2.0\n1.10.0\n2.0.0\n", string(versions),
		"remote-tracking version refs must count toward the version axis")
}

func TestFetchAggregate_MissingRemoteFlag(t *testing.T) {
	err := runFetchAggregate([]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-remote is required")
}

func TestFetchAggregate_BadFlag(t *testing.T) {
	err := runFetchAggregate([]string{"-bogus"})
	require.Error(t, err)
}

func TestFetchAggregate_RemoteNotARepo(t *testing.T) {
	err := fetchAggregate(fetchOptions{
		Remote:      t.TempDir(),
		Branch:      "docs-aggregate",
		ContentDir:  filepath.Join(t.TempDir(), "content"),
		VersionsOut: filepath.Join(t.TempDir(), "versions.txt"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing versions")
	assert.Contains(t, err.Error(), "hint:")
}

func TestFetchAggregate_MissingBranch(t *testing.T) {
	corpus := initCorpusRepo(t)
	err := fetchAggregate(fetchOptions{
		Remote:      corpus,
		Branch:      "no-such-branch",
		ContentDir:  filepath.Join(t.TempDir(), "content"),
		VersionsOut: filepath.Join(t.TempDir(), "versions.txt"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `fetching branch "no-such-branch"`)
}

func TestFetchAggregate_NoDocsInBranch(t *testing.T) {
	// A repo whose aggregate branch has no <version>/docs/*.md files
	dir := t.TempDir()
	gitT(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hi"), 0o644))
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "no docs")
	gitT(t, dir, "branch", "docs-aggregate")
	gitT(t, dir, "branch", "1.0.0")

	err := fetchAggregate(fetchOptions{
		Remote:      dir,
		Branch:      "docs-aggregate",
		ContentDir:  filepath.Join(t.TempDir(), "content"),
		VersionsOut: filepath.Join(t.TempDir(), "versions.txt"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains no <version>/docs/*.md files")
}

func TestListSemverVersions_LocalRepo(t *testing.T) {
	corpus := initCorpusRepo(t)
	versions, err := listSemverVersions(corpus)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"1.0.0", "1.2.0", "1.10.0"}, versions)
}
