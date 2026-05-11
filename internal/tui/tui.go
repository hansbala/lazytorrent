package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazytorrent/internal/transmission"
)

type pane int

const (
	paneList pane = iota
	paneDetails
)

type model struct {
	client         *transmission.Client
	torrents       []transmission.Torrent
	selected       int
	focus          pane
	width          int
	height         int
	lastErr        error
	ready          bool
	lastRefresh    time.Time
	modal          addModal
	defaultSaveDir string
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

// Run launches the Bubble Tea TUI against the local transmission-daemon.
func Run() error {
	c := transmission.New(transmission.DefaultURL)
	m := model{client: c}
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
		if len(m.torrents) == 0 {
			m.selected = 0
		} else if m.selected >= len(m.torrents) {
			m.selected = len(m.torrents) - 1
		}
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
		m.modal.submitting = false
		if msg.err != nil {
			m.modal.err = cleanError(msg.err)
			return m, nil
		}
		if msg.result != nil && msg.result.Duplicate {
			m.modal.err = "already added: " + msg.result.Name
			return m, nil
		}
		m.modal.active = false
		return m, refresh(m.client)

	case tea.KeyMsg:
		if m.modal.active {
			return m.handleModalKey(msg)
		}
		return m.handleKey(msg)
	}

	// Forward unhandled messages (e.g., cursor blink) to the focused modal input.
	if m.modal.active {
		cmd := m.modal.forwardKey(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.focus == paneList && len(m.torrents) > 0 {
			m.selected = min(m.selected+1, len(m.torrents)-1)
		}
	case "k", "up":
		if m.focus == paneList && len(m.torrents) > 0 {
			m.selected = max(m.selected-1, 0)
		}
	case "g":
		if m.focus == paneList {
			m.selected = 0
		}
	case "G":
		if m.focus == paneList && len(m.torrents) > 0 {
			m.selected = len(m.torrents) - 1
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
		m.modal = newAddModal(m.defaultSaveDir)
		m.modal.active = true
		cmd := m.modal.magnet.Focus()
		return m, cmd
	}
	return m, nil
}

func (m model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal.active = false
		return m, nil
	case "tab":
		return m, m.modal.setFocus(m.modal.focused + 1)
	case "shift+tab":
		return m, m.modal.setFocus(m.modal.focused - 1)
	case "enter":
		return m.submitAdd()
	}
	cmd := m.modal.forwardKey(msg)
	return m, cmd
}

func (m model) submitAdd() (tea.Model, tea.Cmd) {
	magnet, saveDir, vErr := prepareAdd(m.modal.magnet.Value(), m.modal.saveDir.Value())
	if vErr != "" {
		m.modal.err = vErr
		return m, nil
	}
	m.modal.err = ""
	m.modal.submitting = true
	client := m.client
	return m, func() tea.Msg {
		r, err := client.TorrentAdd(magnet, saveDir)
		return addResultMsg{result: r, err: err}
	}
}

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
