package main

// ResolveVersionMap maps each software version to the documented version whose
// docs should be displayed. The rule is:
//
//  1. Use the nearest documented version <= this software version.
//  2. If none exists (software version predates all docs), use the nearest
//     documented version > this software version.
//
// Versions are treated as ordered by their position in the software_versions
// slice (i.e., the user provides them in release order).
func ResolveVersionMap(softwareVersions, documentedVersions []string) map[string]string {
	// Build a set for quick lookup and an index map for ordering
	docSet := make(map[string]bool, len(documentedVersions))
	for _, dv := range documentedVersions {
		docSet[dv] = true
	}

	// Index each software version by position
	svIndex := make(map[string]int, len(softwareVersions))
	for i, sv := range softwareVersions {
		svIndex[sv] = i
	}

	// For each documented version, record its position in the software version list
	type docPos struct {
		version string
		index   int
	}
	var docPositions []docPos
	for _, sv := range softwareVersions {
		if docSet[sv] {
			docPositions = append(docPositions, docPos{sv, svIndex[sv]})
		}
	}

	result := make(map[string]string, len(softwareVersions))

	for _, sv := range softwareVersions {
		idx := svIndex[sv]

		// If this version itself is documented, use it
		if docSet[sv] {
			result[sv] = sv
			continue
		}

		// Find nearest documented version <= this one
		bestBefore := ""
		for _, dp := range docPositions {
			if dp.index <= idx {
				bestBefore = dp.version // last one <= idx wins (they're in order)
			}
		}

		if bestBefore != "" {
			result[sv] = bestBefore
			continue
		}

		// Fallback: nearest documented version > this one
		for _, dp := range docPositions {
			if dp.index > idx {
				result[sv] = dp.version
				break
			}
		}
	}

	return result
}
