---
name: fleet-tmux-safety
description: Use when changing Fleet tmux behavior, internal/tmux, socket handling, session creation, attach, kill, list, capture-pane, or status detection.
---

# Fleet tmux safety

Fleet must never touch normal tmux sessions.

Checklist:

- All tmux behavior lives behind `internal/tmux.Client` or its interface.
- Every tmux command includes `-L <socket>`.
- Default socket is `fleet`; tests/dev may override to another non-empty socket.
- Never invoke raw/default `tmux`.
- Managed session names start with `fleet-`.
- New sessions set Fleet-specific status/chrome so attached users can see they are in Fleet.
- Fleet socket may bind no-prefix back keys like Ctrl-minus (`C-_` in tmux) / `C-g`; remember these affect all Fleet sessions but never normal tmux.
- Do not bind raw `-`; it is too easy to trigger inside child processes.
- Tests assert exact command construction for new/changed tmux methods.
- Treat `no server running` while listing as an empty Fleet server, not a fatal normal-tmux issue.
