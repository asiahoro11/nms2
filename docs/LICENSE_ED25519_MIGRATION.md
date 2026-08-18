# Ed25519 License V1 遷移指南

## 安全邊界

- License Issuer 僅保存 `NMS_LICENSE_PRIVATE_KEY_B64`。
- NMS Runtime 僅保存 `NMS_LICENSE_PUBLIC_KEY_B64`。
- 私鑰不得提交至 Git、複製到 NMS 主機、寫入安裝包或輸出至 CI log。

## 建立發證金鑰

請在隔離的發證工作站執行：

```powershell
go run ./apps/backend/cmd/license-keygen
```

將私鑰放入發證端的 Secret Store，公鑰則在正式建置時透過
`NMS_LICENSE_PUBLIC_KEY_B64` 注入 NMS 執行檔。私鑰必須另外保留加密備份；遺失後無法續發相容授權。

## 簽發新授權

一般人工簽發可將私鑰以 `issuer_private.key` 放在 `license-issuer` 同一目錄，直接執行互動式精靈。工具會依序詢問模式、Machine ID、授權套件、數量與期限，確認後自動輸出 License TXT。詳細步驟請參閱 `LICENSE_GENERATOR_GUIDE_ZH-TW.md`。

自動化簽發可準備 `SignedLicense` JSON，再於離線發證端執行：

```powershell
$env:NMS_LICENSE_PRIVATE_KEY_B64 = '<issuer-private-key>'
go run ./apps/backend/cmd/license-issuer -payload .\payload.json -output .\customer-license.txt
```

請勿將 payload、私鑰或完整授權寫入公開 log。產生器與私鑰不得交付客戶或放入 NMS 主機。

## 舊 License 轉換

舊 AES License 預設停用。只有在受控轉換期間才可暫時設定
`NMS_ALLOW_LEGACY_LICENSE=1`，完成轉換後必須移除。

在隔離的發證工作站執行：

```powershell
$env:NMS_LICENSE_PRIVATE_KEY_B64 = '<issuer-private-key>'
go run ./apps/backend/cmd/license-migrate -legacy-file .\legacy.txt -machine-id '<machine-id>'
```

工具會先驗證舊授權，再以相同權益簽發 Ed25519 V1 License。PoC License 不需要
`-machine-id`。轉換工具與私鑰都不得安裝在 NMS 主機。

## Runtime 驗證

正式建置必須提供：

```text
NMS_LICENSE_PUBLIC_KEY_B64=<public-key>
```

Runtime 只使用公鑰驗證簽章、機器綁定與到期時間；資料庫內遭竄改或無效的授權不會被計入設備、攝影機或功能額度。
