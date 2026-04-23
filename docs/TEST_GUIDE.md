# Management Server v1.0.9sp4 測試指南

**版本**: v1.0.9sp4 (Final - Modal Fix)
**測試日期**: 2026-02-17
**測試重點**: 密碼變更 Modal 穩定性

---

## 主要測試項目

### 測試 1: 密碼變更 Modal 不閃退

**步驟**:

1. 確認目前執行中的 `Management Server.exe`
2. 執行路徑：

   ```
   C:\Users\HP\.gemini\antigravity\playground\azure-universe\nms_sync\windows_release\v1.0.9sp4\Management Server.exe
   ```

3. 開啟瀏覽器至 `http://localhost:8080`
4. 按 F12 開啟開發者工具的 Console 分頁
5. 登入: `admin` / `admin123`

**預期結果**:

- 密碼變更 Modal 出現
- Modal **不會顯示超過 3 秒**（或閃退）
- Console 顯示:

  ```
  [Auth] showChangePasswordModal called, forced: true
  [Modal] Opening modal: 請設定新密碼
  [Modal] isModalOpen = true
  [Auth] modalForcedLock = true
  ```

- Console **不應該**顯示: `Fresh page load detected. Auto-logout in 3 seconds...`

**實際結果**:

- [ ] Pass / [ ] Fail
- 備註: ________________

---

### 測試 2: 密碼變更功能正常

**步驟**:

1. 在 Modal 中填入：
   - 舊密碼: `admin123`
   - 新密碼: `NewPassword123!`
   - 確認密碼: `NewPassword123!`
2. 點擊「確定」

**預期結果**:

- 顯示「密碼已更新，系統將重新載入」
- 1 秒後自動重新載入頁面
- 跳轉到登入頁面
- 可以用新密碼 `NewPassword123!` 登入

**實際結果**:

- [ ] Pass / [ ] Fail
- 備註: ________________

---

### 測試 3: 頁面顯示正常

**步驟**:

1. 登入系統
2. 檢查頁面外觀

**預期結果**:

- CSS 正常載入（沒有樣式跑版）
- 側邊欄顯示正常
- Dashboard 圖表顯示正常
- 沒有紅色錯誤訊息（F12 Console）

**實際結果**:

- [ ] Pass / [ ] Fail
- 備註: ________________

---

### 測試 4: Session 功能正常

**步驟 A - Idle Timeout**:

1. 登入後等候 6 分鐘（不要動滑鼠或鍵盤）
2. 預期: 顯示「您已閒置過久，系統將自動登出」

**步驟 B - Visibility Check**:

1. 登入後切換到其他分頁
2. 在其他分頁停留 30 秒
3. 切回 NMS 分頁
4. 預期: Session 檢查正常運作（如已過期就登出）

**實際結果**:

- [ ] Pass / [ ] Fail
- 備註: ________________

---

## 除錯檢查清單

如果測試失敗，請檢查以下項目：

### Console 錯誤

打開 F12 Console，檢查是否有紅色錯誤訊息：

- [ ] JavaScript 錯誤
- [ ] CSS 載入失敗
- [ ] API 錯誤

### 版本確認

在 Console 中輸入以下指令，確認版本：

```javascript
console.log(document.querySelector('[data-version]')?.dataset.version);
```

應該顯示: `v1.0.9sp4-fix3` 或更新版本

### Local Storage 檢查

在 Console 中輸入：

```javascript
console.log({
  token: localStorage.getItem('nms_token') ? 'exists' : 'none',
  requirePwdChange: localStorage.getItem('nms_require_pwd_change'),
  forceRefresh: localStorage.getItem('nms_force_init_refresh')
});
```

### Session Storage 檢查

在 Console 中輸入：

```javascript
console.log({
  sessionActive: sessionStorage.getItem('nms_session_active')
});
```

---

## 完整測試報告模板

```
=== Management Server v1.0.9sp4 測試報告 ===

測試日期: ____________
測試人員: ____________
測試環境: Windows 10/11

測試結果:
[ ] 測試 1: 密碼變更 Modal 不閃退 - Pass/Fail
[ ] 測試 2: 密碼變更功能正常 - Pass/Fail
[ ] 測試 3: 頁面顯示正常 - Pass/Fail
[ ] 測試 4: Session 功能正常 - Pass/Fail

Console 日誌: (請附上)

問題描述: (如有問題)


總結: __個測試通過 / __個測試失敗

備註:


```

---

## 已知問題 (如有)

暫無已知問題。

---

## 回報方式

如果測試失敗，請提供：

1. F12 Console 截圖（含完整 log）
2. 測試步驟
3. 預期結果 vs 實際結果
4. 瀏覽器與版本（Chrome/Firefox/Edge）

---

**祝測試順利！**
