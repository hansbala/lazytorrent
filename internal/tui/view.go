package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"lazytorrent/internal/transmission"
)

var (
	activeColor = lipgloss.Color("12")
	accentColor = lipgloss.Color("14")
	mutedColor  = lipgloss.Color("8")
	warnColor   = lipgloss.Color("9")
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

	var body string
	if m.modal.active {
		body = m.renderModal(m.width, bodyHeight)
	} else {
		listWidth := m.width / 2
		detailsWidth := m.width - listWidth
		listView := m.renderList(listWidth, bodyHeight)
		detailsView := m.renderDetails(detailsWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, listView, detailsView)
	}

	help := m.renderHelp(m.width)
	return lipgloss.JoinVertical(lipgloss.Left, body, help)
}

func (m model) renderModal(totalWidth, totalHeight int) string {
	title := lipgloss.NewStyle().Bold(true).Foreground(activeColor).Render("Add torrent")
	labelStyle := lipgloss.NewStyle().Faint(true)

	parts := []string{
		title,
		"",
		labelStyle.Render("Source"),
		m.modal.magnet.View(),
		"",
		labelStyle.Render("Save to"),
		m.modal.saveDir.View(),
	}

	if m.modal.err != "" {
		parts = append(parts, "")
		parts = append(parts, lipgloss.NewStyle().Foreground(warnColor).Render("⚠ "+m.modal.err))
	}
	if m.modal.submitting {
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
	title := lipgloss.NewStyle().Bold(true).Foreground(activeColor).
		Render(fmt.Sprintf("Torrents (%d)", len(m.torrents)))

	rows := []string{title, ""}

	if len(m.torrents) == 0 {
		empty := lipgloss.NewStyle().Foreground(mutedColor).Render("No torrents.")
		rows = append(rows, empty)
	} else {
		inner := width - 4
		for i, t := range m.torrents {
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

	if len(m.torrents) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(mutedColor).
			Render("Select a torrent to view its details."))
	} else {
		t := m.torrents[m.selected]
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

func (m model) renderHelp(width int) string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	var parts []string
	if m.modal.active {
		parts = []string{
			keyStyle.Render("Tab") + " next field",
			keyStyle.Render("Enter") + " add",
			keyStyle.Render("Esc") + " cancel",
		}
	} else {
		parts = []string{
			keyStyle.Render("a") + " add",
			keyStyle.Render("j/k") + " navigate",
			keyStyle.Render("h/l") + " switch panel",
			keyStyle.Render("g/G") + " top/bottom",
			keyStyle.Render("q") + " quit",
		}
	}
	left := strings.Join(parts, "   ")

	right := ""
	if !m.modal.active {
		if m.lastErr != nil {
			right = lipgloss.NewStyle().Foreground(warnColor).Render("⚠ " + m.lastErr.Error())
		} else if !m.lastRefresh.IsZero() {
			age := time.Since(m.lastRefresh).Truncate(time.Second)
			if age > 3*time.Second {
				right = lipgloss.NewStyle().Foreground(mutedColor).
					Render(fmt.Sprintf("refresh: %s ago", age))
			}
		}
	}

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
