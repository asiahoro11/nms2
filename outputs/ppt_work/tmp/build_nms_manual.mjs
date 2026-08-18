import fs from "node:fs/promises";
import path from "node:path";
import { Presentation, PresentationFile } from "@oai/artifact-tool";

const OUT = "D:/claude-sandbox/nms_server/outputs/nms_server_operation_manual_zh-TW.pptx";
const TMP = "D:/claude-sandbox/nms_server/outputs/ppt_work/tmp";
const PREVIEW = path.join(TMP, "preview");

const W = 1280;
const H = 720;
const C = {
  ink: "#111111",
  muted: "#555555",
  soft: "#EDEDED",
  rule: "#B8BCC4",
  accent: "#FF6B35",
  white: "#FFFFFF",
  green: "#0F8A5F",
  red: "#B42318",
  blue: "#1D4ED8",
};

const font = {
  title: "Microsoft JhengHei",
  body: "Microsoft JhengHei",
  mono: "Consolas",
};

function exists(p) {
  return fs.stat(p).then(() => true).catch(() => false);
}

async function writeBlob(file, blob) {
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, new Uint8Array(await blob.arrayBuffer()));
}

function addText(slide, text, pos, opt = {}) {
  const box = slide.shapes.add({
    geometry: "textbox",
    position: pos,
    fill: opt.fill ?? "none",
    line: { style: "solid", fill: opt.line ?? "none", width: opt.lineWidth ?? 0 },
  });
  box.text = text;
  box.text.style = {
    fontSize: opt.size ?? 22,
    bold: opt.bold ?? false,
    color: opt.color ?? C.ink,
    typeface: opt.mono ? font.mono : font.body,
    alignment: opt.align ?? "left",
  };
  return box;
}

function rect(slide, pos, opt = {}) {
  return slide.shapes.add({
    geometry: opt.round ? "roundRect" : "rect",
    position: pos,
    fill: opt.fill ?? C.soft,
    line: { style: "solid", fill: opt.line ?? "none", width: opt.lineWidth ?? 0 },
  });
}

function line(slide, x1, y1, x2, y2, opt = {}) {
  return slide.shapes.add({
    geometry: "line",
    position: { left: x1, top: y1, width: x2 - x1, height: y2 - y1 },
    fill: "none",
    line: { style: "solid", fill: opt.color ?? C.rule, width: opt.width ?? 1 },
  });
}

function baseSlide(p, title, section = "NMS 操作手冊") {
  const slide = p.slides.add();
  slide.background.fill = C.white;
  addText(slide, section, { left: 42, top: 34, width: 420, height: 28 }, { size: 15, bold: true, color: C.muted });
  addText(slide, title, { left: 42, top: 72, width: 940, height: 76 }, { size: 39, bold: true });
  line(slide, 42, 155, 1238, 155);
  addText(slide, String(p.slides.items.length).padStart(2, "0"), { left: 1184, top: 658, width: 54, height: 24 }, { size: 14, color: C.muted, align: "right" });
  return slide;
}

function cover(p) {
  const s = p.slides.add();
  s.background.fill = C.white;
  rect(s, { left: 42, top: 38, width: 1196, height: 108 }, { fill: C.soft });
  addText(s, "Network Management System", { left: 42, top: 188, width: 920, height: 80 }, { size: 58, bold: true });
  addText(s, "nms_server 全功能操作手冊", { left: 42, top: 286, width: 780, height: 60 }, { size: 34, bold: true, color: C.accent });
  addText(s, "適用對象：管理員、維運人員、系統整合與現場工程師", { left: 42, top: 376, width: 830, height: 42 }, { size: 24, color: C.muted });
  addText(s, "版本依據：目前程式碼、API 文件與 release notes", { left: 42, top: 586, width: 680, height: 34 }, { size: 18, color: C.muted });
  addText(s, "YTSworks NMS", { left: 994, top: 48, width: 210, height: 34 }, { size: 20, bold: true, align: "right" });
}

function agenda(p) {
  const s = baseSlide(p, "使用者先理解路徑，再進入各模組操作", "導覽");
  const items = [
    ["01", "登入、角色與授權", "先確認可用功能、2FA、密碼與 License 狀態。"],
    ["02", "日常監控", "使用儀表板、設備清單、拓樸、通知與日誌掌握現場狀態。"],
    ["03", "專用模組", "攝影機/NVR、門禁、PDU/UPS、IoT/Modbus 的設定與操作。"],
    ["04", "管理與整合", "使用者、報表、備份還原、API、iframe 與行動端。"],
  ];
  items.forEach((it, i) => {
    const top = 204 + i * 104;
    addText(s, it[0], { left: 60, top, width: 88, height: 64 }, { size: 46, bold: true, color: C.accent });
    addText(s, it[1], { left: 180, top: top + 2, width: 360, height: 34 }, { size: 28, bold: true });
    addText(s, it[2], { left: 180, top: top + 44, width: 900, height: 34 }, { size: 21, color: C.muted });
    line(s, 60, top + 82, 1120, top + 82);
  });
}

function roleMatrix(p) {
  const s = baseSlide(p, "權限與授權決定每位使用者看得到什麼", "開始使用");
  const rows = [
    ["Viewer", "查看儀表板、狀態、部分日誌與唯讀資料", "不能新增、刪除或控制設備"],
    ["Editor", "設備維護、拓樸編輯、報表匯出、部分模組操作", "不能管理使用者、授權與系統設定"],
    ["Admin", "完整管理、License、備份還原、安全、整合與模組設定", "高風險操作需保留稽核紀錄"],
  ];
  addText(s, "常見操作原則", { left: 62, top: 190, width: 420, height: 40 }, { size: 28, bold: true });
  addText(s, "功能開關會同時受角色、License 與系統設定影響。若側邊欄出現鎖定標記，先到系統管理查看授權與模組狀態。", { left: 62, top: 242, width: 480, height: 120 }, { size: 22, color: C.muted });
  rows.forEach((r, i) => {
    const top = 190 + i * 126;
    rect(s, { left: 620, top, width: 560, height: 96 }, { fill: i === 2 ? "#FFF3EE" : C.soft });
    addText(s, r[0], { left: 644, top: top + 14, width: 140, height: 30 }, { size: 25, bold: true, color: i === 2 ? C.accent : C.ink });
    addText(s, r[1], { left: 810, top: top + 12, width: 320, height: 34 }, { size: 19 });
    addText(s, r[2], { left: 810, top: top + 50, width: 320, height: 28 }, { size: 16, color: C.muted });
  });
}

function flowSlide(p) {
  const s = baseSlide(p, "從登入到第一個監控畫面的標準流程", "開始使用");
  const steps = [
    ["選擇模式", "管理模式進入完整後台；監看模式用於現場大螢幕或輪播。"],
    ["登入帳號", "輸入站點帳號密碼；若啟用 2FA，完成 TOTP 驗證。"],
    ["檢查授權", "若 License lock 生效，只保留授權與使用者相關管理入口。"],
    ["進入首頁", "儀表板載入設備總覽、Top 5、事件、版本與通知。"],
  ];
  steps.forEach((st, i) => {
    const left = 72 + i * 292;
    rect(s, { left, top: 236, width: 238, height: 210 }, { fill: i === 0 ? "#FFF3EE" : C.soft });
    addText(s, String(i + 1), { left: left + 18, top: 254, width: 42, height: 38 }, { size: 31, bold: true, color: C.accent });
    addText(s, st[0], { left: left + 22, top: 314, width: 190, height: 36 }, { size: 25, bold: true });
    addText(s, st[1], { left: left + 22, top: 366, width: 188, height: 70 }, { size: 17, color: C.muted });
    if (i < steps.length - 1) line(s, left + 244, 341, left + 280, 341, { color: C.accent, width: 2 });
  });
}

function dashboard(p) {
  const s = baseSlide(p, "儀表板用來判斷今天要先處理哪裡", "日常監控");
  const blocks = [
    ["狀態總覽", "總設備數、在線、離線、記憶體使用量。"],
    ["設備分布", "依設備類型快速看現場資產結構。"],
    ["Top 5", "流量、CPU、Memory 可切換排行。"],
    ["最近事件", "掌握剛發生的離線、告警與異常。"],
  ];
  blocks.forEach((b, i) => {
    const col = i % 2;
    const row = Math.floor(i / 2);
    const left = 70 + col * 575;
    const top = 210 + row * 170;
    rect(s, { left, top, width: 500, height: 118 }, { fill: C.soft });
    addText(s, b[0], { left: left + 24, top: top + 18, width: 210, height: 34 }, { size: 26, bold: true });
    addText(s, b[1], { left: left + 24, top: top + 62, width: 430, height: 44 }, { size: 19, color: C.muted });
  });
  addText(s, "操作建議：先看離線數與最近事件，再進入設備或拓樸頁定位影響範圍。", { left: 70, top: 594, width: 980, height: 34 }, { size: 22, bold: true, color: C.accent });
}

function devices(p) {
  const s = baseSlide(p, "設備清單是所有監控與控制功能的入口", "設備管理");
  const cols = [
    ["新增方式", "單筆新增、IP range、CIDR 子網掃描、Ping fallback。"],
    ["監控設定", "Ping、SNMP v1/v2c/v3、CLI/Web 帳密、設備圖片。"],
    ["清單操作", "搜尋、依類型/狀態篩選、/24 子網群組、批次編輯與批次刪除。"],
  ];
  cols.forEach((c, i) => {
    const left = 64 + i * 390;
    rect(s, { left, top: 206, width: 338, height: 244 }, { fill: i === 1 ? "#FFF3EE" : C.soft });
    addText(s, c[0], { left: left + 22, top: 228, width: 280, height: 38 }, { size: 28, bold: true });
    addText(s, c[1], { left: left + 22, top: 292, width: 280, height: 104 }, { size: 21, color: C.muted });
  });
  addText(s, "新增 SNMP 設備時，Read community 用於監控，Write community 與 CLI 帳密只在需要 PoE、重啟、備份等控制動作時填寫。", { left: 76, top: 536, width: 1040, height: 54 }, { size: 21 });
}

function deviceDetail(p) {
  const s = baseSlide(p, "設備詳情頁把狀態、流量與控制集中在一起", "設備管理");
  const items = [
    ["健康資訊", "CPU、Memory、Disk、SNMP uptime、延遲與最後上線時間。"],
    ["介面與流量", "查看 port 狀態、速率、IN/OUT 流量與 Port matrix。"],
    ["PoE / AP", "EdgeCore PoE on/off/recycle，AP 狀態與無線資訊。"],
    ["維運動作", "立即 poll、重啟設備、儲存設定備份、下載備份。"],
    ["WebSSH", "透過瀏覽器建立 SSH relay，免離開 NMS 操作。"],
  ];
  items.forEach((it, i) => {
    const top = 194 + i * 78;
    addText(s, it[0], { left: 74, top, width: 200, height: 32 }, { size: 24, bold: true, color: i === 4 ? C.accent : C.ink });
    addText(s, it[1], { left: 310, top: top + 2, width: 790, height: 34 }, { size: 21, color: C.muted });
    line(s, 74, top + 52, 1120, top + 52);
  });
}

function topology(p) {
  const s = baseSlide(p, "拓樸頁把設備關係轉成可操作的地圖", "拓樸管理");
  const ops = [
    ["自動探索", "使用 LLDP 建立或更新連線。"],
    ["手動連線", "選擇來源與目標介面，設定 link type 與速率。"],
    ["位置保存", "拖曳節點後儲存座標，維持固定視圖。"],
    ["多視圖", "力導向、樹狀、監看模式，各自對應不同現場需求。"],
  ];
  ops.forEach((o, i) => {
    const left = 74 + (i % 2) * 548;
    const top = 206 + Math.floor(i / 2) * 158;
    rect(s, { left, top, width: 480, height: 108 }, { fill: C.soft });
    addText(s, o[0], { left: left + 22, top: top + 14, width: 190, height: 30 }, { size: 25, bold: true });
    addText(s, o[1], { left: left + 220, top: top + 18, width: 220, height: 58 }, { size: 18, color: C.muted });
  });
  addText(s, "HA、VRF、Spine-Leaf 等 link type 可讓拓樸圖更貼近真實網路關係。", { left: 80, top: 572, width: 900, height: 34 }, { size: 22, bold: true, color: C.accent });
}

async function imageSlide(p, title, imgPath, caption) {
  const s = baseSlide(p, title, "畫面範例");
  if (await exists(imgPath)) {
    const bytes = await fs.readFile(imgPath);
    s.images.add({
      blob: bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength),
      contentType: "image/png",
      fit: "contain",
      alt: caption,
      position: { left: 90, top: 190, width: 760, height: 430 },
    });
    rect(s, { left: 890, top: 214, width: 270, height: 250 }, { fill: C.soft });
    addText(s, caption, { left: 914, top: 242, width: 220, height: 170 }, { size: 22, color: C.muted });
  } else {
    addText(s, caption, { left: 90, top: 250, width: 860, height: 90 }, { size: 30, bold: true, color: C.muted });
  }
}

function cameras(p) {
  const s = baseSlide(p, "攝影機模組負責即時預覽、分割監看與 NVR", "攝影機 / NVR");
  const rows = [
    ["新增與探索", "手動新增 RTSP/ONVIF，或用 ONVIF probe 自動取得 RTSP。"],
    ["監看配置", "4/9/16 格、拖曳指派、分頁支援至授權上限。"],
    ["影像路徑", "MJPEG 預設低延遲；WebRTC/MSE 可作進階預覽。"],
    ["錄影管理", "批次錄影、時間軸、播放、匯出、刪除與 NVR 儲存路徑。"],
    ["安全處理", "RTSP 帳密與敏感 URL 由後端管理，API 回應會遮蔽。"],
  ];
  rows.forEach((r, i) => {
    const top = 190 + i * 80;
    addText(s, r[0], { left: 76, top, width: 210, height: 30 }, { size: 24, bold: true });
    addText(s, r[1], { left: 318, top: top + 2, width: 760, height: 36 }, { size: 21, color: C.muted });
  });
}

function accessControl(p) {
  const s = baseSlide(p, "門禁模組把門、卡片、時段與事件放在同一個流程", "門禁管理");
  const flow = [
    ["門", "新增門點，設定 IP、協定、帳密、位置、廠牌與型號。"],
    ["卡片", "建立卡號、持有人、部門、有效日期與啟用狀態。"],
    ["時段", "設定允許日期、星期、時間、門點範圍，並走審核。"],
    ["事件", "查詢 access、denied、alarm、open、close、tamper 等紀錄。"],
  ];
  flow.forEach((f, i) => {
    const left = 72 + i * 292;
    rect(s, { left, top: 226, width: 236, height: 230 }, { fill: i === 2 ? "#FFF3EE" : C.soft });
    addText(s, f[0], { left: left + 22, top: 250, width: 180, height: 42 }, { size: 34, bold: true, color: i === 2 ? C.accent : C.ink });
    addText(s, f[1], { left: left + 22, top: 318, width: 185, height: 90 }, { size: 18, color: C.muted });
  });
}

function pdu(p) {
  const s = baseSlide(p, "PDU/UPS 模組用 SNMP 做電力設備盤點與巡檢", "PDU / UPS");
  const bullets = [
    "新增 UPS 或 PDU，設定 IP、SNMP port、community、版本、位置、廠牌與型號。",
    "手動 Poll 可立即讀取 battery capacity、runtime、temperature、input voltage、output load 與 alarm。",
    "狀態分為 normal、warning、critical、offline、unknown，適合日常巡檢或事故確認。",
    "PDU/UPS 屬授權模組，未授權時頁面保留提示但停用新增與操作按鈕。",
  ];
  bullets.forEach((b, i) => {
    addText(s, String(i + 1), { left: 88, top: 204 + i * 88, width: 36, height: 30 }, { size: 25, bold: true, color: C.accent });
    addText(s, b, { left: 150, top: 204 + i * 88, width: 900, height: 56 }, { size: 23, color: i === 1 ? C.ink : C.muted });
  });
}

function iot(p) {
  const s = baseSlide(p, "IoT / Modbus 把現場感測值標準化進 NMS", "IoT / Modbus");
  const cols = [
    ["Direct Poll", "Modbus TCP、RTU、RS485、RTU over TCP。設定 Unit ID、register、FC、data type、scale、offset 與 endian。"],
    ["Gateway Ingest", "REST ingest 讓 MQTT、OPC-UA、BACnet gateway 推送標準化資料。"],
    ["Store and Forward", "本地 SQLite 先寫入，Webhook 離線時排隊，恢復後批次補送。"],
  ];
  cols.forEach((c, i) => {
    const left = 68 + i * 390;
    rect(s, { left, top: 214, width: 338, height: 250 }, { fill: i === 0 ? "#FFF3EE" : C.soft });
    addText(s, c[0], { left: left + 22, top: 238, width: 270, height: 34 }, { size: 26, bold: true, color: i === 0 ? C.accent : C.ink });
    addText(s, c[1], { left: left + 22, top: 298, width: 282, height: 116 }, { size: 18, color: C.muted });
  });
  addText(s, "溫濕度二合一感測器可自動使用 0.1 scale 與雙 register 解析，適合 RS485 現場。", { left: 76, top: 560, width: 940, height: 34 }, { size: 22, bold: true, color: C.accent });
}

function logs(p) {
  const s = baseSlide(p, "日誌中心讓問題追查有時間線與證據包", "日誌與通知");
  const rows = [
    ["系統日誌", "服務、模組、背景任務與系統事件。"],
    ["設備日誌", "設備狀態、告警、ack、來源 IP/MAC。"],
    ["稽核日誌", "登入、登出、設定、設備、攝影機、備份等操作。"],
    ["設定變更", "before / after、變更來源、recordset_id 與 correlation_id。"],
    ["匯出", "CSV、JSON、證據包，供維運交接與稽核留存。"],
  ];
  rows.forEach((r, i) => {
    const top = 188 + i * 82;
    rect(s, { left: 70, top, width: 250, height: 52 }, { fill: i === 4 ? "#FFF3EE" : C.soft });
    addText(s, r[0], { left: 92, top: top + 10, width: 200, height: 26 }, { size: 22, bold: true, color: i === 4 ? C.accent : C.ink });
    addText(s, r[1], { left: 360, top: top + 10, width: 720, height: 30 }, { size: 21, color: C.muted });
  });
}

function reports(p) {
  const s = baseSlide(p, "報表匯出支援維運、盤點、SLA 與合規佐證", "報表");
  const reports = [
    "設備、Inventory、Interface、PoE、Traffic",
    "Health、Health Trend、Availability、SLA",
    "Logs、Audit、Config Change",
    "License Capacity、Camera、PDU/UPS、Access Control",
  ];
  reports.forEach((r, i) => {
    rect(s, { left: 90, top: 204 + i * 92, width: 950, height: 62 }, { fill: i % 2 === 0 ? C.soft : "#F7F7F7" });
    addText(s, r, { left: 120, top: 218 + i * 92, width: 850, height: 34 }, { size: 24, bold: i === 0 });
  });
  addText(s, "匯出內容會避免暴露密碼、SNMP community、完整 license key 與 RTSP 敏感資訊。", { left: 90, top: 598, width: 920, height: 34 }, { size: 22, color: C.accent, bold: true });
}

function admin(p) {
  const s = baseSlide(p, "系統管理集中處理帳號、授權、安全與維護", "系統管理");
  const list = [
    ["使用者", "新增、停用、角色、密碼政策與密碼變更。"],
    ["License", "Machine ID、啟用、重發、容量與功能旗標。"],
    ["安全", "全域 2FA、個人 TOTP、recovery codes、密碼到期天數。"],
    ["告警", "Email、Webhook、Syslog 等通道設定與測試。"],
    ["備份還原", "一般備份、加密備份、還原 readiness 與加密密碼。"],
    ["品牌與工具", "Logo、公司名稱、Ping、Traceroute、主機狀態。"],
  ];
  list.forEach((it, i) => {
    const col = i % 2;
    const row = Math.floor(i / 2);
    const left = 72 + col * 548;
    const top = 186 + row * 118;
    addText(s, it[0], { left, top, width: 180, height: 30 }, { size: 24, bold: true, color: i === 1 ? C.accent : C.ink });
    addText(s, it[1], { left: left + 184, top: top + 2, width: 320, height: 54 }, { size: 18, color: C.muted });
    line(s, left, top + 70, left + 500, top + 70);
  });
}

function integrations(p) {
  const s = baseSlide(p, "整合 API 與 iframe 讓外部系統安全取用 NMS 資料", "系統整合");
  const left = [
    ["Network Snapshot", "一次取得 dashboard、devices、topology，適合外部儀表板刷新。"],
    ["Embed Token", "建立只讀 iframe token，限制 view 與有效時間，可列出與撤銷。"],
    ["Frame Ancestors", "設定允許嵌入的 HTTPS origin，避免任意網站 iframe。"],
  ];
  left.forEach((it, i) => {
    addText(s, it[0], { left: 70, top: 204 + i * 102, width: 260, height: 30 }, { size: 24, bold: true });
    addText(s, it[1], { left: 350, top: 205 + i * 102, width: 430, height: 54 }, { size: 19, color: C.muted });
  });
  rect(s, { left: 840, top: 216, width: 310, height: 210 }, { fill: "#111111" });
  addText(s, "GET /api/v1/integrations/network-snapshot\n\n/embed.html?view=dashboard&token=...\n\nPUT /api/v1/integrations/settings", { left: 862, top: 242, width: 266, height: 146 }, { size: 16, color: C.white, mono: true });
  addText(s, "建議：外部客戶入口使用 embed token，不要把 Admin JWT 放進 iframe URL。", { left: 74, top: 570, width: 940, height: 34 }, { size: 22, bold: true, color: C.accent });
}

function mobile(p) {
  const s = baseSlide(p, "行動端是監控 companion，不取代伺服器", "行動端");
  addText(s, "Android / iOS App 透過既有 HTTPS API 連到 NMS 主機。第一階段聚焦監控：登入、2FA、Dashboard、設備狀態、告警、攝影機預覽、拓樸唯讀、日誌與報表。", { left: 78, top: 202, width: 780, height: 116 }, { size: 24 });
  const apis = ["GET /system/info", "POST /auth/login", "POST /auth/verify-2fa", "GET /auth/me", "GET /dashboard"];
  apis.forEach((a, i) => {
    rect(s, { left: 102, top: 378 + i * 42, width: 420, height: 28 }, { fill: C.soft });
    addText(s, a, { left: 118, top: 382 + i * 42, width: 390, height: 20 }, { size: 16, mono: true });
  });
  rect(s, { left: 910, top: 190, width: 220, height: 384 }, { fill: "#111111" });
  rect(s, { left: 934, top: 222, width: 172, height: 302 }, { fill: "#F7F7F7" });
  addText(s, "NMS\nMobile", { left: 956, top: 288, width: 128, height: 72 }, { size: 30, bold: true, align: "center" });
}

function maintenance(p) {
  const s = baseSlide(p, "日常維運要把啟動、備份、更新與安全檢查固定化", "維運");
  const items = [
    ["啟動", "go run ./apps/backend 或發佈包 start_nms.ps1 / start_nms.sh。"],
    ["備份", "定期匯出系統備份；敏感環境使用加密備份並妥善保存密碼。"],
    ["更新", "更新前先備份 runtime/data；發佈包不應覆蓋既有 nms.db。"],
    ["檢查", "確認 8080/TCP、514/UDP、162/UDP，Syslog/SNMP Trap 依部署需求開放。"],
    ["安全", "分段 OT 網路，不直接暴露 Modbus、SNMP、RTSP、ONVIF 等埠到 Internet。"],
  ];
  items.forEach((it, i) => {
    const top = 190 + i * 82;
    addText(s, it[0], { left: 78, top, width: 120, height: 30 }, { size: 24, bold: true, color: i === 4 ? C.accent : C.ink });
    addText(s, it[1], { left: 232, top: top + 1, width: 850, height: 34 }, { size: 21, color: C.muted });
  });
}

function troubleshooting(p) {
  const s = baseSlide(p, "遇到問題時先判斷是權限、授權、網路還是設備端", "故障排除");
  const cases = [
    ["看不到模組", "檢查 License features、角色、模組開關與鎖定提示。"],
    ["設備沒有資料", "確認 Ping/SNMP 可達、community/v3 權限、poll interval 與防火牆。"],
    ["攝影機黑畫面", "先測 snapshot，再看 RTSP/ONVIF、transport、編碼與 go2rtc/FFmpeg 狀態。"],
    ["IoT 未送出", "查看 queue status、forward URL/token、batch size、flush 與 failed 記錄。"],
    ["頁面卡住", "檢查 host-status、license、machine ID 請求 timeout 與 server.log。"],
  ];
  cases.forEach((c, i) => {
    const left = 74 + (i % 2) * 548;
    const top = 190 + Math.floor(i / 2) * 132;
    rect(s, { left, top, width: 486, height: 88 }, { fill: i === 0 ? "#FFF3EE" : C.soft });
    addText(s, c[0], { left: left + 22, top: top + 12, width: 190, height: 28 }, { size: 23, bold: true, color: i === 0 ? C.accent : C.ink });
    addText(s, c[1], { left: left + 218, top: top + 12, width: 230, height: 50 }, { size: 17, color: C.muted });
  });
}

function close(p) {
  const s = p.slides.add();
  s.background.fill = C.white;
  addText(s, "操作手冊的核心目標", { left: 42, top: 72, width: 800, height: 70 }, { size: 50, bold: true });
  addText(s, "讓維運人員能快速定位狀態、完成設備與模組操作，並在需要對外整合或交付佐證時，有一致、安全、可追溯的流程。", { left: 42, top: 206, width: 930, height: 120 }, { size: 30, color: C.muted });
  rect(s, { left: 42, top: 448, width: 1196, height: 92 }, { fill: "#FFF3EE" });
  addText(s, "建議交付時同步附上 API_MANUAL、部署手冊、資料庫維護手冊與版本 release notes。", { left: 76, top: 476, width: 1080, height: 34 }, { size: 24, bold: true, color: C.accent });
}

async function main() {
  await fs.mkdir(PREVIEW, { recursive: true });
  const p = Presentation.create({ slideSize: { width: W, height: H } });
  cover(p);
  agenda(p);
  roleMatrix(p);
  flowSlide(p);
  dashboard(p);
  devices(p);
  deviceDetail(p);
  topology(p);
  await imageSlide(p, "IoT 頁面範例顯示感測值與整合設定", "D:/claude-sandbox/nms_server/nms-iot-page-v1246.png", "IoT 頁面包含感測卡片、設備清單、近期量測、forward queue、ingest 測試與 iframe token 設定。");
  cameras(p);
  await imageSlide(p, "監看模式適合現場大螢幕與行動巡檢", "D:/claude-sandbox/nms_server/monitor-mobile-qa.png", "監看模式聚焦即時狀態，不需要進入完整管理後台。");
  accessControl(p);
  pdu(p);
  iot(p);
  logs(p);
  reports(p);
  admin(p);
  integrations(p);
  await imageSlide(p, "iframe embed 讓外部入口取得只讀視圖", "D:/claude-sandbox/nms_server/nms-embed-iot-v1246.png", "Embed token 可限制 view 與有效時間，並可從管理介面撤銷。");
  mobile(p);
  maintenance(p);
  troubleshooting(p);
  close(p);

  for (const [i, slide] of p.slides.items.entries()) {
    const stem = `slide-${String(i + 1).padStart(2, "0")}`;
    await writeBlob(path.join(PREVIEW, `${stem}.png`), await p.export({ slide, format: "png", scale: 1 }));
    await fs.writeFile(path.join(PREVIEW, `${stem}.layout.json`), await (await slide.export({ format: "layout" })).text(), "utf8");
  }
  await writeBlob(path.join(PREVIEW, "montage.webp"), await p.export({ format: "webp", montage: true, scale: 1 }));
  const pptx = await PresentationFile.exportPptx(p);
  await pptx.save(OUT);
  console.log(OUT);
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
