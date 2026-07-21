package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPageVersions() map[string]map[string]bool {
	return map[string]map[string]bool{
		"guide.md": {"v1": true, "v3": true},
		"old.md":   {"v1": true},
	}
}

func TestNewTermination_Valid(t *testing.T) {
	software := []string{"v1", "v2", "v3", "v4"}

	term, err := newTermination(software, map[string]string{"old.md": "v2"}, testPageVersions())
	require.NoError(t, err)
	require.NotNil(t, term)
	assert.Equal(t, "v2", term.terminatedAt("old.md"))
	assert.Equal(t, "", term.terminatedAt("guide.md"))
}

func TestNewTermination_NoTombstones(t *testing.T) {
	term, err := newTermination([]string{"v1"}, nil, testPageVersions())
	require.NoError(t, err)
	assert.True(t, term.visibleAt("old.md", "v1"))
	assert.False(t, term.removedBefore("old.md", "v1"))
}

func TestNewTermination_UnknownVersion(t *testing.T) {
	software := []string{"v1", "v2"}

	_, err := newTermination(software, map[string]string{"old.md": "v99"}, testPageVersions())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in the software versions list")
	assert.Contains(t, err.Error(), "v99")
}

func TestNewTermination_UnknownDoc(t *testing.T) {
	software := []string{"v1", "v2"}

	_, err := newTermination(software, map[string]string{"nope.md": "v1"}, testPageVersions())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match any known document")
	assert.Contains(t, err.Error(), "nope.md")
}

func TestNewTermination_AuthoredAfterTermination(t *testing.T) {
	software := []string{"v1", "v2", "v3", "v4"}

	// guide.md has authored content at v3, newer than the tombstone at v2
	_, err := newTermination(software, map[string]string{"guide.md": "v2"}, testPageVersions())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authored content newer than the termination version")
}

func TestTermination_VisibleAtBoundary(t *testing.T) {
	software := []string{"v1", "v2", "v3", "v4"}

	term, err := newTermination(software, map[string]string{"old.md": "v2"}, testPageVersions())
	require.NoError(t, err)

	// Visible up to and including the termination version...
	assert.True(t, term.visibleAt("old.md", "v1"))
	assert.True(t, term.visibleAt("old.md", "v2"), "doc must still be visible AT the termination version")

	// ...hidden strictly after it
	assert.False(t, term.visibleAt("old.md", "v3"), "doc must be hidden after the termination version")
	assert.False(t, term.visibleAt("old.md", "v4"))

	// Non-terminated docs are visible everywhere
	for _, v := range software {
		assert.True(t, term.visibleAt("guide.md", v))
	}
}

func TestTermination_RemovedBefore(t *testing.T) {
	software := []string{"v1", "v2", "v3"}

	term, err := newTermination(software, map[string]string{"old.md": "v2"}, testPageVersions())
	require.NoError(t, err)

	// Not removed at or before its termination version
	assert.False(t, term.removedBefore("old.md", "v1"))
	assert.False(t, term.removedBefore("old.md", "v2"))

	// Removed relative to any later version (e.g. current = v3)
	assert.True(t, term.removedBefore("old.md", "v3"))

	// Non-terminated docs are never removed
	assert.False(t, term.removedBefore("guide.md", "v3"))
}

func TestTermination_TerminatedAtNewestVersion(t *testing.T) {
	// Tombstone at the CURRENT (newest) version: the doc still applies
	// everywhere and is not removed anywhere.
	software := []string{"v1", "v2", "v3"}

	term, err := newTermination(software, map[string]string{"old.md": "v3"}, testPageVersions())
	require.NoError(t, err)

	for _, v := range software {
		assert.True(t, term.visibleAt("old.md", v))
	}
	assert.False(t, term.removedBefore("old.md", "v3"), "T == current must stay in the default listing")
}

func TestTermination_TerminatedAtOldestVersion(t *testing.T) {
	// Tombstone at the very first version: visible only there.
	software := []string{"v1", "v2", "v3"}

	term, err := newTermination(software, map[string]string{"old.md": "v1"}, testPageVersions())
	require.NoError(t, err)

	assert.True(t, term.visibleAt("old.md", "v1"))
	assert.False(t, term.visibleAt("old.md", "v2"))
	assert.False(t, term.visibleAt("old.md", "v3"))
	assert.True(t, term.removedBefore("old.md", "v3"))
}
