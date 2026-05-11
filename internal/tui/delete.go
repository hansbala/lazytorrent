package tui

// deleteModal holds state for the delete-confirmation dialog.
// It's a value type — when the dialog isn't active, the model's mode
// field will not be modeDelete and the contents here are stale-but-harmless.
type deleteModal struct {
	torrentID   int64
	torrentName string
	withFiles   bool // true when invoked via D (shift-d)
}
