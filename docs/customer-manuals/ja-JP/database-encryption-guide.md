# Management Server データベースとバックアップ暗号化手順書

バージョン: `v1.2.4.8`

## 範囲

本書は暗号化バックアップ パスワード保護 安全な保管について説明します  
完全なデータベース暗号化の利用可否は顧客ライセンスとデプロイポリシーに依存します

## 基本原則

- バックアップファイルは暗号化する
- バックアップパスワードはログインパスワードと同じにしない
- パスワードは顧客の password vault に保存する
- script repo email ticket document にパスワードを書かない

## 推奨パスワード規則

- 12 文字以上
- 大文字 小文字 数字 記号を含める
- 会社名 プロジェクト名 device 名を使わない
- 納品または保守後に必要に応じてローテーションする

## 暗号化バックアップ

```http
POST /api/v1/system/backup/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: application/json
```

```json
{
  "password": "<backup-password>"
}
```

## リストア

```http
POST /api/v1/system/restore/encrypted
Authorization: Bearer <admin-jwt>
Content-Type: multipart/form-data
```

| Field | 説明 |
| --- | --- |
| `file` | 暗号化バックアップファイル |
| `password` | バックアップパスワード |

## 受け入れ確認

- バックアップファイルに平文データが含まれない
- 誤ったパスワードではリストアできない
- 正しいパスワードでテスト環境にリストアできる
- リストア後 `/api/v1/system/info` が正常
- users permissions devices topology audit data が残っている

## 保管ポリシー

- daily 7  weekly 4  monthly 3 以上を保持
- 少なくとも 1 つは offline または offsite に保管
- 定期的にリストアテストを行う
- 期限切れバックアップは安全に削除する

