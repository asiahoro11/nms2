# v1.2.3 Final Acceptance

## Summary
`v1.2.3` is accepted as the current candidate stable release for `new_nms_sync`.

This acceptance is based on:
- workspace fixes completed in `new_nms_sync`
- release artifacts rebuilt for Windows and Linux
- live smoke verification against `http://10.100.100.11:8080`
- manual user verification for the remaining topology and monitor issues

This document is the single reference point for the current final acceptance state.

## Final Acceptance Decision
Status: `Accepted as candidate stable release`

Release version:
- `v1.2.3`

Artifacts:
- Windows: `artifacts/windows/v1.2.3`
- Windows zip: `artifacts/windows/v1.2.3.zip`
- Linux: `artifacts/linux/v1.2.3`
- Linux zip: `artifacts/linux/v1.2.3.zip`

## Validated Scope
The following items were validated and are considered passed for this release.

### Core Access and Runtime
- `/login` loads normally
- `/mode-selection.html` loads normally
- `/index.html` loads normally
- `/monitor` loads normally
- login with administrator credentials succeeds
- `/api/v1/system/info` returns `v1.2.3`

### Camera Module
- camera list API is available
- monitor camera list API is available
- camera snapshot endpoint returns `200 image/jpeg`
- camera MJPEG stream endpoint returns `200 multipart/x-mixed-replace`
- management camera preview freeze after `monitor -> management` switching was fixed
- recording naming now prefers `camera_recordings.camera_name`
- new recordings preserve camera name snapshots instead of falling back to `cam{id}`

### Monitor and Topology
- visible `??` issues in monitor UI were cleared
- monitor tree collapse hidden-offline breathing indicator works
- backend/admin topology tree collapse hidden-offline breathing indicator works
- topology data API is available
- live topology assets contain the hidden-alert logic now required by the UI

### Audit
- audit endpoint is available at `/api/v1/audit-logs`
- login and login failure records are present
- at least one non-auth change record was observed in live data
  - example: `update_camera`

### Release Consistency
- backend version source is `v1.2.3`
- frontend displayed version source is `v1.2.3`
- frontend asset cache-busting version is `v1.2.3`
- release artifacts include `RELEASE_NOTE.txt`
- release artifacts include `RELEASE_NOTES.md`

## Live Smoke Test Result
Target:
- `http://10.100.100.11:8080`

Smoke result:
- `PASS` for page load, login, version endpoint, monitor page, topology endpoint, camera endpoint, monitor camera endpoint, snapshot endpoint, and MJPEG endpoint

Important live observations:
- system info returned version `v1.2.3`
- live monitor HTML contains the cleaned tooltip binding
- live topology JS contains `has-hidden-alert`
- live topology JS contains `countHiddenOffline`
- live camera endpoints were responsive

## Manual Verification Result
The following were manually verified by the user after deployment:
- monitor no longer shows visible `??`
- backend topology collapsed offline breathing indicator is stable
- monitor collapsed offline breathing indicator is stable

These manual checks are considered part of the final acceptance evidence because they cover browser interaction behavior that cannot be fully signed off by HTTP-only smoke checks.

## Known Non-Blocking Technical Debt
These items do not block `v1.2.3`, but remain open for future work.

### Audit Coverage
- audit logging is still not fully wired for every important write operation
- device create, device update, device delete, topology change, user management, license change, and system setting change coverage should be expanded

### Audit Presentation
- audit actions, statuses, and detail payloads still need better Chinese presentation and i18n-driven formatting
- raw action keys and raw JSON-style detail values should be further normalized in the UI

### Source Cleanup
- some source files still contain comment-level garble or legacy notes
- current remaining `??` in source are not known to be visible in accepted UI paths, but the source is not yet fully clean

### Automated Verification Debt
- `go test ./...` is still affected by an existing repo-relative static path problem in this workspace
- this is treated as verification debt, not as a release-blocking runtime failure for `v1.2.3`

### Long-Run Soak Testing
- long-duration browser soak tests are still recommended
- especially:
  - camera preview after long idle periods
  - repeated `monitor <-> management` switching
  - long fullscreen sessions
  - extended monitor runtime with topology refreshes

### i18n Quality
- i18n structure and fallback behavior are usable
- wording and terminology still deserve another human pass for polish

## Recommended Next Actions
When work resumes after `v1.2.3`, recommended priority is:

1. Expand audit coverage across all high-value write actions
2. Normalize audit UI text into Chinese first, then map through i18n
3. Clean remaining source-level garble and legacy comments
4. Fix the backend `go test ./...` static path issue
5. Run a formal long-duration frontend soak test

## Acceptance Statement
`v1.2.3` is accepted as the current candidate stable release of `new_nms_sync`.

It is suitable for deployment and operational use within the validated scope above.

Further work should continue as controlled follow-up technical debt reduction, not as a blocker for this acceptance.
