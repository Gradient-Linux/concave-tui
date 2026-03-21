package ui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPrinterFormatsStatusLines(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	Pass("Docker", "running")
	Fail("GPU", "missing")
	Warn("Internet", "slow")
	Info("Pulling", "image")

	out := buf.String()
	for _, token := range []string{"✓", "✗", "⚠", "→"} {
		if !strings.Contains(out, token) {
			t.Fatalf("expected %q in output %q", token, out)
		}
	}
	if !strings.Contains(out, "Docker") || !strings.Contains(out, "running") {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestSpinnerStartStop(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	spinner := NewSpinner("Pulling")
	spinner.Start()
	spinner.Stop("done")

	if !strings.Contains(buf.String(), "Pulling") {
		t.Fatalf("expected spinner output, got %q", buf.String())
	}
}

func TestHeaderConfirmAndChecklist(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	Header("Title")
	Line("Raw line")
	if !strings.Contains(buf.String(), "Title") {
		t.Fatalf("expected header output, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "Raw line") {
		t.Fatalf("expected line output, got %q", buf.String())
	}

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	if _, err := w.WriteString("yes\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	if !Confirm("Proceed?") {
		t.Fatal("expected Confirm to accept yes")
	}
	_ = r.Close()

	r, w, err = os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	if _, err := w.WriteString("2, 1, 2, 9\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	selected := Checklist([]string{"boosting", "flow"})
	_ = r.Close()
	os.Stdin = oldStdin

	if len(selected) != 2 || selected[0] != "flow" || selected[1] != "boosting" {
		t.Fatalf("Checklist() = %#v", selected)
	}
}
