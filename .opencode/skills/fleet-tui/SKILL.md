---
name: fleet-tui
description: Use when changing Fleet Bubble Tea TUI dashboard, keybindings, list rendering, grouping, filtering, forms, attach flow, or styling.
---

# Fleet TUI

UI requirements:

- Pinned sessions first in all views.
- Group-by-directory mode still keeps pinned sessions at the top.
- Show display name, directory, and latest captured response in the list; show raw status in details/debug contexts.
- Show activity freshness based on pane-output changes so users can tell whether child work is still moving.
- Details panel shows selected session, including raw tmux name.
- Grouped-by-directory view should use directory section headers and hide the per-row directory column.
- Footer help must match implemented keybindings.
- Prefer Bubble Tea commands for side effects.
- Do not expose raw tmux names as the main identity; use friendly display names.
- Attach flow should return to the Fleet dashboard after the user detaches from the child tmux session.
- Keep render dimensions stable where practical; variable-width/height dashboard output can cause terminal flicker during selection changes.
- Default theme is Catppuccin Mocha.
- Prefer a top overlay/panel over repainting the whole screen; Ghostty can visibly flash on full-screen repaints.
- Directory inputs should keep `Ctrl+F` fzf selection working when fzf is installed, with manual path entry as fallback.
- New-session flow should default to current directory and derive the display name from the selected path.
