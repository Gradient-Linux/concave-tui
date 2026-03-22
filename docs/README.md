# concave-tui docs

This directory contains the repository-level documentation for `concave-tui`.

## Reading order

- [architecture.md](architecture.md): application structure, server-backed client
  model, role-aware UI boundaries, and repository layout.
- [views.md](views.md): view-by-view behavior, role exposure, and action mapping.
- [runtime.md](runtime.md): config paths, session handling, startup flow, and
  troubleshooting.

## Relationship to the rest of the stack

- `concave` owns the backend control plane and privileged behavior.
- `concave-web` owns the browser frontend.
- `concave-tui` owns the terminal UI client.
