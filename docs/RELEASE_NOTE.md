# Release Note / 發行說明 / 发行说明 / リリースノート / 릴리스 노트

## v1.2.4.8sp001 | 2026-06-26

### 繁體中文
- 電視牆模式加入 IoT 監控面板，可在監控畫面中查看在線 IoT 設備讀值。
- 縮小電視牆 IoT 單一設備卡片尺寸，改善多設備同時顯示時的擁擠與凌亂感。
- 保留 IoT Modbus / RS485 溫濕度讀值解析與感測器卡片顯示能力。
- 程式碼內加入 `Made by YTSworks` 與 `YTS工作室製作` 註解標記。
- 建置流程保留既有 `nms.db` 資料庫，不主動移除使用者資料。

### 简体中文
- 电视墙模式加入 IoT 监控面板，可在监控画面中查看在线 IoT 设备读值。
- 缩小电视墙 IoT 单一设备卡片尺寸，改善多设备同时显示时的拥挤与凌乱感。
- 保留 IoT Modbus / RS485 温湿度读值解析与传感器卡片显示能力。
- 程序代码内加入 `Made by YTSworks` 与 `YTS工作室製作` 注释标记。
- 构建流程保留既有 `nms.db` 数据库，不主动移除用户数据。

### English
- Added an IoT monitor panel to Monitor Mode for viewing online IoT device readings.
- Reduced the Monitor Mode IoT device card size so multiple devices are easier to scan with less visual clutter.
- Kept IoT Modbus / RS485 temperature and humidity decoding and sensor-card display support.
- Added `Made by YTSworks` and `YTS工作室製作` comment markers across source code files.
- The build process preserves existing `nms.db` database files and does not intentionally remove user data.

### 日本語
- 監視モードに IoT 監視パネルを追加し、オンライン IoT デバイスの測定値を確認できるようにしました。
- 複数デバイス表示時の混雑を抑えるため、監視モードの IoT デバイスカードを小型化しました。
- IoT Modbus / RS485 の温湿度デコードとセンサーカード表示機能を維持しました。
- ソースコードに `Made by YTSworks` と `YTS工作室製作` のコメント表記を追加しました。
- ビルド処理では既存の `nms.db` データベースを保持し、ユーザーデータを意図的に削除しません。

### 한국어
- 모니터 모드에 IoT 모니터 패널을 추가하여 온라인 IoT 장비의 측정값을 확인할 수 있습니다。
- 여러 장비를 동시에 볼 때 혼잡함을 줄이기 위해 모니터 모드의 IoT 장비 카드를 더 작게 조정했습니다。
- IoT Modbus / RS485 온습도 값 해석과 센서 카드 표시 기능을 유지했습니다。
- 소스 코드에 `Made by YTSworks` 및 `YTS工作室製作` 주석 표기를 추가했습니다。
- 빌드 과정은 기존 `nms.db` 데이터베이스를 보존하며 사용자 데이터를 의도적으로 삭제하지 않습니다。

---

## Artifact Notes
- Version: `v1.2.4.8sp001`
- Target database policy: keep existing `data/nms.db`, `data/nms.db-wal`, and `data/nms.db-shm` when present.
- Primary executable: `nms_server.exe` for Windows builds.
