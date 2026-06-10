# Management Server 資料庫維護手冊

版本: `v1.2.4.8`

## 範圍

本手冊提供客戶維運人員執行 SQLite 資料庫備份 還原 健康檢查與安全維護的基本流程

## 重要資料

| 路徑 | 用途 |
| --- | --- |
| `data/nms.db` | 主要資料庫 |
| `data/` | runtime data |
| `config.yaml` | 客戶環境設定 |

## 備份

停止服務後備份

```bash
sudo systemctl stop nms
cp -a data "data-backup-$(date +%Y%m%d-%H%M%S)"
sudo systemctl start nms
```

線上備份建議使用系統內建備份功能  或由客戶 DB/檔案備份工具執行一致性快照

## 還原

```bash
sudo systemctl stop nms
cp -a data data-before-restore
rm -rf data
cp -a data-backup-YYYYMMDD-HHMMSS data
sudo systemctl start nms
```

## 健康檢查

```bash
sqlite3 data/nms.db "PRAGMA integrity_check;"
sqlite3 data/nms.db "PRAGMA quick_check;"
```

預期結果

```text
ok
```

## 清理與效能

```bash
sqlite3 data/nms.db "VACUUM;"
sqlite3 data/nms.db "ANALYZE;"
```

建議在維護時段執行

## 安全維護

- 限制 `data/` 只有服務帳號可讀寫
- 備份檔需加密保存
- 備份檔不可放在 Web root
- 不要在 ticket email 或聊天工具中傳送 `nms.db`
- 離職或權限調整後需更換 admin password 與 integration token

## 升級前檢查

- 已備份 `data/`
- 已備份 `config.yaml`
- 已確認可回復上一版 artifact
- 已記錄目前版本與部署路徑

