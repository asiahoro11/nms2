import fs from "node:fs/promises";
import path from "node:path";
import { Presentation, PresentationFile } from "@oai/artifact-tool";

const ROOT = "D:/claude-sandbox/nms_server";
const OUT_DIR = `${ROOT}/outputs/ppt_work/iot_operation_manual/tmp/preview`;
const FINAL = `${ROOT}/outputs/iot-operation-manual.pptx`;
await fs.mkdir(OUT_DIR, { recursive: true });
await fs.mkdir(path.dirname(FINAL), { recursive: true });

const W = 1280;
const H = 720;
const deck = Presentation.create({ slideSize: { width: W, height: H } });

const colors = {
  bg: "#08111f",
  panel: "#101b2c",
  panel2: "#15243a",
  ink: "#f8fafc",
  muted: "#b9c2d3",
  line: "#2a3b55",
  accent: "#7c5cff",
  accent2: "#14b8a6",
  warn: "#f59e0b",
  danger: "#ef4444",
  white: "#ffffff",
};

const page = { left: 56, top: 48, width: 1168, height: 624 };

async function imageBlob(rel) {
  const bytes = await fs.readFile(path.join(ROOT, rel));
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
}

function addBg(slide) {
  slide.background.fill = colors.bg;
  slide.shapes.add({
    geometry: "rect",
    position: { left: 0, top: 0, width: W, height: H },
    fill: colors.bg,
    line: { style: "solid", fill: "none", width: 0 },
  });
  slide.shapes.add({
    geometry: "rect",
    position: { left: 0, top: 0, width: W, height: 12 },
    fill: colors.accent,
    line: { style: "solid", fill: "none", width: 0 },
  });
}

function text(slide, value, x, y, w, h, style = {}) {
  const shape = slide.shapes.add({
    geometry: "textbox",
    position: { left: x, top: y, width: w, height: h },
    fill: "none",
    line: { style: "solid", fill: "none", width: 0 },
  });
  shape.text = value;
  shape.text.style = {
    fontFace: "Microsoft JhengHei",
    fontSize: style.fontSize ?? 20,
    color: style.color ?? colors.ink,
    bold: style.bold ?? false,
    alignment: style.alignment ?? "left",
  };
  return shape;
}

function title(slide, value, sub = "") {
  text(slide, value, page.left, 50, 800, 58, { fontSize: 35, bold: true });
  if (sub) text(slide, sub, page.left, 108, 900, 34, { fontSize: 18, color: colors.muted });
}

function pill(slide, value, x, y, w, fill = colors.panel2) {
  slide.shapes.add({
    geometry: "roundRect",
    position: { left: x, top: y, width: w, height: 34 },
    fill,
    line: { style: "solid", fill: colors.line, width: 1 },
    borderRadius: "rounded-xl",
  });
  text(slide, value, x + 14, y + 7, w - 28, 20, { fontSize: 14, bold: true, color: colors.ink });
}

function card(slide, x, y, w, h, heading, body, accent = colors.accent2) {
  slide.shapes.add({
    geometry: "roundRect",
    position: { left: x, top: y, width: w, height: h },
    fill: colors.panel,
    line: { style: "solid", fill: colors.line, width: 1 },
    borderRadius: "rounded-xl",
  });
  slide.shapes.add({
    geometry: "rect",
    position: { left: x, top: y, width: 6, height: h },
    fill: accent,
    line: { style: "solid", fill: "none", width: 0 },
  });
  text(slide, heading, x + 22, y + 18, w - 42, 30, { fontSize: 22, bold: true });
  text(slide, body, x + 22, y + 58, w - 42, h - 72, { fontSize: 16, color: colors.muted });
}

function bulletList(slide, items, x, y, w, gap = 44, fontSize = 19) {
  items.forEach((item, i) => {
    const yy = y + i * gap;
    slide.shapes.add({
      geometry: "ellipse",
      position: { left: x, top: yy + 6, width: 11, height: 11 },
      fill: colors.accent2,
      line: { style: "solid", fill: "none", width: 0 },
    });
    text(slide, item, x + 26, yy, w - 26, gap - 4, { fontSize, color: colors.ink });
  });
}

function row(slide, y, cols, values, opts = {}) {
  let x = page.left;
  values.forEach((v, i) => {
    const w = cols[i];
    slide.shapes.add({
      geometry: "rect",
      position: { left: x, top: y, width: w, height: opts.h ?? 48 },
      fill: opts.header ? colors.accent : (i % 2 === 0 ? colors.panel : colors.panel2),
      line: { style: "solid", fill: colors.line, width: 1 },
    });
    text(slide, v, x + 10, y + 8, w - 20, (opts.h ?? 48) - 12, {
      fontSize: opts.header ? 16 : 14,
      bold: opts.header,
      color: colors.ink,
    });
    x += w;
  });
}

function footer(slide, n) {
  text(slide, "NMS IoT / Modbus 操作手冊", page.left, 682, 360, 20, { fontSize: 12, color: "#7f8aa3" });
  text(slide, String(n).padStart(2, "0"), 1190, 676, 40, 24, { fontSize: 14, color: "#7f8aa3", alignment: "right" });
}

function newSlide(n, heading, sub = "") {
  const slide = deck.slides.add();
  addBg(slide);
  if (heading) title(slide, heading, sub);
  footer(slide, n);
  return slide;
}

// 1
{
  const s = newSlide(1, "");
  text(s, "IoT / Modbus", 78, 110, 760, 72, { fontSize: 58, bold: true });
  text(s, "圖文操作手冊", 82, 190, 520, 42, { fontSize: 35, bold: true, color: colors.accent2 });
  text(s, "給現場工程師與系統管理員的實際操作導覽", 82, 260, 720, 36, { fontSize: 24, color: colors.muted });
  card(s, 82, 350, 500, 145, "本手冊要完成的事", "從登入、授權確認、新增 Modbus 設備，到讀值確認、資料轉發與 iframe 嵌入。每個設定欄位都會說明用途與常見填法。", colors.accent);
  s.images.add({
    blob: await imageBlob("nms-iot-page-v1246.png"),
    contentType: "image/png",
    alt: "IoT module screenshot",
    fit: "cover",
    position: { left: 700, top: 92, width: 470, height: 480 },
    geometry: "roundRect",
    borderRadius: "rounded-xl",
  });
}

// 2
{
  const s = newSlide(2, "先確認授權，再開始接設備", "未啟用 IoT License 時，頁面只會顯示提示，無法新增或輪詢設備。");
  s.images.add({
    blob: await imageBlob("outputs/manual_screens/08_iot.png"),
    contentType: "image/png",
    alt: "IoT license notice",
    fit: "contain",
    position: { left: 72, top: 160, width: 690, height: 390 },
    geometry: "roundRect",
    borderRadius: "rounded-xl",
  });
  card(s, 805, 176, 355, 92, "誰可以操作", "Admin / Editor 可以進入 IoT 模組；Viewer 不做設備設定。", colors.accent2);
  card(s, 805, 288, 355, 108, "授權狀態", "看到 License 提示時，先到系統管理匯入包含 iot 功能的 License。", colors.warn);
  card(s, 805, 416, 355, 110, "授權後畫面", "會顯示狀態卡、感測器讀值、設備清單、最近量測與設定區。", colors.accent);
}

// 3
{
  const s = newSlide(3, "主畫面先看三件事", "進入 IoT 頁後，先判斷設備數、讀值狀態與轉發佇列。");
  card(s, 78, 170, 345, 250, "1. IoT Status", "Devices 顯示已建立設備數；Enabled 顯示會被輪詢的設備數；Measurements 顯示已收到或輪詢到的資料量。", colors.accent);
  card(s, 468, 170, 345, 250, "2. Sensor Readings", "用卡片看每個感測器的最新值、單位、協定、Unit ID 與 Online / Offline / Error 狀態。", colors.accent2);
  card(s, 858, 170, 345, 250, "3. Forward Queue", "Pending 增加代表資料正在等待轉送；Failed 增加代表 webhook、Token 或對方服務需檢查。", colors.warn);
  bulletList(s, [
    "新增設備後先按 Poll，不必等下一輪排程。",
    "數值正常但 pending 累積時，問題通常在外部接收端。",
    "Error 狀態優先看 Last error，再調整通訊或暫存器設定。"
  ], 112, 485, 1060, 44, 19);
}

// 4
{
  const s = newSlide(4, "選對通訊方式，後面的設定才會準", "新增設備視窗提供四種 Modbus 進線方式。");
  card(s, 76, 162, 255, 230, "Modbus TCP", "設備有 Ethernet，NMS 直接用 IP + Port 輪詢。最常見 Port 是 502。", colors.accent);
  card(s, 362, 162, 255, 230, "Modbus RTU", "NMS 主機透過 COM Port 或 USB-RS485 轉接器讀取現場設備。", colors.accent2);
  card(s, 648, 162, 255, 230, "Modbus RS485", "半雙工匯流排，多台設備共線，重點是 Unit ID 不能重複。", colors.warn);
  card(s, 934, 162, 255, 230, "RTU over TCP", "透明串口伺服器，走 TCP 但封包仍是 RTU frame，不是標準 Modbus TCP。", colors.danger);
  text(s, "現場判斷", 86, 455, 180, 28, { fontSize: 24, bold: true });
  bulletList(s, [
    "設備直接給 IP：選 Modbus TCP。",
    "設備接在 USB/COM 轉接器：選 Modbus RTU 或 RS485。",
    "設備接在 RS485 轉乙太網透明轉換器：多半選 RTU over TCP。"
  ], 90, 500, 1080, 40, 18);
}

// 5
{
  const s = newSlide(5, "設備名稱與感測器類型決定日後維護效率", "先把位置、用途、指標命名清楚，後面查表與串接會輕鬆很多。");
  row(s, 168, [240, 410, 430], ["欄位", "怎麼填", "為什麼重要"], { header: true, h: 44 });
  row(s, 212, [240, 410, 430], ["Device name", "機房溫濕度-01、UPS Room CO2", "列表、卡片、告警與維修時都靠它辨識設備"], { h: 70 });
  row(s, 282, [240, 410, 430], ["Sensor type", "Temperature、Humidity、Power、CO2", "會影響顯示單位；溫濕度會套用常用預設值"], { h: 70 });
  row(s, 352, [240, 410, 430], ["Metric name", "temperature、humidity、voltage", "會寫入量測資料與 forward payload，建議用英文小寫"], { h: 70 });
  row(s, 422, [240, 410, 430], ["Enabled", "預設啟用", "只有啟用設備會被背景輪詢"], { h: 70 });
  card(s, 112, 535, 1010, 70, "現場命名建議", "設備名稱包含「位置 + 量測用途 + 編號」，例如 2F-UPS-Room-TempHumi-01。", colors.accent2);
}

// 6
{
  const s = newSlide(6, "TCP 類設備只要把網路目標填準", "Modbus TCP 與 RTU over TCP 都會顯示 Host / IP 與 Port。");
  row(s, 170, [250, 360, 470], ["設定", "常見值", "操作說明"], { header: true, h: 44 });
  row(s, 214, [250, 360, 470], ["Host / IP", "192.168.1.100", "填設備或透明轉換器 IP；NMS 主機必須連得到。"], { h: 76 });
  row(s, 290, [250, 360, 470], ["Port", "502、4001、8899", "Modbus TCP 常用 502；透明串口伺服器依廠牌設定。"], { h: 76 });
  row(s, 366, [250, 360, 470], ["Unit ID", "1 至 247", "即使 TCP 也常需要 Slave ID；閘道後面有多台設備時更重要。"], { h: 76 });
  row(s, 442, [250, 360, 470], ["Poll interval", "60 秒", "設備多時不要設太短，避免匯流排或設備負載過高。"], { h: 76 });
  pill(s, "先 ping IP，再測 Port，最後才調暫存器。", 300, 562, 680, colors.panel2);
}

// 7
{
  const s = newSlide(7, "RTU / RS485 的重點是序列埠參數一致", "只要 baud、parity、stop bits 不一致，就很容易 timeout 或 CRC 錯誤。");
  row(s, 165, [230, 260, 590], ["設定", "常見值", "現場提醒"], { header: true, h: 44 });
  row(s, 209, [230, 260, 590], ["Serial port", "COM3、/dev/ttyUSB0", "Windows 同一時間通常只能一個程式開啟同一個 COM Port。"], { h: 72 });
  row(s, 281, [230, 260, 590], ["Baud rate", "9600、19200", "需與設備 DIP switch 或手冊一致。"], { h: 72 });
  row(s, 353, [230, 260, 590], ["Data bits", "8", "多數 Modbus RTU 是 8。"], { h: 72 });
  row(s, 425, [230, 260, 590], ["Parity", "N、E、O", "None / Even / Odd 必須完全一致。"], { h: 72 });
  row(s, 497, [230, 260, 590], ["Stop bits", "1、2", "少數設備使用 2，設定錯誤會造成讀不到。"], { h: 72 });
}

// 8
{
  const s = newSlide(8, "暫存器設定決定讀到哪一個值", "看設備手冊時，要把位址、功能碼、資料型別分開判讀。");
  row(s, 164, [230, 300, 550], ["設定", "用途", "注意事項"], { header: true, h: 44 });
  row(s, 208, [230, 300, 550], ["Register address", "起始暫存器位址", "手冊寫 40001 時常填 0；寫 30002 時常填 1。"], { h: 78 });
  row(s, 286, [230, 300, 550], ["Function code", "決定讀取區域", "FC03 是 Holding Register；FC04 是 Input Register。"], { h: 78 });
  row(s, 364, [230, 300, 550], ["Quantity", "讀取暫存器數", "16-bit 通常 1；32-bit 或 float32 需要 2。"], { h: 78 });
  row(s, 442, [230, 300, 550], ["Unit ID", "設備站號", "RS485 匯流排上每台設備不可重複。"], { h: 78 });
  card(s, 170, 555, 920, 70, "位址差一格很常見", "如果值一直不合理，先確認手冊是 0-based address 還是 1-based register number。", colors.warn);
}

// 9
{
  const s = newSlide(9, "資料型別與位元組順序負責把 raw data 變成正確數字", "讀到資料不代表數值一定正確；型別、倍率與 endian 都會影響解析。");
  row(s, 158, [230, 260, 590], ["設定", "常見選項", "什麼時候調整"], { header: true, h: 44 });
  row(s, 202, [230, 260, 590], ["Data type", "uint16、int16、float32", "依手冊選；負溫度通常是 int16，電表常見 float32。"], { h: 72 });
  row(s, 274, [230, 260, 590], ["Scale", "1、0.1、0.01", "數值差 10 倍或 100 倍時先改這裡。"], { h: 72 });
  row(s, 346, [230, 260, 590], ["Offset", "0、-2、+1.5", "用於校正固定偏差。"], { h: 72 });
  row(s, 418, [230, 260, 590], ["Byte order", "Big / Little", "16-bit 內的 byte 順序錯時調整。"], { h: 72 });
  row(s, 490, [230, 260, 590], ["Word order", "Big / Little", "float32 或 32-bit 數值亂跳時調整。"], { h: 72 });
}

// 10
{
  const s = newSlide(10, "範例：新增一台溫濕度感測器", "把設備手冊資料轉成 NMS 欄位。");
  row(s, 160, [270, 350, 460], ["設備資料", "填入值", "原因"], { header: true, h: 44 });
  row(s, 204, [270, 350, 460], ["通訊方式", "Modbus TCP", "設備可直接用 IP 存取。"], { h: 60 });
  row(s, 264, [270, 350, 460], ["IP / Port", "192.168.10.50 / 502", "標準 Modbus TCP。"], { h: 60 });
  row(s, 324, [270, 350, 460], ["Unit ID", "1", "設備站號。"], { h: 60 });
  row(s, 384, [270, 350, 460], ["Address / FC", "0 / FC04", "從第一個 Input Register 讀溫濕度。"], { h: 60 });
  row(s, 444, [270, 350, 460], ["Data type / Scale", "int16 / 0.1", "原始值 253 代表 25.3。"], { h: 60 });
  row(s, 504, [270, 350, 460], ["Poll interval", "60 秒", "現場維運足夠且不會過度輪詢。"], { h: 60 });
}

// 11
{
  const s = newSlide(11, "新增後先按 Poll，立刻確認讀值", "不要等排程；新增成功後當場驗證連線、解析與資料寫入。");
  bulletList(s, [
    "在設備清單找到剛新增的感測器。",
    "按 Poll 立即讀取一次。",
    "看 Last value 是否出現合理數值。",
    "看 Last seen 是否更新為剛剛。",
    "若顯示 Error，先讀 Last error 再調整設定。"
  ], 90, 170, 560, 58, 22);
  card(s, 720, 170, 420, 120, "Online", "最近一次輪詢成功，卡片與清單都會顯示最新值。", colors.accent2);
  card(s, 720, 320, 420, 120, "Offline", "尚未成功讀值，通常是新設備還沒 Poll 或連線未通。", colors.warn);
  card(s, 720, 470, 420, 120, "Error", "最近一次輪詢失敗，檢查 timeout、CRC、位址或資料型別。", colors.danger);
}

// 12
{
  const s = newSlide(12, "REST Ingest 是給外部閘道推資料用", "MQTT、OPC-UA、BACnet 目前透過 bridge/gateway 正規化後推進 NMS。");
  row(s, 160, [240, 360, 480], ["欄位", "範例", "功能"], { header: true, h: 44 });
  row(s, 204, [240, 360, 480], ["External ID", "demo-sensor-1", "外部來源的固定識別碼。"], { h: 66 });
  row(s, 270, [240, 360, 480], ["Name", "Demo Sensor", "在 NMS 顯示的來源名稱。"], { h: 66 });
  row(s, 336, [240, 360, 480], ["Protocol", "rest、mqtt、opcua", "標記資料來源協定。"], { h: 66 });
  row(s, 402, [240, 360, 480], ["Metric", "temperature", "寫入量測資料的指標名稱。"], { h: 66 });
  row(s, 468, [240, 360, 480], ["Value", "25.5", "本次推送的數值。"], { h: 66 });
  pill(s, "送出後應該在最近量測表看到一筆新資料。", 314, 570, 650, colors.panel2);
}

// 13
{
  const s = newSlide(13, "Protocol Capabilities 告訴你每種接入方式的成熟度", "Ready 代表可直接使用；Bridge Ready 代表要透過外部 gateway。");
  row(s, 152, [250, 220, 610], ["協定", "狀態", "現場用途"], { header: true, h: 44 });
  row(s, 196, [250, 220, 610], ["Modbus TCP / RTU / RS485", "Ready", "NMS 主動輪詢設備或匯流排。"], { h: 64 });
  row(s, 260, [250, 220, 610], ["RTU over TCP", "Ready", "透明串口伺服器，TCP 內承載 RTU frame。"], { h: 64 });
  row(s, 324, [250, 220, 610], ["REST / Webhook", "Ready", "外部系統主動推量測資料。"], { h: 64 });
  row(s, 388, [250, 220, 610], ["HTTP Forward Queue", "Ready", "NMS 統一收資料後再轉送客戶系統。"], { h: 64 });
  row(s, 452, [250, 220, 610], ["MQTT / OPC-UA / BACnet", "Bridge Ready", "需要 gateway 將資料轉成 REST ingest。"], { h: 74 });
}

// 14
{
  const s = newSlide(14, "HTTP Forward Queue 把 NMS 量測資料送到客戶系統", "適合由 NMS 集中讀取 Modbus，再用 webhook 統一交付資料。");
  row(s, 158, [240, 300, 540], ["設定", "建議值", "功能"], { header: true, h: 44 });
  row(s, 202, [240, 300, 540], ["Enabled", "測通後再開啟", "控制是否啟用轉發。"], { h: 70 });
  row(s, 272, [240, 300, 540], ["URL", "客戶 webhook URL", "對方必須可接收 POST。"], { h: 70 });
  row(s, 342, [240, 300, 540], ["Token", "Bearer Token", "若對方 API 需要授權，填入 token。"], { h: 70 });
  row(s, 412, [240, 300, 540], ["Batch size", "50", "每批最多轉送筆數。"], { h: 70 });
  row(s, 482, [240, 300, 540], ["Flush Queue", "測試時手動按", "立即送出 pending 資料。"], { h: 70 });
}

// 15
{
  const s = newSlide(15, "轉發狀態可以快速定位問題在哪裡", "讀值成功不代表客戶系統已收到；要看 forward status。");
  card(s, 100, 175, 300, 210, "pending", "資料已存入 NMS，等待背景轉送或手動 Flush Queue。若長時間增加，檢查 Enabled 與 URL。", colors.warn);
  card(s, 490, 175, 300, 210, "sent", "NMS 已送出，且對方回應 2xx。這代表串接路徑正常。", colors.accent2);
  card(s, 880, 175, 300, 210, "failed", "NMS 送出失敗。檢查網路、URL、Token、對方服務與錯誤訊息。", colors.danger);
  bulletList(s, [
    "新增設備後：先確認 Measurements 增加。",
    "串接客戶系統時：再確認 pending 會轉成 sent。",
    "外部 API 維護期間：pending 可保留，恢復後再 Flush。"
  ], 140, 465, 980, 44, 19);
}

// 16
{
  const s = newSlide(16, "iframe Token 提供只讀嵌入，不是管理入口", "客戶入口網站可以嵌入 IoT 檢視，但 Token 要控管有效期與網域。");
  s.images.add({
    blob: await imageBlob("nms-embed-iot-v1246.png"),
    contentType: "image/png",
    alt: "IoT embed screenshot",
    fit: "contain",
    position: { left: 74, top: 156, width: 590, height: 386 },
    geometry: "roundRect",
    borderRadius: "rounded-xl",
  });
  card(s, 725, 165, 405, 92, "Token name", "用容易辨識的名稱，例如 Customer Portal - IoT。", colors.accent);
  card(s, 725, 278, 405, 92, "View", "只嵌入 IoT 時選 iot，避免給過多檢視。", colors.accent2);
  card(s, 725, 391, 405, 92, "Expiry", "測試用短時間，正式依資安規範設定。", colors.warn);
  card(s, 725, 504, 405, 78, "Revoke", "Token 外洩或不再使用時立即撤銷。", colors.danger);
}

// 17
{
  const s = newSlide(17, "iframe Allowlist 控制誰可以嵌入 NMS", "Token 控制內容權限，Allowlist 控制來源網站。兩者都要設定。");
  card(s, 92, 174, 500, 135, "frame ancestors", "加入允許嵌入 NMS 的來源，例如 'self' 或 https://portal.example.com。", colors.accent);
  card(s, 92, 340, 500, 135, "Reload / Save", "修改前先 Reload 確認現值，修改後 Save 並重新測試客戶入口。", colors.accent2);
  card(s, 690, 174, 430, 135, "安全建議", "使用 HTTPS、避免萬用字元、不要把 Token URL 放在公開文件。", colors.warn);
  card(s, 690, 340, 430, 135, "空白頁排查", "若 iframe 空白，檢查 Token 是否過期，以及瀏覽器 console 是否被 CSP / frame-ancestors 擋下。", colors.danger);
}

// 18
{
  const s = newSlide(18, "常見異常通常能從錯誤型態反推設定", "先看現象，再回到對應欄位調整。");
  row(s, 150, [300, 330, 450], ["現象", "可能原因", "處理方式"], { header: true, h: 44 });
  row(s, 194, [300, 330, 450], ["Poll timeout", "網路不通或序列參數錯", "Ping、Port、baud、parity、stop bits。"], { h: 58 });
  row(s, 252, [300, 330, 450], ["CRC mismatch", "RTU 線路或參數不符", "檢查 A/B 線、終端電阻與 parity。"], { h: 58 });
  row(s, 310, [300, 330, 450], ["數值差 10 倍", "Scale 設錯", "依手冊改成 0.1 或 0.01。"], { h: 58 });
  row(s, 368, [300, 330, 450], ["float32 亂跳", "Byte / Word order 不符", "切換 endian 組合測試。"], { h: 58 });
  row(s, 426, [300, 330, 450], ["Forward pending 增加", "webhook 沒通", "檢查 Enabled、URL、Token、對方服務。"], { h: 58 });
  row(s, 484, [300, 330, 450], ["iframe 空白", "Token 或 allowlist 問題", "重新產生 Token 並加入來源網域。"], { h: 58 });
}

// 19
{
  const s = newSlide(19, "交付前用這張表跑一次現場檢查", "確定資料真的讀得到、送得出、嵌得進。");
  bulletList(s, [
    "IoT License 已啟用，Admin / Editor 可操作。",
    "每個設備名稱包含位置、用途與編號。",
    "RTU / RS485 的 Unit ID 不重複。",
    "新增後已按 Poll，Last value 與 Last seen 正常。",
    "Scale、Offset、Byte order、Word order 已校正。",
    "最近量測會持續增加。",
    "HTTP Forward 顯示 sent，iframe Token 與 Allowlist 已測通。"
  ], 112, 162, 1020, 58, 21);
}

// 20
{
  const s = newSlide(20, "完成後，NMS 就是現場 IoT 數據的入口", "設備讀值、歷史量測、轉發佇列與嵌入檢視都由同一個 IoT / Modbus 模組管理。");
  card(s, 120, 180, 310, 230, "讀得到", "用 Modbus TCP / RTU / RS485 / RTU over TCP 直接輪詢現場設備。", colors.accent);
  card(s, 485, 180, 310, 230, "看得懂", "用 Sensor type、Metric、Scale、Offset 與 endian 設定把 raw data 轉成工程值。", colors.accent2);
  card(s, 850, 180, 310, 230, "送得出", "用 REST ingest、HTTP Forward Queue 與 iframe Token 串接客戶系統。", colors.warn);
  text(s, "最重要的現場習慣：新增後立刻 Poll，讀值合理後才交付。", 190, 505, 900, 42, { fontSize: 28, bold: true, alignment: "center" });
}

async function writeBlob(file, blob) {
  await fs.writeFile(file, new Uint8Array(await blob.arrayBuffer()));
}

for (const [index, slide] of deck.slides.items.entries()) {
  const stem = `slide-${String(index + 1).padStart(2, "0")}`;
  await writeBlob(`${OUT_DIR}/${stem}.png`, await deck.export({ slide, format: "png", scale: 1 }));
  const layout = await slide.export({ format: "layout" });
  await fs.writeFile(`${OUT_DIR}/${stem}.layout.json`, await layout.text());
}
await writeBlob(`${OUT_DIR}/montage.webp`, await deck.export({ format: "webp", montage: true, scale: 1 }));

const pptx = await PresentationFile.exportPptx(deck);
await pptx.save(FINAL);
console.log(JSON.stringify({ final: FINAL, slides: deck.slides.items.length, preview: OUT_DIR }, null, 2));
