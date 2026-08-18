# zh-TW / zh-CN i18n Audit & Fix — 2026-07-18

**Requested by:** user ("幫我檢查中文的i18n" → "請修正翻譯。")
**Done by:** Claude, this session
**Status:** Fixes applied and synced. Not yet committed. **Requesting Codex verification.**

## Scope

Audited the 5 locale files in `apps/frontend/i18n/` (source of truth per `CLAUDE.md`):
`zh-TW.json`, `zh-CN.json`, `en-US.json`, `ja-JP.json`, `ko-KR.json` — 1236 flattened keys each.

## Method

1. Flattened all 5 JSON files to `dot.path -> value` maps in Python.
2. Checked structural parity (key sets identical across all 5 locales — confirmed, no missing/extra keys).
3. Checked for corruption markers already known to have occurred in this repo before (see
   `isUnresolvedTranslationValue()` in `apps/frontend/js/i18n.js`): empty strings, value equal to
   key path, `�`/PUA characters, `??`, double-encoded mojibake. **None found.**
4. Checked placeholder token (`{xxx}`) consistency between en-US and each zh locale. **All matched.**
5. Found untranslated values by comparing each zh value against the en-US value (exact string
   match after `.strip()`), then cross-referenced against ja-JP/ko-KR to see whether those locales
   *had* translated the same key — used as a heuristic for "this should be translatable, not an
   intentional English/brand-name convention."
6. Also directly compared zh-TW vs zh-CN for the same key, using a CJK-character-ratio heuristic
   (not just exact-string match) to catch cases where the untranslated zh-TW value differs from
   English only in casing/spacing (e.g. `"Byte Order"` vs en `"Byte order"`), which a naive
   string-equality check would miss.

## Findings (18 keys total, all confirmed genuine gaps — not proper nouns/brand names)

### A. Untranslated in *both* zh-TW and zh-CN (8 keys)

| Key | Was (en) |
|---|---|
| `devices.modal.snmpv3_auth_pw` | `Auth Password` (sibling keys `snmpv3_auth_p`/`snmpv3_level`/`snmpv3_priv_p`/`snmpv3_user` in the same block already had `"... (中文)"` annotations — these two were the only ones missed) |
| `devices.modal.snmpv3_priv_pw` | `Priv Password` |
| `devices.matrix.legend_link_up` | `Link Up` |
| `devices.matrix.legend_link_down` | `Link Down` |
| `devices.matrix.legend_poe` | `PoE` (fixed for consistency with `devices.detail.legend_poe`, see below — not in the original report table but same root cause) |
| `ac.port` | `Port` (rest of the `ac.*` / Access Control block was ~100% translated) |
| `iot.modal.host` + `iot.modal.host_required` | `Host / IP` / `Host / IP is required` |
| `admin.alerts.phone_number` | `Phone Number` (plain English word, no technical/brand reason to keep it) |

### B. zh-TW behind zh-CN (zh-CN already had a translation, zh-TW did not — 9 keys)

Confirmed **zero** cases in the reverse direction (zh-CN behind zh-TW).

| Key | en | zh-CN (already had) | zh-TW (was untranslated) |
|---|---|---|---|
| `devices.detail.legend_link_up` | Link Up | 连接正常 | Link Up |
| `devices.detail.legend_link_down` | Link Down | 已断开 | Link Down |
| `devices.detail.legend_poe` | PoE | PoE 供电 | PoE |
| `cameras.port` | Port | 端口 | Port |
| `iot.modal.port` | Port | 端口 | Port |
| `iot.modal.fc_03` | FC03 Holding Register | FC03 保持寄存器 | FC03 Holding Register |
| `iot.modal.fc_04` | FC04 Input Register | FC04 输入寄存器 | FC04 Input Register |
| `iot.modal.byte_order` | Byte order | 字节序 | Byte Order |
| `iot.modal.word_order` | Word order | 字序 | Word Order |

### Deliberately left as-is (not bugs)

Brand names (`Telegram`/`Discord`/`Slack`/`WhatsApp`/`LINE Notify`), industry-standard protocol
names (`Modbus TCP/RTU/RS485`), universal technical acronyms/units (`CPU`, `PM2.5`, `IoT / Modbus`),
and technical IDs also kept in English by ja-JP/ko-KR (`Recordset ID`, `Correlation ID`). Also left
`admin.alerts.webhook_url` / `bot_token` / `chat_id` / `api_key` as English — stylistic judgment
call, not a clear-cut bug like `phone_number` was.

## Fixes applied

All 18 keys fixed in both `apps/frontend/i18n/zh-TW.json` and `apps/frontend/i18n/zh-CN.json`.
Exact before/after values:

```
snmpv3_auth_pw:  TW "Auth Password" → "Auth Password (認證密碼)"   | CN "Auth Password" → "Auth Password (认证密码)"
snmpv3_priv_pw:  TW "Priv Password" → "Priv Password (私密密碼)"   | CN "Priv Password" → "Priv Password (私密密码)"
matrix.legend_link_up:   TW "Link Up" → "連接正常"   | CN "Link Up" → "连接正常"
matrix.legend_link_down: TW "Link Down" → "已斷線"   | CN "Link Down" → "已断开"
matrix.legend_poe:       TW "PoE" → "PoE 供電"       | CN "PoE" → "PoE 供电"
detail.legend_link_up:   TW "Link Up" → "連接正常"   | CN already "连接正常" (unchanged)
detail.legend_link_down: TW "Link Down" → "已斷線"   | CN already "已断开" (unchanged)
detail.legend_poe:       TW "PoE" → "PoE 供電"       | CN already "PoE 供电" (unchanged)
ac.port:         TW "Port" → "連接埠"   | CN "Port" → "端口"
cameras.port:    TW "Port" → "連接埠"   | CN already "端口" (unchanged)
iot.modal.port:  TW "Port" → "連接埠"   | CN already "端口" (unchanged)
iot.modal.host:          TW "Host / IP" → "主機 / IP"        | CN "Host / IP" → "主机 / IP"
iot.modal.host_required: TW "請輸入 Host / IP" → "請輸入主機 / IP" | CN "请输入 Host / IP" → "请输入主机 / IP"
iot.modal.fc_03: TW "FC03 Holding Register" → "FC03 保持暫存器" | CN already "FC03 保持寄存器" (unchanged)
iot.modal.fc_04: TW "FC04 Input Register" → "FC04 輸入暫存器"   | CN already "FC04 输入寄存器" (unchanged)
iot.modal.byte_order: TW "Byte Order" → "位元組順序" | CN already "字节序" (unchanged)
iot.modal.word_order: TW "Word Order" → "字組順序"   | CN already "字序" (unchanged)
admin.alerts.phone_number: TW "Phone Number" → "電話號碼" | CN "Phone Number" → "电话号码"
```

Full diff: `git diff apps/frontend/i18n/zh-TW.json apps/frontend/i18n/zh-CN.json`

## Verification already done by Claude

- [x] Both files parse as valid JSON after edits (`python3 -c "json.load(...)"`).
- [x] Ran `powershell -ExecutionPolicy Bypass -File .\scripts\sync-static.ps1` to propagate
      `apps/frontend/i18n/{zh-TW,zh-CN}.json` → `apps/backend/static/i18n/` and
      `apps/backend/cmd/agent/static/i18n/`.
- [x] `diff`'d all 4 destination files against the source-of-truth copies — byte-identical.
- [x] `git status` confirms exactly 6 files changed (2 source + 4 synced copies), nothing else
      touched.

## What's being asked of Codex

Please independently verify:

1. **Correctness of the Chinese wording itself** — is `連接埠`/`端口` (Port), `主機/主机 / IP` (Host),
   `連接正常`/`连接正常` (Link Up), `已斷線`/`已断开` (Link Down), `位元組順序`/`字節順序`-style choice
   for Byte Order, etc. natural and consistent with the rest of each file's existing terminology?
   (Claude picked terms to match neighboring already-translated strings in the same file, e.g.
   zh-TW's own `fc_01`/`fc_02` pattern for `保持暫存器`/`輸入暫存器`.)
2. **No other zh-TW/zh-CN drift was missed.** The method above only catches keys where the
   *value* still exactly equals (or is a near-cognate of) the English string, or where a
   CJK-ratio heuristic flags near-zero Chinese content. It would **not** catch a key that has
   *some* Chinese but is semantically wrong or mistranslated. A fresh read-through of
   `apps/frontend/i18n/zh-TW.json` and `zh-CN.json` for semantic correctness (not just
   "is it in Chinese") would be valuable.
3. **No regressions in the live UI.** These strings are used in Devices detail/matrix legends,
   Access Control device modal, Camera modal, IoT/Modbus device modal, and Admin alert channel
   settings. Worth spot-checking those screens in zh-TW and zh-CN modes.
4. Confirm the `apps/backend/static/i18n/` and `apps/backend/cmd/agent/static/i18n/` copies are
   still in sync with `apps/frontend/i18n/` (they should never be hand-edited — always re-run
   `scripts/sync-static.ps1`/`.sh` after any `apps/frontend/i18n/*.json` change).

No commit has been made yet — all changes are currently unstaged in the working tree.
