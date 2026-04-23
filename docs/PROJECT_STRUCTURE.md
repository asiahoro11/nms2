# Project Structure

## Purpose

`new_nms_sync` keeps the current `nms_sync` product behavior while removing the layout problems that made the old repository hard to maintain.

## Directory Rules

- `apps/backend`
  - Go source only
  - embedded static directories live here because `go:embed` requires them
  - do not keep packaged binaries, logs, or databases here
- `apps/frontend`
  - only editable frontend source
  - any frontend change starts here
- `runtime/data`
  - writable runtime state
  - database, uploads, backups generated at runtime
- `runtime/bin`
  - optional runtime binaries such as `ffmpeg`
  - copied into release artifacts as `bin/`
- `artifacts`
  - generated outputs only
  - safe to delete and rebuild
- `docs`
  - maintained operator and developer documentation
- `scripts/build`
  - supported build entrypoints
- `scripts/deploy`
  - legacy deployment scripts retained for reference
- `tools`
  - one-off utilities retained from the old project

## Frontend Ownership

The frontend source of truth is:

- [apps/frontend](../apps/frontend)

Generated embed targets are:

- [apps/backend/static](../apps/backend/static)
- [apps/backend/cmd/agent/static](../apps/backend/cmd/agent/static)

These embed targets are refreshed by:

- [scripts/sync-static.ps1](../scripts/sync-static.ps1)
- [scripts/sync-static.sh](../scripts/sync-static.sh)

## Safety Rules

- No release outputs under `apps/`
- No runtime logs under `apps/`
- No checked-in database files
- No manual editing of generated static mirrors
- No build script may mutate the source tree for release-only protection steps

## Migration Notes

This repository was assembled from:

- `nms_sync`: latest feature-bearing source
- `nms_sync2`: cleaned layout baseline

That means feature parity comes from current `nms_sync`, while maintainability comes from the `nms_sync2` layout pattern.
