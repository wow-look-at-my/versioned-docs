package main

import "fmt"

// termination models the tombstone configuration from config.yaml:
//
//	terminated:
//	  some-doc.md: 1.2.3
//
// A doc path P mapped to version T applies UP TO AND INCLUDING T and is
// removed from every version AFTER T:
//
//   - version == T  -> the doc is still visible
//   - version >  T  -> the doc is hidden
//   - T < current   -> the doc is dropped from the default (doc-centric)
//     listing and moves to the "Removed" section
//
// "Newer/older" follows the positional order of the software versions list
// (the version_command output order), consistent with the inheritance
// resolver.
type termination struct {
	svIndex map[string]int
	tomb    map[string]string
}

// newTermination validates the tombstone map against the software versions
// list and the set of known doc paths, and returns the resolved termination
// index. Validation errors:
//
//   - the termination version is not in the software versions list
//   - the doc path does not match any known document
//   - a version newer than the termination version has authored content for
//     the doc (the tombstone would contradict the corpus)
func newTermination(softwareVersions []string, tomb map[string]string, pageVersions map[string]map[string]bool) (*termination, error) {
	svIndex := make(map[string]int, len(softwareVersions))
	for i, v := range softwareVersions {
		svIndex[v] = i
	}

	for path, tv := range tomb {
		tIdx, ok := svIndex[tv]
		if !ok {
			return nil, fmt.Errorf("terminated: %q -> %q: version %q is not in the software versions list", path, tv, tv)
		}
		authored, ok := pageVersions[path]
		if !ok {
			return nil, fmt.Errorf("terminated: %q does not match any known document", path)
		}
		for av := range authored {
			if svIndex[av] > tIdx {
				return nil, fmt.Errorf("terminated: %q -> %q: version %q has authored content newer than the termination version", path, tv, av)
			}
		}
	}

	return &termination{svIndex: svIndex, tomb: tomb}, nil
}

// visibleAt reports whether doc path p should appear in version v's view.
// Non-terminated docs are always visible; terminated docs are visible up to
// and including their termination version.
func (t *termination) visibleAt(p, v string) bool {
	tv, ok := t.tomb[p]
	if !ok {
		return true
	}
	return t.svIndex[v] <= t.svIndex[tv]
}

// terminatedAt returns the termination version for p, or "" when p is not
// terminated.
func (t *termination) terminatedAt(p string) string {
	return t.tomb[p]
}

// removedBefore reports whether p is terminated strictly before version v,
// i.e. whether p no longer applies at v. With v = the current (newest)
// software version this decides whether p belongs in the "Removed" section.
func (t *termination) removedBefore(p, v string) bool {
	tv, ok := t.tomb[p]
	if !ok {
		return false
	}
	return t.svIndex[tv] < t.svIndex[v]
}
