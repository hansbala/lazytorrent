package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir on this system: %v", err)
	}

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty stays empty", "", ""},
		{"absolute path unchanged", "/absolute/path", "/absolute/path"},
		{"relative path unchanged", "relative/path", "relative/path"},
		{"lone tilde expands to home", "~", home},
		{"~/ prefix expands", "~/Downloads", filepath.Join(home, "Downloads")},
		{"~/ with nested path", "~/foo/bar/baz", filepath.Join(home, "foo", "bar", "baz")},
		{"~user is left as-is (not supported)", "~someoneelse", "~someoneelse"},
		{"~user/foo is left as-is", "~someoneelse/foo", "~someoneelse/foo"},
		{"tilde mid-path is not expanded", "/some/~/thing", "/some/~/thing"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExpandHome(tc.in)
			if got != tc.want {
				t.Errorf("ExpandHome(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}
