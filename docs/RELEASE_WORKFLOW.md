# Release Workflow

## Required Outcome

Every supported release build must emit:

- Windows artifact set
- Linux artifact set
- copied release notes

## Supported Entrypoints

- PowerShell: [scripts/build/build_both_releases.ps1](../scripts/build/build_both_releases.ps1)
- Bash: [scripts/build/build_release.sh](../scripts/build/build_release.sh)

## Version Source

Version is read from:

- [apps/backend/config/config.go](../apps/backend/config/config.go)

## Build Behavior

1. Read version from backend config.
2. Create a temporary backend workspace under `artifacts/.tmp`.
3. Sync `apps/frontend` into backend embed directories inside that temporary workspace.
4. If Node tooling is available, obfuscate JS and minify HTML in the temporary workspace.
5. Build Windows and Linux binaries from the temporary backend workspace.
6. Copy `runtime/bin` into each artifact as `bin/` when present.
7. Copy `docs/RELEASE_NOTES.md` into each artifact.
8. Generate a small `RELEASE_NOTE.txt` manifest in each artifact.

## Release Artifact Paths

- `artifacts/windows/<version>`
- `artifacts/linux/<version>`

## Release Discipline

- Update release notes before running the build.
- Test from `artifacts/`, not from source directories.
- If `npx` is missing, the build is still valid, but the output is less protected.
