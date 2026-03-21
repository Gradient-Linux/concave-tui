package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 5 * time.Minute

// Runner executes external commands for Docker interactions.
type Runner interface {
	RunCommand(ctx context.Context, name string, args ...string) ([]byte, error)
}

// DefaultRunner uses exec.CommandContext.
type DefaultRunner struct{}

// RunCommand executes a command and captures combined output.
func (DefaultRunner) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

var (
	commandRunner Runner = DefaultRunner{}
	streamCommand        = func(ctx context.Context, name string, args []string, onLine func(string)) error {
		cmd := exec.CommandContext(ctx, name, args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("stdout pipe: %w", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("stderr pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return err
		}

		var (
			wg      sync.WaitGroup
			scanErr error
			errMu   sync.Mutex
		)
		for _, stream := range []io.Reader{stdout, stderr} {
			wg.Add(1)
			go func(reader io.Reader) {
				defer wg.Done()

				scanner := bufio.NewScanner(reader)
				for scanner.Scan() {
					if onLine != nil {
						onLine(scanner.Text())
					}
				}
				if err := scanner.Err(); err != nil {
					errMu.Lock()
					if scanErr == nil {
						scanErr = err
					}
					errMu.Unlock()
				}
			}(stream)
		}
		wg.Wait()
		if scanErr != nil {
			return fmt.Errorf("stream output: %w", scanErr)
		}

		if err := cmd.Wait(); err != nil {
			return err
		}
		return nil
	}
	runInteractiveCommand = func(ctx context.Context, name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
)

// Run executes docker run --rm with the supplied image and arguments.
func Run(ctx context.Context, image string, args ...string) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	command := append([]string{"run", "--rm", image}, args...)
	if _, err := commandRunner.RunCommand(ctx, "docker", command...); err != nil {
		return fmt.Errorf("docker run %s: %w", image, err)
	}
	return nil
}

// Exec executes a command inside a running container.
func Exec(ctx context.Context, container string, cmd ...string) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	command := append([]string{"exec", container}, cmd...)
	if _, err := commandRunner.RunCommand(ctx, "docker", command...); err != nil {
		return fmt.Errorf("docker exec %s: %w", container, err)
	}
	return nil
}

// Pull pulls an image and streams progress lines to the callback.
func Pull(ctx context.Context, image string, onProgress func(line string)) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	if err := streamCommand(ctx, "docker", []string{"pull", image}, onProgress); err != nil {
		return fmt.Errorf("docker pull %s: %w", image, err)
	}
	return nil
}

// ComposeUp starts a compose application.
func ComposeUp(ctx context.Context, composePath string, detach bool) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	command := []string{"compose", "-f", composePath, "up"}
	if detach {
		command = append(command, "-d")
	}
	if _, err := commandRunner.RunCommand(ctx, "docker", command...); err != nil {
		return fmt.Errorf("docker compose up %s: %w", composePath, err)
	}
	return nil
}

// ComposeDown stops and removes a compose application.
func ComposeDown(ctx context.Context, composePath string) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	command := []string{"compose", "-f", composePath, "down"}
	if _, err := commandRunner.RunCommand(ctx, "docker", command...); err != nil {
		return fmt.Errorf("docker compose down %s: %w", composePath, err)
	}
	return nil
}

// ContainerStatus returns a container's current status.
func ContainerStatus(ctx context.Context, name string) (string, error) {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	out, err := commandRunner.RunCommand(ctx, "docker", "inspect", "-f", "{{.State.Status}}", name)
	if err != nil {
		if isMissingContainer(err, out) {
			return "not found", nil
		}
		return "error", fmt.Errorf("docker inspect %s: %w", name, err)
	}

	switch strings.TrimSpace(string(out)) {
	case "running":
		return "running", nil
	case "created", "exited", "dead", "paused", "restarting", "removing":
		return "stopped", nil
	default:
		return "error", fmt.Errorf("docker inspect %s returned unknown status %q", name, strings.TrimSpace(string(out)))
	}
}

// ContainerLogs streams container logs to stdout/stderr.
func ContainerLogs(ctx context.Context, name string, follow bool) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, name)
	if err := runInteractiveCommand(ctx, "docker", args...); err != nil {
		return fmt.Errorf("docker logs %s: %w", name, err)
	}
	return nil
}

func withDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), defaultTimeout)
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= defaultTimeout {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

func isMissingContainer(err error, out []byte) bool {
	text := strings.ToLower(err.Error() + " " + string(out))
	return strings.Contains(text, "no such object") || strings.Contains(text, "no such container")
}
