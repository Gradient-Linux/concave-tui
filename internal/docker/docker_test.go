package docker

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/workspace"
)

type mockRunner struct {
	run func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func (m mockRunner) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return m.run(ctx, name, args...)
}

func TestPullStreamsProgress(t *testing.T) {
	oldStream := streamCommand
	t.Cleanup(func() { streamCommand = oldStream })

	var lines []string
	streamCommand = func(ctx context.Context, name string, args []string, onLine func(string)) error {
		onLine("layer one")
		onLine("layer two")
		return nil
	}

	if err := Pull(context.Background(), "busybox", func(line string) { lines = append(lines, line) }); err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected streamed lines, got %#v", lines)
	}
}

func TestRunnerAndClientWrappers(t *testing.T) {
	oldRunner := commandRunner
	oldInteractive := runInteractiveCommand
	t.Cleanup(func() {
		commandRunner = oldRunner
		runInteractiveCommand = oldInteractive
	})

	var calls []string
	commandRunner = mockRunner{run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		switch args[0] {
		case "inspect":
			return []byte("running"), nil
		default:
			return []byte("ok"), nil
		}
	}}
	runInteractiveCommand = func(ctx context.Context, name string, args ...string) error {
		calls = append(calls, strings.Join(args, " "))
		return nil
	}

	if _, err := (DefaultRunner{}).RunCommand(context.Background(), "sh", "-c", "printf ok"); err != nil {
		t.Fatalf("DefaultRunner.RunCommand() error = %v", err)
	}
	if err := Run(context.Background(), "busybox", "echo", "ok"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if err := Exec(context.Background(), "demo", "echo", "ok"); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if err := ComposeUp(context.Background(), "/tmp/demo.yml", true); err != nil {
		t.Fatalf("ComposeUp() error = %v", err)
	}
	if err := ComposeDown(context.Background(), "/tmp/demo.yml"); err != nil {
		t.Fatalf("ComposeDown() error = %v", err)
	}
	if err := ContainerLogs(context.Background(), "demo", true); err != nil {
		t.Fatalf("ContainerLogs() error = %v", err)
	}
	if len(calls) == 0 {
		t.Fatal("expected docker calls")
	}
}

func TestContainerStatusNormalizesStates(t *testing.T) {
	oldRunner := commandRunner
	t.Cleanup(func() { commandRunner = oldRunner })

	tests := []struct {
		name    string
		out     []byte
		err     error
		want    string
		wantErr bool
	}{
		{name: "running", out: []byte("running\n"), want: "running"},
		{name: "stopped", out: []byte("exited\n"), want: "stopped"},
		{name: "missing", out: []byte("Error: No such object"), err: errors.New("missing"), want: "not found"},
		{name: "generic error", out: []byte("boom"), err: errors.New("boom"), want: "error", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandRunner = mockRunner{run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return tt.out, tt.err
			}}
			got, err := ContainerStatus(context.Background(), "demo")
			if got != tt.want {
				t.Fatalf("ContainerStatus() = %q, want %q", got, tt.want)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("ContainerStatus() err = %v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteComposeUsesManifestOverrides(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := workspace.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	manifest := config.RecordInstall(config.VersionManifest{}, testInstallRecord{
		name: "boosting",
		images: map[string]string{
			"gradient-boost-core": "python:3.12-slim",
		},
	})
	manifest = config.RecordUpdate(manifest, "boosting", "gradient-boost-core", "python:3.12-alpine")
	if err := config.SaveManifest(manifest); err != nil {
		t.Fatalf("SaveManifest() error = %v", err)
	}

	oldReadTemplate := readTemplateFile
	oldRunner := commandRunner
	t.Cleanup(func() {
		readTemplateFile = oldReadTemplate
		commandRunner = oldRunner
	})

	readTemplateFile = func(path string) ([]byte, error) {
		return []byte("services:\n  gradient-boost-core:\n    image: python:3.12-slim\n    volumes:\n      - \"{{WORKSPACE_ROOT}}/data:/data\"\nnetworks:\n  \"{{COMPOSE_NETWORK}}\": {}\n"), nil
	}
	commandRunner = mockRunner{run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}}

	path, err := WriteCompose("boosting")
	if err != nil {
		t.Fatalf("WriteCompose() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "python:3.12-alpine") {
		t.Fatalf("expected manifest override in compose:\n%s", text)
	}
	if !strings.Contains(text, workspace.Root()) {
		t.Fatalf("expected workspace root substitution in compose:\n%s", text)
	}
}

type testInstallRecord struct {
	name   string
	images map[string]string
}

func (t testInstallRecord) RecordName() string {
	return t.name
}

func (t testInstallRecord) RecordImages() map[string]string {
	return t.images
}

func TestWriteComposeMissingTemplate(t *testing.T) {
	oldReadTemplate := readTemplateFile
	t.Cleanup(func() { readTemplateFile = oldReadTemplate })

	readTemplateFile = func(path string) ([]byte, error) {
		return nil, os.ErrNotExist
	}

	if _, err := WriteCompose("missing"); err == nil || err.Error() != "no compose template found for suite: missing" {
		t.Fatalf("WriteCompose() error = %v", err)
	}
}

func TestWriteRawComposeValidationFailureRemovesTempFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := workspace.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	oldRunner := commandRunner
	t.Cleanup(func() { commandRunner = oldRunner })
	commandRunner = mockRunner{run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("invalid compose")
	}}

	_, err := WriteRawCompose("boosting", []byte("services:\n  demo:\n    image: busybox\n"))
	if err == nil {
		t.Fatal("expected validation error")
	}
	if _, statErr := os.Stat(ComposePath("boosting") + ".tmp"); !os.IsNotExist(statErr) {
		t.Fatalf("expected temp file removal, got %v", statErr)
	}
}

func TestImageHelpers(t *testing.T) {
	oldRunner := commandRunner
	oldStream := streamCommand
	t.Cleanup(func() {
		commandRunner = oldRunner
		streamCommand = oldStream
	})

	var calls []string
	commandRunner = mockRunner{run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		switch strings.Join(args, " ") {
		case "image inspect busybox:1.36":
			return []byte("present"), nil
		case "image inspect busybox:gradient-previous":
			return []byte("present"), nil
		default:
			return []byte("ok"), nil
		}
	}}
	streamCommand = func(ctx context.Context, name string, args []string, onLine func(string)) error {
		if onLine != nil {
			onLine("pulling")
		}
		return nil
	}

	exists, err := ImageExists("busybox:1.36")
	if err != nil || !exists {
		t.Fatalf("ImageExists() = %v, %v", exists, err)
	}
	if err := TagAsPrevious("busybox:1.36"); err != nil {
		t.Fatalf("TagAsPrevious() error = %v", err)
	}
	if err := PullWithRollbackSafety(context.Background(), "busybox:1.36", nil); err != nil {
		t.Fatalf("PullWithRollbackSafety() error = %v", err)
	}
	if err := PullWithProgress(context.Background(), "busybox:1.36", nil); err != nil {
		t.Fatalf("PullWithProgress() error = %v", err)
	}
	if err := RevertToPrevious("busybox:1.36"); err != nil {
		t.Fatalf("RevertToPrevious() error = %v", err)
	}
	if len(calls) == 0 {
		t.Fatal("expected docker calls")
	}
}

func TestImageHelpersMissingPaths(t *testing.T) {
	oldRunner := commandRunner
	t.Cleanup(func() { commandRunner = oldRunner })

	commandRunner = mockRunner{run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("no such image"), errors.New("missing")
	}}

	exists, err := ImageExists("busybox:missing")
	if err != nil {
		t.Fatalf("ImageExists() error = %v", err)
	}
	if exists {
		t.Fatal("expected image to be missing")
	}
	if err := TagAsPrevious("busybox:missing"); err != nil {
		t.Fatalf("TagAsPrevious() error = %v", err)
	}
	if err := RevertToPrevious("busybox:missing"); err == nil {
		t.Fatal("expected RevertToPrevious() to fail without previous tag")
	}
	if !isMissingImage(errors.New("missing"), []byte("No such image")) {
		t.Fatal("expected missing-image helper to match")
	}
}

func TestDefaultTimeoutApplied(t *testing.T) {
	oldRunner := commandRunner
	t.Cleanup(func() { commandRunner = oldRunner })

	commandRunner = mockRunner{run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected deadline to be set")
		}
		if remaining := time.Until(deadline); remaining > defaultTimeout || remaining <= 0 {
			t.Fatalf("unexpected remaining timeout %v", remaining)
		}
		return []byte("running"), nil
	}}

	if _, err := ContainerStatus(context.Background(), "demo"); err != nil {
		t.Fatalf("ContainerStatus() error = %v", err)
	}
}
