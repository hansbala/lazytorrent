package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome resolves a leading `~` or `~/` to the user's home directory.
// Other paths (absolute, relative, `~user/...`) are returned unchanged.
// If the home directory cannot be determined, the input is returned as-is.
func ExpandHome(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
