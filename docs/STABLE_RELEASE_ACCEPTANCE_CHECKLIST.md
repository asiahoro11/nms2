# Stable Release Acceptance Checklist

This checklist tracks release-readiness for the current `new_nms_sync` stable line.

Current target:

- release version: `v1.2.0`
- default port: `8080`
- frontend source of truth: `apps/frontend`
- backend runtime version source: `apps/backend/config/config.go`

## Acceptance Status Legend

- `PASS`: verified in current workspace/runtime
- `PENDING`: not yet verified end-to-end
- `BLOCKED`: known issue prevents sign-off

## A. Build and Version Consistency

- `PASS` Backend version source unified to `v1.2.0`
- `PASS` Frontend shared version source unified to `v1.2.0`
- `PASS` Visible page version display unified through `apps/frontend/js/config.js`
- `PASS` Local static asset cache-busting version unified to `v1.2.0`
- `PASS` Default backend port remains `8080`
- `PASS` Frontend static assets synced to both backend static mirrors
- `PASS` `go test ./...` passes in `apps/backend`
- `PASS` `node --check` passes for version-related frontend JS

Sign-off rule:

- all main pages must report one version only
- no page may expose older `v1.2.1`, `v1.2.2`, or alpha strings in runtime UI

## B. Authentication and Core Entry Pages

- `PASS` Login page loads with unified version footer
- `PASS` Mode-selection page loads with unified version footer
- `PASS` Dashboard system version has frontend fallback and backend override path
- `PASS` Token-seeded login -> mode-selection -> dashboard route smoke test on local Windows runtime
- `PENDING` Logout flow smoke test
- `PENDING` Reset-password page smoke test

Sign-off rule:

- no broken redirects
- no 404 on `/login`, `/mode-selection`, `/index`, `/monitor`

## C. Translation and Branding

- `PASS` Shared i18n fallback path is in place
- `PASS` Product naming converged to `Management System` in current successor workspace/runtime
- `PENDING` `zh-TW` manual page-by-page smoke test
- `PENDING` `en-US` manual page-by-page smoke test
- `PENDING` `zh-CN` manual page-by-page smoke test
- `PENDING` `ja-JP` manual page-by-page smoke test
- `PENDING` `ko-KR` manual page-by-page smoke test
- `PENDING` Confirm no raw i18n keys remain visible on live pages

Sign-off rule:

- no visible `nav.*`, `dashboard.*`, `ac.*`, `pdu.*`, `cameras.*`, `admin.*` keys
- no mixed old product names remain in UI

## D. Camera Stability

- `PENDING` Grid preview smoke test with normal MJPEG cameras
- `BLOCKED` WebRTC preview path needs final stabilization review
- `BLOCKED` Single-camera fullscreen still needs final regression verification
- `PENDING` NVR page smoke test
- `PENDING` Camera add/edit/delete smoke test
- `PENDING` Camera license notice and empty-state review
- `PENDING` Recording config smoke test with `RTSP`
- `PENDING` Recording config smoke test with `ONVIF`

Sign-off rule:

- opening fullscreen must not instantly collapse
- preview mode must not destabilize the whole camera wall
- record path must remain independent from preview mode

## E. Topology and Monitor Stability

- `PENDING` Force view smoke test
- `PENDING` Tree view smoke test
- `PENDING` Collapse / expand behavior smoke test
- `PENDING` Offline branch warning indication smoke test
- `PENDING` Monitor topology + camera split layout smoke test
- `PENDING` Long-session monitor stability test
- `PENDING` Topology legend / controls readability review

Sign-off rule:

- topology must remain readable and interactive after repeated refreshes
- monitor mode must behave like an operational page, not a partial demo

## F. Module-by-Module Functional Smoke Test

- `PENDING` Dashboard
- `PENDING` Devices
- `PENDING` Topology
- `PENDING` Cameras
- `PENDING` Access Control
- `PENDING` PDU
- `PENDING` Logs
- `PENDING` Admin

For each module verify:

- page opens without JS errors
- labels are human-readable
- empty state is acceptable
- permissions and license gating behave correctly

## G. Event and Notification Integrity

- `PENDING` Event wall smoke test
- `PENDING` Notification bell smoke test
- `PENDING` Confirm no `%!(EXTRA ...)` formatting errors remain
- `PENDING` Confirm Chinese event text is not broken or garbled

Sign-off rule:

- no malformed format strings in monitor, logs, dashboard events, or notifications

## H. Performance and Endurance

- `PENDING` 30-minute idle dashboard test
- `PENDING` 30-minute monitor test
- `PENDING` Camera wall CPU/memory observation test
- `PENDING` Topology interaction stress test
- `PENDING` Repeated page-switching stress test
- `PENDING` Browser hard-refresh and cache-bust test

Minimum observation points:

- browser CPU does not trend upward uncontrollably
- memory usage does not leak noticeably during repeated page switches
- camera/monitor views do not degrade over time

## I. Release Packaging

- `PASS` Rebuilt Windows release artifact for `v1.2.0`
- `PENDING` Rebuild Linux release artifact for `v1.2.0`
- `PASS` Runtime launch smoke test on Windows package
- `PENDING` Runtime launch smoke test on Linux package
- `PENDING` Final release note review
- `BLOCKED` Windows Defender / firewall first-run behavior review

Packaging policy:

- default release builds should stay clean and readable
- JS obfuscation and HTML minification are disabled by default to reduce false-positive AV risk
- only enable obfuscation explicitly when needed through `NMS_ENABLE_OBFUSCATION=1`
- Go binary obfuscation is optional and only activates when `NMS_ENABLE_BINARY_OBFUSCATION=1`
- protected builds prefer `garble build`; when `garble` is missing the scripts warn and fall back to normal `go build`

## Recommended Execution Order

1. Run core authentication smoke test.
2. Run camera preview/fullscreen stabilization test.
3. Run topology/monitor smoke test.
4. Run module-by-module functional smoke test.
5. Run translation smoke test across all languages.
6. Run endurance/performance checks.
7. Rebuild release artifacts and do final packaged runtime validation.

## Current Release Readiness Summary

- ready for continued verification: yes
- version consistency stabilized: yes
- build safety baseline verified: yes
- stable release sign-off complete: no

Main remaining risks:

- camera preview / fullscreen behavior
- topology and monitor long-session stability
- residual event-formatting defects
- cross-language visual validation not yet completed
- Windows Defender / firewall first-run trust flow for unsigned local binaries
