package main

import (
	"testing"
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
		"1": "2", // no prior docs, falls forward to 2
		"2": "2", // authored
		"3": "3", // authored
		"4": "3", // inherits from 3
		"5": "5", // authored
		"6": "5", // inherits from 5
		"7": "5", // inherits from 5
		"8": "8", // authored
	}

	for v, want := range expected {
		if got[v] != want {
			t.Errorf("version %s: got docs %q, want %q", v, got[v], want)
		}
	}
}

func TestResolveVersionMap_AllDocumented(t *testing.T) {
	software := []string{"1.0", "2.0", "3.0"}
	documented := []string{"1.0", "2.0", "3.0"}

	got := ResolveVersionMap(software, documented)

	for _, v := range software {
		if got[v] != v {
			t.Errorf("version %s: got %q, want %q", v, got[v], v)
		}
	}
}

func TestResolveVersionMap_OnlyLastDocumented(t *testing.T) {
	software := []string{"1", "2", "3"}
	documented := []string{"3"}

	got := ResolveVersionMap(software, documented)

	// 1 and 2 have no prior docs, should fall forward to 3
	for _, v := range software {
		if got[v] != "3" {
			t.Errorf("version %s: got %q, want %q", v, got[v], "3")
		}
	}
}
