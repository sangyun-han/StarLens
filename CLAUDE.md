# Working in this repository

Guidance for AI coding assistants (and a quick orientation for humans).
See [README.md](README.md) for what StarLens is and how to run it.

## Commit messages

- **Never append tool session links or telemetry trailers** (for example
  `Claude-Session: https://…`). They are dead links for every other reader,
  they are permanent once pushed to public history, and they point at a
  conversation whose contents this project cannot vouch for.
- Attribution is welcome in standard form — `Co-Authored-By: Name <email>`.
- Explain *why* a change is the way it is, not just what changed. The
  StarRocks-specific reasoning (version-drifting SHOW columns, shared-data vs
  shared-nothing, read-only guards) is the part future readers cannot recover
  from the diff.

A `commit-msg` hook enforces the first rule. Enable it once per clone:

```bash
git config core.hooksPath .githooks
```

## Conventions

- Backend follows Controller (`internal/api`) → Service (`internal/service`) →
  Repository (`internal/repository`); business logic never lives in handlers.
- StarRocks `SHOW` output changes between releases: read columns **by name with
  fallbacks**, and let a missing column serialize as absent rather than zero.
- The UI is fully translated. New user-facing strings go into
  `frontend/src/locales/en.json` first — see
  [`frontend/src/locales/README.md`](frontend/src/locales/README.md).
- Run `make check` (lint + tests + build) before committing.
