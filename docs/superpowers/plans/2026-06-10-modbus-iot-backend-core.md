# Modbus IoT v2 — Plan 1/3:後端核心 實作計畫

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `modules/iot` 建立 v2 後端核心:設備→多點位資料模型、三種 Modbus 傳輸、合併讀取輪詢引擎、歷史儲存、v1 遷移、v2 讀取 API——產出可經 API 操作、可被測試完整驗證的監控核心。

**Architecture:** 原地演進:v2 資料表與 v1 並存,啟動時冪等遷移;每設備一個輪詢 goroutine,同匯流排共用連線(mutex 序列化);量測除寫入 v2 歷史外,同步餵入既有 v1 `iot_measurements` 續傳佇列,確保客戶系統續傳不中斷。寫入控制、警報、範本為 Plan 2;前端為 Plan 3。

**Tech Stack:** Go 1.24 + Gin + modernc.org/sqlite(WAL 已啟用)、`github.com/simonvetter/modbus`(tcp/rtu/rtuovertcp 三傳輸 + 內建測試 server)、`go.bug.st/serial`(串列埠列舉)。

**規範(全計畫適用):**
- 所有指令在 `apps/backend/` 目錄下執行。
- 對應 spec:`docs/superpowers/specs/2026-06-10-modbus-iot-design.md` §3–§6、§8–§11。
- 每個 commit 前先 `gofmt -w <改過的檔案>`。
- 既有 v1 行為(REST ingest、forwarder、v1 端點)不得改變;`go test ./...` 全程保持綠燈。

---

## 檔案結構(本計畫新增/修改)

| 檔案 | 動作 | 職責 |
|------|------|------|
| `go.mod` / `go.sum` | 修改 | 新增 simonvetter/modbus、go.bug.st/serial |
| `modules/iot/decode.go` | 新增 | 暫存器→數值解碼(純函式) |
| `modules/iot/decode_test.go` | 新增 | 解碼矩陣測試 |
| `modules/iot/types_v2.go` | 新增 | v2 型別與輸入正規化/驗證 |
| `modules/iot/schema_v2.go` | 新增 | v2 資料表 DDL |
| `modules/iot/migrate.go` | 新增 | v1→v2 冪等遷移 |
| `modules/iot/migrate_test.go` | 新增 | 遷移測試 |
| `modules/iot/service_v2.go` | 新增 | 設備/點位 CRUD + 設定 |
| `modules/iot/service_v2_test.go` | 新增 | CRUD/schema 測試 |
| `modules/iot/planner.go` | 新增 | 合併讀取規劃(純函式) |
| `modules/iot/planner_test.go` | 新增 | 規劃測試 |
| `modules/iot/client.go` | 新增 | 連線管理員(共用匯流排連線) |
| `modules/iot/client_test.go` | 新增 | 連線 key/共用 + TCP 煙霧測試 |
| `modules/iot/poller.go` | 新增 | 輪詢引擎 + 離線退避 |
| `modules/iot/poller_test.go` | 新增 | in-process server 整合測試 |
| `modules/iot/history.go` | 新增 | 歷史批次寫入/聚合/清理/查詢 |
| `modules/iot/history_test.go` | 新增 | 歷史測試 |
| `modules/iot/serialports.go` | 新增 | 串列埠列舉 |
| `modules/iot/service.go` | 修改 | `EnsureTables` 呼叫 v2 DDL+遷移;`StartBackgroundLoop` 啟動 v2 goroutines |
| `api/handlers/iot_v2.go` | 新增 | v2 HTTP handlers |
| `api/handlers/iot_v2_test.go` | 新增 | handler 測試 |
| `api/router.go` | 修改 | v2 路由掛載(三層權限群組) |

---

### Task 1: 加入 Modbus 與串列埠依賴

**Files:**
- Modify: `apps/backend/go.mod`, `apps/backend/go.sum`

- [ ] **Step 1: 安裝依賴**

```bash
go get github.com/simonvetter/modbus@latest
go get go.bug.st/serial@latest
```

- [ ] **Step 2: 驗證關鍵 API 簽名**

```bash
go doc github.com/simonvetter/modbus ClientConfiguration
go doc github.com/simonvetter/modbus ModbusClient.ReadRegisters
go doc github.com/simonvetter/modbus ModbusClient.ReadCoils
go doc github.com/simonvetter/modbus NewServer
go doc go.bug.st/serial GetPortsList
```

預期:`ClientConfiguration` 含 `URL string`、`Speed`、`DataBits`、`Parity`、`StopBits`、`Timeout`;`ReadRegisters(addr uint16, quantity uint16, regType RegType) ([]uint16, error)`;`ReadCoils(addr uint16, quantity uint16) ([]bool, error)`。**若欄位型別與後續任務代碼不符(如 Speed 是 uint),以 `go doc` 為準調整型別轉換,其餘邏輯不變。**

- [ ] **Step 3: 確認可建置**

Run: `go build ./...`
Expected: 無錯誤輸出。

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "feat(iot): add simonvetter/modbus and go.bug.st/serial dependencies"
```

---

### Task 2: 解碼器 decode.go(純函式,TDD)

**Files:**
- Create: `apps/backend/modules/iot/decode.go`
- Test: `apps/backend/modules/iot/decode_test.go`

- [ ] **Step 1: 寫失敗測試**

```go
package iot

import (
	"math"
	"testing"
)

func TestQuantityForTypeV2(t *testing.T) {
	cases := map[string]int{
		"bool": 1, "uint16": 1, "int16": 1,
		"uint32": 2, "int32": 2, "float32": 2,
		"uint64": 4, "int64": 4, "float64": 4,
		"unknown": 1,
	}
	for dt, want := range cases {
		if got := quantityForTypeV2(dt); got != want {
			t.Fatalf("quantityForTypeV2(%q) = %d, want %d", dt, got, want)
		}
	}
}

func TestDecodeRegisters(t *testing.T) {
	cases := []struct {
		name      string
		regs      []uint16
		dataType  string
		byteOrder string
		wordOrder string
		want      float64
	}{
		{"uint16", []uint16{0x1234}, "uint16", "AB", "high_first", 4660},
		{"uint16 byte swap", []uint16{0x1234}, "uint16", "BA", "high_first", 13330},
		{"int16 negative", []uint16{0xFFFE}, "int16", "AB", "high_first", -2},
		{"uint32 high_first", []uint16{0x0001, 0x0000}, "uint32", "AB", "high_first", 65536},
		{"uint32 low_first", []uint16{0x0000, 0x0001}, "uint32", "AB", "low_first", 65536},
		{"int32 negative", []uint16{0xFFFF, 0xFFFF}, "int32", "AB", "high_first", -1},
		{"float32", []uint16{0x42F6, 0xE979}, "float32", "AB", "high_first", 123.456},
		{"float32 low_first", []uint16{0xE979, 0x42F6}, "float32", "AB", "low_first", 123.456},
		{"float32 BA low_first", []uint16{0x79E9, 0xF642}, "float32", "BA", "low_first", 123.456},
		{"float64", []uint16{0x405E, 0xDD2F, 0x1A9F, 0xBE77}, "float64", "AB", "high_first", 123.456},
		{"uint64", []uint16{0, 0, 0, 2}, "uint64", "AB", "high_first", 2},
		{"int64 negative", []uint16{0xFFFF, 0xFFFF, 0xFFFF, 0xFFFE}, "int64", "AB", "high_first", -2},
		{"bool true", []uint16{1}, "bool", "AB", "high_first", 1},
		{"bool false", []uint16{0}, "bool", "AB", "high_first", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeRegisters(tc.regs, tc.dataType, tc.byteOrder, tc.wordOrder)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if math.Abs(got-tc.want) > 0.001 {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDecodeRegistersErrors(t *testing.T) {
	if _, err := decodeRegisters([]uint16{1}, "uint32", "AB", "high_first"); err == nil {
		t.Fatal("expected error for short register slice")
	}
	if _, err := decodeRegisters([]uint16{1}, "complex128", "AB", "high_first"); err == nil {
		t.Fatal("expected error for unsupported data type")
	}
}

func TestApplyScale(t *testing.T) {
	if got := applyScale(100, 0.1, -5); math.Abs(got-5) > 1e-9 {
		t.Fatalf("applyScale = %v, want 5", got)
	}
	if got := applyScale(7, 0, 0); got != 7 {
		t.Fatalf("scale 0 should be treated as 1, got %v", got)
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./modules/iot/ -run "TestQuantityForTypeV2|TestDecodeRegisters|TestApplyScale" -v`
Expected: FAIL(`undefined: quantityForTypeV2` 等編譯錯誤)。

- [ ] **Step 3: 實作 decode.go**

注意:v1 `service.go` 已有 `quantityForType`,**不要動它**;v2 用獨立的 `quantityForTypeV2`。

```go
package iot

import (
	"encoding/binary"
	"fmt"
	"math"
)

// v2 register areas.
const (
	AreaCoil     = "coil"
	AreaDiscrete = "discrete"
	AreaInput    = "input"
	AreaHolding  = "holding"
)

// quantityForTypeV2 returns how many 16-bit registers a data type occupies.
func quantityForTypeV2(dataType string) int {
	switch dataType {
	case "uint32", "int32", "float32":
		return 2
	case "uint64", "int64", "float64":
		return 4
	default:
		return 1
	}
}

// decodeRegisters converts raw registers into a numeric value according to
// data type, byte order within each word (AB/BA) and word order across words.
func decodeRegisters(regs []uint16, dataType, byteOrder, wordOrder string) (float64, error) {
	want := quantityForTypeV2(dataType)
	if len(regs) < want {
		return 0, fmt.Errorf("need %d registers for %s, got %d", want, dataType, len(regs))
	}
	words := make([]uint16, want)
	copy(words, regs[:want])
	if byteOrder == "BA" {
		for i, w := range words {
			words[i] = w<<8 | w>>8
		}
	}
	if wordOrder == "low_first" {
		for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
			words[i], words[j] = words[j], words[i]
		}
	}
	buf := make([]byte, len(words)*2)
	for i, w := range words {
		binary.BigEndian.PutUint16(buf[i*2:], w)
	}
	switch dataType {
	case "bool":
		if words[0] != 0 {
			return 1, nil
		}
		return 0, nil
	case "uint16":
		return float64(binary.BigEndian.Uint16(buf)), nil
	case "int16":
		return float64(int16(binary.BigEndian.Uint16(buf))), nil
	case "uint32":
		return float64(binary.BigEndian.Uint32(buf)), nil
	case "int32":
		return float64(int32(binary.BigEndian.Uint32(buf))), nil
	case "float32":
		return float64(math.Float32frombits(binary.BigEndian.Uint32(buf))), nil
	case "uint64":
		return float64(binary.BigEndian.Uint64(buf)), nil
	case "int64":
		return float64(int64(binary.BigEndian.Uint64(buf))), nil
	case "float64":
		return math.Float64frombits(binary.BigEndian.Uint64(buf)), nil
	default:
		return 0, fmt.Errorf("unsupported data type %q", dataType)
	}
}

// applyScale converts a raw value into its engineering value.
func applyScale(raw, scale, offset float64) float64 {
	if scale == 0 {
		scale = 1
	}
	return raw*scale + offset
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./modules/iot/ -run "TestQuantityForTypeV2|TestDecodeRegisters|TestApplyScale" -v`
Expected: PASS(全部子測試)。

- [ ] **Step 5: Commit**

```bash
gofmt -w modules/iot/decode.go modules/iot/decode_test.go
git add modules/iot/decode.go modules/iot/decode_test.go
git commit -m "feat(iot): add v2 register decoder with byte/word order support"
```

---

### Task 3: v2 型別與資料表(TDD)

**Files:**
- Create: `apps/backend/modules/iot/types_v2.go`
- Create: `apps/backend/modules/iot/schema_v2.go`
- Create: `apps/backend/modules/iot/service_v2_test.go`
- Modify: `apps/backend/modules/iot/service.go`(`EnsureTables` 末端)

- [ ] **Step 1: 寫失敗測試**

`service_v2_test.go`(沿用既有 `setupTestService` helper,它已建 `system_config` 並呼叫 `EnsureTables`):

```go
package iot

import "testing"

func TestEnsureTablesV2CreatesTables(t *testing.T) {
	_, db := setupTestService(t)
	for _, table := range []string{"iot_devices_v2", "iot_points", "iot_history_raw", "iot_history_hourly"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./modules/iot/ -run TestEnsureTablesV2CreatesTables -v`
Expected: FAIL(`table iot_devices_v2 missing`)。

- [ ] **Step 3: 實作 types_v2.go**

```go
package iot

import (
	"fmt"
	"strings"
)

type DeviceV2 struct {
	ID                  int     `json:"id"`
	Name                string  `json:"name"`
	Description         string  `json:"description,omitempty"`
	Transport           string  `json:"transport"`
	Host                string  `json:"host,omitempty"`
	Port                int     `json:"port,omitempty"`
	SerialPort          string  `json:"serial_port,omitempty"`
	BaudRate            int     `json:"baud_rate,omitempty"`
	DataBits            int     `json:"data_bits,omitempty"`
	Parity              string  `json:"parity,omitempty"`
	StopBits            int     `json:"stop_bits,omitempty"`
	UnitID              int     `json:"unit_id"`
	TimeoutMs           int     `json:"timeout_ms"`
	InterRequestDelayMs int     `json:"inter_request_delay_ms"`
	PollIntervalSeconds int     `json:"poll_interval_seconds"`
	Enabled             bool    `json:"enabled"`
	TemplateKey         string  `json:"template_key,omitempty"`
	Status              string  `json:"status"`
	LastSeen            string  `json:"last_seen,omitempty"`
	LastError           string  `json:"last_error,omitempty"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	Points              []Point `json:"points,omitempty"`
}

type Point struct {
	ID           int      `json:"id"`
	DeviceID     int      `json:"device_id"`
	Name         string   `json:"name"`
	Metric       string   `json:"metric,omitempty"`
	RegisterArea string   `json:"register_area"`
	Address      int      `json:"address"`
	DataType     string   `json:"data_type"`
	ByteOrder    string   `json:"byte_order"`
	WordOrder    string   `json:"word_order"`
	Scale        float64  `json:"scale"`
	Offset       float64  `json:"offset"`
	Unit         string   `json:"unit,omitempty"`
	Decimals     int      `json:"decimals"`
	Writable     bool     `json:"writable"`
	WriteMin     *float64 `json:"write_min,omitempty"`
	WriteMax     *float64 `json:"write_max,omitempty"`
	Pinned       bool     `json:"pinned"`
	SortOrder    int      `json:"sort_order"`
	Enabled      bool     `json:"enabled"`
	LastValue    *float64 `json:"last_value,omitempty"`
	LastRaw      string   `json:"last_raw,omitempty"`
	LastQuality  string   `json:"last_quality,omitempty"`
	LastError    string   `json:"last_error,omitempty"`
	LastPolledAt string   `json:"last_polled_at,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type UpsertDeviceV2Input struct {
	Name                string             `json:"name"`
	Description         string             `json:"description"`
	Transport           string             `json:"transport"`
	Host                string             `json:"host"`
	Port                int                `json:"port"`
	SerialPort          string             `json:"serial_port"`
	BaudRate            int                `json:"baud_rate"`
	DataBits            int                `json:"data_bits"`
	Parity              string             `json:"parity"`
	StopBits            int                `json:"stop_bits"`
	UnitID              int                `json:"unit_id"`
	TimeoutMs           int                `json:"timeout_ms"`
	InterRequestDelayMs int                `json:"inter_request_delay_ms"`
	PollIntervalSeconds int                `json:"poll_interval_seconds"`
	Enabled             *bool              `json:"enabled"`
	TemplateKey         string             `json:"template_key"`
	Points              []UpsertPointInput `json:"points"`
}

type UpsertPointInput struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Metric       string   `json:"metric"`
	RegisterArea string   `json:"register_area"`
	Address      int      `json:"address"`
	DataType     string   `json:"data_type"`
	ByteOrder    string   `json:"byte_order"`
	WordOrder    string   `json:"word_order"`
	Scale        float64  `json:"scale"`
	Offset       float64  `json:"offset"`
	Unit         string   `json:"unit"`
	Decimals     *int     `json:"decimals"`
	Writable     bool     `json:"writable"`
	WriteMin     *float64 `json:"write_min"`
	WriteMax     *float64 `json:"write_max"`
	Pinned       bool     `json:"pinned"`
	SortOrder    int      `json:"sort_order"`
	Enabled      *bool    `json:"enabled"`
}

func normalizeDeviceV2Input(in UpsertDeviceV2Input) (UpsertDeviceV2Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, fmt.Errorf("device name is required")
	}
	in.Transport = strings.ToLower(strings.TrimSpace(in.Transport))
	switch in.Transport {
	case "tcp", "rtuovertcp":
		if strings.TrimSpace(in.Host) == "" {
			return in, fmt.Errorf("host is required for transport %s", in.Transport)
		}
		if in.Port <= 0 || in.Port > 65535 {
			in.Port = 502
		}
	case "rtu":
		if strings.TrimSpace(in.SerialPort) == "" {
			return in, fmt.Errorf("serial_port is required for transport rtu")
		}
	default:
		return in, fmt.Errorf("unsupported transport %q", in.Transport)
	}
	if in.BaudRate <= 0 {
		in.BaudRate = 9600
	}
	if in.DataBits != 7 && in.DataBits != 8 {
		in.DataBits = 8
	}
	in.Parity = strings.ToUpper(strings.TrimSpace(in.Parity))
	if in.Parity != "E" && in.Parity != "O" {
		in.Parity = "N"
	}
	if in.StopBits != 2 {
		in.StopBits = 1
	}
	if in.UnitID <= 0 || in.UnitID > 247 {
		in.UnitID = 1
	}
	if in.TimeoutMs < 100 || in.TimeoutMs > 60000 {
		in.TimeoutMs = 1000
	}
	if in.InterRequestDelayMs < 0 || in.InterRequestDelayMs > 5000 {
		in.InterRequestDelayMs = 0
	}
	if in.PollIntervalSeconds < 1 || in.PollIntervalSeconds > 86400 {
		in.PollIntervalSeconds = 10
	}
	return in, nil
}

func normalizePointInput(in UpsertPointInput) (UpsertPointInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, fmt.Errorf("point name is required")
	}
	in.RegisterArea = strings.ToLower(strings.TrimSpace(in.RegisterArea))
	switch in.RegisterArea {
	case AreaCoil, AreaDiscrete, AreaInput, AreaHolding:
	default:
		return in, fmt.Errorf("unsupported register_area %q", in.RegisterArea)
	}
	if in.Address < 0 || in.Address > 65535 {
		return in, fmt.Errorf("address out of range")
	}
	in.DataType = strings.ToLower(strings.TrimSpace(in.DataType))
	switch in.DataType {
	case "bool", "uint16", "int16", "uint32", "int32", "float32", "uint64", "int64", "float64":
	case "":
		in.DataType = "uint16"
	default:
		return in, fmt.Errorf("unsupported data_type %q", in.DataType)
	}
	if in.RegisterArea == AreaCoil || in.RegisterArea == AreaDiscrete {
		in.DataType = "bool"
	}
	in.ByteOrder = strings.ToUpper(strings.TrimSpace(in.ByteOrder))
	if in.ByteOrder != "BA" {
		in.ByteOrder = "AB"
	}
	in.WordOrder = strings.ToLower(strings.TrimSpace(in.WordOrder))
	if in.WordOrder != "low_first" {
		in.WordOrder = "high_first"
	}
	if in.Writable && in.RegisterArea != AreaCoil && in.RegisterArea != AreaHolding {
		return in, fmt.Errorf("only coil and holding points can be writable")
	}
	if in.WriteMin != nil && in.WriteMax != nil && *in.WriteMin > *in.WriteMax {
		return in, fmt.Errorf("write_min must be <= write_max")
	}
	return in, nil
}
```

- [ ] **Step 4: 實作 schema_v2.go**

注意:modernc sqlite 預設不啟用 `PRAGMA foreign_keys`,**刪除設備時要在交易內手動刪點位**,不要依賴 CASCADE。

```go
package iot

func (s *Service) ensureTablesV2() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS iot_devices_v2 (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			transport TEXT NOT NULL DEFAULT 'tcp',
			host TEXT DEFAULT '',
			port INTEGER DEFAULT 502,
			serial_port TEXT DEFAULT '',
			baud_rate INTEGER DEFAULT 9600,
			data_bits INTEGER DEFAULT 8,
			parity TEXT DEFAULT 'N',
			stop_bits INTEGER DEFAULT 1,
			unit_id INTEGER DEFAULT 1,
			timeout_ms INTEGER DEFAULT 1000,
			inter_request_delay_ms INTEGER DEFAULT 0,
			poll_interval_seconds INTEGER DEFAULT 10,
			enabled BOOLEAN DEFAULT 1,
			template_key TEXT DEFAULT '',
			status TEXT DEFAULT 'unknown',
			last_seen DATETIME,
			last_error TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS iot_points (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			metric TEXT DEFAULT '',
			register_area TEXT NOT NULL DEFAULT 'holding',
			address INTEGER NOT NULL DEFAULT 0,
			data_type TEXT DEFAULT 'uint16',
			byte_order TEXT DEFAULT 'AB',
			word_order TEXT DEFAULT 'high_first',
			scale REAL DEFAULT 1,
			offset REAL DEFAULT 0,
			unit TEXT DEFAULT '',
			decimals INTEGER DEFAULT 1,
			writable BOOLEAN DEFAULT 0,
			write_min REAL,
			write_max REAL,
			pinned BOOLEAN DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			enabled BOOLEAN DEFAULT 1,
			last_value REAL,
			last_raw TEXT DEFAULT '',
			last_quality TEXT DEFAULT '',
			last_error TEXT DEFAULT '',
			last_polled_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iot_points_device ON iot_points(device_id)`,
		`CREATE TABLE IF NOT EXISTS iot_history_raw (
			point_id INTEGER NOT NULL,
			ts DATETIME NOT NULL,
			value REAL NOT NULL,
			PRIMARY KEY (point_id, ts)
		) WITHOUT ROWID`,
		`CREATE TABLE IF NOT EXISTS iot_history_hourly (
			point_id INTEGER NOT NULL,
			ts_hour DATETIME NOT NULL,
			min REAL NOT NULL,
			max REAL NOT NULL,
			avg REAL NOT NULL,
			count INTEGER NOT NULL,
			PRIMARY KEY (point_id, ts_hour)
		) WITHOUT ROWID`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 5: 接上 EnsureTables**

在 `modules/iot/service.go` 的 `EnsureTables`(line 40 起)函式 **return nil 前**加:

```go
	if err := s.ensureTablesV2(); err != nil {
		return err
	}
```

(遷移呼叫在 Task 4 再加。)

- [ ] **Step 6: 跑測試確認通過**

Run: `go test ./modules/iot/ -run TestEnsureTablesV2CreatesTables -v`
Expected: PASS。再跑 `go test ./modules/iot/ -count=1`,既有測試全綠。

- [ ] **Step 7: Commit**

```bash
gofmt -w modules/iot/types_v2.go modules/iot/schema_v2.go modules/iot/service_v2_test.go modules/iot/service.go
git add modules/iot/types_v2.go modules/iot/schema_v2.go modules/iot/service_v2_test.go modules/iot/service.go
git commit -m "feat(iot): add v2 device/point types and schema"
```

---

### Task 4: v1→v2 遷移(TDD)

**Files:**
- Create: `apps/backend/modules/iot/migrate.go`
- Create: `apps/backend/modules/iot/migrate_test.go`
- Modify: `apps/backend/modules/iot/service.go`(`EnsureTables` 末端)

- [ ] **Step 1: 先確認 v1 欄位名**

Run: `grep -n "ALTER TABLE iot_devices\|CREATE TABLE IF NOT EXISTS iot_devices" modules/iot/service.go`
Expected: 看到 v1 `iot_devices` 的完整欄位(含 `function_code`、`byte_order`、`word_order`、`offset`、`metric`、`poll_interval_seconds`——可能以 ALTER TABLE 形式存在)。下面 SELECT 的欄位若名稱不符,以實際 DDL 為準調整。

- [ ] **Step 2: 寫失敗測試**

`migrate_test.go`:

```go
package iot

import "testing"

func TestMigrateLegacyDevices(t *testing.T) {
	svc, db := setupTestService(t)
	// setupTestService 已跑過 EnsureTables -> 遷移已在空表上執行並寫入標記;
	// 清掉標記讓本測試的遷移真正執行。
	if _, err := db.Exec(`DELETE FROM system_config WHERE config_key = 'iot_v2_migrated'`); err != nil {
		t.Fatalf("clear migration marker: %v", err)
	}
	_, err := db.Exec(`INSERT INTO iot_devices
		(name, protocol, host, port, unit_id, address, quantity, function_code,
		 data_type, byte_order, word_order, scale, offset, metric, poll_interval_seconds, enabled)
		VALUES ('Legacy Meter', 'modbus_tcp', '10.0.0.5', 1502, 3, 100, 2, 4,
		 'float32', 'AB', 'high_first', 0.1, -5, 'temperature', 30, 1)`)
	if err != nil {
		t.Fatalf("insert legacy device: %v", err)
	}
	if err := svc.MigrateLegacyDevices(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	devices, err := svc.ListDevicesV2(false)
	if err != nil {
		t.Fatalf("list v2: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 migrated device, got %d", len(devices))
	}
	d := devices[0]
	if d.Transport != "tcp" || d.Host != "10.0.0.5" || d.Port != 1502 || d.UnitID != 3 ||
		d.PollIntervalSeconds != 30 || !d.Enabled {
		t.Fatalf("device fields wrong: %+v", d)
	}
	points, err := svc.ListPoints(d.ID)
	if err != nil {
		t.Fatalf("list points: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	p := points[0]
	if p.RegisterArea != AreaInput || p.Address != 100 || p.DataType != "float32" ||
		p.Scale != 0.1 || p.Offset != -5 || p.Metric != "temperature" {
		t.Fatalf("point fields wrong: %+v", p)
	}

	// idempotent: second run must not duplicate
	if err := svc.MigrateLegacyDevices(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	devices, _ = svc.ListDevicesV2(false)
	if len(devices) != 1 {
		t.Fatalf("migration not idempotent: %d devices", len(devices))
	}
}

func TestAreaForFunctionCode(t *testing.T) {
	cases := map[int]string{1: AreaCoil, 2: AreaDiscrete, 3: AreaHolding, 4: AreaInput, 99: AreaHolding}
	for fc, want := range cases {
		if got := areaForFunctionCode(fc); got != want {
			t.Fatalf("areaForFunctionCode(%d) = %s, want %s", fc, got, want)
		}
	}
}
```

注意:此測試引用 `ListDevicesV2` / `ListPoints`(Task 5 實作)。**Task 4 與 Task 5 的測試會一起在 Task 5 結束時全綠**;本任務先讓 `MigrateLegacyDevices`、`areaForFunctionCode` 編譯通過,`ListDevicesV2`/`ListPoints` 先以最小可用版本一併放進 Task 5 之前無法編譯——因此實際順序:**先做 Step 3 的 migrate.go,再做 Task 5 的 service_v2.go,最後一起跑兩個任務的測試**。(兩個 commit 分開,測試在第二個 commit 前全部通過。)

- [ ] **Step 3: 實作 migrate.go**

```go
package iot

import "fmt"

const migrationMarkerKey = "iot_v2_migrated"

func areaForFunctionCode(fc int) string {
	switch fc {
	case 1:
		return AreaCoil
	case 2:
		return AreaDiscrete
	case 4:
		return AreaInput
	default:
		return AreaHolding
	}
}

// MigrateLegacyDevices converts v1 single-register modbus devices into v2
// devices with one point each. Idempotent via a system_config marker.
func (s *Service) MigrateLegacyDevices() error {
	if s.getConfig(migrationMarkerKey) == "done" {
		return nil
	}
	rows, err := s.db.Query(`
		SELECT id, name, host, port, unit_id, address, function_code,
		       COALESCE(data_type,'uint16'), COALESCE(byte_order,'AB'), COALESCE(word_order,'high_first'),
		       COALESCE(scale,1), COALESCE(offset,0), COALESCE(metric,''),
		       COALESCE(poll_interval_seconds,60), enabled
		FROM iot_devices
		WHERE protocol LIKE 'modbus%'`)
	if err != nil {
		return fmt.Errorf("query legacy devices: %w", err)
	}
	type legacy struct {
		id, port, unitID, address, fc, pollSecs int
		name, host, dataType, byteOrder, wordOrder, metric string
		scale, offset float64
		enabled bool
	}
	var items []legacy
	for rows.Next() {
		var l legacy
		if err := rows.Scan(&l.id, &l.name, &l.host, &l.port, &l.unitID, &l.address, &l.fc,
			&l.dataType, &l.byteOrder, &l.wordOrder, &l.scale, &l.offset, &l.metric,
			&l.pollSecs, &l.enabled); err != nil {
			rows.Close()
			return fmt.Errorf("scan legacy device: %w", err)
		}
		items = append(items, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, l := range items {
		res, err := tx.Exec(`
			INSERT INTO iot_devices_v2
				(name, transport, host, port, unit_id, poll_interval_seconds, enabled, status)
			VALUES (?, 'tcp', ?, ?, ?, ?, ?, 'unknown')`,
			l.name, l.host, l.port, l.unitID, l.pollSecs, l.enabled)
		if err != nil {
			return fmt.Errorf("insert v2 device: %w", err)
		}
		devID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		dataType := l.dataType
		area := areaForFunctionCode(l.fc)
		if area == AreaCoil || area == AreaDiscrete {
			dataType = "bool"
		}
		if _, err := tx.Exec(`
			INSERT INTO iot_points
				(device_id, name, metric, register_area, address, data_type,
				 byte_order, word_order, scale, offset, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			devID, l.name, l.metric, area, l.address, dataType,
			l.byteOrder, l.wordOrder, l.scale, l.offset); err != nil {
			return fmt.Errorf("insert migrated point: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.setConfig(migrationMarkerKey, "done", "iot v1->v2 migration completed")
}
```

並在 `service.go` 的 `EnsureTables` 中、`ensureTablesV2()` 呼叫之後加:

```go
	if err := s.MigrateLegacyDevices(); err != nil {
		return err
	}
```

- [ ] **Step 4: Commit(暫不跑測試,見 Step 2 說明)**

```bash
gofmt -w modules/iot/migrate.go modules/iot/migrate_test.go modules/iot/service.go
git add modules/iot/migrate.go modules/iot/migrate_test.go modules/iot/service.go
git commit -m "feat(iot): add idempotent v1 to v2 device migration"
```

(此時 `go build ./...` 會因 `ListDevicesV2` 未定義而失敗——Task 5 立刻補上;若你的流程要求每個 commit 可建置,把本 commit 與 Task 5 的 commit 合併成一個。)

---

### Task 5: 設備/點位 CRUD service 層(TDD)

**Files:**
- Create: `apps/backend/modules/iot/service_v2.go`
- Modify: `apps/backend/modules/iot/service_v2_test.go`(追加測試)

- [ ] **Step 1: 追加失敗測試**

在 `service_v2_test.go` 追加:

```go
func boolPtr(b bool) *bool { return &b }

func TestDeviceV2CRUD(t *testing.T) {
	svc, _ := setupTestService(t)
	id, err := svc.CreateDeviceV2(UpsertDeviceV2Input{
		Name: "Meter A", Transport: "tcp", Host: "10.0.0.9", Port: 502, UnitID: 1,
		Points: []UpsertPointInput{
			{Name: "Voltage", RegisterArea: "holding", Address: 0, DataType: "uint16", Metric: "voltage"},
			{Name: "Power", RegisterArea: "holding", Address: 10, DataType: "float32"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	dev, err := svc.GetDeviceV2(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if dev.Name != "Meter A" || len(dev.Points) != 2 {
		t.Fatalf("unexpected device: %+v", dev)
	}

	if err := svc.UpdateDeviceV2(id, UpsertDeviceV2Input{
		Name: "Meter B", Transport: "rtu", SerialPort: "COM3", BaudRate: 19200, UnitID: 5,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	dev, _ = svc.GetDeviceV2(id)
	if dev.Name != "Meter B" || dev.Transport != "rtu" || dev.SerialPort != "COM3" {
		t.Fatalf("update not applied: %+v", dev)
	}
	if len(dev.Points) != 2 {
		t.Fatalf("update must not touch points, got %d", len(dev.Points))
	}

	// bulk point upsert: keep one (update), add one, drop the rest
	kept := dev.Points[0].ID
	if err := svc.UpsertPointsV2(id, []UpsertPointInput{
		{ID: kept, Name: "Voltage L1", RegisterArea: "holding", Address: 0, DataType: "uint16"},
		{Name: "Current", RegisterArea: "input", Address: 6, DataType: "uint32"},
	}); err != nil {
		t.Fatalf("upsert points: %v", err)
	}
	points, _ := svc.ListPoints(id)
	if len(points) != 2 {
		t.Fatalf("expected 2 points after upsert, got %d", len(points))
	}
	if points[0].ID != kept || points[0].Name != "Voltage L1" {
		t.Fatalf("kept point not updated: %+v", points[0])
	}

	if err := svc.DeleteDeviceV2(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if pts, _ := svc.ListPoints(id); len(pts) != 0 {
		t.Fatalf("points must be deleted with device, got %d", len(pts))
	}
}

func TestCreateDeviceV2Validation(t *testing.T) {
	svc, _ := setupTestService(t)
	if _, err := svc.CreateDeviceV2(UpsertDeviceV2Input{Name: "X", Transport: "tcp"}); err == nil {
		t.Fatal("expected error: tcp without host")
	}
	if _, err := svc.CreateDeviceV2(UpsertDeviceV2Input{Name: "X", Transport: "rtu"}); err == nil {
		t.Fatal("expected error: rtu without serial_port")
	}
	if _, err := svc.CreateDeviceV2(UpsertDeviceV2Input{
		Name: "X", Transport: "tcp", Host: "h",
		Points: []UpsertPointInput{{Name: "P", RegisterArea: "input", Address: 0, DataType: "uint16", Writable: true}},
	}); err == nil {
		t.Fatal("expected error: input area cannot be writable")
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./modules/iot/ -run "TestDeviceV2CRUD|TestCreateDeviceV2Validation|TestMigrateLegacyDevices" -v`
Expected: FAIL(編譯錯誤 `undefined: ListDevicesV2` 等)。

- [ ] **Step 3: 實作 service_v2.go**

```go
package iot

import (
	"database/sql"
	"fmt"
)

const deviceV2Columns = `id, name, description, transport, host, port, serial_port,
	baud_rate, data_bits, parity, stop_bits, unit_id, timeout_ms, inter_request_delay_ms,
	poll_interval_seconds, enabled, template_key, status,
	COALESCE(last_seen,''), COALESCE(last_error,''), created_at, updated_at`

func scanDeviceV2(row interface{ Scan(...interface{}) error }) (DeviceV2, error) {
	var d DeviceV2
	err := row.Scan(&d.ID, &d.Name, &d.Description, &d.Transport, &d.Host, &d.Port, &d.SerialPort,
		&d.BaudRate, &d.DataBits, &d.Parity, &d.StopBits, &d.UnitID, &d.TimeoutMs, &d.InterRequestDelayMs,
		&d.PollIntervalSeconds, &d.Enabled, &d.TemplateKey, &d.Status,
		&d.LastSeen, &d.LastError, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (s *Service) ListDevicesV2(includePoints bool) ([]DeviceV2, error) {
	rows, err := s.db.Query(`SELECT ` + deviceV2Columns + ` FROM iot_devices_v2 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceV2
	for rows.Next() {
		d, err := scanDeviceV2(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if includePoints {
		for i := range out {
			pts, err := s.ListPoints(out[i].ID)
			if err != nil {
				return nil, err
			}
			out[i].Points = pts
		}
	}
	return out, nil
}

func (s *Service) GetDeviceV2(id int) (DeviceV2, error) {
	d, err := scanDeviceV2(s.db.QueryRow(`SELECT `+deviceV2Columns+` FROM iot_devices_v2 WHERE id = ?`, id))
	if err != nil {
		return d, err
	}
	d.Points, err = s.ListPoints(id)
	return d, err
}

func (s *Service) CreateDeviceV2(input UpsertDeviceV2Input) (int, error) {
	in, err := normalizeDeviceV2Input(input)
	if err != nil {
		return 0, err
	}
	points := make([]UpsertPointInput, 0, len(in.Points))
	for _, p := range in.Points {
		np, err := normalizePointInput(p)
		if err != nil {
			return 0, err
		}
		points = append(points, np)
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		INSERT INTO iot_devices_v2
			(name, description, transport, host, port, serial_port, baud_rate, data_bits,
			 parity, stop_bits, unit_id, timeout_ms, inter_request_delay_ms,
			 poll_interval_seconds, enabled, template_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.Description, in.Transport, in.Host, in.Port, in.SerialPort, in.BaudRate, in.DataBits,
		in.Parity, in.StopBits, in.UnitID, in.TimeoutMs, in.InterRequestDelayMs,
		in.PollIntervalSeconds, enabled, in.TemplateKey)
	if err != nil {
		return 0, err
	}
	id64, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, p := range points {
		if err := insertPointTx(tx, int(id64), p); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.kickPollers()
	return int(id64), nil
}

func (s *Service) UpdateDeviceV2(id int, input UpsertDeviceV2Input) error {
	in, err := normalizeDeviceV2Input(input)
	if err != nil {
		return err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	res, err := s.db.Exec(`
		UPDATE iot_devices_v2 SET
			name=?, description=?, transport=?, host=?, port=?, serial_port=?, baud_rate=?,
			data_bits=?, parity=?, stop_bits=?, unit_id=?, timeout_ms=?, inter_request_delay_ms=?,
			poll_interval_seconds=?, enabled=?, template_key=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		in.Name, in.Description, in.Transport, in.Host, in.Port, in.SerialPort, in.BaudRate,
		in.DataBits, in.Parity, in.StopBits, in.UnitID, in.TimeoutMs, in.InterRequestDelayMs,
		in.PollIntervalSeconds, enabled, in.TemplateKey, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	s.kickPollers()
	return nil
}

func (s *Service) DeleteDeviceV2(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM iot_points WHERE device_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM iot_devices_v2 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.kickPollers()
	return nil
}

const pointColumns = `id, device_id, name, metric, register_area, address, data_type,
	byte_order, word_order, scale, offset, unit, decimals, writable, write_min, write_max,
	pinned, sort_order, enabled, last_value, COALESCE(last_raw,''), COALESCE(last_quality,''),
	COALESCE(last_error,''), COALESCE(last_polled_at,''), created_at, updated_at`

func scanPoint(row interface{ Scan(...interface{}) error }) (Point, error) {
	var p Point
	err := row.Scan(&p.ID, &p.DeviceID, &p.Name, &p.Metric, &p.RegisterArea, &p.Address, &p.DataType,
		&p.ByteOrder, &p.WordOrder, &p.Scale, &p.Offset, &p.Unit, &p.Decimals, &p.Writable,
		&p.WriteMin, &p.WriteMax, &p.Pinned, &p.SortOrder, &p.Enabled, &p.LastValue,
		&p.LastRaw, &p.LastQuality, &p.LastError, &p.LastPolledAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Service) ListPoints(deviceID int) ([]Point, error) {
	rows, err := s.db.Query(`SELECT `+pointColumns+` FROM iot_points
		WHERE device_id = ? ORDER BY sort_order, address, id`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Point
	for rows.Next() {
		p, err := scanPoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func insertPointTx(tx *sql.Tx, deviceID int, p UpsertPointInput) error {
	decimals := 1
	if p.Decimals != nil {
		decimals = *p.Decimals
	}
	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	scale := p.Scale
	if scale == 0 {
		scale = 1
	}
	_, err := tx.Exec(`
		INSERT INTO iot_points
			(device_id, name, metric, register_area, address, data_type, byte_order, word_order,
			 scale, offset, unit, decimals, writable, write_min, write_max, pinned, sort_order, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deviceID, p.Name, p.Metric, p.RegisterArea, p.Address, p.DataType, p.ByteOrder, p.WordOrder,
		scale, p.Offset, p.Unit, decimals, p.Writable, p.WriteMin, p.WriteMax, p.Pinned, p.SortOrder, enabled)
	return err
}

// UpsertPointsV2 replaces the device's point set: rows with a known id are
// updated, rows without id are inserted, ids not present are deleted.
func (s *Service) UpsertPointsV2(deviceID int, inputs []UpsertPointInput) error {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM iot_devices_v2 WHERE id = ?`, deviceID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return sql.ErrNoRows
	}
	normalized := make([]UpsertPointInput, 0, len(inputs))
	for _, p := range inputs {
		np, err := normalizePointInput(p)
		if err != nil {
			return err
		}
		normalized = append(normalized, np)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	keep := make([]interface{}, 0, len(normalized))
	for _, p := range normalized {
		if p.ID > 0 {
			keep = append(keep, p.ID)
		}
	}
	query := `DELETE FROM iot_points WHERE device_id = ?`
	args := []interface{}{deviceID}
	if len(keep) > 0 {
		query += ` AND id NOT IN (?` + repeatPlaceholders(len(keep)-1) + `)`
		args = append(args, keep...)
	}
	if _, err := tx.Exec(query, args...); err != nil {
		return err
	}
	for _, p := range normalized {
		if p.ID > 0 {
			decimals := 1
			if p.Decimals != nil {
				decimals = *p.Decimals
			}
			enabled := true
			if p.Enabled != nil {
				enabled = *p.Enabled
			}
			scale := p.Scale
			if scale == 0 {
				scale = 1
			}
			if _, err := tx.Exec(`
				UPDATE iot_points SET
					name=?, metric=?, register_area=?, address=?, data_type=?, byte_order=?,
					word_order=?, scale=?, offset=?, unit=?, decimals=?, writable=?,
					write_min=?, write_max=?, pinned=?, sort_order=?, enabled=?, updated_at=CURRENT_TIMESTAMP
				WHERE id=? AND device_id=?`,
				p.Name, p.Metric, p.RegisterArea, p.Address, p.DataType, p.ByteOrder,
				p.WordOrder, scale, p.Offset, p.Unit, decimals, p.Writable,
				p.WriteMin, p.WriteMax, p.Pinned, p.SortOrder, enabled, p.ID, deviceID); err != nil {
				return err
			}
		} else if err := insertPointTx(tx, deviceID, p); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.kickPollers()
	return nil
}

func repeatPlaceholders(n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += ",?"
	}
	return out
}

// kickPollers asks the v2 poll supervisor to reconcile immediately.
// The supervisor is added in the poller task; the buffered channel makes
// this safe to call before the supervisor starts.
func (s *Service) kickPollers() {
	select {
	case s.pollerKick <- struct{}{}:
	default:
	}
}
```

(Task 5 階段 `service_v2.go` 的 import 只需要 `database/sql`——上方 import 區塊的 `fmt` 拿掉;Task 10 追加 `TestConnectionV2` 時再加 `time`。)

並在 `service.go` 的 `Service` struct(line 29 附近)加一個欄位、`NewService` 初始化:

```go
	pollerKick chan struct{}
```

```go
	return &Service{db: db, stopCh: make(chan struct{}), pollerKick: make(chan struct{}, 1)}
```

(`fmtErrNotFound` 若最終沒被引用就刪掉,不留死碼。)

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./modules/iot/ -run "TestDeviceV2CRUD|TestCreateDeviceV2Validation|TestMigrateLegacyDevices|TestAreaForFunctionCode" -v`
Expected: PASS(含 Task 4 的遷移測試)。再跑 `go test ./modules/iot/ -count=1` 全綠、`go build ./...` 成功。

- [ ] **Step 5: Commit**

```bash
gofmt -w modules/iot/service_v2.go modules/iot/service_v2_test.go modules/iot/service.go
git add modules/iot/service_v2.go modules/iot/service_v2_test.go modules/iot/service.go
git commit -m "feat(iot): add v2 device and point CRUD with bulk point upsert"
```

---

### Task 6: 合併讀取規劃 planner.go(純函式,TDD)

**Files:**
- Create: `apps/backend/modules/iot/planner.go`
- Test: `apps/backend/modules/iot/planner_test.go`

- [ ] **Step 1: 寫失敗測試**

```go
package iot

import "testing"

func mkPoint(id int, area string, addr int, dataType string) *Point {
	return &Point{ID: id, RegisterArea: area, Address: addr, DataType: dataType, Enabled: true}
}

func TestPlanReadsMergesContiguous(t *testing.T) {
	points := []*Point{
		mkPoint(1, AreaHolding, 0, "uint16"),
		mkPoint(2, AreaHolding, 1, "uint16"),
		mkPoint(3, AreaHolding, 5, "float32"), // gap 4 <= 8 -> merge, covers 5..6
		mkPoint(4, AreaHolding, 50, "uint16"), // gap 43 > 8 -> new batch
	}
	batches := planReads(points)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d: %+v", len(batches), batches)
	}
	if batches[0].Start != 0 || batches[0].Quantity != 7 {
		t.Fatalf("batch0 = start %d qty %d, want 0/7", batches[0].Start, batches[0].Quantity)
	}
	if len(batches[0].Reads) != 3 {
		t.Fatalf("batch0 reads = %d, want 3", len(batches[0].Reads))
	}
	if batches[0].Reads[2].Offset != 5 || batches[0].Reads[2].Count != 2 {
		t.Fatalf("float32 read offset/count wrong: %+v", batches[0].Reads[2])
	}
	if batches[1].Start != 50 || batches[1].Quantity != 1 {
		t.Fatalf("batch1 = start %d qty %d, want 50/1", batches[1].Start, batches[1].Quantity)
	}
}

func TestPlanReadsSeparatesAreasAndSkipsDisabled(t *testing.T) {
	disabled := mkPoint(9, AreaHolding, 2, "uint16")
	disabled.Enabled = false
	points := []*Point{
		mkPoint(1, AreaCoil, 0, "bool"),
		mkPoint(2, AreaHolding, 0, "uint16"),
		disabled,
	}
	batches := planReads(points)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches (coil + holding), got %d", len(batches))
	}
	for _, b := range batches {
		for _, r := range b.Reads {
			if r.Point.ID == 9 {
				t.Fatal("disabled point must be skipped")
			}
		}
	}
}

func TestPlanReadsRespectsMaxQuantity(t *testing.T) {
	var points []*Point
	for i := 0; i < 70; i++ { // 70 x float64 = 280 words > 120 limit
		points = append(points, mkPoint(i+1, AreaHolding, i*4, "float64"))
	}
	batches := planReads(points)
	for _, b := range batches {
		if b.Quantity > maxRegistersPerRead {
			t.Fatalf("batch quantity %d exceeds limit %d", b.Quantity, maxRegistersPerRead)
		}
	}
	total := 0
	for _, b := range batches {
		total += len(b.Reads)
	}
	if total != 70 {
		t.Fatalf("all points must be covered, got %d/70", total)
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./modules/iot/ -run TestPlanReads -v`
Expected: FAIL(`undefined: planReads`)。

- [ ] **Step 3: 實作 planner.go**

```go
package iot

import "sort"

const (
	maxRegistersPerRead = 120
	maxBitsPerRead      = 1968
	maxMergeGapWords    = 8
)

// pointRead locates one point's data inside a batch read.
type pointRead struct {
	Point  *Point
	Offset int // word (or bit) offset from batch start
	Count  int // words occupied (1 for bit areas)
}

type readBatch struct {
	Area     string
	Start    int
	Quantity int
	Reads    []pointRead
}

// planReads groups enabled points into as few Modbus reads as possible:
// per register area, sorted by address, merged while the address gap stays
// within maxMergeGapWords and the batch stays under protocol limits.
func planReads(points []*Point) []readBatch {
	byArea := map[string][]*Point{}
	for _, p := range points {
		if p == nil || !p.Enabled {
			continue
		}
		byArea[p.RegisterArea] = append(byArea[p.RegisterArea], p)
	}
	var batches []readBatch
	for _, area := range []string{AreaCoil, AreaDiscrete, AreaInput, AreaHolding} {
		pts := byArea[area]
		if len(pts) == 0 {
			continue
		}
		sort.Slice(pts, func(i, j int) bool { return pts[i].Address < pts[j].Address })
		bitArea := area == AreaCoil || area == AreaDiscrete
		maxQty := maxRegistersPerRead
		if bitArea {
			maxQty = maxBitsPerRead
		}
		var cur *readBatch
		for _, p := range pts {
			count := 1
			if !bitArea {
				count = quantityForTypeV2(p.DataType)
			}
			end := p.Address + count
			if cur != nil &&
				p.Address <= cur.Start+cur.Quantity+maxMergeGapWords &&
				end-cur.Start <= maxQty {
				if end-cur.Start > cur.Quantity {
					cur.Quantity = end - cur.Start
				}
				cur.Reads = append(cur.Reads, pointRead{Point: p, Offset: p.Address - cur.Start, Count: count})
				continue
			}
			if cur != nil {
				batches = append(batches, *cur)
			}
			cur = &readBatch{Area: area, Start: p.Address, Quantity: count,
				Reads: []pointRead{{Point: p, Offset: 0, Count: count}}}
		}
		if cur != nil {
			batches = append(batches, *cur)
		}
	}
	return batches
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./modules/iot/ -run TestPlanReads -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
gofmt -w modules/iot/planner.go modules/iot/planner_test.go
git add modules/iot/planner.go modules/iot/planner_test.go
git commit -m "feat(iot): add batched read planner with gap merge and protocol limits"
```

---

### Task 7: 連線管理員 client.go(TDD)

**Files:**
- Create: `apps/backend/modules/iot/client.go`
- Test: `apps/backend/modules/iot/client_test.go`

- [ ] **Step 1: 寫失敗測試**

測試用 simonvetter/modbus **內建 server** 起本機 TCP server。

```go
package iot

import (
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	mb "github.com/simonvetter/modbus"
)

type fixtureHandler struct {
	lock     sync.RWMutex
	holdings [200]uint16
	inputs   [200]uint16
	coils    [200]bool
}

func (h *fixtureHandler) HandleCoils(req *mb.CoilsRequest) ([]bool, error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if int(req.Addr)+int(req.Quantity) > len(h.coils) {
		return nil, mb.ErrIllegalDataAddress
	}
	res := make([]bool, req.Quantity)
	for i := 0; i < int(req.Quantity); i++ {
		if req.IsWrite {
			h.coils[int(req.Addr)+i] = req.Args[i]
		}
		res[i] = h.coils[int(req.Addr)+i]
	}
	return res, nil
}

func (h *fixtureHandler) HandleDiscreteInputs(req *mb.DiscreteInputsRequest) ([]bool, error) {
	return make([]bool, req.Quantity), nil
}

func (h *fixtureHandler) HandleHoldingRegisters(req *mb.HoldingRegistersRequest) ([]uint16, error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if int(req.Addr)+int(req.Quantity) > len(h.holdings) {
		return nil, mb.ErrIllegalDataAddress
	}
	res := make([]uint16, req.Quantity)
	for i := 0; i < int(req.Quantity); i++ {
		if req.IsWrite {
			h.holdings[int(req.Addr)+i] = req.Args[i]
		}
		res[i] = h.holdings[int(req.Addr)+i]
	}
	return res, nil
}

func (h *fixtureHandler) HandleInputRegisters(req *mb.InputRegistersRequest) ([]uint16, error) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if int(req.Addr)+int(req.Quantity) > len(h.inputs) {
		return nil, mb.ErrIllegalDataAddress
	}
	res := make([]uint16, req.Quantity)
	copy(res, h.inputs[req.Addr:int(req.Addr)+int(req.Quantity)])
	return res, nil
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func startFixtureServer(t *testing.T, h *fixtureHandler) (host string, port int) {
	t.Helper()
	port = freePort(t)
	server, err := mb.NewServer(&mb.ServerConfiguration{
		URL:        "tcp://127.0.0.1:" + strconv.Itoa(port),
		Timeout:    10 * time.Second,
		MaxClients: 5,
	}, h)
	if err != nil {
		t.Fatalf("new modbus server: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("start modbus server: %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	return "127.0.0.1", port
}

func TestConnKeySharing(t *testing.T) {
	a := &DeviceV2{Transport: "rtu", SerialPort: "COM3", UnitID: 1}
	b := &DeviceV2{Transport: "rtu", SerialPort: "COM3", UnitID: 5}
	c := &DeviceV2{Transport: "tcp", Host: "10.0.0.1", Port: 502}
	if connKey(a) != connKey(b) {
		t.Fatal("same serial bus must share a key")
	}
	if connKey(a) == connKey(c) {
		t.Fatal("different transports must not share a key")
	}
	m := newConnManager()
	ca := m.get(a)
	cb := m.get(b)
	cc := m.get(c)
	if ca != cb {
		t.Fatal("same bus must return the same shared connection")
	}
	if ca == cc {
		t.Fatal("different bus must return a different connection")
	}
}

func TestConnReadAgainstServer(t *testing.T) {
	h := &fixtureHandler{}
	h.holdings[10] = 0x1234
	host, port := startFixtureServer(t, h)

	d := &DeviceV2{Transport: "tcp", Host: host, Port: port, UnitID: 1, TimeoutMs: 1000}
	m := newConnManager()
	conn := m.get(d)
	regs, err := conn.readRegisters(d, AreaHolding, 10, 1)
	if err != nil {
		t.Fatalf("readRegisters: %v", err)
	}
	if len(regs) != 1 || regs[0] != 0x1234 {
		t.Fatalf("got %v, want [0x1234]", regs)
	}
	bits, err := conn.readBits(d, AreaCoil, 0, 4)
	if err != nil {
		t.Fatalf("readBits: %v", err)
	}
	if len(bits) != 4 {
		t.Fatalf("got %d bits, want 4", len(bits))
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./modules/iot/ -run "TestConnKeySharing|TestConnReadAgainstServer" -v`
Expected: FAIL(`undefined: connKey` 等)。

- [ ] **Step 3: 實作 client.go**

```go
package iot

import (
	"fmt"
	"sync"
	"time"

	mb "github.com/simonvetter/modbus"
)

// sharedConn is one physical bus/socket shared by all devices on it.
// mu serialises all Modbus transactions on the bus (mandatory for RS-485).
type sharedConn struct {
	key    string
	mu     sync.Mutex
	client *mb.ModbusClient
	opened bool
}

type connManager struct {
	mu    sync.Mutex
	conns map[string]*sharedConn
}

func newConnManager() *connManager {
	return &connManager{conns: map[string]*sharedConn{}}
}

func connKey(d *DeviceV2) string {
	switch d.Transport {
	case "rtu":
		return "rtu://" + d.SerialPort
	case "rtuovertcp":
		return fmt.Sprintf("rtuovertcp://%s:%d", d.Host, d.Port)
	default:
		return fmt.Sprintf("tcp://%s:%d", d.Host, d.Port)
	}
}

func clientConfigFor(d *DeviceV2) *mb.ClientConfiguration {
	cfg := &mb.ClientConfiguration{
		URL:     connKey(d),
		Timeout: time.Duration(d.TimeoutMs) * time.Millisecond,
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second
	}
	if d.Transport == "rtu" || d.Transport == "rtuovertcp" {
		cfg.Speed = uint(d.BaudRate)
		cfg.DataBits = uint(d.DataBits)
		cfg.StopBits = uint(d.StopBits)
		switch d.Parity {
		case "E":
			cfg.Parity = mb.PARITY_EVEN
		case "O":
			cfg.Parity = mb.PARITY_ODD
		default:
			cfg.Parity = mb.PARITY_NONE
		}
	}
	return cfg
}

// get returns the shared connection for the device's bus, creating the entry
// if needed. The underlying client is opened lazily on first use.
func (m *connManager) get(d *DeviceV2) *sharedConn {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := connKey(d)
	if c, ok := m.conns[key]; ok {
		return c
	}
	c := &sharedConn{key: key}
	m.conns[key] = c
	return c
}

// drop closes and forgets a connection (used when a device's bus config changes).
func (m *connManager) drop(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[key]; ok {
		c.mu.Lock()
		if c.opened && c.client != nil {
			_ = c.client.Close()
			c.opened = false
		}
		c.mu.Unlock()
		delete(m.conns, key)
	}
}

func (c *sharedConn) ensureOpen(d *DeviceV2) error {
	if c.client == nil {
		client, err := mb.NewClient(clientConfigFor(d))
		if err != nil {
			return err
		}
		c.client = client
	}
	if !c.opened {
		if err := c.client.Open(); err != nil {
			return err
		}
		c.opened = true
	}
	return nil
}

func (c *sharedConn) markBroken() {
	if c.opened && c.client != nil {
		_ = c.client.Close()
	}
	c.opened = false
}

// readRegisters reads input/holding registers for the given device with the
// bus lock held. On transport errors the socket is closed so the next call
// reconnects cleanly.
func (c *sharedConn) readRegisters(d *DeviceV2, area string, start, quantity int) ([]uint16, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureOpen(d); err != nil {
		return nil, err
	}
	if err := c.client.SetUnitId(uint8(d.UnitID)); err != nil {
		return nil, err
	}
	regType := mb.HOLDING_REGISTER
	if area == AreaInput {
		regType = mb.INPUT_REGISTER
	}
	if d.InterRequestDelayMs > 0 {
		time.Sleep(time.Duration(d.InterRequestDelayMs) * time.Millisecond)
	}
	regs, err := c.client.ReadRegisters(uint16(start), uint16(quantity), regType)
	if err != nil {
		c.markBroken()
		return nil, err
	}
	return regs, nil
}

// readBits reads coils/discrete inputs with the bus lock held.
func (c *sharedConn) readBits(d *DeviceV2, area string, start, quantity int) ([]bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureOpen(d); err != nil {
		return nil, err
	}
	if err := c.client.SetUnitId(uint8(d.UnitID)); err != nil {
		return nil, err
	}
	if d.InterRequestDelayMs > 0 {
		time.Sleep(time.Duration(d.InterRequestDelayMs) * time.Millisecond)
	}
	var (
		bits []bool
		err  error
	)
	if area == AreaCoil {
		bits, err = c.client.ReadCoils(uint16(start), uint16(quantity))
	} else {
		bits, err = c.client.ReadDiscreteInputs(uint16(start), uint16(quantity))
	}
	if err != nil {
		c.markBroken()
		return nil, err
	}
	return bits, nil
}
```

注意:若 Step 1 的 `go doc`(Task 1)顯示 `SetUnitId` 不回傳 error 或 `Speed`/`DataBits`/`StopBits` 型別不同,以 `go doc` 為準調整(僅改型別/回傳值處理,不改邏輯)。Modbus 例外錯誤(如 `ErrIllegalDataAddress`)**不是**傳輸錯誤,不應 `markBroken`——以 `errors.Is(err, mb.ErrIllegalDataAddress)` 等協定錯誤清單判斷後直接回傳;為了簡化,初版僅在 `ReadXxx` 回錯時統一 markBroken,整合測試若因此抖動再細分(在 poller 測試中觀察)。

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./modules/iot/ -run "TestConnKeySharing|TestConnReadAgainstServer" -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
gofmt -w modules/iot/client.go modules/iot/client_test.go
git add modules/iot/client.go modules/iot/client_test.go
git commit -m "feat(iot): add shared-bus connection manager for tcp/rtu/rtuovertcp"
```

---

### Task 8: 輪詢引擎 poller.go(TDD,整合測試)

**Files:**
- Create: `apps/backend/modules/iot/poller.go`
- Test: `apps/backend/modules/iot/poller_test.go`
- Modify: `apps/backend/modules/iot/service.go`(struct 欄位 + `StartBackgroundLoop`)

- [ ] **Step 1: 寫失敗測試**

```go
package iot

import (
	"math"
	"testing"
	"time"
)

func TestPollDeviceV2ReadsDecodesAndStores(t *testing.T) {
	svc, db := setupTestService(t)
	h := &fixtureHandler{}
	h.holdings[0] = 0x1234       // uint16 -> 4660
	h.holdings[5] = 0x42F6       // float32 hi word
	h.holdings[6] = 0xE979       // float32 lo word -> 123.456
	h.coils[2] = true
	host, port := startFixtureServer(t, h)

	id, err := svc.CreateDeviceV2(UpsertDeviceV2Input{
		Name: "Sim", Transport: "tcp", Host: host, Port: port, UnitID: 1,
		PollIntervalSeconds: 1,
		Points: []UpsertPointInput{
			{Name: "V", RegisterArea: "holding", Address: 0, DataType: "uint16", Scale: 0.1, Metric: "voltage"},
			{Name: "P", RegisterArea: "holding", Address: 5, DataType: "float32"},
			{Name: "Relay", RegisterArea: "coil", Address: 2, DataType: "bool"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	dev, err := svc.GetDeviceV2(id)
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if err := svc.pollDeviceV2(&dev); err != nil {
		t.Fatalf("pollDeviceV2: %v", err)
	}

	points, _ := svc.ListPoints(id)
	byName := map[string]Point{}
	for _, p := range points {
		byName[p.Name] = p
	}
	if v := byName["V"].LastValue; v == nil || math.Abs(*v-466.0) > 0.001 {
		t.Fatalf("V last_value = %v, want 466 (4660 * 0.1)", v)
	}
	if v := byName["P"].LastValue; v == nil || math.Abs(*v-123.456) > 0.001 {
		t.Fatalf("P last_value = %v, want 123.456", v)
	}
	if v := byName["Relay"].LastValue; v == nil || *v != 1 {
		t.Fatalf("Relay last_value = %v, want 1", v)
	}
	if byName["V"].LastQuality != "good" {
		t.Fatalf("quality = %q, want good", byName["V"].LastQuality)
	}

	// history raw written via synchronous flush helper
	svc.flushHistoryNow()
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM iot_history_raw`).Scan(&n)
	if n != 3 {
		t.Fatalf("history rows = %d, want 3", n)
	}

	// metric point feeds the v1 forward queue
	var qn int
	_ = db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE external_id LIKE 'v2-point-%'`).Scan(&qn)
	if qn != 1 {
		t.Fatalf("forward queue rows = %d, want 1 (only metric point)", qn)
	}
}

func TestPollDeviceV2OfflineBackoff(t *testing.T) {
	svc, _ := setupTestService(t)
	id, err := svc.CreateDeviceV2(UpsertDeviceV2Input{
		Name: "Dead", Transport: "tcp", Host: "127.0.0.1", Port: freePort(t), UnitID: 1,
		TimeoutMs: 200, PollIntervalSeconds: 1,
		Points: []UpsertPointInput{{Name: "X", RegisterArea: "holding", Address: 0, DataType: "uint16"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	dev, _ := svc.GetDeviceV2(id)
	st := &devicePollState{interval: time.Second}
	for i := 0; i < 3; i++ {
		err := svc.pollDeviceV2(&dev)
		if err == nil {
			t.Fatal("expected poll error against closed port")
		}
		st.recordFailure(err)
	}
	if !st.offline() {
		t.Fatalf("3 consecutive failures must mark offline, failures=%d", st.failures)
	}
	if st.nextDelay() <= time.Second {
		t.Fatalf("backoff must exceed base interval, got %v", st.nextDelay())
	}
	st.recordSuccess()
	if st.offline() || st.nextDelay() != time.Second {
		t.Fatal("success must reset state")
	}

	points, _ := svc.ListPoints(id)
	if points[0].LastQuality != "error" {
		t.Fatalf("quality = %q, want error", points[0].LastQuality)
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./modules/iot/ -run "TestPollDeviceV2" -v`
Expected: FAIL(`undefined: pollDeviceV2` 等)。

- [ ] **Step 3: 實作 poller.go**

```go
package iot

import (
	"fmt"
	"time"
)

const (
	offlineFailureThreshold = 3
	maxOfflineBackoff       = 10 * time.Minute
	pollerReconcileInterval = 15 * time.Second
)

// devicePollState tracks failure streaks and backoff for one device.
type devicePollState struct {
	interval time.Duration
	failures int
	delay    time.Duration
}

func (st *devicePollState) recordFailure(err error) {
	st.failures++
	if st.delay == 0 {
		st.delay = st.interval
	}
	if st.failures >= offlineFailureThreshold {
		st.delay *= 2
		if st.delay > maxOfflineBackoff {
			st.delay = maxOfflineBackoff
		}
	}
}

func (st *devicePollState) recordSuccess() {
	st.failures = 0
	st.delay = 0
}

func (st *devicePollState) offline() bool {
	return st.failures >= offlineFailureThreshold
}

func (st *devicePollState) nextDelay() time.Duration {
	if st.delay > st.interval {
		return st.delay
	}
	return st.interval
}

type deviceRunner struct {
	stop      chan struct{}
	updatedAt string
}

// superviseV2Pollers reconciles one poll goroutine per enabled v2 device.
func (s *Service) superviseV2Pollers() {
	runners := map[int]*deviceRunner{}
	ticker := time.NewTicker(pollerReconcileInterval)
	defer ticker.Stop()
	reconcile := func() {
		devices, err := s.ListDevicesV2(false)
		if err != nil {
			return
		}
		seen := map[int]bool{}
		for _, d := range devices {
			seen[d.ID] = true
			r, ok := runners[d.ID]
			if ok && (!d.Enabled || r.updatedAt != d.UpdatedAt) {
				close(r.stop)
				delete(runners, d.ID)
				ok = false
			}
			if !ok && d.Enabled {
				nr := &deviceRunner{stop: make(chan struct{}), updatedAt: d.UpdatedAt}
				runners[d.ID] = nr
				dev := d
				go s.runDevicePoller(dev, nr.stop)
			}
		}
		for id, r := range runners {
			if !seen[id] {
				close(r.stop)
				delete(runners, id)
			}
		}
	}
	reconcile()
	for {
		select {
		case <-s.stopCh:
			for _, r := range runners {
				close(r.stop)
			}
			return
		case <-ticker.C:
			reconcile()
		case <-s.pollerKick:
			reconcile()
		}
	}
}

func (s *Service) runDevicePoller(d DeviceV2, stop chan struct{}) {
	st := &devicePollState{interval: time.Duration(d.PollIntervalSeconds) * time.Second}
	if st.interval < time.Second {
		st.interval = time.Second
	}
	for {
		select {
		case <-stop:
			return
		case <-time.After(st.nextDelay()):
		}
		err := s.pollDeviceV2(&d)
		if err != nil {
			st.recordFailure(err)
			status := "error"
			if st.offline() {
				status = "offline"
			}
			s.setDeviceStatusV2(d.ID, status, err)
		} else {
			st.recordSuccess()
			s.setDeviceStatusV2(d.ID, "online", nil)
		}
	}
}

// pollDeviceV2 performs one poll cycle: plan batches, read, decode, store.
// Returns an error only when every batch failed (treated as device-level failure).
func (s *Service) pollDeviceV2(d *DeviceV2) error {
	points, err := s.ListPoints(d.ID)
	if err != nil {
		return err
	}
	ptrs := make([]*Point, len(points))
	for i := range points {
		ptrs[i] = &points[i]
	}
	batches := planReads(ptrs)
	if len(batches) == 0 {
		return nil
	}
	conn := s.connsV2().get(d)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	failed := 0
	var lastErr error
	for _, b := range batches {
		var regs []uint16
		var bits []bool
		var readErr error
		if b.Area == AreaCoil || b.Area == AreaDiscrete {
			bits, readErr = conn.readBits(d, b.Area, b.Start, b.Quantity)
		} else {
			regs, readErr = conn.readRegisters(d, b.Area, b.Start, b.Quantity)
		}
		if readErr != nil {
			failed++
			lastErr = readErr
			s.markPointsError(b, now, readErr)
			continue
		}
		for _, r := range b.Reads {
			var raw float64
			var rawText string
			if bits != nil {
				v := bits[r.Offset]
				raw = 0
				if v {
					raw = 1
				}
				rawText = fmt.Sprintf("%v", v)
			} else {
				slice := regs[r.Offset : r.Offset+r.Count]
				decoded, decErr := decodeRegisters(slice, r.Point.DataType, r.Point.ByteOrder, r.Point.WordOrder)
				if decErr != nil {
					s.updatePointError(r.Point.ID, now, decErr)
					continue
				}
				raw = decoded
				rawText = fmt.Sprintf("%v", slice)
			}
			value := applyScale(raw, r.Point.Scale, r.Point.Offset)
			s.updatePointValue(r.Point.ID, value, rawText, now)
			s.enqueueHistory(historySample{PointID: r.Point.ID, TS: time.Now().UTC(), Value: value})
			if r.Point.Metric != "" {
				externalID := fmt.Sprintf("v2-point-%d", r.Point.ID)
				_, _ = s.recordMeasurement(nil, externalID, r.Point.Metric, value, rawText)
			}
		}
	}
	if failed == len(batches) {
		return fmt.Errorf("all %d reads failed: %w", failed, lastErr)
	}
	return nil
}

func (s *Service) markPointsError(b readBatch, now string, err error) {
	for _, r := range b.Reads {
		s.updatePointError(r.Point.ID, now, err)
	}
}

func (s *Service) updatePointValue(pointID int, value float64, rawText, now string) {
	_, _ = s.db.Exec(`UPDATE iot_points SET last_value=?, last_raw=?, last_quality='good',
		last_error='', last_polled_at=? WHERE id=?`, value, rawText, now, pointID)
}

func (s *Service) updatePointError(pointID int, now string, err error) {
	_, _ = s.db.Exec(`UPDATE iot_points SET last_quality='error', last_error=?, last_polled_at=?
		WHERE id=?`, truncateError(err), now, pointID)
}

func (s *Service) setDeviceStatusV2(id int, status string, cause error) {
	msg := ""
	if cause != nil {
		msg = truncateError(cause)
	}
	if status == "online" {
		_, _ = s.db.Exec(`UPDATE iot_devices_v2 SET status=?, last_seen=CURRENT_TIMESTAMP, last_error=''
			WHERE id=?`, status, id)
		return
	}
	_, _ = s.db.Exec(`UPDATE iot_devices_v2 SET status=?, last_error=? WHERE id=?`, status, msg, id)
}
```

`connsV2()` 是 `Service` 上的 lazy 連線管理員;在 `service.go` struct 加欄位:

```go
	connMgr     *connManager
	connMgrOnce sync.Once
```

並在 `poller.go` 加:

```go
func (s *Service) connsV2() *connManager {
	s.connMgrOnce.Do(func() { s.connMgr = newConnManager() })
	return s.connMgr
}
```

`StartBackgroundLoop`(service.go line 128)改為:

```go
func (s *Service) StartBackgroundLoop() {
	s.loopOnce.Do(func() {
		go s.backgroundLoop()
		go s.superviseV2Pollers()
		go s.runHistoryWriter()
		go s.runHistoryMaintenance()
	})
}
```

(`runHistoryWriter` / `runHistoryMaintenance` / `enqueueHistory` / `flushHistoryNow` 在 Task 9 實作;為讓本任務可編譯,Task 8 與 Task 9 **一起寫完後再跑測試與 commit**,或先在 history.go 放 Task 9 的完整實作——建議直接先做 Task 9 Step 3 再回來跑本任務測試。)

- [ ] **Step 4: 先完成 Task 9 Step 1–3,回來跑兩個任務的測試**

Run: `go test ./modules/iot/ -count=1 -v -run "TestPollDeviceV2|TestHistory"`
Expected: PASS。

- [ ] **Step 5: Commit(poller 部分)**

```bash
gofmt -w modules/iot/poller.go modules/iot/poller_test.go modules/iot/service.go
git add modules/iot/poller.go modules/iot/poller_test.go modules/iot/service.go
git commit -m "feat(iot): add v2 polling engine with shared-bus reads and offline backoff"
```

---

### Task 9: 歷史儲存 history.go(TDD)

**Files:**
- Create: `apps/backend/modules/iot/history.go`
- Test: `apps/backend/modules/iot/history_test.go`

- [ ] **Step 1: 寫失敗測試**

```go
package iot

import (
	"testing"
	"time"
)

func TestHistoryFlushAggregateCleanupQuery(t *testing.T) {
	svc, db := setupTestService(t)

	// 用相對時間(上一個整點)避免測試隨日曆翻紅;樣本全部落在同一小時內。
	base := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	for i := 0; i < 10; i++ {
		svc.enqueueHistory(historySample{PointID: 1, TS: base.Add(time.Duration(i) * time.Minute), Value: float64(i)})
	}
	svc.flushHistoryNow()

	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM iot_history_raw WHERE point_id=1`).Scan(&n)
	if n != 10 {
		t.Fatalf("raw rows = %d, want 10", n)
	}

	if err := svc.aggregateHourly(); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	var minV, maxV, avgV float64
	var cnt int
	hourKey := base.Format("2006-01-02 15:04:05") // base 已截到整點,即 HH:00:00
	err := db.QueryRow(`SELECT min, max, avg, count FROM iot_history_hourly
		WHERE point_id=1 AND ts_hour=?`, hourKey).Scan(&minV, &maxV, &avgV, &cnt)
	if err != nil {
		t.Fatalf("hourly row missing: %v", err)
	}
	if minV != 0 || maxV != 9 || cnt != 10 || avgV != 4.5 {
		t.Fatalf("hourly agg wrong: min=%v max=%v avg=%v count=%d", minV, maxV, avgV, cnt)
	}

	// aggregation is idempotent
	if err := svc.aggregateHourly(); err != nil {
		t.Fatalf("aggregate twice: %v", err)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM iot_history_hourly WHERE point_id=1`).Scan(&n)
	if n != 1 {
		t.Fatalf("hourly rows = %d, want 1", n)
	}

	// query picks raw for short ranges
	res, err := svc.QueryHistory([]int{1}, base.Add(-time.Hour), base.Add(time.Hour), 1000)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.Source != "raw" || len(res.Series) != 1 || len(res.Series[0].Samples) != 10 {
		t.Fatalf("raw query wrong: source=%s series=%d", res.Source, len(res.Series))
	}

	// cleanup respects retention
	if err := svc.cleanupHistory(0, 0); err != nil { // 0 days keeps nothing
		t.Fatalf("cleanup: %v", err)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM iot_history_raw`).Scan(&n)
	if n != 0 {
		t.Fatalf("raw rows after cleanup = %d, want 0", n)
	}
}

func TestQueryHistoryFallsBackToHourly(t *testing.T) {
	svc, db := setupTestService(t)
	seedHour := time.Now().UTC().AddDate(0, 0, -60).Truncate(time.Hour)
	_, err := db.Exec(`INSERT INTO iot_history_hourly (point_id, ts_hour, min, max, avg, count)
		VALUES (1, ?, 1, 5, 3, 60)`, seedHour.Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("seed hourly: %v", err)
	}
	from := time.Now().UTC().AddDate(0, 0, -70) // 70 天前,遠超 raw 保留期 -> 走 hourly
	to := time.Now().UTC().AddDate(0, 0, -50)
	res, err := svc.QueryHistory([]int{1}, from, to, 1000)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.Source != "hourly" || len(res.Series[0].Samples) != 1 {
		t.Fatalf("hourly fallback wrong: %+v", res)
	}
	s0 := res.Series[0].Samples[0]
	if s0.Min == nil || *s0.Min != 1 || s0.Max == nil || *s0.Max != 5 {
		t.Fatalf("min/max band missing: %+v", s0)
	}
}

func TestHistoryDecimation(t *testing.T) {
	samples := make([]HistorySample, 1000)
	for i := range samples {
		samples[i] = HistorySample{Value: float64(i)}
	}
	out := decimateSamples(samples, 100)
	if len(out) > 100 {
		t.Fatalf("decimated to %d, want <= 100", len(out))
	}
	if out[0].Value != 0 || out[len(out)-1].Value != 999 {
		t.Fatal("decimation must keep first and last samples")
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./modules/iot/ -run "TestHistory|TestQueryHistory" -v`
Expected: FAIL(`undefined: historySample` 等)。

- [ ] **Step 3: 實作 history.go**

```go
package iot

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	historyFlushInterval   = 2 * time.Second
	historyFlushBatchSize  = 500
	defaultRawRetentionDay = 7
	defaultHourlyRetention = 365
	sqliteTimeLayout       = "2006-01-02 15:04:05"
)

type historySample struct {
	PointID int
	TS      time.Time
	Value   float64
}

// HistorySample is one output sample; Min/Max present for hourly data.
type HistorySample struct {
	TS    string   `json:"ts"`
	Value float64  `json:"value"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
}

type HistorySeries struct {
	PointID int             `json:"point_id"`
	Samples []HistorySample `json:"samples"`
}

type HistoryResult struct {
	Source string          `json:"source"` // raw | hourly
	Series []HistorySeries `json:"series"`
}

// enqueueHistory buffers a sample for the background writer. The channel is
// lazily created so unit tests can use the service without StartBackgroundLoop.
func (s *Service) enqueueHistory(sample historySample) {
	s.historyOnce.Do(func() { s.historyCh = make(chan historySample, 4096) })
	select {
	case s.historyCh <- sample:
	default: // drop on overload rather than blocking the poller
	}
}

func (s *Service) runHistoryWriter() {
	s.historyOnce.Do(func() { s.historyCh = make(chan historySample, 4096) })
	ticker := time.NewTicker(historyFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			s.flushHistoryNow()
			return
		case <-ticker.C:
			s.flushHistoryNow()
		}
	}
}

// flushHistoryNow drains the buffer synchronously (also used by tests).
func (s *Service) flushHistoryNow() {
	s.historyOnce.Do(func() { s.historyCh = make(chan historySample, 4096) })
	var batch []historySample
	for {
		select {
		case smp := <-s.historyCh:
			batch = append(batch, smp)
			if len(batch) >= historyFlushBatchSize {
				_ = s.writeHistoryBatch(batch)
				batch = batch[:0]
			}
		default:
			if len(batch) > 0 {
				_ = s.writeHistoryBatch(batch)
			}
			return
		}
	}
}

func (s *Service) writeHistoryBatch(batch []historySample) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO iot_history_raw (point_id, ts, value) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, smp := range batch {
		if _, err := stmt.Exec(smp.PointID, smp.TS.UTC().Format(sqliteTimeLayout), smp.Value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// aggregateHourly recomputes hourly min/max/avg for every hour that still has
// raw data (idempotent: full recompute per hour, REPLACE semantics).
func (s *Service) aggregateHourly() error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO iot_history_hourly (point_id, ts_hour, min, max, avg, count)
		SELECT point_id, strftime('%Y-%m-%d %H:00:00', ts), MIN(value), MAX(value), AVG(value), COUNT(*)
		FROM iot_history_raw
		GROUP BY point_id, strftime('%Y-%m-%d %H:00:00', ts)`)
	return err
}

// cleanupHistory removes raw/hourly rows older than the retention windows (days).
func (s *Service) cleanupHistory(rawDays, hourlyDays int) error {
	rawCut := time.Now().UTC().AddDate(0, 0, -rawDays).Format(sqliteTimeLayout)
	hourlyCut := time.Now().UTC().AddDate(0, 0, -hourlyDays).Format(sqliteTimeLayout)
	if _, err := s.db.Exec(`DELETE FROM iot_history_raw WHERE ts < ?`, rawCut); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM iot_history_hourly WHERE ts_hour < ?`, hourlyCut)
	return err
}

func (s *Service) historyRetentionDays() (rawDays, hourlyDays int) {
	rawDays = defaultRawRetentionDay
	hourlyDays = defaultHourlyRetention
	if v, err := strconv.Atoi(s.getConfig("iot_v2_raw_retention_days")); err == nil && v >= 1 && v <= 30 {
		rawDays = v
	}
	if v, err := strconv.Atoi(s.getConfig("iot_v2_hourly_retention_days")); err == nil && v >= 30 && v <= 1095 {
		hourlyDays = v
	}
	return
}

func (s *Service) runHistoryMaintenance() {
	aggTicker := time.NewTicker(time.Hour)
	cleanTicker := time.NewTicker(24 * time.Hour)
	defer aggTicker.Stop()
	defer cleanTicker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-aggTicker.C:
			_ = s.aggregateHourly()
		case <-cleanTicker.C:
			raw, hourly := s.historyRetentionDays()
			_ = s.cleanupHistory(raw, hourly)
		}
	}
}

// QueryHistory returns series for the given points. Ranges fully inside the
// raw retention window use raw samples; longer ranges use hourly aggregates.
func (s *Service) QueryHistory(pointIDs []int, from, to time.Time, maxPoints int) (HistoryResult, error) {
	if maxPoints <= 0 || maxPoints > 5000 {
		maxPoints = 800
	}
	rawDays, _ := s.historyRetentionDays()
	useRaw := time.Since(from) <= time.Duration(rawDays)*24*time.Hour
	res := HistoryResult{Source: "hourly"}
	if useRaw {
		res.Source = "raw"
	}
	for _, pid := range pointIDs {
		series := HistorySeries{PointID: pid, Samples: []HistorySample{}}
		var query string
		if useRaw {
			query = `SELECT ts, value, NULL, NULL FROM iot_history_raw
				WHERE point_id=? AND ts>=? AND ts<=? ORDER BY ts`
		} else {
			query = `SELECT ts_hour, avg, min, max FROM iot_history_hourly
				WHERE point_id=? AND ts_hour>=? AND ts_hour<=? ORDER BY ts_hour`
		}
		rows, err := s.db.Query(query, pid,
			from.UTC().Format(sqliteTimeLayout), to.UTC().Format(sqliteTimeLayout))
		if err != nil {
			return res, err
		}
		for rows.Next() {
			var smp HistorySample
			if err := rows.Scan(&smp.TS, &smp.Value, &smp.Min, &smp.Max); err != nil {
				rows.Close()
				return res, err
			}
			series.Samples = append(series.Samples, smp)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return res, err
		}
		series.Samples = decimateSamples(series.Samples, maxPoints)
		res.Series = append(res.Series, series)
	}
	return res, nil
}

// decimateSamples reduces a series by stride sampling, always keeping the
// first and last samples.
func decimateSamples(samples []HistorySample, maxPoints int) []HistorySample {
	if len(samples) <= maxPoints || maxPoints < 2 {
		return samples
	}
	stride := float64(len(samples)-1) / float64(maxPoints-1)
	out := make([]HistorySample, 0, maxPoints)
	for i := 0; i < maxPoints-1; i++ {
		out = append(out, samples[int(float64(i)*stride)])
	}
	return append(out, samples[len(samples)-1])
}

// exportHistoryCSV renders a simple CSV for the same query.
func (s *Service) ExportHistoryCSV(pointIDs []int, from, to time.Time) (string, error) {
	res, err := s.QueryHistory(pointIDs, from, to, 5000)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("point_id,ts,value,min,max\n")
	for _, series := range res.Series {
		for _, smp := range series.Samples {
			minStr, maxStr := "", ""
			if smp.Min != nil {
				minStr = fmt.Sprintf("%g", *smp.Min)
			}
			if smp.Max != nil {
				maxStr = fmt.Sprintf("%g", *smp.Max)
			}
			fmt.Fprintf(&b, "%d,%s,%g,%s,%s\n", series.PointID, smp.TS, smp.Value, minStr, maxStr)
		}
	}
	return b.String(), nil
}
```

`service.go` 的 `Service` struct 加:

```go
	historyOnce sync.Once
	historyCh   chan historySample
```

- [ ] **Step 4: 跑測試確認通過(連同 Task 8)**

Run: `go test ./modules/iot/ -count=1`
Expected: PASS(全部,含既有 v1 測試)。

- [ ] **Step 5: Commit**

```bash
gofmt -w modules/iot/history.go modules/iot/history_test.go modules/iot/service.go
git add modules/iot/history.go modules/iot/history_test.go modules/iot/service.go
git commit -m "feat(iot): add history storage with hourly aggregation, retention and query"
```

---

### Task 10: 串列埠列舉 + v2 HTTP API + 路由(TDD)

**Files:**
- Create: `apps/backend/modules/iot/serialports.go`
- Create: `apps/backend/api/handlers/iot_v2.go`
- Create: `apps/backend/api/handlers/iot_v2_test.go`
- Modify: `apps/backend/api/router.go`

- [ ] **Step 1: 實作 serialports.go(無單元測試——硬體相依,由 handler 測試覆蓋空清單情形)**

```go
package iot

import "go.bug.st/serial"

// ListSerialPorts enumerates host serial ports (COMx on Windows, /dev/tty* on Linux).
func ListSerialPorts() []string {
	ports, err := serial.GetPortsList()
	if err != nil || ports == nil {
		return []string{}
	}
	return ports
}
```

- [ ] **Step 2: 寫失敗 handler 測試**

`api/handlers/iot_v2_test.go`(參考既有 `integration_test.go` 的 Handler 組裝方式——先看該檔 `iot: iotmodule.NewService(db)` 那段的 setup helper,沿用同一個 helper;若該 helper 不可重用,使用下面的最小組裝):

```go
package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	iotmodule "management-server/modules/iot"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func setupIoTV2Handler(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE system_config (
		config_key TEXT PRIMARY KEY, config_value TEXT, description TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create system_config: %v", err)
	}
	svc := iotmodule.NewService(db)
	if err := svc.EnsureTables(); err != nil {
		t.Fatalf("ensure tables: %v", err)
	}
	h := &Handler{iot: svc}
	r := gin.New()
	r.GET("/iot/v2/devices", h.ListIoTDevicesV2)
	r.POST("/iot/v2/devices", h.CreateIoTDeviceV2)
	r.GET("/iot/v2/overview", h.GetIoTOverview)
	r.GET("/iot/v2/serial-ports", h.GetIoTSerialPorts)
	return h, r
}

func TestIoTV2CreateAndListDevices(t *testing.T) {
	_, r := setupIoTV2Handler(t)
	body, _ := json.Marshal(map[string]interface{}{
		"name": "Meter", "transport": "tcp", "host": "10.0.0.2", "port": 502,
		"points": []map[string]interface{}{
			{"name": "V", "register_area": "holding", "address": 0, "data_type": "uint16"},
		},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iot/v2/devices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/iot/v2/devices?include=points", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("list status = %d", w2.Code)
	}
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Name   string `json:"name"`
			Points []struct {
				Name string `json:"name"`
			} `json:"points"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success || len(resp.Data) != 1 || len(resp.Data[0].Points) != 1 {
		t.Fatalf("unexpected list payload: %s", w2.Body.String())
	}
}

func TestIoTV2CreateValidationError(t *testing.T) {
	_, r := setupIoTV2Handler(t)
	body, _ := json.Marshal(map[string]interface{}{"name": "X", "transport": "tcp"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iot/v2/devices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestIoTV2OverviewAndSerialPorts(t *testing.T) {
	_, r := setupIoTV2Handler(t)
	for _, path := range []string{"/iot/v2/overview", "/iot/v2/serial-ports"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, w.Code)
		}
	}
}
```

- [ ] **Step 3: 跑測試確認失敗**

Run: `go test ./api/handlers/ -run TestIoTV2 -v`
Expected: FAIL(`undefined: h.ListIoTDevicesV2` 等)。

- [ ] **Step 4: 實作 iot_v2.go**

```go
package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	iotmodule "management-server/modules/iot"

	"github.com/gin-gonic/gin"
)

func parseIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid id"})
		return 0, false
	}
	return id, true
}

func (h *Handler) GetIoTOverview(c *gin.Context) {
	overview, err := h.iot.OverviewV2()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: overview})
}

func (h *Handler) ListIoTDevicesV2(c *gin.Context) {
	includePoints := strings.Contains(c.Query("include"), "points")
	devices, err := h.iot.ListDevicesV2(includePoints)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if devices == nil {
		devices = []iotmodule.DeviceV2{}
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: devices})
}

func (h *Handler) GetIoTDeviceV2(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	device, err := h.iot.GetDeviceV2(id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "device not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: device})
}

func (h *Handler) CreateIoTDeviceV2(c *gin.Context) {
	var input iotmodule.UpsertDeviceV2Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	id, err := h.iot.CreateDeviceV2(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"id": id}})
}

func (h *Handler) UpdateIoTDeviceV2(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var input iotmodule.UpsertDeviceV2Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	err := h.iot.UpdateDeviceV2(id, input)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "device not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) DeleteIoTDeviceV2(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	err := h.iot.DeleteDeviceV2(id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "device not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) PutIoTDevicePoints(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var inputs []iotmodule.UpsertPointInput
	if err := c.ShouldBindJSON(&inputs); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	err := h.iot.UpsertPointsV2(id, inputs)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "device not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) PollIoTDeviceV2(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	result, err := h.iot.PollDeviceV2Now(id)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "device not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: result})
}

func (h *Handler) TestIoTDeviceConnection(c *gin.Context) {
	var input iotmodule.UpsertDeviceV2Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	result, err := h.iot.TestConnectionV2(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: result})
}

func (h *Handler) GetIoTSerialPorts(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Success: true, Data: iotmodule.ListSerialPorts()})
}

func parseHistoryQuery(c *gin.Context) (ids []int, from, to time.Time, ok bool) {
	for _, part := range strings.Split(c.Query("point_ids"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid point_ids"})
			return nil, from, to, false
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "point_ids required"})
		return nil, from, to, false
	}
	var err error
	to = time.Now().UTC()
	from = to.Add(-24 * time.Hour)
	if v := c.Query("from"); v != "" {
		if from, err = time.Parse(time.RFC3339, v); err != nil {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid from"})
			return nil, from, to, false
		}
	}
	if v := c.Query("to"); v != "" {
		if to, err = time.Parse(time.RFC3339, v); err != nil {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid to"})
			return nil, from, to, false
		}
	}
	return ids, from, to, true
}

func (h *Handler) GetIoTHistory(c *gin.Context) {
	ids, from, to, ok := parseHistoryQuery(c)
	if !ok {
		return
	}
	maxPoints, _ := strconv.Atoi(c.DefaultQuery("max_points", "800"))
	res, err := h.iot.QueryHistory(ids, from, to, maxPoints)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: res})
}

func (h *Handler) GetIoTHistoryCSV(c *gin.Context) {
	ids, from, to, ok := parseHistoryQuery(c)
	if !ok {
		return
	}
	csv, err := h.iot.ExportHistoryCSV(ids, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=iot-history.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte(csv))
}
```

還需要在 `modules/iot/service_v2.go` 補三個 service 方法(handler 引用):

```go
// OverviewV2 aggregates dashboard numbers plus pinned point snapshots.
type OverviewV2Data struct {
	DeviceCount  int     `json:"device_count"`
	OnlineCount  int     `json:"online_count"`
	PointCount   int     `json:"point_count"`
	PinnedPoints []Point `json:"pinned_points"`
	Queue        QueueStatus `json:"queue"`
}

func (s *Service) OverviewV2() (OverviewV2Data, error) {
	var out OverviewV2Data
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM iot_devices_v2`).Scan(&out.DeviceCount); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM iot_devices_v2 WHERE status='online'`).Scan(&out.OnlineCount); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM iot_points`).Scan(&out.PointCount); err != nil {
		return out, err
	}
	rows, err := s.db.Query(`SELECT ` + pointColumns + ` FROM iot_points WHERE pinned=1 ORDER BY sort_order, id`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.PinnedPoints = []Point{}
	for rows.Next() {
		p, err := scanPoint(rows)
		if err != nil {
			return out, err
		}
		out.PinnedPoints = append(out.PinnedPoints, p)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if q, err := s.QueueStatus(); err == nil {
		out.Queue = q
	}
	return out, nil
}

// PollDeviceV2Now runs one poll cycle on demand and returns fresh points.
func (s *Service) PollDeviceV2Now(id int) ([]Point, error) {
	dev, err := s.GetDeviceV2(id)
	if err != nil {
		return nil, err
	}
	if err := s.pollDeviceV2(&dev); err != nil {
		s.setDeviceStatusV2(dev.ID, "error", err)
		return nil, err
	}
	s.setDeviceStatusV2(dev.ID, "online", nil)
	return s.ListPoints(id)
}

// TestConnectionV2 opens a throwaway connection and optionally reads one
// register, reporting round-trip latency. Nothing is persisted.
type TestConnectionResult struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms"`
	Message   string `json:"message,omitempty"`
}

func (s *Service) TestConnectionV2(input UpsertDeviceV2Input) (TestConnectionResult, error) {
	in, err := normalizeDeviceV2Input(input)
	if err != nil {
		return TestConnectionResult{}, err
	}
	enabled := true
	d := DeviceV2{
		Name: in.Name, Transport: in.Transport, Host: in.Host, Port: in.Port,
		SerialPort: in.SerialPort, BaudRate: in.BaudRate, DataBits: in.DataBits,
		Parity: in.Parity, StopBits: in.StopBits, UnitID: in.UnitID,
		TimeoutMs: in.TimeoutMs, Enabled: enabled,
	}
	conn := &sharedConn{key: connKey(&d)}
	start := time.Now()
	_, readErr := conn.readRegisters(&d, AreaHolding, 0, 1)
	latency := time.Since(start).Milliseconds()
	conn.mu.Lock()
	conn.markBroken()
	conn.mu.Unlock()
	if readErr != nil {
		return TestConnectionResult{OK: false, LatencyMs: latency, Message: truncateError(readErr)}, nil
	}
	return TestConnectionResult{OK: true, LatencyMs: latency}, nil
}
```

(`service_v2.go` 需要 `import "time"`。)

- [ ] **Step 5: 掛路由**

`api/router.go`:

在 `deviceMgmtRead` 群組(line 197 附近、v1 iot GET 之後)加:

```go
			deviceMgmtRead.GET("/iot/v2/overview", h.GetIoTOverview)
			deviceMgmtRead.GET("/iot/v2/devices", h.ListIoTDevicesV2)
			deviceMgmtRead.GET("/iot/v2/devices/:id", h.GetIoTDeviceV2)
			deviceMgmtRead.GET("/iot/v2/history", h.GetIoTHistory)
			deviceMgmtRead.GET("/iot/v2/history.csv", h.GetIoTHistoryCSV)
			deviceMgmtRead.GET("/iot/v2/serial-ports", h.GetIoTSerialPorts)
```

在 `editor` 群組(line 247 附近)加:

```go
			editor.POST("/iot/v2/devices/:id/poll", h.PollIoTDeviceV2)
```

在 `admin` 群組(line 290 附近)加:

```go
			admin.POST("/iot/v2/devices", h.CreateIoTDeviceV2)
			admin.PUT("/iot/v2/devices/:id", h.UpdateIoTDeviceV2)
			admin.DELETE("/iot/v2/devices/:id", h.DeleteIoTDeviceV2)
			admin.PUT("/iot/v2/devices/:id/points", h.PutIoTDevicePoints)
			admin.POST("/iot/v2/devices/test", h.TestIoTDeviceConnection)
```

- [ ] **Step 6: 跑測試確認通過**

Run: `go test ./api/handlers/ -run TestIoTV2 -v && go test ./... -count=1`
Expected: 全部 PASS。

- [ ] **Step 7: Commit**

```bash
gofmt -w modules/iot/serialports.go modules/iot/service_v2.go api/handlers/iot_v2.go api/handlers/iot_v2_test.go api/router.go
git add modules/iot/serialports.go modules/iot/service_v2.go api/handlers/iot_v2.go api/handlers/iot_v2_test.go api/router.go
git commit -m "feat(iot): add v2 REST API, serial port listing and route wiring"
```

---

### Task 11: 總驗證

- [ ] **Step 1: 全套建置與測試**

```bash
go build ./...
go vet ./modules/iot/ ./api/handlers/
go test ./... -count=1
```

Expected: 全綠;`go vet` 無新增警告。

- [ ] **Step 2: 啟動冒煙測試(手動)**

```bash
go run . 
```

啟動後以瀏覽器或 curl 確認(帶登入 token):
- `GET /api/iot/v2/overview` 回 200 與 JSON 計數
- `GET /api/iot/v2/serial-ports` 回 200(無串列埠時為 `[]`)
- 既有 IoT 頁(v1)行為不變

(路徑前綴以 router.go 實際掛載為準;若 API root 是 `/api`,以上路徑加前綴。)確認後停止伺服器。

- [ ] **Step 3: 最終 commit(如有殘餘格式化/小修)**

```bash
git status
gofmt -l modules/iot api/handlers
git add -A modules/iot api/handlers
git commit -m "chore(iot): final formatting and smoke-test fixes for v2 backend core" || echo "nothing to commit"
```

---

## Plan 1 完成定義(DoD)

1. `go test ./... -count=1` 全綠(新測試 ≥ 12 個,涵蓋解碼矩陣、規劃、遷移、CRUD、輪詢整合、歷史、handler)。
2. 透過 API 可:建立 TCP/RTU/RTU over TCP 設備與多點位、立即輪詢拿到解碼值、查歷史(raw/hourly 自動切換)、匯出 CSV、列舉串列埠、測試連線。
3. v1 遷移冪等;v1 既有端點與測試行為不變;v2 量測進 v1 續傳佇列。
4. 未做(Plan 2/3):寫入控制、警報、範本、forwarder 強化、`GET/PUT /iot/v2/settings` 設定端點(保留天數本計畫先由 `system_config` 鍵值 `iot_v2_raw_retention_days` / `iot_v2_hourly_retention_days` 控制)、前端 UI。
