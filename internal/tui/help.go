package tui

// helpGroup is one column/section in the help overlay.
type helpGroup struct {
	title string
	keys  [][2]string // {key, description}
}

func helpContent() []helpGroup {
	return []helpGroup{
		{
			title: "Navigation",
			keys: [][2]string{
				{"j / k", "down / up"},
				{"g / G", "top / bottom"},
				{"h / l", "switch panel"},
				{"Tab", "cycle panes"},
				{"/", "filter by name"},
			},
		},
		{
			title: "Torrent",
			keys: [][2]string{
				{"a", "add new torrent"},
				{"Space", "pause / resume"},
				{"d", "delete (keep files)"},
				{"D", "delete (remove files)"},
				{"v", "verify local data"},
				{"R", "re-announce to trackers"},
				{"y", "copy magnet to clipboard"},
				{"o", "open save folder"},
			},
		},
		{
			title: "Misc",
			keys: [][2]string{
				{"r", "refresh now"},
				{"?", "this help"},
				{"q / Ctrl-C", "quit"},
			},
		},
	}
}
