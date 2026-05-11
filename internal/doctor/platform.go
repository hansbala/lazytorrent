package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// platform produces install/start/configure hints appropriate for the host.
// Fields are exposed so tests can construct synthetic platforms.
type platform struct {
	GOOS   string
	Home   string
	HasBin func(string) bool
}

func currentPlatform() platform {
	home, _ := os.UserHomeDir()
	return platform{
		GOOS: runtime.GOOS,
		Home: home,
		HasBin: func(b string) bool {
			_, err := exec.LookPath(b)
			return err == nil
		},
	}
}

func (p platform) InstallCommand() string {
	switch p.GOOS {
	case "darwin":
		if p.HasBin("brew") {
			return "brew install transmission-cli"
		}
		return "Install transmission-daemon via Homebrew, MacPorts, or your preferred package manager."
	case "linux":
		switch {
		case p.HasBin("apt"):
			return "sudo apt install transmission-daemon"
		case p.HasBin("dnf"):
			return "sudo dnf install transmission"
		case p.HasBin("pacman"):
			return "sudo pacman -S transmission-cli"
		case p.HasBin("zypper"):
			return "sudo zypper install transmission"
		}
		return "Install transmission-daemon using your distribution's package manager."
	}
	return "Install transmission-daemon for your platform."
}

func (p platform) StartCommand() string {
	switch p.GOOS {
	case "darwin":
		if p.HasBin("brew") {
			return "brew services start transmission-cli"
		}
		return "transmission-daemon"
	case "linux":
		if p.HasBin("systemctl") {
			return "systemctl --user start transmission-daemon\n# or: sudo systemctl start transmission-daemon"
		}
		return "transmission-daemon"
	}
	return "transmission-daemon"
}

func (p platform) SettingsPath() string {
	if p.Home == "" {
		return "the transmission-daemon settings.json"
	}
	switch p.GOOS {
	case "darwin":
		return filepath.Join(p.Home, "Library", "Application Support", "transmission-daemon", "settings.json")
	case "linux":
		return filepath.Join(p.Home, ".config", "transmission-daemon", "settings.json")
	}
	return filepath.Join(p.Home, ".config", "transmission-daemon", "settings.json")
}
