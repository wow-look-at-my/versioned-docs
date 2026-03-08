package main

import (
	"testing"
	"github.com/wow-look-at-my/testify/assert"
)

func TestResolveVersionMap(t *testing.T) {
	// Matches the user's table exactly:
	// | Version | Has docs? | Displayed docs |
	// | 1       |           | 2              |
	// | 2       | x         | 2              |
	// | 3       | x         | 3              |
	// | 4       |           | 3              |
	// | 5       | x         | 5              |
	// | 6       |           | 5              |
	// | 7       |           | 5              |
	// | 8       | x         | 8              |

	software := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	documented := []string{"2", "3", "5", "8"}

	got := ResolveVersionMap(software, documented)

	expected := map[string]string{
		"1":	"2",	// no prior docs, falls forward to 2
		"2":	"2",	// authored
		"3":	"3",	// authored
		"4":	"3",	// inherits from 3
		"5":	"5",	// authored
		"6":	"5",	// inherits from 5
		"7":	"5",	// inherits from 5
		"8":	"8",	// authored
	}

	for v, want := range expected {
		assert.Equal(t, want, got[v])

	}
}

func TestResolveVersionMap_AllDocumented(t *testing.T) {
	software := []string{"1.0", "2.0", "3.0"}
	documented := []string{"1.0", "2.0", "3.0"}

	got := ResolveVersionMap(software, documented)

	for _, v := range software {
		assert.Equal(t, v, got[v])

	}
}

func TestResolveVersionMap_OnlyLastDocumented(t *testing.T) {
	software := []string{"1", "2", "3"}
	documented := []string{"3"}

	got := ResolveVersionMap(software, documented)

	// 1 and 2 have no prior docs, should fall forward to 3
	for _, v := range software {
		assert.Equal(t, "3", got[v])

	}
}

func TestResolvePageVersion(t *testing.T) {
	software := []string{"v1", "v2", "v3", "v4"}

	tests := []struct {
		name		string
		softwareVersion	string
		pageVersions	map[string]bool	// versions that have this page
		want		string
	}{
		{
			name:			"page exists in same version",
			softwareVersion:	"v2",
			pageVersions:		map[string]bool{"v2": true},
			want:			"v2",
		},
		{
			name:			"page inherited from older version",
			softwareVersion:	"v3",
			pageVersions:		map[string]bool{"v2": true},
			want:			"v2",
		},
		{
			name:			"page inherited from newer version (no older exists)",
			softwareVersion:	"v1",
			pageVersions:		map[string]bool{"v2": true},
			want:			"v2",
		},
		{
			name:			"page exists in multiple versions, picks nearest older",
			softwareVersion:	"v3",
			pageVersions:		map[string]bool{"v1": true, "v2": true, "v4": true},
			want:			"v2",
		},
		{
			name:			"page only in newest version",
			softwareVersion:	"v2",
			pageVersions:		map[string]bool{"v4": true},
			want:			"v4",
		},
		{
			name:			"no version has page",
			softwareVersion:	"v2",
			pageVersions:		map[string]bool{},
			want:			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolvePageVersion(tt.softwareVersion, "test.md", software, tt.pageVersions)
			assert.Equal(t, tt.want, got)

		})
	}
}

func TestResolvePageVersion_PageLevelInheritance(t *testing.T) {
	// Test the scenario from the handoff notes:
	// v2 has: index.md, api.md
	// v3 has: index.md only
	// v3 should get api.md from v2

	software := []string{"v1", "v2", "v3"}

	indexVersions := map[string]bool{"v2": true, "v3": true}
	apiVersions := map[string]bool{"v2": true}

	// v3's index should come from v3 (authored)
	got := ResolvePageVersion("v3", "index.md", software, indexVersions)
	assert.Equal(t, "v3", got)

	// v3's api should come from v2 (inherited)
	got = ResolvePageVersion("v3", "api.md", software, apiVersions)
	assert.Equal(t, "v2", got)

	// v1's index should come from v2 (falls forward)
	got = ResolvePageVersion("v1", "index.md", software, indexVersions)
	assert.Equal(t, "v2", got)

}
