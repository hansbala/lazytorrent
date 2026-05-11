package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"lazytorrent/internal/pathutil"
)

// actionResultMsg is sent back to Update after a fire-and-forget action.
type actionResultMsg struct {
	label   string
	err     error
	success string // success message override (else uses label)
}

// runAction wraps a side-effecting call as a tea.Cmd that produces an
// actionResultMsg. The label is what gets shown to the user with a ✓/✗ prefix.
func runAction(label string, fn func() error) tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg{label: label, err: fn()}
	}
}

// openFolder launches the OS file manager pointed at `path`. The process
// is started and detached; we don't wait for it.
func openFolder(path string) error {
	if path == "" {
		return errors.New("no path to open")
	}
	expanded := pathutil.ExpandHome(path)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", expanded)
	case "linux":
		cmd = exec.Command("xdg-open", expanded)
	default:
		return fmt.Errorf("opening folders is not supported on %s", runtime.GOOS)
	}
	return cmd.Start()
}
