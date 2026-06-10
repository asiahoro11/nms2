# Modbus IoT 管理系統 v2 — 設計文件

日期:2026-06-10
狀態:已與需求方逐段確認核可
範圍:`new_nms_sync` 既有 `modules/iot` 的原地演進改造

---

## 1. 背景與目標

現有 IoT/Modbus 模組(`apps/backend/modules/iot`)採「一台設備 = 一個暫存器讀取」的簡化模型,僅支援 Modbus TCP,前端頁面為基本表單與表格。本次改造目標:

1. 升級為完整的 Modbus 設備管理:**設備 → 多點位** 兩層模型,支援 TCP、RTU 串列埠、RTU over TCP 三種傳輸。
2. 監控 + 控制:讀取四種暫存器區域,並可寫入 coil 與 holding register(含安全邊界、二次確認、回讀驗證、稽核)。
3. 警報規則與通知、歷史資料與趨勢圖、設備範本庫、總覽儀表板、值班模式。
4. 強化既有「斷線暫存、恢復續傳」機制:量測資料在無法送達客戶系統時本地保留,恢復後續傳且可去重。
5. 全新 UI/UX:沿用 NMS 設計語言(風格 C 為骨架),總覽採趨勢卡片(風格 B),值班模式採高密度深色大數字(風格 A)。

## 2. 已確認的需求決策

| 問題 | 決策 |
|------|------|
| 系統定位 | 升級現有 NMS 的 iot 模組(原地演進,不另建模組、不做獨立系統) |
| 傳輸範圍 | Modbus TCP + RTU 串列埠 + RTU over TCP |
| 讀寫範圍 | 監控 + 控制(完整版) |
| 功能範圍 | 警報與通知、歷史與趨勢、範本庫、總覽儀表板、斷線暫存續傳 |
| 規模 | 依客戶部署彈性;小型開箱即用,中型靠合併讀取與降取樣,上限明文化 |
| UI 風格 | 混搭:C(NMS 一致性)為骨架 + B(趨勢卡片)總覽 + A(SCADA)值班模式 |
| 改造路線 | 原地演進:v2 資料表 + 自動遷移,同一側欄頁面整頁換新 |
| 授權閘 | 不另設授權閘,與現有 IoT 頁一致 |
| 警報外送 | 警報走 NMS 通知中心;續傳佇列本版只送量測值,維持 v1 payload 契約 |

## 3. 總體架構

- 後端:Go(Gin)+ SQLite(已啟用 WAL、busy_timeout=30000),模組維持在 `apps/backend/modules/iot/`,由單一 `service.go` 拆分為多檔。
- Modbus 函式庫:新增 `github.com/simonvetter/modbus`(MIT),單一函式庫支援 `tcp://`、`rtu://`、`rtuovertcp://` 三種傳輸,串列埠底層為 `go.bug.st/serial`(跨平台,Windows COM 與 Linux tty 皆可列舉)。
- 前端:Vanilla JS,維持三份拷貝同步慣例(`apps/frontend` → `apps/backend/static` → `apps/backend/cmd/agent/static`)。趨勢圖引入 vendored **uPlot**(MIT,約 45KB,零依賴),隨 `go:embed` 離線打包。
- 路由:沿用既有三層權限群組——讀取(登入即可)、editor(操作)、admin(設定)。v2 端點掛 `/iot/v2/*`,v1 端點保留(見 §9)。

## 4. 資料模型(SQLite)

### 4.1 `iot_devices_v2` — 連線層

| 欄位 | 型別 | 說明 |
|------|------|------|
| id | INTEGER PK | |
| name, description | TEXT | |
| transport | TEXT | `tcp` / `rtuovertcp` / `rtu` |
| host, port | TEXT, INTEGER | tcp / rtuovertcp 用,port 預設 502 |
| serial_port | TEXT | rtu 用,如 `COM3`、`/dev/ttyUSB0` |
| baud_rate, data_bits, parity, stop_bits | INTEGER/TEXT | rtu 用,預設 9600-8-N-1;parity 為 `N`/`E`/`O` |
| unit_id | INTEGER | 預設 1 |
| timeout_ms | INTEGER | 預設 1000 |
| inter_request_delay_ms | INTEGER | 幀間延遲,預設 0,慢設備可調 |
| poll_interval_seconds | INTEGER | 預設 10 |
| enabled | BOOLEAN | |
| template_key | TEXT | 套用過的範本(僅資訊性) |
| status | TEXT | `unknown` / `online` / `offline` / `error` / `disabled` |
| last_seen, last_error | DATETIME, TEXT | |
| created_at, updated_at | DATETIME | |

### 4.2 `iot_points` — 點位層

| 欄位 | 說明 |
|------|------|
| id, device_id(FK, ON DELETE CASCADE) | |
| name | 顯示名,如「A 相電壓」 |
| metric | 機器名(續傳 payload 用),如 `voltage_a` |
| register_area | `coil` / `discrete` / `input` / `holding` |
| address | 0-based 暫存器位址 |
| data_type | `bool` / `uint16` / `int16` / `uint32` / `int32` / `float32` / `uint64` / `int64` / `float64`(quantity 由 data_type 推導) |
| byte_order | `AB` / `BA`(單字內位元組序) |
| word_order | `high_first` / `low_first`(多字資料的字序) |
| scale, offset | 顯示值 = raw × scale + offset |
| unit, decimals | 顯示單位與小數位 |
| writable | 是否允許寫入(僅 coil / holding 可為真) |
| write_min, write_max | 寫入安全上下限(以換算後的顯示值為準) |
| pinned | 釘選到總覽與值班模式 |
| sort_order, enabled | |
| last_value, last_raw, last_quality(`good`/`stale`/`error`), last_error, last_polled_at | 即時快取 |
| created_at, updated_at | |

### 4.3 `iot_alarm_rules`

rule:point_id(FK)、kind(`high`/`low`/`equal`/`not_equal`/`offline`,offline 掛在設備層、point_id 為空)、threshold、hysteresis(遲滯)、duration_seconds(持續達標才觸發)、severity(`info`/`warning`/`critical`)、enabled、notify(是否推 NMS 通知中心)。

### 4.4 `iot_alarm_events`

rule_id、point_id、device_id、state(`active` → `acknowledged` → `cleared`;cleared 可不經 ack)、triggered_at、cleared_at、acked_by、acked_at、trigger_value、message。作用中狀態唯一(同 rule 不重複開 active 事件)。

### 4.5 歷史資料

- `iot_history_raw(point_id, ts, value)`,PK(point_id, ts),預設保留 **7 天**(可設 1–30)。
- `iot_history_hourly(point_id, ts_hour, min, max, avg, count)`,PK(point_id, ts_hour),預設保留 **365 天**(可設 30–1095)。
- 聚合工作每小時跑一次(冪等 upsert);清理工作每日跑一次。

### 4.6 `iot_forward_queue` — 續傳佇列(與歷史資料分離)

id、event_id(TEXT UNIQUE,UUID,客戶端去重用)、payload_json、status(`pending` / `sent`)、attempts、next_attempt_at(指數退避)、created_at、sent_at。已送出資料依既有 sent-hold 保留分鐘數清除。**預設不丟棄未送出資料**;佇列筆數上限預設 100,000(可設),超過時丟最舊並累計丟棄計數、發系統通知。

### 4.7 `iot_templates`

id、key(UNIQUE)、name、description、vendor、builtin(內建範本由程式嵌入 JSON 提供,啟動時 seed/更新)、points_json(點位定義陣列)、defaults_json(建議傳輸參數,如 baud)、created_at、updated_at。內建範本初版三個:通用電表、溫濕度計、UPS。

### 4.8 模組設定

沿用既有 iot 設定儲存機制(forwarder settings 同表),新增:raw/hourly 保留天數、佇列上限、前端更新間隔(一般/值班)、預設輪詢間隔、離線判定失敗次數。

## 5. 後端設計(`modules/iot/` 檔案布局)

| 檔案 | 職責 |
|------|------|
| `types.go` | v2 型別 + 保留 v1 型別 |
| `service.go` | 編排:EnsureTables、遷移、CRUD、設定(瘦身後) |
| `client.go` | simonvetter/modbus 包裝。**連線以「transport + 位址(host:port 或 serial_port)」為 key 共用**:同一 RS-485 匯流排上多台設備(不同 unit_id)共用一個 client 與一把 mutex,序列化存取;每請求前依設備設定套 inter_request_delay |
| `poller.go` | 每設備一個排程 goroutine。讀取規劃:同 register_area 且位址連續(或間隔 ≤ 8 字)的點位合併為一次 Modbus 讀取(單次上限:暫存器 120 字、coil 1968 點)。失敗退避:連續 3 次連線失敗 → status=offline + 可選離線警報,間隔 ×2 封頂 10 分鐘,成功即重置 |
| `decode.go` | data_type × byte_order × word_order 編解碼,純函式 |
| `history.go` | 緩衝 channel 批次寫入(每 2 秒或滿 500 筆)、每時聚合、每日清理、查詢(依時間範圍自動選 raw 或 hourly,回傳含 min/max 區間) |
| `alarms.go` | 規則評估(遲滯 + 持續時間)、事件生命週期、推送 NMS 通知中心 |
| `forwarder.go` | 批次 POST + token(維持 v1 payload 契約);指數退避 1m→2m→4m→…→30m 封頂;重啟後從佇列續跑 |
| `writes.go` | 寫入管線:檢查 writable + 上下限 → 編碼(coil 用 WriteSingleCoil;holding 單字用 WriteSingleRegister、多字用 WriteMultipleRegisters)→ **回讀驗證** → 寫入既有 audit 稽核(操作者、點位、舊值、新值、結果) |
| `templates.go` | 內建範本嵌入 + 自訂 CRUD + 匯入/匯出 JSON |
| `serialports.go` | 串列埠列舉(`go.bug.st/serial.GetPortsList`) |

並行模型:約 200 台設備 = 約 200 個輪詢 goroutine + 每連線一把 mutex + 一個歷史寫入 goroutine + 一個 forwarder goroutine + 聚合/清理 timer,對 Go 而言負擔輕。

## 6. API 設計

### 讀取群組(登入即可)

```
GET /iot/v2/overview                      總覽聚合(統計、釘選點位即時值+sparkline 資料、作用中警報、佇列狀態)
GET /iot/v2/devices?include=points        設備清單(可含點位與即時值)
GET /iot/v2/devices/:id                   單一設備 + 點位
GET /iot/v2/history?point_ids=&from=&to=&max_points=   歷史查詢(自動 raw/hourly)
GET /iot/v2/history.csv?point_ids=&from=&to=           CSV 匯出
GET /iot/v2/alarms?state=active|history&severity=&device_id=
GET /iot/v2/alarm-rules?point_id=
GET /iot/v2/templates
GET /iot/v2/serial-ports
```

### editor 群組

```
POST /iot/v2/devices/:id/poll             立即輪詢
POST /iot/v2/points/:id/write {value}     控制寫入
POST /iot/v2/alarms/:id/ack               警報確認
```

### admin 群組

```
POST/PUT/DELETE /iot/v2/devices[/:id]     設備 CRUD(POST 可帶 points 陣列一次建立)
PUT  /iot/v2/devices/:id/points           點位批次 upsert(wizard / 批次編輯用)
POST /iot/v2/devices/test                 連線測試(不存檔,試讀一筆並回報延遲或錯誤)
POST/PUT/DELETE /iot/v2/alarm-rules[/:id]
POST/PUT/DELETE /iot/v2/templates[/:id] + POST /iot/v2/templates/import + GET /iot/v2/templates/:id/export
GET/PUT /iot/v2/settings                  模組設定(保留天數、佇列上限、更新間隔等)
```

既有 `/iot/forwarder/settings`、`/iot/queue/*`、`/iot/ingest` 端點不動。

## 7. 前端 UI/UX 設計

檔案:`js/iot.js` 重寫(趨勢圖另拆 `js/iot-trends.js`)、新增 `css/iot-module.css`、`index.html` 的 `#iot` 區段重寫、`vendor/uplot.iife.min.js` + `vendor/uplot.min.css`、五語系 i18n key(`iot_*`)。

側欄維持單一「IoT / Modbus」入口;頁內六分頁 + 值班模式按鈕。所有元件沿用 NMS 設計 token,亮/暗主題自動跟隨。

### 7.1 總覽(風格 B)

四張統計卡(設備在線 x/y、監測點位、作用中警報、續傳佇列+同步狀態)→ 釘選點位趨勢卡 grid(即時值 + 單位 + 狀態 badge + sparkline;警報中紅框)→ 作用中警報列表(severity 圓點、訊息含持續時間、可直接確認)。空狀態:大 CTA「新增第一台設備」/「從範本開始」。

### 7.2 設備(風格 C)

表格:名稱、傳輸(badge:`TCP · 192.168.1.60` / `RTU · COM3`)、unit、點位數、狀態、最後通訊、錯誤圖示(tooltip 顯示 last_error)。點列開**右側抽屜**:

- 標頭:名稱 + 傳輸 chip + 狀態;動作:測試連線、立即輪詢、編輯、刪除。
- 點位即時值表:點位名(掛規則的顯示鈴鐺)、位址(一律 0-based,顯示為 `holding 12`,tooltip 補充傳統 4xxxx 表示法;系統內部與 API 全程 0-based,避免 off-by-one)、即時值(含單位與小數位)、品質(良好 / ◐ 逾時 n 秒 / ✕ 錯誤)、操作(趨勢、釘選;writable 點顯示控制元件)。
- 抽屜底部安全說明列(寫入保護摘要)。

**新增設備 wizard(三步)**:① 傳輸與連線參數(transport 切換動態表單;RTU 串列埠用下拉列舉)+ 測試連線;② 套範本(套用後仍可改)或手動批次表格編輯點位;③ 確認摘要。

### 7.3 趨勢

uPlot 多點位疊圖;點位選擇器依設備分組;時間快選 1h / 24h / 7d / 30d / 自訂;範圍超過 raw 保留期自動切 hourly 並畫 min/max 區間帶;CSV 匯出鈕。

### 7.4 警報

「作用中」(確認鈕)與「歷史」(時間/嚴重度/設備篩選)兩檢視 + 規則總表(啟用開關、編輯)。

### 7.5 範本

內建與自訂範本卡片(名稱、廠商、點位數);動作:套用(跳轉 wizard 步驟①)、匯出 JSON;頁面層級:匯入 JSON。內建範本不可刪改,可「另存為自訂」。

### 7.6 傳輸(admin 限定)

客戶系統 URL / token / 批次大小 / sent 保留分鐘;佇列即時狀態(待送、重試倒數、已送暫存)、丟棄計數;手動 flush;最近失敗原因(白話 + 原始錯誤)。

### 7.7 值班模式(風格 A)

全螢幕、**強制深色**(不跟主題);所有釘選點位以大數字 tile 自動排滿(設備名+點位名、大值、單位、狀態左邊框色);作用中警報置頂橫條輪播;更新間隔預設 3 秒(可設 2–5);ESC 或按鈕退出。

### 7.8 控制寫入 UX

- coil:toggle → 確認彈窗(顯示目標狀態)→ pending 動畫 → 回讀成功才切換,失敗 toast + 還原。
- holding:「設定」彈窗:目前值、允許範圍(write_min–write_max)、數值輸入驗證 → 確認 → pending → 回讀結果。
- 所有寫入結果(含失敗的 Modbus 例外碼白話翻譯)以 toast 呈現;稽核記錄可在既有稽核頁查到。

### 7.9 更新機制

HTTP 輪詢:一般分頁 5–10 秒(預設 5,可設定)、值班模式 2–5 秒。不引入 WebSocket/SSE。

## 8. 錯誤處理

| 情境 | 行為 |
|------|------|
| 點位讀取失敗 | quality=`error` + last_error;設備整體連不上 → 連續 3 次後 status=`offline`、觸發離線警報(若有設規則)、輪詢退避 ×2 封頂 10 分鐘 |
| 值過期 | last_polled 距今 > 2× 輪詢間隔 → 前端顯示 `stale`(◐ 逾時) |
| RTU 匯流排 | 同埠共用連線 + mutex 序列化;inter_request_delay 可調;串列埠打不開 → 設備層明確錯誤 |
| 寫入失敗 | Modbus 例外碼對映五語系白話訊息;回讀不一致 → 警告;所有嘗試進稽核 |
| 續傳失敗 | 指數退避封頂 30m;不丟棄直到送達;佇列上限保護(丟最舊+計數+通知);event_id 去重;重啟無縫續傳 |
| 歷史寫入失敗 | 記 log、不中斷輪詢;聚合/清理失敗下輪重試 |

## 9. 遷移與相容

- 啟動時冪等遷移:v1 `iot_devices` 中 protocol 以 `modbus` 開頭(如 `modbus_tcp`)的列 → 一台 v2 設備(transport=`tcp`)+ 一個點位;FC 對映:FC1→coil、FC2→discrete、FC3→holding、FC4→input;byte/word order、scale、offset、metric、輪詢間隔一併帶過。遷移完成寫標記,不重跑。
- v1 REST ingest(`POST /iot/ingest`)、forwarder 設定、queue 端點、payload 契約全部保留不動。
- v1 modbus 設備 CRUD 端點保留但標記棄用(文件註明),避免破壞既有外部整合。
- 舊 `iot_measurements` 資料不轉換,僅供查舊資料;新歷史一律進 v2 表。

## 10. 測試策略

- 單元:`decode.go` 全型別 × byte/word order 矩陣;讀取合併規劃(連續/間隔/跨區域);警報評估(遲滯、持續時間、解除、ack 流程);歷史聚合與保留清理(in-memory SQLite)。
- 整合:以 simonvetter/modbus 內建 server 在測試中起 in-process Modbus TCP server,跑「輪詢→解碼→歷史→警報→佇列」全流程與寫入回讀。
- 續傳驗收:`httptest` 模擬客戶系統:正常收 → 停機累積 → 重啟驗證續傳完整且 event_id 不重複。
- 遷移:v1 列 → v2 設備+點位對映正確、冪等。
- 既有 `service_test.go` 維持通過(必要時隨 v1 行為調整)。
- 前端:手動驗收清單(六分頁 + wizard + 控制寫入 + 值班模式 + 亮/暗主題 + 五語系),實作完成後以瀏覽器實測。

## 11. 效能定位(出貨文件用)

| 規模 | 條件 | 設定建議 |
|------|------|----------|
| 小型 | ≤50 台、≤500 點、輪詢 ≥1s | 預設值 |
| 中型 | ≤200 台、≤5,000 點、輪詢 ≥5s | raw 保留降為 3 天;確認合併讀取生效 |
| 超出 | >200 台或次秒級輪詢 | 不支援,明文標示超出單機 SQLite 設計範圍 |

## 12. 範圍外(本版不做)

MQTT 訂閱、Modbus ASCII、警報事件外送客戶系統(僅走 NMS 通知中心)、跨 NMS 多節點彙整、報表模組整合(留資料面鉤子)、WebSocket 即時推送。
