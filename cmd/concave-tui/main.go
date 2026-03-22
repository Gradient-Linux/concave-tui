package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
	"github.com/Gradient-Linux/concave-tui/cmd/concave-tui/model"
	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
)

// Version is injected at build time.
var Version = "dev"

var (
	terminalSupportedFn = terminalSupported
	loadConfigFn        = tuiconfig.Load
	loadSessionFn       = tuiauth.LoadSession
	exitProgram         = os.Exit
	runProgramFn        = func(root tea.Model) error {
		program := tea.NewProgram(root, tea.WithAltScreen(), tea.WithMouseCellMotion())
		_, err := program.Run()
		return err
	}
)

func main() {
	if code := run(os.Args[1:]); code != 0 {
		exitProgram(code)
	}
}

func run(args []string) int {
	fs := flag.NewFlagSet("concave-tui", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: concave-tui [--help] [--version]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "A full-screen terminal interface for concave.")
	}

	showHelp := fs.Bool("help", false, "show help")
	fs.BoolVar(showHelp, "h", false, "show help")
	showVersion := fs.Bool("version", false, "print version")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if *showHelp {
		fs.Usage()
		return 0
	}
	if *showVersion {
		fmt.Fprintln(os.Stdout, Version)
		return 0
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "concave-tui accepts no positional arguments")
		return 1
	}

	if !terminalSupportedFn() {
		fmt.Fprintln(os.Stderr, "concave-tui requires an interactive ANSI terminal")
		return 1
	}
	cfg, err := loadConfigFn()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	session, err := loadSessionFn()
	if err != nil {
		session = tuiauth.Session{}
	}

	root := model.NewRootModel(Version, cfg, session)
	if err := runProgramFn(root); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func terminalSupported() bool {
	if term := os.Getenv("TERM"); term == "" || term == "dumb" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func init() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.CommandLine.SetOutput(os.Stdout)
	time.Local = time.UTC
}
