package doctor

import (
	"strings"
	"testing"
)

func hasBin(present ...string) func(string) bool {
	set := make(map[string]bool, len(present))
	for _, b := range present {
		set[b] = true
	}
	return func(b string) bool { return set[b] }
}

func TestInstallCommand(t *testing.T) {
	cases := []struct {
		name     string
		goos     string
		bins     []string
		contains string
	}{
		{"darwin with brew", "darwin", []string{"brew"}, "brew install transmission-cli"},
		{"darwin without brew", "darwin", nil, "Homebrew"},
		{"linux with apt", "linux", []string{"apt"}, "sudo apt install transmission-daemon"},
		{"linux with dnf", "linux", []string{"dnf"}, "sudo dnf install transmission"},
		{"linux with pacman", "linux", []string{"pacman"}, "sudo pacman -S transmission-cli"},
		{"linux with zypper", "linux", []string{"zypper"}, "sudo zypper install transmission"},
		{"linux without any pkg mgr", "linux", nil, "distribution's package manager"},
		{"linux apt preferred over dnf", "linux", []string{"apt", "dnf"}, "sudo apt"},
		{"unknown GOOS falls back", "freebsd", nil, "transmission-daemon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := platform{GOOS: tc.goos, HasBin: hasBin(tc.bins...)}
			got := p.InstallCommand()
			if !strings.Contains(got, tc.contains) {
				t.Errorf("InstallCommand() = %q; want it to contain %q", got, tc.contains)
			}
		})
	}
}

func TestStartCommand(t *testing.T) {
	cases := []struct {
		name     string
		goos     string
		bins     []string
		contains string
	}{
		{"darwin with brew", "darwin", []string{"brew"}, "brew services start transmission-cli"},
		{"darwin without brew", "darwin", nil, "transmission-daemon"},
		{"linux with systemctl", "linux", []string{"systemctl"}, "systemctl --user start transmission-daemon"},
		{"linux without systemctl", "linux", nil, "transmission-daemon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := platform{GOOS: tc.goos, HasBin: hasBin(tc.bins...)}
			got := p.StartCommand()
			if !strings.Contains(got, tc.contains) {
				t.Errorf("StartCommand() = %q; want it to contain %q", got, tc.contains)
			}
		})
	}
}

func TestStartCommand_LinuxSystemctlOffersBothUserAndSystem(t *testing.T) {
	p := platform{GOOS: "linux", HasBin: hasBin("systemctl")}
	got := p.StartCommand()
	if !strings.Contains(got, "--user") {
		t.Errorf("expected user-mode hint, got %q", got)
	}
	if !strings.Contains(got, "sudo systemctl") {
		t.Errorf("expected system-wide hint, got %q", got)
	}
}

func TestSettingsPath(t *testing.T) {
	cases := []struct {
		name string
		goos string
		home string
		want string
	}{
		{"darwin", "darwin", "/Users/jane", "/Users/jane/Library/Application Support/transmission-daemon/settings.json"},
		{"linux", "linux", "/home/jane", "/home/jane/.config/transmission-daemon/settings.json"},
		{"unknown GOOS falls back to .config", "freebsd", "/home/jane", "/home/jane/.config/transmission-daemon/settings.json"},
		{"no home returns a generic placeholder", "linux", "", "the transmission-daemon settings.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := platform{GOOS: tc.goos, Home: tc.home, HasBin: hasBin()}
			got := p.SettingsPath()
			if got != tc.want {
				t.Errorf("SettingsPath() = %q; want %q", got, tc.want)
			}
		})
	}
}
