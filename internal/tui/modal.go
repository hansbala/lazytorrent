package tui

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"lazytorrent/internal/pathutil"
)

const magnetPrefix = "magnet:?"

type addModal struct {
	active     bool
	magnet     textinput.Model
	saveDir    textinput.Model
	focused    int // 0=source, 1=saveDir
	err        string
	submitting bool
}

func newAddModal(defaultSaveDir string) addModal {
	m := textinput.New()
	m.Placeholder = "magnet:?, .torrent file path, or URL"
	m.Prompt = ""
	m.Width = 60
	m.CharLimit = 4096

	s := textinput.New()
	s.Placeholder = defaultSaveDir
	s.Prompt = ""
	s.Width = 60
	s.SetValue(defaultSaveDir)

	if clip, err := clipboard.ReadAll(); err == nil {
		clip = strings.TrimSpace(clip)
		if isAddableSource(clip) {
			m.SetValue(clip)
		}
	}

	return addModal{magnet: m, saveDir: s, focused: 0}
}

// setFocus moves focus between the modal's two fields and returns the
// cursor-blink Cmd from the newly-focused input.
func (a *addModal) setFocus(idx int) tea.Cmd {
	a.focused = ((idx % 2) + 2) % 2
	if a.focused == 0 {
		a.saveDir.Blur()
		return a.magnet.Focus()
	}
	a.magnet.Blur()
	return a.saveDir.Focus()
}

// forwardKey routes a key (or other) message to the focused textinput.
func (a *addModal) forwardKey(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if a.focused == 0 {
		a.magnet, cmd = a.magnet.Update(msg)
	} else {
		a.saveDir, cmd = a.saveDir.Update(msg)
	}
	return cmd
}

// prepareAdd validates and normalizes the modal's inputs. Accepts:
//   - magnet URIs ("magnet:?...")     — passed through
//   - HTTP(S) URLs                    — passed through (daemon fetches)
//   - local .torrent file paths       — tilde-expanded, resolved to absolute,
//     must exist on disk and end in .torrent
//
// Returns ("", "", validationErr) if the input doesn't match any of these.
// `saveDir` is always tilde-expanded.
func prepareAdd(rawSource, rawSaveDir string) (source, saveDir, validationErr string) {
	source = strings.TrimSpace(rawSource)
	saveDir = pathutil.ExpandHome(strings.TrimSpace(rawSaveDir))

	if source == "" {
		return "", "", "magnet URI, .torrent file path, or URL required"
	}

	if strings.HasPrefix(source, magnetPrefix) {
		return source, saveDir, ""
	}

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return source, saveDir, ""
	}

	expanded := pathutil.ExpandHome(source)
	if abs, err := filepath.Abs(expanded); err == nil {
		expanded = abs
	}
	if !strings.HasSuffix(strings.ToLower(expanded), ".torrent") {
		return "", "", "input must be a magnet URI, .torrent file path, or http(s) URL"
	}
	if _, err := os.Stat(expanded); err != nil {
		if os.IsNotExist(err) {
			return "", "", "file not found: " + expanded
		}
		return "", "", "cannot read file: " + err.Error()
	}
	return expanded, saveDir, ""
}

// isAddableSource is the more conservative classifier used for clipboard
// auto-paste. We only auto-paste magnet URIs and .torrent URLs to avoid
// hijacking the field when the user has arbitrary URLs or text in their
// clipboard.
func isAddableSource(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, magnetPrefix) {
		return true
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		u, err := url.Parse(s)
		if err != nil {
			return false
		}
		return strings.HasSuffix(strings.ToLower(u.Path), ".torrent")
	}
	return false
}

// cleanError formats an error from the daemon for display in the modal.
// Strips internal "RPC: " prefixes that aren't useful to the user.
func cleanError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimPrefix(err.Error(), "RPC: ")
}
