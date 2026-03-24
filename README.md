# concave-tui

Terminal control surface for Gradient Linux, backed by the local `concave` API.

## What it does

`concave-tui` provides a full-screen Bubble Tea interface for suite lifecycle, live logs, workspace health, environment drift, fleet visibility, and admin views. It is a client of `concave serve` on `127.0.0.1:7777`. It does not embed `concave` as a library and it does not own machine authority.

## Requirements

- Ubuntu 24.04 LTS
- `concave serve` running on `127.0.0.1:7777`
- ANSI-capable terminal

## Install

`concave-tui` ships with the Gradient Linux distribution and can also be installed from release artifacts when they are published. For source builds:

```bash
go build -o concave-tui ./cmd/concave-tui/
```

## Usage

Launch the TUI against the local `concave` API:

```bash
concave-tui
```

Common keys after login:

| Key | Action |
|---|---|
| `1`-`9` | Jump to a visible view |
| `tab` / `shift+tab` | Move to the next or previous view |
| `?` or `F1` | Open the help overlay |
| `,` | Open settings |
| `q` | Quit |

## Configuration

`concave-tui` reads user display settings from `~/.config/concave-tui/config.toml` and writes workspace-backed presets to `~/gradient/config/concave-tui.toml`. Session tokens are cached in `~/.config/concave/session.json`.

## Architecture

`concave-tui` is a presentation layer. It authenticates against `concave serve`, renders the returned data in a terminal UI, and opens terminals or suite actions through the same local API. Local helpers are limited to terminal-focused metrics and cached preferences.

## Development

### Prerequisites

Install Go 1.25 or newer and run a local `concave serve` instance if you want to test the live UI.

### Build

```bash
go build -o concave-tui ./cmd/concave-tui/
```

### Test

```bash
go test ./...
go test -race ./...
```

### Repo layout

```text
concave-tui/
  cmd/concave-tui/   Bubble Tea entrypoint, models, config
  internal/client/   HTTP and WebSocket client for concave serve
  internal/auth/     session model and role helpers
  docs/              user and contributor docs
```

## Roadmap

The current line covers suite control, logs, workspace monitoring, resolver and fleet status, and admin-only system and user views. Upcoming work focuses on deeper command parity with the web client and richer multi-node operations.

## License

License terms have not been published in this repository yet.
