---
name: fleet-store
description: Use when changing Fleet JSON persistence, session model, config paths, state.json schema, or future SQLite migration boundaries.
---

# Fleet store

Persistence rules:

- Missing state file loads as empty state.
- Auto-create parent directories.
- Write atomically via temp file then rename.
- Avoid hardcoded user paths; use XDG/fallback config helpers.
- Keep store behind interface so SQLite can replace JSON later.
- Update tests and `examples/state.json` when schema changes.
