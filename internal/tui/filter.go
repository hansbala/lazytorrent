package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"lazytorrent/internal/transmission"
)

type filterState struct {
	query string         // currently applied filter (case-insensitive substring)
	saved string         // value before entering modeFilter — used to revert on Esc
	input textinput.Model
}

func newFilterState() filterState {
	ti := textinput.New()
	ti.Placeholder = "filter by name"
	ti.Prompt = "/ "
	ti.Width = 30
	ti.CharLimit = 200
	return filterState{input: ti}
}

// matches reports whether a torrent name matches the current filter query.
// An empty query matches everything.
func (f filterState) matches(name string) bool {
	if f.query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(f.query))
}

// apply returns the subset of torrents whose names match the filter,
// in their original order.
func (f filterState) apply(torrents []transmission.Torrent) []transmission.Torrent {
	if f.query == "" {
		return torrents
	}
	out := make([]transmission.Torrent, 0, len(torrents))
	for _, t := range torrents {
		if f.matches(t.Name) {
			out = append(out, t)
		}
	}
	return out
}
