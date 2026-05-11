package tui

import (
	"testing"

	"lazytorrent/internal/transmission"
)

func TestFilter_Matches(t *testing.T) {
	cases := []struct {
		name  string
		query string
		input string
		want  bool
	}{
		{"empty query matches anything", "", "anything", true},
		{"exact case-insensitive match", "ubuntu", "Ubuntu-24.04", true},
		{"substring match", "buntu", "ubuntu-24.04", true},
		{"no match returns false", "debian", "ubuntu-24.04", false},
		{"empty name with non-empty query", "x", "", false},
		{"case-insensitive on both sides", "UBUNTU", "ubuntu-server", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := filterState{query: tc.query}
			if got := f.matches(tc.input); got != tc.want {
				t.Errorf("matches(query=%q, name=%q) = %v; want %v", tc.query, tc.input, got, tc.want)
			}
		})
	}
}

func TestFilter_ApplyPreservesOrderAndFiltersOut(t *testing.T) {
	torrents := []transmission.Torrent{
		{ID: 1, Name: "ubuntu-24.04-desktop.iso"},
		{ID: 2, Name: "debian-12.5.0-amd64-netinst.iso"},
		{ID: 3, Name: "ubuntu-22.04-server.iso"},
		{ID: 4, Name: "manjaro-kde.iso"},
	}

	f := filterState{query: "ubuntu"}
	got := f.apply(torrents)

	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(got), got)
	}
	if got[0].ID != 1 || got[1].ID != 3 {
		t.Errorf("expected IDs [1,3] in order, got [%d,%d]", got[0].ID, got[1].ID)
	}
}

func TestFilter_ApplyWithEmptyQueryReturnsInput(t *testing.T) {
	torrents := []transmission.Torrent{{ID: 1, Name: "x"}, {ID: 2, Name: "y"}}
	f := filterState{query: ""}
	got := f.apply(torrents)
	if len(got) != 2 {
		t.Errorf("empty query should return all torrents, got %d", len(got))
	}
}
