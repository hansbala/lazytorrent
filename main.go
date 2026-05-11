package main

import (
	"flag"
	"fmt"
	"os"

	"lazytorrent/internal/doctor"
	"lazytorrent/internal/tui"
)

func main() {
	doctorFlag := flag.Bool("doctor", false, "Run system diagnostic checks and exit")
	flag.Parse()

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
