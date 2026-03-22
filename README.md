# concave-tui

`concave-tui` is the terminal frontend for Gradient Linux. It provides a dedicated
Bubble Tea interface for suite operations, workspace inspection, logs, health, and
admin views while treating `concave serve` as the backend authority.

## Responsibilities

This repository owns:

- the `concave-tui` terminal application
- login and cached-session bootstrap against `concave serve`
- role-aware terminal views and keybindings
- terminal-first rendering for workspace, suites, logs, doctor, system, and users pages

This repository does not own:

- PAM auth
- Unix group resolution
- privileged backend operations
- browser-facing UI

Those live in `concave` and `concave-web`.

## Runtime expectations

`concave-tui` expects:

- a reachable `concave serve` instance
- a valid cached session or valid host credentials for login
- ANSI-capable terminal output

Important paths:

- session cache: `~/.config/concave/session.json`
- TUI config: `~/.config/concave-tui/config.toml`

## Views

The TUI currently exposes:

- Workspace
- Suites
- Logs
- Doctor
- System
- Users

Role-aware behavior:

- viewer: read-only operational views
- developer: interactive lab/shell/exec-style suite access where allowed
- operator: suite lifecycle and workspace mutation
- admin: System and Users views plus machine-level actions

## Build

```bash
go test ./...
go test -race ./...
go vet ./...
CGO_ENABLED=0 go build -o concave-tui ./cmd/concave-tui/
```

## Run

```bash
./concave-tui --help
./concave-tui
```

If a valid cached session exists, startup skips the login prompt. Otherwise the TUI
authenticates against `concave serve` and writes a new local session file.

## Documentation

Start with [docs/README.md](docs/README.md).

Important docs:

- [docs/architecture.md](docs/architecture.md)
- [docs/views.md](docs/views.md)
- [docs/runtime.md](docs/runtime.md)

## Relationship to the stack

- `concave`: infrastructure CLI and authenticated control plane
- `concave-web`: browser control plane
- `concave-tui`: terminal control plane client

The split keeps Bubble Tea and terminal UX dependencies out of the infrastructure
binary.
