# Stable Release Acceptance Table

Target release:

- version: `v1.2.0`
- platform under active verification: `Windows`
- runtime verified from: `artifacts/windows/v1.2.0`
- verification date: `2026-04-19`

## Acceptance Table

| Area | Item | Status | Evidence / Notes |
| --- | --- | --- | --- |
| Build | Backend version source unified | PASS | `apps/backend/config/config.go` now reports `v1.2.0` |
| Build | Frontend shared version source unified | PASS | `apps/frontend/js/config.js` now reports `v1.2.0` |
| Build | Static asset cache-busting unified | PASS | Main HTML entrypoints now use `?v=v1.2.0` |
| Build | Backend tests | PASS | `go test ./...` passed in `apps/backend` |
| Build | Frontend version JS syntax | PASS | `node --check apps/frontend/js/config.js` passed |
| Build | Frontend static sync | PASS | `scripts/sync-static.ps1` completed successfully |
| Packaging | Windows `v1.2.0` artifact rebuilt | PASS | `artifacts/windows/v1.2.0` rebuilt after version rollback |
| Packaging | Clean build policy active | PASS | Windows/Linux build scripts now disable obfuscation by default |
| Packaging | Optional binary obfuscation mode available | PASS | `NMS_ENABLE_BINARY_OBFUSCATION=1` now switches build scripts to `garble build` when available |
| Packaging | Linux `v1.2.0` artifact rebuild | PENDING | Not rerun after clean-build policy change |
| Runtime | Windows package boot | PASS | `nms_server.exe` started and served local routes |
| Runtime | System info version | PASS | `/api/v1/system/info` returned `v1.2.0` |
| Runtime | Login page response | PASS | `GET /login` returned `200` on local runtime |
| Auth | API login | PASS | `POST /api/v1/auth/login` succeeded with local test admin on local runtime |
| Auth | Force password change state | PASS | Local runtime currently returns `require_password_change=true` for the seeded admin account |
| Auth | Logout flow | PENDING | Not yet executed end-to-end |
| Auth | Reset-password flow | PENDING | Not yet executed end-to-end |
| Routing | Mode-selection page load | PASS | Playwright token-seeded route smoke reached `/mode-selection.html` |
| Routing | Dashboard page load | PASS | Playwright token-seeded route smoke reached `/index.html` |
| Routing | Monitor page load | PASS | Playwright token-seeded route smoke reached `/monitor` |
| Translation | Major entry pages render readable text | PASS | `login / mode-selection / index / monitor` returned readable titles/text in local smoke |
| Translation | Cross-language smoke test | PENDING | Not yet executed page-by-page across all locales |
| Camera | Module status endpoint | PASS | `/api/v1/cameras/status` returned `licensed=false`, `locked_ui=true`, `count=0`, `gpu_available=true` on local runtime |
| Camera | Preview stability | BLOCKED | Local stable runtime test data is unlicensed for cameras, so real camera-wall smoke could not be executed without altering license state |
| Camera | Fullscreen stability | BLOCKED | JS/CSS fullscreen logic was updated, but final operational sign-off is blocked by local camera license/test-data state |
| Topology | Basic route load | PASS | Dashboard and monitor topology pages load |
| Topology | Force/tree/collapse behavior | PENDING | Interaction smoke not yet completed |
| Monitor | Long-session stability | PENDING | No endurance run yet |
| Events | No malformed `%!(EXTRA ...)` messages | PENDING | Not revalidated in this local stable pass |
| Security UX | Windows firewall prompt | BLOCKED | Expected for a listening server unless allow rule/signing is in place |
| Security UX | Defender / AV false-positive risk | BLOCKED | Reduced by clean build, but unsigned local EXE may still be flagged |

## Current Sign-off Summary

| Category | Status |
| --- | --- |
| Version consistency | PASS |
| Build baseline | PASS |
| Windows package boot | PASS |
| Core route smoke | PASS |
| Camera stability | PENDING / BLOCKED |
| Topology / monitor interaction | PENDING |
| Long-session performance | PENDING |
| AV / firewall trust flow | BLOCKED |
| Final stable release sign-off | NOT YET |

## Immediate Next Actions

1. Prepare a licensed local camera test dataset, or use a controlled licensed staging runtime.
2. Finish camera preview and fullscreen verification.
3. Run topology force/tree/collapse interaction smoke.
4. Run 30-minute monitor and dashboard endurance checks.
5. Rebuild and verify Linux `v1.2.0` package after the clean-build change.
6. Decide operational handling for Windows Defender and firewall prompts:
   - installer or scripted firewall allow rule
   - code signing for production distribution
   - Defender allowlisting / sample submission if needed
