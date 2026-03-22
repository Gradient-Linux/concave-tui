# concave-tui

`concave-tui` is the terminal interface for Gradient Linux operations.

It is intentionally a separate repository from `concave`. The `concave` project
stays focused on infrastructure concerns: Docker lifecycle, GPU detection,
workspace state, and headless-safe automation. `concave-tui` owns the Bubble Tea
UI layer and the terminal interaction model.

## What This Repo Contains

- the `concave-tui` Bubble Tea application
- a sessioned client for `concave serve`
- role-aware Workspace, Suites, Logs, Doctor, System, and Users views
- local hardware/workspace rendering helpers that complement the authenticated API

## Build

```bash
go test ./...
CGO_ENABLED=0 go build -o concave-tui ./cmd/concave-tui/
./concave-tui --help
```

## Runtime Expectations

- a reachable `concave serve` instance
- a valid cached session or valid Gradient Linux credentials for login
- a terminal with ANSI support

## Relationship to concave

- `concave` remains the infrastructure/backend project
- `concave-tui` is the terminal frontend project
- `concave-tui` authenticates against `concave serve` instead of owning privileged backend logic locally
- the two repos can evolve independently without pulling Bubble Tea dependencies into headless infrastructure builds
