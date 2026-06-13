# Fleet Agent Guide

## Product

Fleet is a terminal-only AI coding session manager. It uses tmux as runtime while staying isolated from the user's normal tmux work.

## Non-negotiable tmux safety

- Never call plain `tmux` for Fleet behavior.
- Every tmux invocation must go through `internal/tmux`.
- Every tmux invocation must include `-L <socket>`.
- Production default socket: `fleet`.
- Test/dev sockets must be explicit (`fleet-test`, `fleet-dev`, etc.). Empty socket names are invalid.
- Fleet-managed tmux session names must start with `fleet-`.
- Fleet child sessions should show Fleet tmux chrome/status so users know they are inside Fleet.
- Fleet socket binds Ctrl-minus (`C-_` in tmux) and `C-g` to `detach-client` as “back to Fleet”; this affects only the Fleet tmux socket.
- Do not bind raw `-`; it is too easy to trigger inside child processes.
- Add or update tmux command-construction tests when changing tmux behavior.

## State

- Default state file: `$XDG_DATA_HOME/fleet/state.json`, fallback `~/.local/share/fleet/state.json`.
- `FLEET_STATE_PATH` may override state path for tests/debugging.
- Missing state file means empty state, not an error.
- Store writes should be atomic: temp file then rename.
- Keep persistence behind interfaces; JSON is MVP, SQLite should be swappable later.

## Debug / test env

```sh
FLEET_STATE_PATH=/tmp/fleet-dev/state.json \
FLEET_TMUX_SOCKET=fleet-dev \
FLEET_DEBUG=1 \
fleet
```

Integration tests, if added, must use a non-default tmux socket and clean up their sessions/server.

## UI rules

- Show friendly display names in primary UI.
- Show raw tmux names only in details/debug contexts.
- Pinned sessions first, always.
- Directory and latest captured response visible in dashboard; raw status belongs in details/debug contexts.
- Show activity freshness from pane-output changes so users can tell whether child work is still moving.
- Keep footer help accurate when keybindings change.
- Avoid blocking work directly in Bubble Tea `Update`; use commands where practical.
- Attaching to a child session should suspend Fleet and return to the Fleet dashboard after detach, not drop users back to shell.
- Default TUI theme: Catppuccin Mocha.
- Prefer top overlay/panel rendering over full-screen repainting to reduce Ghostty flash.
- Directory prompts should support `Ctrl+F` fzf suggestions with manual typing fallback.
- New sessions default to current directory; display name defaults to directory basename unless user renames later.

## Documentation rules

- Update this file when discovering reusable project rules.
- Update `.opencode/skills/*/SKILL.md` when a recurring workflow/invariant should be reusable by agents.
- Update README for user-facing behavior changes.
- Update `examples/state.json` when state schema changes.
