package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"lazytorrent/internal/doctor"
	"lazytorrent/internal/tui"
)

// Version is set at build time via -ldflags "-X main.Version=v0.1.0".
// Local `go build` produces a binary that reports "dev" (or the module
// version reported by debug.BuildInfo if installed via `go install`).
var Version = "dev"

func main() {
	doctorFlag := flag.Bool("doctor", false, "Run system diagnostic checks and exit")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("lazytorrent", versionString())
		return
	}

	if *doctorFlag {
		if err := doctor.Run(os.Stdout); err != nil {
			os.Exit(1)
		}
		return
	}

	if msg, err := doctor.Precheck(); err != nil {
		fmt.Fprintln(os.Stderr, msg)
		_ = err
		os.Exit(1)
	}

	if err := tui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lazytorrent: %v\n", err)
		os.Exit(1)
	}
}

func versionString() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return Version
}
