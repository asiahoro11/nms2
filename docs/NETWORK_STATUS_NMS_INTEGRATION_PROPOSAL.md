# new_nms_sync Network Status / NMS 串接提案

## 1. 目前系統判讀

這次只以 `new_nms_sync` 為範圍。

目前可確認：

- 後端是 `Go + Gin`
- 前端是 `apps/frontend` 下的靜態 `HTML / CSS / JS`
- 系統已經有設備與拓樸能力，不是從零開始

已存在的相關位置：

- 前端頁面：
  - `apps/frontend/index.html`
  - `apps/frontend/js/devices.js`
  - `apps/frontend/js/topology.js`
- 後端 API：
  - `GET /api/v1/devices`
  - `GET /api/v1/devices/:id`
  - `GET /api/v1/devices/:id/metrics`
  - `GET /api/v1/devices/:id/interfaces`
  - `GET /api/v1/devices/:id/events`
  - `GET /api/v1/topology`
  - `POST /api/v1/devices/bulk-scan`
  - `POST /api/v1/topology/discover`

所以這個需求最合理的做法不是另外包一套頁面，而是：

1. 在左側新增一個 `網路狀態 Network Status`
2. 進入後整合「搜尋設備 + 左側設備狀態 + 右側拓樸圖」
3. 優先沿用既有 `devices` 與 `topology` API
4. 若客戶有既有 NMS，再從 backend 加 adapter，不讓前端直接打外部 NMS

## 2. 建議畫面架構

```text
+----------------------------------------------------------------------------------+
| 左側主選單                                                                       |
| Dashboard                                                                        |
| Devices                                                                          |
| Topology                                                                         |
| Logs                                                                             |
| Cameras                                                                          |
| PDU                                                                              |
| Network Status   <-- 新增                                                        |
+----------------------------------------------------------------------------------+

+----------------------------------------------------------------------------------+
| Network Status                                                                   |
|----------------------------------------------------------------------------------|
| 搜尋列: [名稱 / IP / MAC / 區域] [設備類型] [狀態] [廠牌] [搜尋] [重新整理]      |
| 摘要列: Total 128 | Online 118 | Warning 4 | Offline 6 | Last Sync 10:32         |
|----------------------------------------------------------------------------------|
| 左側：設備清單與狀態                    | 右側：拓樸圖                           |
|-----------------------------------------|----------------------------------------|
| [UP ] Core-SW-01   10.10.1.1            |                 [Core]                 |
| [UP ] Dist-SW-01   10.10.1.11           |                /  |  \                 |
| [WARN] Dist-SW-02  10.10.1.12           |             [FW] [D1] [D2]             |
| [DOWN] AP-3F-001   10.10.3.21           |                 /   \                  |
| [UP ] UPS-01       10.10.9.10           |              [AP]   [Server]           |
|                                         |                                        |
| 篩選條件                                 | 圖例                                   |
| - Online / Offline / Warning            | - 綠色: 正常                           |
| - Router / Switch / AP / Firewall       | - 黃色: 告警                           |
| - Site / Floor / VLAN / Vendor          | - 紅色: 離線                           |
|                                         |                                        |
| 點選設備後顯示：                         | 操作區                                 |
| - CPU / Memory / Temperature            | - Auto Discover                        |
| - Port 狀態 / 流量                      | - Refresh Topology                     |
| - 最近事件 / 最後上線時間               | - 聚焦到該設備                         |
+----------------------------------------------------------------------------------+
```

示意圖檔：

- `artifacts/network-status-wireframe.svg`

## 3. 對 new_nms_sync 的建議整合方式

### 方案 A：直接使用 new_nms_sync 既有能力

適用情境：

- 客戶沒有另外一套獨立 NMS
- 目前設備盤點、SNMP 掃描、拓樸探索希望留在同一套系統

資料流：

```text
網路設備
  -> SNMP / ICMP / CLI
  -> new_nms_sync collector
  -> devices / device_interfaces / topology
  -> /api/v1/devices + /api/v1/topology
  -> Network Status 頁面
```

優點：

- 開發速度最快
- 風險最低
- 沿用現有 API 與資料表
- 不需要處理外部 NMS 認證與跨系統同步

### 方案 B：外部 NMS 經由 backend adapter 串進來

適用情境：

- 客戶已經有既有 NMS
- 客戶希望 new_nms_sync 只做整合展示層與管理層

資料流：

```text
外部 NMS
  -> REST API / Webhook / Trap / 匯出資料
  -> new_nms_sync backend adapter
  -> 正規化後寫入內部模型
  -> /api/v1/network-status/*
  -> 前端頁面
```

優點：

- 前端不需要知道外部 NMS 的格式
- 認證資訊留在 backend
- 未來更換 NMS 時，前端不用重寫
- 可以同時整合多來源資料

## 4. 建議採用的正式架構

建議採用 `Backend Adapter + Internal Canonical Model`。

```text
                     +----------------------------------+
                     | apps/frontend                    |
                     | Network Status Page              |
                     +----------------+-----------------+
                                      |
                                      v
                     +----------------------------------+
                     | apps/backend API                 |
                     | Go / Gin                         |
                     +----------------+-----------------+
                                      |
          +---------------------------+---------------------------+
          |                                                       |
          v                                                       v
+-----------------------------+                  +--------------------------------+
| Native Device Collection    |                  | External NMS Adapter           |
| SNMP / ICMP / CLI           |                  | REST / webhook / export / trap |
+--------------+--------------+                  +----------------+---------------+
               |                                                  |
               +--------------------------+-----------------------+
                                          |
                                          v
                     +----------------------------------+
                     | Canonical Network Data Model     |
                     | device inventory                 |
                     | interface status                 |
                     | topology graph                   |
                     | events / health / sync status    |
                     +----------------------------------+
```

這樣做的原因：

- 前端契約固定
- 外部系統格式差異被隔離在 backend
- 新增其他廠牌或其他 NMS 時成本低
- 可保留 `new_nms_sync` 既有設備資料與拓樸邏輯

## 5. 對 new_nms_sync 最實際的 API 建議

### 5.1 最小改動版本

先直接沿用現有 API：

- `GET /api/v1/devices?search=&type=&status=`
- `GET /api/v1/devices/:id`
- `GET /api/v1/devices/:id/metrics`
- `GET /api/v1/devices/:id/interfaces`
- `GET /api/v1/devices/:id/events`
- `GET /api/v1/topology`

前端頁面流程：

1. 頁面進入時先抓 `/devices` 與 `/topology`
2. 搜尋條件更新時重抓 `/devices`
3. 點選左側設備時抓 detail / metrics / interfaces / events
4. 右側拓樸點擊節點時同步聚焦左側設備

### 5.2 建議升級版本

新增一組聚合 API，減少前端併發請求數：

- `GET /api/v1/network-status/summary`
- `GET /api/v1/network-status/devices`
- `GET /api/v1/network-status/topology`
- `GET /api/v1/network-status/device/:id/detail`
- `POST /api/v1/network-status/discovery/scan`
- `POST /api/v1/network-status/sync/external`

建議回傳格式：

```json
{
  "summary": {
    "total": 128,
    "online": 118,
    "warning": 4,
    "offline": 6,
    "last_sync_at": "2026-04-22T10:32:00+08:00"
  },
  "devices": [
    {
      "id": 1,
      "name": "Core-SW-01",
      "ip_address": "10.10.1.1",
      "device_type": "switch",
      "vendor": "Cisco",
      "is_online": true,
      "health_score": 98,
      "last_seen": "2026-04-22T10:31:30+08:00"
    }
  ],
  "topology": {
    "nodes": [],
    "links": []
  }
}
```

## 6. 建議的資料欄位正規化

若要對接外部 NMS，建議先做一層 mapping：

### sync mapping

- `source_system`
- `external_device_id`
- `external_interface_id`
- `internal_device_id`
- `last_sync_at`
- `sync_status`
- `sync_error`

### canonical device model

- `name`
- `sys_name`
- `ip_address`
- `mac_address`
- `device_type`
- `vendor`
- `model`
- `site`
- `floor`
- `is_online`
- `health_score`
- `last_seen`

### canonical interface model

- `if_index`
- `if_name`
- `if_status`
- `if_admin_status`
- `bandwidth_in`
- `bandwidth_out`
- `if_speed`

### canonical topology model

- `source_device_id`
- `target_device_id`
- `source_if_id`
- `target_if_id`
- `link_speed`
- `link_type`
- `link_label`

## 7. 建議的同步策略

### 初版建議

- Inventory 同步：每 5 到 15 分鐘
- Health / online status：每 1 到 5 分鐘
- 單一設備 detail：使用者點選時即時抓
- Event / alert：若外部 NMS 支援，優先 webhook 或 trap

### 不建議的做法

- 前端直接連外部 NMS
- 前端直接打 SNMP
- UI 直接吃各家廠牌原始 payload

## 8. 對 new_nms_sync 的實作建議

### 前端

建議新增：

- `apps/frontend/js/network-status.js`
- 視需要新增 `apps/frontend/css/network-status.css`

原因：

- `devices.js` 與 `topology.js` 已有明確職責
- 新頁面整合搜尋、清單、拓樸與摘要時，獨立檔案較好維護
- 可避免把現有頁面耦合得更重

### 後端

若只是快速驗證：

- 先沿用既有 `devices` / `topology` handler

若要正式上線：

- 新增 `network-status` 聚合 handler
- 若對接外部 NMS，adapter 建議放 backend module / service 層，不放 frontend

### 選單與權限

建議：

- 左側新增 `Network Status`
- 可掛在既有授權或 module gate 之下
- 讓 menu 顯示與 API 權限一致

## 9. 給客戶可直接使用的建議說法

可直接這樣說：

> 我們建議在現有系統中新增一個 `Network Status` 模組。
> 使用者進入後可以先搜尋設備，左側顯示設備狀態清單，右側顯示對應網路拓樸。
> 若客戶已有既有 NMS，建議由 backend 進行串接與資料正規化，再提供給前端顯示，避免前端直接依賴外部 NMS 格式與認證機制。
> 這樣可以兼顧安全性、擴充性，以及未來更換或增加 NMS 來源時的維護成本。

## 10. 建議下一步

最務實的執行順序：

1. 先在 `index.html` 新增 `Network Status` 頁面入口
2. 先串現有 `/devices` 與 `/topology`
3. 確認客戶是否已有外部 NMS
4. 如果有，再定義 adapter 與 sync mapping
5. 最後再補 summary API 與快取策略
