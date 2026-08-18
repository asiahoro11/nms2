# License 產生器簡易操作手冊

本工具只允許在公司內部、離線的授權工作站使用。它採 Ed25519 私鑰簽章；NMS 主機只有公鑰，無法自行仿造 License。

## 一、使用前準備

1. 解壓縮獨立的 License 產生器包，不要放進 NMS 安裝目錄。
2. 將正式簽發私鑰放在產生器執行檔同一目錄，檔名必須為 `issuer_private.key`。
3. 私鑰檔內容可為純 Base64，或 `NMS_LICENSE_PRIVATE_KEY_B64=<Base64 私鑰>`。
4. 私鑰只應存在於受控的離線工作站及加密備份，不可寄給客戶、提交 Git、放入 NMS 包或上傳雲端共用空間。
5. 確認私鑰所對應的公鑰，就是正式 NMS 執行檔內嵌的 License 公鑰。更換私鑰會造成新 License 無法在既有版本啟用。

## 二、Windows 快速操作

1. 雙擊 `start_license_generator.bat`。
2. 選擇授權模式：`1` 是正式授權，必須輸入客戶 Machine ID；`2` 是 PoC，期限從第一次啟用開始計算。
3. 選擇授權套件：完整、設備、攝影機、IoT、PDU/UPS、門禁、通知或自訂。
4. 工具只會詢問該套件需要的數量。例如 IoT 套件不再要求攝影機數量。
5. 正式授權輸入年限，`0` 代表永久；PoC 輸入有效天數。
6. 核對簽發摘要，輸入 `Y` 後才會真正簽發。
7. 工具會在目前目錄建立 `license_<Machine ID>_<日期時間>.txt`。
8. 只將產生的 License TXT 交付客戶，不要交付產生器與私鑰。

## 三、自訂功能

選擇自訂套件後，以逗號輸入需要的功能代碼：

```text
device_management,camera_viewer,camera_recording,access_control,pdu,iot,line,telegram,whatsapp,discord,slack
```

工具會拒絕未知功能；選擇設備或攝影機功能時，也會要求有效的授權數量。

## 四、JSON 自動化模式

既有自動化仍可使用 JSON payload：

```powershell
.\license-issuer.exe -payload .\payload.json -output .\customer-license.txt
```

若私鑰不在執行檔旁，可明確指定：

```powershell
.\license-issuer.exe -private-key D:\Secure\issuer_private.key -payload .\payload.json -output .\customer-license.txt
```

不要把私鑰內容放在命令列。為相容既有 Secret Store，工具仍接受 `NMS_LICENSE_PRIVATE_KEY_B64` 環境變數，且優先於私鑰檔。

## 五、交付前檢查

- Machine ID 與客戶 NMS 顯示完全一致。
- 正式授權／PoC 模式正確。
- 設備數、攝影機數、功能與期限符合訂單。
- 產生檔以 `ED25519-V1.` 開頭。
- 使用對應正式公鑰的測試環境驗證啟用成功。
- 工單只記錄必要摘要，不附私鑰，也不要在公開紀錄貼完整 License。

## 六、常見問題

### 顯示找不到 issuer_private.key

確認私鑰檔位於 `license-issuer.exe` 同一目錄，或使用 `-private-key` 指向受控位置。

### 正式 License 無法啟用

優先檢查 Machine ID、NMS 版本內嵌公鑰與簽發私鑰是否為同一組。不要重新產生金鑰嘗試修正。

### 私鑰疑似外洩

立即停止簽發、封存稽核資料並啟動金鑰輪替。單純刪除外洩檔案不足以恢復安全；需要使用新公鑰重建 NMS，並制定既有客戶 License 遷移方案。
