package system

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type commandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) ([]byte, error) {
	return runCommand(name, args...)
}

var (
	runner      commandRunner = execRunner{}
	dialContext               = (&net.Dialer{Timeout: 3 * time.Second}).DialContext
)

// DockerRunning reports whether the Docker daemon can answer docker info.
func DockerRunning() (bool, error) {
	if _, err := runner.Run("docker", "info"); err != nil {
		return false, fmt.Errorf("docker info: %w", err)
	}
	return true, nil
}

// UserInDockerGroup reports whether the current user belongs to the docker group.
func UserInDockerGroup() (bool, error) {
	out, err := runner.Run("id", "-nG")
	if err != nil {
		return false, fmt.Errorf("id -nG: %w", err)
	}
	for _, group := range strings.Fields(string(out)) {
		if group == "docker" {
			return true, nil
		}
	}
	return false, nil
}

// InternetReachable reports whether the host can establish a TCP connection to a public resolver.
func InternetReachable() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := dialContext(ctx, "tcp", "1.1.1.1:53")
	if err != nil {
		return false, fmt.Errorf("dial 1.1.1.1:53: %w", err)
	}
	_ = conn.Close()
	return true, nil
}
