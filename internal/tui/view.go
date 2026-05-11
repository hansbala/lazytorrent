package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"lazytorrent/internal/transmission"
)

var (
	activeColor  = lipgloss.Color("12")
	accentColor  = lipgloss.Color("14")
	mutedColor   = lipgloss.Color("8")
	warnColor    = lipgloss.Color("9")
	successColor = lipgloss.Color("10")
)

func (m model) View() string {
	if !m.ready {
		return "Loading…"
	}

	helpHeight := 1
	bodyHeight := m.height - helpHeight
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	switch m.mode {
	case modeAdd:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.renderAddModal(m.width, bodyHeight),
			m.renderHelp(m.width))
	case modeDelete:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.renderDeleteModal(m.width, bodyHeight),
			m.renderHelp(m.width))
	case modeHelp:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.renderHelpOverlay(m.width, bodyHeight),
			m.renderHelp(m.width))
	case modeFilter:
		filterBarHeight := 1
		body := m.renderBody(m.width, bodyHeight-filterBarHeight)
		return lipgloss.JoinVertical(lipgloss.Left,
			body,
			m.renderFilterBar(m.width),
			m.renderHelp(m.width))
	default:
		return lipgloss.JoinVertical(lipgloss.Left,
			m.renderBody(m.width, bodyHeight),
			m.renderHelp(m.width))
	}
}

func (m model) renderBody(w, h int) string {
	listWidth := w / 2
	detailsWidth := w - listWidth
	listView := m.renderList(listWidth, h)
	detailsView := m.renderDetails(detailsWidth, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailsView)
}

func paneStyle(width, height int, active bool) lipgloss.Style {
	border := mutedColor
	if active {
		border = activeColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(width - 2).
		Height(height - 2).
		Padding(0, 1)
}

func (m model) renderList(width, height int) string {
	visible := m.visibleTorrents()
	title := fmt.Sprintf("Torrents (%d", len(visible))
	if m.filter.query != "" {
		title += fmt.Sprintf("/%d", len(m.torrents))
	}
	title += ")"
	if m.filter.query != "" {
		title += "  " + lipgloss.NewStyle().Foreground(accentColor).Render("filter: "+m.filter.query)
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(activeColor).Render(title)

	rows := []string{header, ""}
	if len(visible) == 0 {
		var msg string
		if m.filter.query != "" {
			msg = "No torrents match filter."
		} else {
			msg = "No torrents."
		}
		rows = append(rows, lipgloss.NewStyle().Foreground(mutedColor).Render(msg))
	} else {
		inner := width - 4
		for i, t := range visible {
			rows = append(rows, m.formatListRow(t, i == m.selected, inner))
			rows = append(rows, "")
		}
	}

	return paneStyle(width, height, m.focus == paneList).Render(strings.Join(rows, "\n"))
}

func (m model) formatListRow(t transmission.Torrent, selected bool, w int) string {
	indicator := "  "
	nameStyle := lipgloss.NewStyle()
	if selected {
		indicator = "▸ "
		if m.focus == paneList {
			nameStyle = nameStyle.Bold(true).Foreground(activeColor)
		} else {
			nameStyle = nameStyle.Bold(true)
		}
	}

	name := truncate(t.Name, w-2)
	bar := progressBar(t.PercentDone, 20)
	pct := fmt.Sprintf("%3d%%", int(t.PercentDone*100+0.5))

	return strings.Join([]string{
		indicator + nameStyle.Render(name),
		"  " + bar + "  " + pct,
		"  " + statusLine(t),
	}, "\n")
}

func statusLine(t transmission.Torrent) string {
	muted := lipgloss.NewStyle().Foreground(mutedColor)
	switch t.Status {
	case transmission.StatusDownload:
		return fmt.Sprintf("↓ %s/s  ↑ %s/s  ETA %s",
			humanBytes(t.RateDownload), humanBytes(t.RateUpload), humanETA(t.ETA))
	case transmission.StatusSeed:
		return fmt.Sprintf("Seeding  ↑ %s/s  ratio %.2f",
			humanBytes(t.RateUpload), t.UploadRatio)
	case transmission.StatusStopped:
		return muted.Render(fmt.Sprintf("Paused  %d%%", int(t.PercentDone*100+0.5)))
	default:
		return muted.Render(transmission.StatusString(t.Status))
	}
}

func (m model) renderDetails(width, height int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("Details")
	rows := []string{title, ""}

	t, ok := m.currentTorrent()
	if !ok {
		rows = append(rows, lipgloss.NewStyle().Foreground(mutedColor).
			Render("Select a torrent to view its details."))
	} else {
		rows = append(rows, lipgloss.NewStyle().Bold(true).Render(truncate(t.Name, width-4)))
		rows = append(rows, "")
		rows = append(rows, detailRow("Status", transmission.StatusString(t.Status)))
		rows = append(rows, detailRow("Progress", fmt.Sprintf("%s / %s  (%d%%)",
			humanBytes(t.DownloadedEver), humanBytes(t.TotalSize), int(t.PercentDone*100+0.5))))
		rows = append(rows, detailRow("Speed", fmt.Sprintf("↓ %s/s   ↑ %s/s",
			humanBytes(t.RateDownload), humanBytes(t.RateUpload))))
		rows = append(rows, detailRow("Peers", fmt.Sprintf("%d connected", t.PeersConnected)))
		rows = append(rows, detailRow("ETA", humanETA(t.ETA)))
		rows = append(rows, detailRow("Ratio", fmt.Sprintf("%.2f", t.UploadRatio)))
		rows = append(rows, "")
		rows = append(rows, detailRow("Save to", t.DownloadDir))
		rows = append(rows, detailRow("Added", humanDate(t.AddedDate)))
	}

	return paneStyle(width, height, m.focus == paneDetails).Render(strings.Join(rows, "\n"))
}

func detailRow(label, value string) string {
	labelStyle := lipgloss.NewStyle().Faint(true).Width(11)
	return labelStyle.Render(label) + " " + value
}

// --- add modal ---

func (m model) renderAddModal(totalWidth, totalHeight int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(activeColor).Render("Add torrent")
	labelStyle := lipgloss.NewStyle().Faint(true)

	parts := []string{
		title,
		"",
		labelStyle.Render("Source"),
		m.addModal.magnet.View(),
		"",
		labelStyle.Render("Save to"),
		m.addModal.saveDir.View(),
	}

	if m.addModal.err != "" {
		parts = append(parts, "")
		parts = append(parts, lipgloss.NewStyle().Foreground(warnColor).Render("⚠ "+m.addModal.err))
	}
	if m.addModal.submitting {
		parts = append(parts, "")
		parts = append(parts, labelStyle.Render("adding..."))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(activeColor).
		Padding(1, 2).
		Render(strings.Join(parts, "\n"))

	return lipgloss.Place(totalWidth, totalHeight, lipgloss.Center, lipgloss.Center, box)
}

// --- delete modal ---

func (m model) renderDeleteModal(totalWidth, totalHeight int) string {
	border := warnColor
	title := "Delete torrent?"
	bodyText := "This removes it from Transmission but keeps the downloaded files."
	if m.deleteModal.withFiles {
		title = "Delete torrent AND files?"
		bodyText = lipgloss.NewStyle().Foreground(warnColor).Bold(true).
			Render("This will also delete the downloaded files from disk.")
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(border)
	nameStyle := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Faint(true)

	content := strings.Join([]string{
		titleStyle.Render(title),
		"",
		nameStyle.Render(truncate(m.deleteModal.torrentName, 60)),
		"",
		bodyText,
		"",
		muted.Render("y / Enter — confirm    n / Esc — cancel"),
	}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(1, 2).
		Render(content)

	return lipgloss.Place(totalWidth, totalHeight, lipgloss.Center, lipgloss.Center, box)
}

// --- help overlay ---

func (m model) renderHelpOverlay(totalWidth, totalHeight int) string {
	groupStyle := lipgloss.NewStyle().PaddingRight(4)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Width(12)
	muted := lipgloss.NewStyle().Faint(true)

	var columns []string
	for _, g := range helpContent() {
		var rows []string
		rows = append(rows, titleStyle.Render(g.title))
		rows = append(rows, "")
		for _, kd := range g.keys {
			rows = append(rows, keyStyle.Render(kd[0])+" "+kd[1])
		}
		columns = append(columns, groupStyle.Render(strings.Join(rows, "\n")))
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, columns...)
	heading := titleStyle.Render("Keyboard shortcuts")
	footer := muted.Render("any key to close")

	content := strings.Join([]string{heading, "", body, "", footer}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(activeColor).
		Padding(1, 2).
		Render(content)

	return lipgloss.Place(totalWidth, totalHeight, lipgloss.Center, lipgloss.Center, box)
}

// --- filter bar ---

func (m model) renderFilterBar(width int) string {
	prefix := lipgloss.NewStyle().Foreground(activeColor).Bold(true).Render("filter: ")
	bar := prefix + m.filter.input.View()
	return lipgloss.NewStyle().Width(width).Padding(0, 1).Render(bar)
}

// --- help bar at bottom ---

func (m model) renderHelp(width int) string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	var parts []string
	switch m.mode {
	case modeAdd:
		parts = []string{
			keyStyle.Render("Tab") + " next field",
			keyStyle.Render("Enter") + " add",
			keyStyle.Render("Esc") + " cancel",
		}
	case modeDelete:
		parts = []string{
			keyStyle.Render("y/Enter") + " confirm",
			keyStyle.Render("n/Esc") + " cancel",
		}
	case modeFilter:
		parts = []string{
			keyStyle.Render("type") + " to filter live",
			keyStyle.Render("Enter") + " keep",
			keyStyle.Render("Esc") + " cancel",
		}
	case modeHelp:
		parts = []string{
			keyStyle.Render("any key") + " close",
		}
	default:
		parts = []string{
			keyStyle.Render("a") + " add",
			keyStyle.Render("␣") + " pause",
			keyStyle.Render("d/D") + " delete",
			keyStyle.Render("/") + " filter",
			keyStyle.Render("?") + " help",
			keyStyle.Render("q") + " quit",
		}
	}
	left := strings.Join(parts, "   ")
	right := m.renderHelpRight()

	if right == "" {
		return lipgloss.NewStyle().Width(width).Padding(0, 1).Render(left)
	}
	pad := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if pad < 1 {
		pad = 1
	}
	return lipgloss.NewStyle().Padding(0, 1).
		Render(left + strings.Repeat(" ", pad) + right)
}

// renderHelpRight returns the right-aligned status indicator: a transient
// status message (if any), an error indicator, or a stale-refresh hint.
func (m model) renderHelpRight() string {
	if m.status.visible() {
		return lipgloss.NewStyle().Foreground(m.status.color).Render(m.status.text)
	}
	if m.mode != modeNormal {
		return ""
	}
	if m.lastErr != nil {
		return lipgloss.NewStyle().Foreground(warnColor).Render("⚠ " + m.lastErr.Error())
	}
	if !m.lastRefresh.IsZero() {
		age := time.Since(m.lastRefresh).Truncate(time.Second)
		if age > 3*time.Second {
			return lipgloss.NewStyle().Foreground(mutedColor).
				Render(fmt.Sprintf("refresh: %s ago", age))
		}
	}
	return ""
}

// --- helpers ---

func progressBar(pct float64, w int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(float64(w)*pct + 0.5)
	return strings.Repeat("▰", filled) + strings.Repeat("▱", w-filled)
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(runes[:w-1]) + "…"
}

func humanBytes(b int64) string {
	if b < 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div := int64(unit)
	exp := 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func humanETA(s int64) string {
	if s < 0 {
		return "—"
	}
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	if s < 3600 {
		return fmt.Sprintf("%dm %ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %dm", s/3600, (s%3600)/60)
}

func humanDate(epoch int64) string {
	if epoch <= 0 {
		return "—"
	}
	return time.Unix(epoch, 0).Format("2006-01-02  15:04")
}
