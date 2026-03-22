# concave-tui runtime

## Requirements

`concave-tui` expects:

- a terminal with ANSI support
- a reachable `concave serve`
- a valid cached session or valid Gradient Linux credentials

## Config files

User preferences live at:

- `~/.config/concave-tui/config.toml`

Session cache lives at:

- `~/.config/concave/session.json`

Workspace-side preset data, when used, lives inside the Gradient workspace config
area through the TUI config layer.

## Startup flow

1. Load TUI config
2. Attempt to load cached session
3. If the session is valid, continue directly into the application
4. If the session is missing or expired, show the login flow
5. Authenticate against `concave serve`
6. Store the new session locally

## Build and run

```bash
go test ./...
go test -race ./...
CGO_ENABLED=0 go build -o concave-tui ./cmd/concave-tui/
./concave-tui --help
./concave-tui
```

## Troubleshooting

- If login fails, verify that `concave serve` is reachable and the user is in a
  valid `gradient-*` Unix group.
- If the TUI keeps prompting for login, remove the cached session file and retry.
- If admin-only views are missing, the authenticated role is below admin.
- If suite actions are unavailable, check the role and the backend permissions
  returned by `concave serve`.
