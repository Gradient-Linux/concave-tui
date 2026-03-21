package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	outputMu sync.Mutex
	output   io.Writer = os.Stdout
)

// SetOutput directs UI output to the provided writer.
func SetOutput(w io.Writer) {
	outputMu.Lock()
	defer outputMu.Unlock()
	output = w
}

// ResetOutput restores UI output to stdout.
func ResetOutput() {
	SetOutput(os.Stdout)
}

// Pass prints a successful status line.
func Pass(label, detail string) {
	printStatus("✓", label, detail)
}

// Fail prints a failed status line.
func Fail(label, detail string) {
	printStatus("✗", label, detail)
}

// Warn prints a warning status line.
func Warn(label, detail string) {
	printStatus("⚠", label, detail)
}

// Info prints an informational status line.
func Info(label, detail string) {
	printStatus("→", label, detail)
}

// Header prints a section title.
func Header(title string) {
	outputMu.Lock()
	defer outputMu.Unlock()
	_, _ = fmt.Fprintf(output, "%s\n", title)
}

// Line prints raw formatted output through the shared writer.
func Line(text string) {
	outputMu.Lock()
	defer outputMu.Unlock()
	_, _ = fmt.Fprintln(output, text)
}

func printStatus(icon, label, detail string) {
	outputMu.Lock()
	defer outputMu.Unlock()
	_, _ = fmt.Fprintf(output, "  %s  %-20s %s\n", icon, label, detail)
}
