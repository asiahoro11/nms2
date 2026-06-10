# Management Server データベース保守手順書

バージョン: `v1.2.4.8`

## 範囲

本書は顧客運用担当者向けに SQLite データベースのバックアップ リストア ヘルスチェック セキュリティ保守の基本手順を示します

## 重要パス

| パス | 用途 |
| --- | --- |
| `data/nms.db` | メインデータベース |
| `data/` | runtime data |
| `config.yaml` | 顧客環境設定 |

## バックアップ

```bash
sudo systemctl stop nms
cp -a data "data-backup-$(date +%Y%m%d-%H%M%S)"
sudo systemctl start nms
```

オンラインバックアップは 組み込みバックアップ機能または整合性を保証できる顧客側 snapshot ツールを使用してください

## リストア

```bash
sudo systemctl stop nms
cp -a data data-before-restore
rm -rf data
cp -a data-backup-YYYYMMDD-HHMMSS data
sudo systemctl start nms
```

## ヘルスチェック

```bash
sqlite3 data/nms.db "PRAGMA integrity_check;"
sqlite3 data/nms.db "PRAGMA quick_check;"
```

期待結果

```text
ok
```

## クリーンアップと性能

```bash
sqlite3 data/nms.db "VACUUM;"
sqlite3 data/nms.db "ANALYZE;"
```

保守時間帯に実行してください

## セキュリティ保守

- `data/` は service account のみ読み書き可能にする
- バックアップファイルは暗号化して保存する
- バックアップを Web root に置かない
- `nms.db` を ticket email chat で送信しない
- 人員変更や権限変更後に admin password と integration token を変更する

## アップグレード前チェック

- `data/` をバックアップ済み
- `config.yaml` をバックアップ済み
- 前バージョン artifact に戻せる
- 現在のバージョンとデプロイパスを記録済み

