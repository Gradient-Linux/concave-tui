# Contributing to concave-tui

Contributions are welcome for terminal UX, API-backed workflows, session handling, tests, and documentation. Keep the repository focused on presentation and operator ergonomics. Backend business logic belongs in `concave`.

## Before you start

Read [README.md](README.md), [docs/getting-started.md](docs/getting-started.md), and [docs/keybindings.md](docs/keybindings.md) before changing view behavior or the session flow.

## Development setup

Use Ubuntu 24.04 with Go 1.25 or newer.

```bash
git clone <repo-url>
cd concave-tui
go build -o concave-tui ./cmd/concave-tui/
go test ./...
go test -race ./...
./concave-tui --help
```

For end-to-end testing, run `concave serve` locally and then launch `./concave-tui`.

## Making changes

### Branching

Use one of these branch prefixes:

- `feat/<slug>`
- `fix/<slug>`
- `docs/<slug>`

### Commit messages

Format commits as `<type>(<scope>): <summary>`.

Use these types:

- `feat`
- `fix`
- `refactor`
- `test`
- `docs`
- `chore`

Keep the summary under 72 characters.

Examples:

- `feat(suites): add live restart progress panel`
- `fix(login): handle expired cached sessions`
- `docs(keybindings): document fleet view shortcuts`

### Tests

- Add or update tests for every new function and behavior change.
- Run `go test ./...` before opening a pull request.
- Run `go test -race ./...` when you touch state mutation, timers, or async updates.
- Keep tests local. Unit tests must not require a live terminal, Docker daemon, or networked API unless the test is explicitly integration-scoped.

### Pull requests

- Keep pull requests to one logical change.
- Explain what changed in the UI and which API routes or WebSocket flows it depends on.
- Include screenshots or terminal captures for visible UI changes.

## Code conventions

- Keep business logic in `concave`. `concave-tui` is an API client and presentation layer.
- Use Bubble Tea for the event loop and Lip Gloss for styling.
- Keep the color palette aligned with the Gradient Linux terminal look.
- Reuse the authenticated API and returned role metadata instead of re-deriving machine state ad hoc.
- Preserve session caching at `~/.config/concave/session.json`.

## What we don't accept

- Dependencies added without prior discussion in an issue.
- Duplicated suite, auth, or permission rules that should stay in `concave`.
- Shell string interpolation with user-controlled input.
- UI changes that bypass `concave serve` for privileged actions.

## License

By contributing, you agree that your contributions will be released under the repository license when one is published.
