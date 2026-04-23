# new_nms_sync

`new_nms_sync` is the cleaned project layout for the current `nms_sync` feature set.

The goal of this repository is simple:

- keep the current product behavior
- reduce historical clutter and accidental edits
- make frontend/backend ownership explicit
- produce Windows and Linux releases from one build flow
- always ship release artifacts with release notes

## Canonical Structure

- `apps/backend`: Go backend source
- `apps/frontend`: editable frontend source of truth
- `docs`: maintained documentation and release notes
- `runtime/data`: runtime database and writable data location
- `runtime/bin`: optional runtime dependencies such as `ffmpeg`
- `artifacts/windows`: generated Windows release output
- `artifacts/linux`: generated Linux release output
- `scripts/build`: supported build entrypoints
- `scripts/run`: local run helpers
- `scripts/deploy`: legacy deployment helpers, not the primary workflow
- `tools`: one-off maintenance helpers retained from the old project

## Source Of Truth Rules

- Edit frontend files only in `apps/frontend`.
- Do not hand-edit `apps/backend/static`.
- `apps/backend/static` and `apps/backend/cmd/agent/static` are generated from `apps/frontend`.
- Do not commit databases, logs, packaged binaries, or ad-hoc release folders into source directories.

## Daily Commands

Sync frontend into backend embed directories:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\sync-static.ps1
```

```bash
bash ./scripts/sync-static.sh
```

Build Windows and Linux releases together:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build\build_both_releases.ps1
```

```bash
bash ./scripts/build/build_release.sh
```

## Build Outputs

Each build creates:

- `artifacts/windows/<version>`
- `artifacts/linux/<version>`

Each output includes:

- platform binaries
- `bin/` runtime tools if present under `runtime/bin`
- `data/` writable folder
- `RELEASE_NOTES.md`
- `RELEASE_NOTE.txt`

## Security Boundary

- Source code stays readable in `apps/`.
- Release protection is applied in a temporary build workspace, not inside source folders.
- If `npx` is available, release builds obfuscate JavaScript and minify HTML before embedding.
- If `npx` is unavailable, the build still completes, but the script prints a warning.

## Recommended Workflow

1. Edit code in `apps/backend` and `apps/frontend`.
2. Run `scripts/sync-static.*` when static assets need to be refreshed locally.
3. Update [docs/RELEASE_NOTES.md](docs/RELEASE_NOTES.md).
4. Run the unified release build.
5. Test artifacts from `artifacts/windows` and `artifacts/linux`.

## Supporting Docs

- [PROJECT_STRUCTURE.md](docs/PROJECT_STRUCTURE.md)
- [RELEASE_WORKFLOW.md](docs/RELEASE_WORKFLOW.md)
