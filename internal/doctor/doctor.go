package doctor

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lazytorrent/internal/transmission"
)

type check struct {
	name              string
	fn                func() (detail string, err error)
	dependsOn         string
	hint              string
	requiredAtStartup bool
}

type result struct {
	name    string
	ok      bool
	skipped bool
	detail  string
	err     error
	hint    string
}

func allChecks() []check {
	p := currentPlatform()
	return []check{
		{
			name:              "transmission-daemon installed",
			fn:                checkDaemonBinary,
			hint:              "Install with:\n" + indent(p.InstallCommand(), "    "),
			requiredAtStartup: true,
		},
		{
			name:              "daemon running",
			fn:                checkDaemonRunning,
			dependsOn:         "transmission-daemon installed",
			hint:              "Start it:\n" + indent(p.StartCommand(), "    "),
			requiredAtStartup: true,
		},
		{
			name:      "RPC reachable",
			fn:        checkRPCReachable,
			dependsOn: "daemon running",
			hint: "The daemon is running but its RPC port (9091) isn't accepting connections.\n" +
				"Check that `rpc-enabled` is true and `rpc-port` is 9091 in:\n" +
				indent(p.SettingsPath(), "    "),
			requiredAtStartup: true,
		},
		{
			name:              "RPC authenticated",
			fn:                checkRPCAuth,
			dependsOn:         "RPC reachable",
			hint:              "RPC requires credentials. Configure them in:\n    ~/.config/lazytorrent/config.toml",
			requiredAtStartup: true,
		},
		{
			name: "config file",
			fn:   checkConfigFile,
		},
		{
			name:      "default download directory",
			fn:        checkDownloadDir,
			dependsOn: "RPC authenticated",
		},
	}
}

// Run prints the full diagnostic report to w. Returns an error if any check failed.
func Run(w io.Writer) error {
	return renderReport(w, runChecks(allChecks()))
}

// Precheck runs only the startup-critical chain. On the first failure it
// returns a user-facing message (problem statement + hint) and an error.
// On full success it returns ("", nil).
func Precheck() (string, error) {
	return precheckChecks(allChecks())
}

// --- orchestration (testable with synthetic checks) ---

func runChecks(checks []check) []*result {
	results := make(map[string]*result, len(checks))
	ordered := make([]*result, 0, len(checks))

	for _, c := range checks {
		r := &result{name: c.name, hint: c.hint}
		results[c.name] = r
		ordered = append(ordered, r)

		if c.dependsOn != "" {
			dep, ok := results[c.dependsOn]
			if !ok || !dep.ok {
				r.skipped = true
				continue
			}
		}
		detail, err := c.fn()
		if err != nil {
			r.err = err
			continue
		}
		r.ok = true
		r.detail = detail
	}
	return ordered
}

func renderReport(w io.Writer, results []*result) error {
	c := newPainter(w)
	for _, r := range results {
		switch {
		case r.skipped:
			fmt.Fprintf(w, "%s %-32s %s\n", c.dim("○"), r.name, c.dim("skipped"))
		case r.ok:
			fmt.Fprintf(w, "%s %-32s %s\n", c.green("✓"), r.name, r.detail)
		default:
			fmt.Fprintf(w, "%s %-32s %s\n", c.red("✗"), r.name, c.red(r.err.Error()))
		}
	}

	first := firstFailure(results)
	if first == nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, c.green("All checks passed."))
		return nil
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, c.bold(problemLine(first)))
	if first.hint != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, first.hint)
	}
	return fmt.Errorf("doctor checks failed")
}

func precheckChecks(checks []check) (string, error) {
	for _, c := range checks {
		if !c.requiredAtStartup {
			continue
		}
		if _, err := c.fn(); err != nil {
			r := &result{name: c.name, err: err, hint: c.hint}
			msg := problemLine(r)
			if c.hint != "" {
				msg += "\n\n" + c.hint
			}
			msg += "\n\nRun `lazytorrent --doctor` for details."
			return msg, fmt.Errorf("startup precheck failed")
		}
	}
	return "", nil
}

func firstFailure(results []*result) *result {
	for _, r := range results {
		if !r.ok && !r.skipped {
			return r
		}
	}
	return nil
}

func problemLine(r *result) string {
	switch r.name {
	case "transmission-daemon installed":
		return "transmission-daemon is not installed."
	case "daemon running":
		return "Transmission daemon is not running."
	case "RPC reachable":
		return "Transmission RPC is not reachable."
	case "RPC authenticated":
		return "Transmission RPC authentication failed."
	case "default download directory":
		return "Default download directory is unusable."
	}
	return r.name + " failed: " + r.err.Error()
}

// --- individual checks ---

func checkDaemonBinary() (string, error) {
	path, err := exec.LookPath("transmission-daemon")
	if err != nil {
		return "", fmt.Errorf("not on PATH")
	}
	out, _ := exec.Command(path, "--version").CombinedOutput()
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if v := extractVersion(line); v != "" {
		return fmt.Sprintf("%s  v%s", path, v), nil
	}
	return path, nil
}

func checkDaemonRunning() (string, error) {
	out, err := exec.Command("pgrep", "-x", "transmission-daemon").Output()
	if err != nil {
		return "", fmt.Errorf("not running")
	}
	pid := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return "pid " + pid, nil
}

func checkRPCReachable() (string, error) {
	u, err := url.Parse(transmission.DefaultURL)
	if err != nil {
		return "", err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		return "", fmt.Errorf("cannot connect to %s", host)
	}
	conn.Close()
	return transmission.DefaultURL, nil
}

func checkRPCAuth() (string, error) {
	c := transmission.New(transmission.DefaultURL)
	info, err := c.SessionGet()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Transmission %s, RPC v%d", info.Version, info.RPCVersion), nil
}

func checkConfigFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".config", "lazytorrent", "config.toml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "using defaults  (" + path + " not present)", nil
		}
		return "", err
	}
	return path, nil
}

func checkDownloadDir() (string, error) {
	c := transmission.New(transmission.DefaultURL)
	info, err := c.SessionGet()
	if err != nil {
		return "", err
	}

	dir := expandHome(info.DownloadDir)
	stat, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %v", info.DownloadDir, err)
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("%s is not a directory", info.DownloadDir)
	}
	if !isWritable(dir) {
		return "", fmt.Errorf("%s is not writable", info.DownloadDir)
	}

	if free, err := diskFree(dir); err == nil {
		return fmt.Sprintf("%s  (writable, %s free)", info.DownloadDir, humanBytes(free)), nil
	}
	return info.DownloadDir + "  (writable)", nil
}

// --- helpers ---

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

func extractVersion(line string) string {
	for _, p := range strings.Fields(line) {
		if len(p) > 0 && p[0] >= '0' && p[0] <= '9' {
			return p
		}
	}
	return ""
}

func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}

func isWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".lazytorrent-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func diskFree(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// --- color ---

type painter struct{ color bool }

func newPainter(w io.Writer) painter {
	if os.Getenv("NO_COLOR") != "" {
		return painter{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return painter{}
	}
	stat, err := f.Stat()
	if err != nil {
		return painter{}
	}
	return painter{color: stat.Mode()&os.ModeCharDevice != 0}
}

func (p painter) wrap(code, s string) string {
	if !p.color {
		return s
	}
	return code + s + "\x1b[0m"
}

func (p painter) red(s string) string   { return p.wrap("\x1b[31m", s) }
func (p painter) green(s string) string { return p.wrap("\x1b[32m", s) }
func (p painter) dim(s string) string   { return p.wrap("\x1b[2m", s) }
func (p painter) bold(s string) string  { return p.wrap("\x1b[1m", s) }
