# Keybindings

This reference is generated from the current Bubble Tea models in `cmd/concave-tui/model/`.

## Global shell

| Key | Action | Role |
|---|---|---|
| `1`-`9` | Jump to a visible view | All authenticated users |
| `tab` | Next visible view | All authenticated users |
| `shift+tab` | Previous visible view | All authenticated users |
| `?`, `F1` | Toggle the help overlay | All authenticated users |
| `,` | Open settings | All authenticated users |
| `b` | Toggle the sidebar when the active view is not Workspace or Suites | All authenticated users |
| `q`, `ctrl+c` | Quit the TUI | All authenticated users |

## Login

| Key | Action | Role |
|---|---|---|
| `tab`, `shift+tab`, `j`, `k`, `up`, `down` | Move focus between username and password | Public |
| `enter` | Submit credentials | Public |
| `esc`, `q`, `ctrl+c` | Quit | Public |

## Workspace

| Key | Action | Role |
|---|---|---|
| `r` | Refresh workspace data | Viewer+ |
| `b` | Create a notebooks-and-models backup | Operator+ |
| `x` | Open the clean-outputs confirmation | Operator+ |
| `y` | Confirm clean outputs | Operator+ |
| `esc`, `n` | Cancel clean outputs | Operator+ |

## Suites

| Key | Action | Role |
|---|---|---|
| `j`, `k`, `up`, `down` | Move suite selection | Viewer+ |
| `g`, `G` | Jump to first or last suite | Viewer+ |
| `i` | Install the selected suite | Operator+ |
| `r` | Open the remove confirmation | Operator+ |
| `u` | Open the update confirmation | Operator+ |
| `R` | Roll back the selected suite | Operator+ |
| `s` | Start the selected suite | Operator+ |
| `x` | Stop the selected suite | Operator+ |
| `S` | Restart the selected suite | Operator+ |
| `l` | Open JupyterLab for the selected suite | Developer+ |
| `b` | Open an interactive shell in the primary container | Developer+ |
| `e` | Prompt for a command to execute in the primary container | Developer+ |
| `y` | Confirm remove or update | Operator+ |
| `n`, `esc` | Cancel remove or update | Operator+ |

## Forge picker

The Forge picker opens when you install the `forge` suite.

| Key | Action | Role |
|---|---|---|
| `j`, `k`, `up`, `down` | Move component selection | Operator+ |
| `space` | Toggle the highlighted component | Operator+ |
| `enter`, `y` | Install the selected component set | Operator+ |
| `esc`, `n` | Cancel Forge install | Operator+ |

## Logs

| Key | Action | Role |
|---|---|---|
| `j`, `k`, `up`, `down` | Switch container stream | Viewer+ |
| `g` | Jump to top of the viewport | Viewer+ |
| `G`, `end` | Jump to the bottom and resume follow mode | Viewer+ |
| `/` | Open log search | Viewer+ |
| `enter` | Apply the current search term | Viewer+ |
| `esc` | Cancel search input | Viewer+ |
| `n`, `N` | Move to the next or previous search match | Viewer+ |
| `f` | Resume follow mode | Viewer+ |
| `ctrl+d`, `ctrl+u` | Scroll half a page down or up | Viewer+ |
| `ctrl+f`, `ctrl+b` | Scroll a full page down or up | Viewer+ |
| `pgup`, `pgdown` | Scroll through the viewport | Viewer+ |

## Doctor

| Key | Action | Role |
|---|---|---|
| `r` | Rerun the health checks | Viewer+ |

## Environment

| Key | Action | Role |
|---|---|---|
| `r` | Refresh resolver data | Viewer+ |
| `j`, `k`, `up`, `down` | Move report selection | Viewer+ |
| `g`, `G` | Jump to first or last report | Viewer+ |

## Fleet

| Key | Action | Role |
|---|---|---|
| `r` | Refresh fleet data | Viewer+ |
| `j`, `k`, `up`, `down` | Move peer selection | Viewer+ |
| `g`, `G` | Jump to first or last peer | Viewer+ |

## Teams

| Key | Action | Role |
|---|---|---|
| `r` | Refresh team data | Admin only |
| `j`, `k`, `up`, `down` | Move team selection | Admin only |
| `enter` | Expand or collapse the selected team members | Admin only |
| `g`, `G` | Jump to first or last team | Admin only |

## System

| Key | Action | Role |
|---|---|---|
| `r` | Queue a reboot confirmation | Admin only |
| `x` | Queue a shutdown confirmation | Admin only |
| `d` | Queue a Docker restart confirmation | Admin only |
| `y` | Confirm the queued system action | Admin only |
| `esc`, `n` | Cancel the queued system action | Admin only |

## Users

| Key | Action | Role |
|---|---|---|
| `j`, `k`, `up`, `down` | Move user selection | Admin only |
| `enter` | Expand or collapse the selected user containers | Admin only |
| `g`, `G` | Jump to first or last user | Admin only |

## Settings

| Key | Action | Role |
|---|---|---|
| `tab`, `shift+tab`, `j`, `k`, `up`, `down` | Move between settings fields | All authenticated users |
| `h`, `l`, `left`, `right` | Change radio-style settings such as sidebar state | All authenticated users |
| `s` | Save and close settings | All authenticated users |
| `esc` | Leave insert mode or close settings without saving | All authenticated users |
