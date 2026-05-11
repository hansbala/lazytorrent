package tui

import (
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazytorrent/internal/transmission"
)

type pane int

const (
	paneList pane = iota
	paneDetails
)

type tuiMode int

const (
	modeNormal tuiMode = iota
	modeAdd
	modeDelete
	modeFilter
	modeHelp
)

type statusMessage struct {
	text    string
	color   lipgloss.Color
	expires time.Time
}

func (s statusMessage) visible() bool {
	return s.text != "" && time.Now().Before(s.expires)
}

type model struct {
	client         *transmission.Client
	torrents       []transmission.Torrent
	selected       int // index into visibleTorrents()
	focus          pane
	width          int
	height         int
	lastErr        error
	ready          bool
	lastRefresh    time.Time
	mode           tuiMode
	addModal       addModal
	deleteModal    deleteModal
	filter         filterState
	defaultSaveDir string
	status         statusMessage
}

type torrentsMsg struct{ torrents []transmission.Torrent }
type errMsg struct{ err error }
type tickMsg struct{}
type sessionInfoMsg struct{ info *transmission.SessionInfo }
type addResultMsg struct {
	result *transmission.AddResult
	err    error
}

const refreshInterval = time.Second
const statusTTL = 2 * time.Second

// Run launches the Bubble Tea TUI against the local transmission-daemon.
func Run() error {
	c := transmission.New(transmission.DefaultURL)
	m := model{
		client: c,
		filter: newFilterState(),
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(refresh(m.client), tick(), fetchSession(m.client))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case torrentsMsg:
		m.torrents = msg.torrents
		m.lastErr = nil
		m.lastRefresh = time.Now()
		m.clampSelected()
		return m, nil

	case errMsg:
		m.lastErr = msg.err
		return m, nil

	case tickMsg:
		return m, tea.Batch(refresh(m.client), tick())

	case sessionInfoMsg:
		if msg.info != nil {
			m.defaultSaveDir = msg.info.DownloadDir
		}
		return m, nil

	case addResultMsg:
		m.addModal.submitting = false
		if msg.err != nil {
			m.addModal.err = cleanError(msg.err)
			return m, nil
		}
		if msg.result != nil && msg.result.Duplicate {
			m.addModal.err = "already added: " + msg.result.Name
			return m, nil
		}
		m.mode = modeNormal
		return m, refresh(m.client)

	case actionResultMsg:
		if msg.err != nil {
			m.setStatus("✗ "+msg.label+": "+cleanError(msg.err), warnColor)
		} else {
			m.setStatus("✓ "+msg.label, successColor)
		}
		return m, refresh(m.client)

	case tea.KeyMsg:
		return m.routeKey(msg)
	}

	// Forward unhandled messages (e.g., cursor blink) to whichever input is active.
	switch m.mode {
	case modeAdd:
		return m, m.addModal.forwardKey(msg)
	case modeFilter:
		var cmd tea.Cmd
		m.filter.input, cmd = m.filter.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeAdd:
		return m.handleAddKey(msg)
	case modeDelete:
		return m.handleDeleteKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

// --- normal mode ---

func (m model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		visible := m.visibleTorrents()
		if m.focus == paneList && len(visible) > 0 {
			m.selected = min(m.selected+1, len(visible)-1)
		}
	case "k", "up":
		if m.focus == paneList && len(m.visibleTorrents()) > 0 {
			m.selected = max(m.selected-1, 0)
		}
	case "g":
		if m.focus == paneList {
			m.selected = 0
		}
	case "G":
		visible := m.visibleTorrents()
		if m.focus == paneList && len(visible) > 0 {
			m.selected = len(visible) - 1
		}
	case "h", "left":
		m.focus = paneList
	case "l", "right":
		m.focus = paneDetails
	case "tab":
		if m.focus == paneList {
			m.focus = paneDetails
		} else {
			m.focus = paneList
		}

	case "a":
		m.addModal = newAddModal(m.defaultSaveDir)
		m.mode = modeAdd
		return m, m.addModal.magnet.Focus()

	case "?":
		m.mode = modeHelp
		return m, nil

	case "/":
		m.filter.saved = m.filter.query
		m.filter.input.SetValue(m.filter.query)
		m.filter.input.CursorEnd()
		m.mode = modeFilter
		return m, m.filter.input.Focus()

	case "r":
		m.setStatus("✓ refreshing", successColor)
		return m, refresh(m.client)

	case " ":
		t, ok := m.currentTorrent()
		if !ok {
			return m, nil
		}
		client := m.client
		if t.IsPaused() {
			return m, runAction("resumed: "+t.Name, func() error { return client.TorrentStart(t.ID) })
		}
		return m, runAction("paused: "+t.Name, func() error { return client.TorrentStop(t.ID) })

	case "R":
		t, ok := m.currentTorrent()
		if !ok {
			return m, nil
		}
		client := m.client
		return m, runAction("re-announcing: "+t.Name, func() error { return client.TorrentReannounce(t.ID) })

	case "v":
		t, ok := m.currentTorrent()
		if !ok {
			return m, nil
		}
		client := m.client
		return m, runAction("verifying: "+t.Name, func() error { return client.TorrentVerify(t.ID) })

	case "y":
		t, ok := m.currentTorrent()
		if !ok {
			return m, nil
		}
		if t.MagnetLink == "" {
			m.setStatus("✗ no magnet link available", warnColor)
			return m, nil
		}
		if err := clipboard.WriteAll(t.MagnetLink); err != nil {
			m.setStatus("✗ clipboard: "+err.Error(), warnColor)
			return m, nil
		}
		m.setStatus("✓ magnet copied", successColor)
		return m, nil

	case "o":
		t, ok := m.currentTorrent()
		if !ok {
			return m, nil
		}
		if err := openFolder(t.DownloadDir); err != nil {
			m.setStatus("✗ open: "+err.Error(), warnColor)
		} else {
			m.setStatus("✓ opened "+t.DownloadDir, successColor)
		}
		return m, nil

	case "d":
		t, ok := m.currentTorrent()
		if !ok {
			return m, nil
		}
		m.deleteModal = deleteModal{torrentID: t.ID, torrentName: t.Name, withFiles: false}
		m.mode = modeDelete
		return m, nil

	case "D":
		t, ok := m.currentTorrent()
		if !ok {
			return m, nil
		}
		m.deleteModal = deleteModal{torrentID: t.ID, torrentName: t.Name, withFiles: true}
		m.mode = modeDelete
		return m, nil
	}
	return m, nil
}

// --- add modal mode ---

func (m model) handleAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "tab":
		return m, m.addModal.setFocus(m.addModal.focused + 1)
	case "shift+tab":
		return m, m.addModal.setFocus(m.addModal.focused - 1)
	case "enter":
		return m.submitAdd()
	}
	return m, m.addModal.forwardKey(msg)
}

func (m model) submitAdd() (tea.Model, tea.Cmd) {
	magnet, saveDir, vErr := prepareAdd(m.addModal.magnet.Value(), m.addModal.saveDir.Value())
	if vErr != "" {
		m.addModal.err = vErr
		return m, nil
	}
	m.addModal.err = ""
	m.addModal.submitting = true
	client := m.client
	return m, func() tea.Msg {
		r, err := client.TorrentAdd(magnet, saveDir)
		return addResultMsg{result: r, err: err}
	}
}

// --- delete modal mode ---

func (m model) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		id := m.deleteModal.torrentID
		name := m.deleteModal.torrentName
		withFiles := m.deleteModal.withFiles
		m.mode = modeNormal
		label := "deleted: " + name
		if withFiles {
			label = "deleted (with files): " + name
		}
		client := m.client
		return m, runAction(label, func() error { return client.TorrentRemove(withFiles, id) })
	case "n", "N", "esc", "ctrl+c":
		m.mode = modeNormal
	}
	return m, nil
}

// --- filter mode ---

func (m model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filter.query = m.filter.saved
		m.mode = modeNormal
		m.clampSelected()
		return m, nil
	case "enter":
		m.mode = modeNormal
		m.clampSelected()
		return m, nil
	}
	var cmd tea.Cmd
	m.filter.input, cmd = m.filter.input.Update(msg)
	m.filter.query = m.filter.input.Value()
	m.clampSelected()
	return m, cmd
}

// --- help mode ---

func (m model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	m.mode = modeNormal
	return m, nil
}

// --- helpers ---

func (m *model) setStatus(text string, color lipgloss.Color) {
	m.status = statusMessage{
		text:    text,
		color:   color,
		expires: time.Now().Add(statusTTL),
	}
}

func (m *model) clampSelected() {
	visible := m.visibleTorrents()
	if len(visible) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(visible) {
		m.selected = len(visible) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m model) visibleTorrents() []transmission.Torrent {
	return m.filter.apply(m.torrents)
}

func (m model) currentTorrent() (transmission.Torrent, bool) {
	visible := m.visibleTorrents()
	if len(visible) == 0 || m.selected < 0 || m.selected >= len(visible) {
		return transmission.Torrent{}, false
	}
	return visible[m.selected], true
}

// --- commands ---

func refresh(c *transmission.Client) tea.Cmd {
	return func() tea.Msg {
		ts, err := c.TorrentGet()
		if err != nil {
			return errMsg{err}
		}
		return torrentsMsg{torrents: ts}
	}
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func fetchSession(c *transmission.Client) tea.Cmd {
	return func() tea.Msg {
		info, err := c.SessionGet()
		if err != nil {
			return errMsg{err}
		}
		return sessionInfoMsg{info: info}
	}
}
