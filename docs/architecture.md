# concave-tui architecture

`concave-tui` is the terminal frontend for Gradient Linux operations. It is
separate from `concave` so the infrastructure binary can stay headless-safe while
the UI repo can evolve independently.

## Responsibilities

This repository owns:

- the Bubble Tea application
- terminal-oriented navigation and layout
- session bootstrap against `concave serve`
- role-aware action visibility in the TUI
- terminal-friendly rendering for workspace, suites, logs, doctor, system, and users views

This repository does not own:

- PAM or Unix-group role resolution
- machine authority for privileged actions
- the authenticated API surface

Those remain in `concave`.

## Runtime model

The TUI now operates as a client of `concave serve`.

```text
concave-tui -> concave serve
```

The TUI uses:

- cached session data from disk when available
- authenticated HTTP calls for role-sensitive data and actions
- WebSocket log streaming for live log views
- local rendering helpers for host-side metrics where the UI benefits from direct sampling

## Repository layout

- `cmd/concave-tui/main.go`: startup, config load, session bootstrap
- `cmd/concave-tui/config/`: user configuration and preset parsing
- `cmd/concave-tui/model/`: Bubble Tea models for login, root chrome, views, and overlays
- `internal/client/`: HTTP/WebSocket client for `concave serve`
- `internal/auth/`: TUI-local session model and permission helpers
- `internal/ui/`: terminal output helpers
- `internal/workspace/`, `internal/gpu/`, `internal/system/`: local metric helpers retained for terminal rendering support

## UI model

The root model owns:

- active view
- sidebar state
- settings and help overlays
- authenticated session state
- role-aware visibility of views and actions

Child models render the major areas:

- Workspace
- Suites
- Logs
- Doctor
- System
- Users

System and Users are admin-only views.

## Permission model

The UI does not invent its own role hierarchy. It mirrors the role model exposed by
the backend:

- viewer
- developer
- operator
- admin

The TUI uses that role data to:

- hide or disable actions the user cannot perform
- shape the help overlay
- control which views are visible
- decide which interactive actions should be attempted at all
