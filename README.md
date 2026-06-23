# Fleet

Fleet is a terminal-only manager for AI coding sessions.

It gives every coding agent its own isolated tmux session, keeps those sessions visible in one dashboard, and lets you jump in and out without losing context. Think of it as a small control room for running multiple AI coding loops at once.

## Why Fleet Exists

AI coding tools are most useful when they can keep working in the background: one agent fixes a bug, another explores a refactor, another runs tests. The awkward part is everything around them: scattered terminal tabs, forgotten working directories, lost output, and tmux sessions mixed into your normal shell workflow.

Fleet exists to make that workflow boring:

- Start an AI coding session in the right directory.
- See all running sessions in one terminal UI.
- Preview the latest output and selected session terminal snapshot without attaching.
- Attach when you need to steer the work.
- Detach back to the dashboard with one key.
- Keep Fleet-managed tmux sessions separate from your regular tmux setup.

Fleet is intentionally terminal-first. No daemon, no browser app, no heavyweight project model. It is a thin, pragmatic layer over tmux with enough persistent state to remember what you care about.

## Current Status

Fleet is early software. The core loop works, but the public API and storage format may still change while the project settles.

Today Fleet supports:

- A Bubble Tea dashboard for session management.
- Creating default-shell sessions you can steer manually.
- Attaching to running sessions and returning to the dashboard after detach.
- Pinning, renaming, filtering, grouping, killing, and removing sessions.
- Persisted session metadata in a local JSON state file.
- tmux isolation through a dedicated socket and Fleet-prefixed session names.

## Requirements

- Go 1.23.4 or newer.
- tmux available on `PATH`.
- An AI coding CLI if you want one, such as `opencode` or `claude`.
- Optional: `fzf` for directory picking inside the TUI.

## Install

From a clone of this repository:

```sh
go install ./cmd/fleet
```

Or run without installing:

```sh
go run ./cmd/fleet
```

## Quick Start

Open the dashboard:

```sh
fleet
```

Create and attach to a session:

```text
n
```

Fleet opens a new tmux session in the current directory with your default shell. From there, run whatever you want: `opencode`, `claude`, tests, a debugger, or plain shell commands. The display name defaults to the directory basename.

Attach to a running session with `enter`. When you are inside the child tmux session, press `Ctrl+-` or `Ctrl+g` to detach back to Fleet.

## CLI Usage

```sh
fleet                  # open TUI dashboard
fleet new              # create a default-shell session in the current directory
fleet list             # print known sessions
fleet attach <id-name> # attach to a Fleet tmux session
fleet kill <id-name>   # kill and remove a session
fleet doctor           # check tmux and state health
```

`<id-name>` can be a session ID, display name, or raw tmux session name.

## TUI Keys

```text
n        create new session
enter    attach selected running session
p        pin or unpin selected session
d        change selected session directory
r        rename selected session
x        kill running session, or remove dead session
/        search/filter sessions
g        toggle flat view / grouped by directory
q        quit
```

In directory prompts, press `Ctrl+F` to open `fzf` with directory suggestions. If `fzf` is not installed, type the path manually.

Pinned sessions always sort first. In flat view, the dashboard shows each session's friendly display name, activity freshness, directory, and latest captured pane output. The selected session details include a bounded terminal preview from the tmux pane, matching what you will attach to with `enter`. The dashboard refreshes periodically so activity and previews stay current. In grouped view, sessions are separated by directory headers and the per-row directory column is hidden. Activity freshness updates when the captured pane output changes, so you can tell whether child work is still moving. Raw tmux names and raw status stay in details/debug contexts.

## How Fleet Uses tmux

Fleet never uses your default tmux socket. Every tmux command is built with an explicit socket:

```sh
tmux -L fleet ...
```

Fleet-managed session names also start with `fleet-`. That gives Fleet two layers of separation from your normal tmux work:

- A dedicated tmux socket, defaulting to `fleet`.
- A required `fleet-` prefix for managed session names.

Attached Fleet sessions show a tmux status bar so you can tell you are inside Fleet. The Fleet socket binds `Ctrl+-` and `Ctrl+g` to detach back to the dashboard. These bindings apply only to Fleet's tmux socket and do not affect normal tmux sessions.

Fleet intentionally does not bind raw `-`, because it is too easy to trigger accidentally inside child processes.

## State

Fleet stores session metadata locally as JSON.

Default path when `XDG_DATA_HOME` is set:

```text
$XDG_DATA_HOME/fleet/state.json
```

Fallback path:

```text
~/.local/share/fleet/state.json
```

For test or debug runs, override the state path and tmux socket:

```sh
FLEET_STATE_PATH=/tmp/fleet-dev/state.json \
FLEET_TMUX_SOCKET=fleet-dev \
FLEET_DEBUG=1 \
go run ./cmd/fleet
```

See `examples/state.json` for the current schema.

## Development

Run tests:

```sh
go test ./...
```

Run health checks:

```sh
go run ./cmd/fleet doctor
```

Run Fleet against an isolated dev socket:

```sh
FLEET_STATE_PATH=/tmp/fleet-dev/state.json \
FLEET_TMUX_SOCKET=fleet-dev \
go run ./cmd/fleet
```

Project rules live in `AGENTS.md`. Reusable agent workflows live in `.opencode/skills/*/SKILL.md`.

## Contributing

Fleet is being prepared for open source use. Contributions should keep the project small, terminal-native, and safe around tmux.

Important invariants:

- Do not call plain `tmux` for Fleet behavior.
- Route tmux behavior through `internal/tmux`.
- Always include an explicit `-L <socket>` in tmux commands.
- Keep Fleet sessions prefixed with `fleet-`.
- Add or update tmux command-construction tests when changing tmux behavior.
- Keep user-facing behavior documented in this README.

## License

MIT — see `LICENSE`.
