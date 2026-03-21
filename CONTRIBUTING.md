# Contributing to concave-tui

`concave-tui` is the terminal frontend for Gradient Linux operations.

## Scope

- Repository: `github.com/Gradient-Linux/concave-tui`
- Go module: `github.com/Gradient-Linux/concave-tui`
- Target platform: Ubuntu 24.04 LTS
- Primary deliverable: a static `concave-tui` binary

## Local Checks

Run these before opening a pull request:

```bash
go test ./...
go vet ./...
CGO_ENABLED=0 go build -o concave-tui ./cmd/concave-tui/
```

## Dependency Policy

Direct dependencies stay intentionally small.

Approved direct dependencies:

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/bubbles`

New direct dependencies require maintainer approval.

## Project Boundaries

- `concave-tui` owns the terminal UI layer
- `concave` owns the infrastructure CLI and headless-safe backend concerns
- Do not move Bubble Tea or UI rendering code back into the `concave` repository

## Pull Requests

Use small, coherent changes and update docs in the same pull request when behavior
changes.
