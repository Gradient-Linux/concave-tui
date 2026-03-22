# concave-tui views

This document describes the current TUI surface as implemented in the repo.

## Workspace

The default landing view. It focuses on:

- workspace path and storage usage
- GPU, CPU, RAM, and VRAM summaries
- per-core CPU activity
- backup and clean actions where role allows them

Role notes:

- Viewer can inspect
- Operator and above can run backup and clean actions

## Suites

The suite control view for:

- suite summary and state
- container and image details
- install, remove, update, rollback, start, and stop
- Forge-specific install flow with component selection
- lab, shell, and exec shortcuts where role allows them

The view is role-aware and should only surface actions the current user can perform.

## Logs

The live log surface for installed suites and containers.

- suite/container selection
- live log streaming
- search and scroll controls
- log buffer management

## Doctor

The host and suite health view. It renders:

- Docker and connectivity checks
- workspace health
- GPU or CPU-only detection
- suite-level running/degraded/not-installed state

## System

Admin-only. Intended for machine-level state and administrative actions surfaced by
`concave serve`.

Examples:

- machine metadata
- restart Docker
- reboot
- shutdown

## Users

Admin-only. Displays the user-oriented activity data returned by the backend, such
as role, identity, and active work context.
