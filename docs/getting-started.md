# Getting Started

`concave-tui` connects to the local `concave` control plane. Start there first:

```bash
concave serve --addr 127.0.0.1:7777
concave-tui
```

## First launch

On first launch, `concave-tui` opens a centered login panel with two fields:

- `Username`
- `Password`

Use these keys on the login screen:

- `tab`, `shift+tab`, `j`, `k`, `up`, `down` to move between fields
- `enter` to submit
- `esc`, `q`, or `ctrl+c` to quit

Authentication is delegated to `concave serve`, which checks the local PAM and Gradient Linux role configuration.

## Session caching

After a successful login, the TUI stores the returned session token in:

```text
~/.config/concave/session.json
```

If that token is still valid on the next launch, the TUI skips the login screen and opens the default workspace view immediately.

## Local preferences

Display preferences live in:

```text
~/.config/concave-tui/config.toml
~/gradient/config/concave-tui.toml
```

The XDG config file stores display behavior such as refresh interval and sidebar default. The workspace file stores preset definitions.

## If the connection fails

Work through these checks in order:

1. Confirm the API is running:

```bash
concave serve --addr 127.0.0.1:7777
```

2. Confirm the session cache is not stale:

```bash
rm -f ~/.config/concave/session.json
```

3. Confirm the API is reachable:

```bash
curl -sS http://127.0.0.1:7777/api/v1/health
```

4. If you need a different API address for local development, export `CONCAVE_API_BASE_URL` before launching `concave-tui`.

## First views to open

After login, start with these views:

- `Workspace` for disk and hardware status
- `Suites` for install, start, stop, update, rollback, and shell actions
- `Logs` for live container output
- `Doctor` for host health checks
