# concave-tui

`concave-tui` is the terminal interface for Gradient Linux operations.

It is intentionally a separate repository from `concave`. The `concave` project
stays focused on infrastructure concerns: Docker lifecycle, GPU detection,
workspace state, and headless-safe automation. `concave-tui` owns the Bubble Tea
UI layer and the terminal interaction model.

## What This Repo Contains

- the `concave-tui` Bubble Tea application
- the backend packages the TUI needs to manage suites, workspace state, Docker,
  and GPU checks without depending on `concave/internal/*`
- Compose templates required for suite-aware views and lifecycle actions

## Build

```bash
go test ./...
CGO_ENABLED=0 go build -o concave-tui ./cmd/concave-tui/
./concave-tui --help
```

## Runtime Expectations

- Docker Engine available locally
- a writable `~/gradient/` workspace
- a terminal with ANSI support

## Relationship to concave

- `concave` remains the infrastructure/backend project
- `concave-tui` is the terminal frontend project
- the two repos can evolve independently without pulling Bubble Tea dependencies
  into headless infrastructure builds
