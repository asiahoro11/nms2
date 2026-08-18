import fs from "node:fs/promises";
import path from "node:path";
import { Presentation, PresentationFile } from "@oai/artifact-tool";

const ROOT = "D:/claude-sandbox/nms_server";
const OUT_DIR = `${ROOT}/outputs/ppt_work/iot_operation_manual/page_first_preview`;
const FINAL = `${ROOT}/outputs/iot-operation-manual.pptx`;
await fs.mkdir(OUT_DIR, { recursive: true });

const W = 1280;
const H = 720;
const deck = Presentation.create({ slideSize: { width: W, height: H } });

const C = {
  bg: "#08111f",
  panel: "#101b2c",
  panel2: "#15243a",
  ink: "#f8fafc",
  muted: "#b9c2d3",
  line: "#2a3b55",
  accent: "#7c5cff",
  teal: "#14b8a6",
  amber: "#f59e0b",
  red: "#ef4444",
};

async function img(rel) {
  const bytes = await fs.readFile(path.join(ROOT, rel));
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
}

function bg(slide) {
  slide.background.fill = C.bg;
  slide.shapes.add({ geometry: "rect", position: { left: 0, top: 0, width: W, height: H }, fill: C.bg, line: { style: "solid", fill: "none", width: 0 } });
  slide.shapes.add({ geometry: "rect", position: { left: 0, top: 0, width: W, height: 10 }, fill: C.accent, line: { style: "solid", fill: "none", width: 0 } });
}

function tx(slide, value, left, top, width, height, style = {}) {
  const s = slide.shapes.add({
    geometry: "textbox",
    position: { left, top, width, height },
    fill: "none",
    line: { style: "solid", fill: "none", width: 0 },
  });
  s.text = value;
  s.text.style = {
    fontFace: "Microsoft JhengHei",
    fontSize: style.fontSize ?? 18,
    bold: style.bold ?? false,
    color: style.color ?? C.ink,
    alignment: style.alignment ?? "left",
  };
  return s;
}

function header(slide, title, subtitle = "") {
  tx(slide, title, 56, 44, 850, 48, { fontSize: 35, bold: true });
  if (subtitle) tx(slide, subtitle, 56, 94, 980, 28, { fontSize: 17, color: C.muted });
}

function footer(slide, n) {
  tx(slide, "IoT / Modbus 圖文操作手冊", 56, 684, 360, 20, { fontSize: 12, color: "#7f8aa3" });
  tx(slide, String(n).padStart(2, "0"), 1184, 678, 40, 24, { fontSize: 14, color: "#7f8aa3", alignment: "right" });
}

function newSlide(n, title = "", subtitle = "") {
  const slide = deck.slides.add();
  bg(slide);
  if (title) header(slide, title, subtitle);
  footer(slide, n);
  return slide;
}

function card(slide, x, y, w, h, title, body, accent = C.teal) {
  slide.shapes.add({ geometry: "roundRect", position: { left: x, top: y, width: w, height: h }, fill: C.panel, line: { style: "solid", fill: C.line, width: 1 }, borderRadius: "rounded-xl" });
  slide.shapes.add({ geometry: "rect", position: { left: x, top: y, width: 6, height: h }, fill: accent, line: { style: "solid", fill: "none", width: 0 } });
  tx(slide, title, x + 22, y + 16, w - 40, 28, { fontSize: 22, bold: true });
  tx(slide, body, x + 22, y + 54, w - 40, h - 66, { fontSize: 16, color: C.muted });
}

function bullet(slide, items, x, y, w, gap = 44, fontSize = 19) {
  items.forEach((item, i) => {
    const yy = y + i * gap;
    slide.shapes.add({ geometry: "ellipse", position: { left: x, top: yy + 7, width: 10, height: 10 }, fill: C.teal, line: { style: "solid", fill: "none", width: 0 } });
    tx(slide, item, x + 24, yy, w - 24, gap - 2, { fontSize, color: C.ink });
  });
}

function table(slide, x, y, cols, rows, rowH = 52) {
  let yy = y;
  for (const [ri, row] of rows.entries()) {
    let xx = x;
    for (const [ci, value] of row.entries()) {
      const w = cols[ci];
      slide.shapes.add({
        geometry: "rect",
        position: { left: xx, top: yy, width: w, height: ri === 0 ? 42 : rowH },
        fill: ri === 0 ? C.accent : (ci % 2 === 0 ? C.panel : C.panel2),
        line: { style: "solid", fill: C.line, width: 1 },
      });
      tx(slide, value, xx + 10, yy + 8, w - 20, (ri === 0 ? 42 : rowH) - 8, { fontSize: ri === 0 ? 15 : 14, bold: ri === 0 });
      xx += w;
    }
    yy += ri === 0 ? 42 : rowH;
  }
}

async function screenshotSlide(n, title, subtitle, rel, caption) {
  const s = newSlide(n, title, subtitle);
  s.images.add({
    blob: await img(rel),
    contentType: "image/png",
    alt: title,
    fit: "contain",
    position: { left: 56, top: 132, width: 1168, height: 510 },
    geometry: "roundRect",
    borderRadius: "rounded-xl",
  });
  tx(s, caption, 80, 646, 1050, 22, { fontSize: 15, color: C.muted });
}

// 1
{
  const s = newSlide(1);
  tx(s, "IoT / Modbus", 78, 120, 760, 72, { fontSize: 58, bold: true });
  tx(s, "功能頁面截圖式操作手冊", 82, 200, 720, 46, { fontSize: 34, bold: true, color: C.teal });
  tx(s, "每個功能先看真實頁面，再看該頁面的功能細項與現場操作重點。", 82, 270, 870, 36, { fontSize: 22, color: C.muted });
  card(s, 86, 370, 500, 132, "使用帳號", "以 yoyo / admin 權限登入 10.100.100.11:8080 擷取 IoT 功能頁面。", C.accent);
  card(s, 650, 370, 500, 132, "閱讀方式", "看到頁面後，下一張投影片會逐項說明該頁可操作的按鈕、欄位與判斷方式。", C.teal);
}

await screenshotSlide(2, "功能頁面：IoT 首頁總覽", "摘要統計、支援協定與閘道測試集中在這一頁。", "outputs/iot_manual_screens/10_iot_overview_page.png", "截圖來源：10.100.100.11:8080，登入使用者 yoyo。");

// 3
{
  const s = newSlide(3, "功能細項：IoT 首頁總覽", "這一頁用來判斷模組是否正常、協定是否支援，以及外部閘道是否能推資料。");
  card(s, 70, 150, 345, 180, "摘要統計", "設備總數、已啟用、量測筆數、待發送、發送失敗與已發送暫存。現場先看這裡確認資料流是否有進來。", C.accent);
  card(s, 468, 150, 345, 180, "支援協定", "列出 Modbus TCP、RTU、RS485、RTU over TCP、REST/Webhook、HTTP Forward Queue 等能力與狀態。", C.teal);
  card(s, 866, 150, 345, 180, "閘道器測試", "用 REST / MQTT / OPC-UA / BACnet gateway ingest 模擬外部系統推值進 NMS。", C.amber);
  table(s, 90, 388, [250, 380, 480], [
    ["操作項目", "何時使用", "注意事項"],
    ["刷新", "設備或量測資料更新後", "只更新 IoT 頁資料，不需重新登入"],
    ["新增設備", "要建立 Modbus 輪詢點位", "Admin 才能操作"],
    ["Send Ingest", "測試外部 gateway 推送", "成功後最近量測應新增一筆資料"],
  ], 58);
}

await screenshotSlide(4, "功能頁面：線上模組總覽", "此頁以卡片方式顯示已啟用感測器的最新讀值。", "outputs/iot_manual_screens/11_iot_online_module_page.png", "目前測試環境尚未建立設備，因此畫面顯示尚未設定任何設備。");

// 5
{
  const s = newSlide(5, "功能細項：線上模組總覽", "新增設備並成功 Poll 後，這一頁會變成現場感測器儀表板。");
  card(s, 92, 150, 330, 180, "感測器卡片", "顯示設備名稱、協定、Unit ID、最新值與單位。溫濕度類型會分別顯示溫度與濕度。", C.teal);
  card(s, 475, 150, 330, 180, "狀態判斷", "Online 代表最近一次輪詢成功；Offline 代表尚未有成功讀值；Error 代表最近一次輪詢失敗。", C.amber);
  card(s, 858, 150, 330, 180, "現場用途", "巡檢時先看這頁，快速判斷哪些感測器有值、哪些設備需要檢查。", C.accent);
  bullet(s, [
    "新增設備後先按 Poll，確認卡片出現合理數值。",
    "若卡片一直空白，回到設備管理檢查 Last error。",
    "若數值差 10 倍或 100 倍，優先調整 Scale。"
  ], 126, 420, 980, 50, 21);
}

await screenshotSlide(6, "功能頁面：設備管理", "設備清單與最近量測表集中在同一個分頁。", "outputs/iot_manual_screens/12_iot_devices_page.png", "上方表格是設備設定，下方表格是量測紀錄。");

// 7
{
  const s = newSlide(7, "功能細項：設備管理", "這一頁是日常維護點位的主要工作區。");
  table(s, 72, 150, [210, 360, 530], [
    ["欄位 / 按鈕", "用途", "現場判斷"],
    ["名稱 / 類型 / 協定", "辨識設備與通訊方式", "名稱建議包含位置與用途"],
    ["目標 / 暫存器", "顯示 IP、Port、Serial Port、Register", "讀不到時先檢查這些設定"],
    ["指標 / 最後數值", "顯示 metric 與解析後數值", "數值不合理時調 Data type、Scale、Endian"],
    ["最後上線", "最近成功讀值時間", "長時間未更新代表輪詢失敗或設備離線"],
    ["操作", "Poll、Edit、Delete", "新增後先按 Poll 做現場驗證"],
  ], 62);
}

await screenshotSlide(8, "功能頁面：整合設定", "HTTP 轉發、iframe Token、iframe Allowlist 都在整合設定頁。", "outputs/iot_manual_screens/13_iot_integration_page.png", "此頁主要給系統管理員設定資料輸出與外部入口嵌入。");

// 9
{
  const s = newSlide(9, "功能細項：整合設定", "這一頁負責把 IoT 資料送出去，或讓外部入口嵌入唯讀頁面。");
  card(s, 76, 150, 340, 210, "HTTP 轉發佇列", "設定 Webhook URL、Bearer token、批次數量。按立即清空可手動 Flush pending 資料。", C.amber);
  card(s, 470, 150, 340, 210, "iframe 嵌入 Token", "產生只讀 iframe HTML，讓客戶入口可嵌入 dashboard、topology、devices 或 iot 頁面。", C.teal);
  card(s, 864, 150, 340, 210, "iframe 允許清單", "設定允許嵌入 NMS 的來源網站。建議只加入可信任 HTTPS origin。", C.accent);
  table(s, 112, 420, [260, 360, 470], [
    ["狀態", "意思", "處理方式"],
    ["pending", "資料等待轉發", "檢查 URL 後按立即清空"],
    ["sent", "已收到對方 2xx", "代表串接正常"],
    ["failed", "轉發失敗", "檢查 Token、網路與對方 API"],
  ], 56);
}

await screenshotSlide(10, "功能頁面：新增設備 Modbus TCP", "TCP 類設備以 Host / IP 與 Port 作為目標。", "outputs/iot_manual_screens/14_iot_add_device_tcp_modal.png", "新增設備視窗會依分頁切換不同通訊欄位。");

// 11
{
  const s = newSlide(11, "功能細項：新增設備共用欄位", "不論使用哪種協定，這些欄位都會影響讀值與顯示。");
  table(s, 70, 145, [230, 360, 530], [
    ["欄位", "功能", "建議填法"],
    ["設備名稱", "顯示在卡片與清單", "位置 + 用途 + 編號，例如 1F-TempHumi-01"],
    ["感測器類型", "決定顯示單位與部分預設", "溫度、濕度、溫濕度、電壓、電流、CO2"],
    ["指標名稱", "寫入 measurement 與 forward payload", "temperature、humidity、voltage"],
    ["Unit ID", "Modbus Slave ID", "1 至 247，RS485 上不可重複"],
    ["輪詢間隔", "背景 Poll 間隔秒數", "設備多時建議 60 秒以上"],
  ], 62);
}

await screenshotSlide(12, "功能頁面：新增設備 Modbus RTU", "RTU 使用本機序列埠，重點是 baud、parity、stop bits 必須一致。", "outputs/iot_manual_screens/15_iot_add_device_rtu_modal.png", "Windows 常見 COM3；Linux 常見 /dev/ttyUSB0。");

// 13
{
  const s = newSlide(13, "功能細項：RTU / RS485 序列設定", "序列參數錯誤時，最常見現象是 timeout 或 CRC mismatch。");
  table(s, 80, 150, [230, 300, 560], [
    ["欄位", "常見值", "注意事項"],
    ["序列埠", "COM3、/dev/ttyUSB0", "同一個 Port 不要被其他程式占用"],
    ["鮑率", "9600、19200", "必須與設備 DIP switch 或手冊一致"],
    ["資料位元", "8", "大多數 Modbus RTU 使用 8"],
    ["同位檢查", "無、Even、Odd", "設定錯會造成讀不到或 CRC 錯"],
    ["停止位元", "1、2", "多數設備使用 1"],
  ], 62);
}

await screenshotSlide(14, "功能頁面：新增設備 Modbus RS485", "RS485 是現場多點匯流排，設定方式與 RTU 類似。", "outputs/iot_manual_screens/16_iot_add_device_rs485_modal.png", "同一條 RS485 線上，每台設備的 Unit ID 必須不同。");

await screenshotSlide(15, "功能頁面：新增設備 RTU over TCP", "透明串口伺服器走 TCP，但封包仍使用 RTU frame。", "outputs/iot_manual_screens/17_iot_add_device_rtu_tcp_modal.png", "若 RS485-to-Ethernet 轉換器不是 Modbus TCP Gateway，通常使用此模式。");

// 16
{
  const s = newSlide(16, "功能細項：暫存器與資料解析", "這些欄位決定 NMS 讀哪個位址，以及如何把 raw data 轉成工程值。");
  table(s, 70, 142, [220, 350, 540], [
    ["欄位", "功能", "常見判斷"],
    ["暫存器位址", "起始 address", "手冊寫 40001 通常填 0；寫 30002 通常填 1"],
    ["功能碼", "讀取區域", "FC03 讀 Holding Register；FC04 讀 Input Register"],
    ["資料型別", "解析 raw bytes", "uint16、int16、uint32、int32、float32"],
    ["倍率 Scale", "原始值乘數", "值差 10 倍時改 0.1；差 100 倍時改 0.01"],
    ["偏移 Offset", "固定校正", "讀值固定偏高 2，可填 -2"],
    ["Byte / Word order", "Endian 順序", "float32 亂跳時嘗試切換"],
  ], 54);
}

// 17
{
  const s = newSlide(17, "現場操作順序", "照這個流程做，能最快確認問題在設備、設定還是轉發。");
  card(s, 80, 150, 250, 140, "1. 開頁面", "進入 IoT 感測器，確認 License 與功能頁能正常顯示。", C.accent);
  card(s, 370, 150, 250, 140, "2. 新增設備", "選 TCP、RTU、RS485 或 RTU over TCP，填入通訊與暫存器欄位。", C.teal);
  card(s, 660, 150, 250, 140, "3. 立即 Poll", "不要等排程，新增後先手動 Poll 驗證讀值。", C.amber);
  card(s, 950, 150, 250, 140, "4. 看量測", "回設備管理確認 Last value、Last seen 與最近量測資料。", C.accent);
  bullet(s, [
    "讀不到：先檢查 IP / Port / Serial Port / Unit ID。",
    "讀到但值怪：檢查 Register address、Data type、Scale、Byte order、Word order。",
    "讀值正常但客戶收不到：檢查 HTTP 轉發佇列狀態。"
  ], 130, 390, 1000, 54, 22);
}

// 18
{
  const s = newSlide(18, "常見問題對照", "用畫面上的狀態反推該檢查哪個設定。");
  table(s, 72, 145, [280, 340, 490], [
    ["現象", "可能原因", "處理方式"],
    ["IoT 頁只顯示授權提示", "IoT License 未啟用", "到授權管理匯入含 iot 功能的 License"],
    ["Poll timeout", "網路不通或序列參數錯", "檢查 IP、Port、baud、parity、stop bits"],
    ["CRC mismatch", "RTU 線路或參數不符", "檢查 A/B 線、終端電阻、baud、parity"],
    ["數值差 10 倍", "Scale 設錯", "依手冊改成 0.1 或 0.01"],
    ["Forward pending 增加", "Webhook 未通", "檢查 URL、Token、對方 API，按立即清空重送"],
    ["iframe 空白", "Token 或 Allowlist 問題", "重新產生 Token 並加入入口網站 origin"],
  ], 54);
}

async function writeBlob(file, blob) {
  await fs.writeFile(file, new Uint8Array(await blob.arrayBuffer()));
}

for (const [i, slide] of deck.slides.items.entries()) {
  const stem = `slide-${String(i + 1).padStart(2, "0")}`;
  await writeBlob(`${OUT_DIR}/${stem}.png`, await deck.export({ slide, format: "png", scale: 1 }));
  const layout = await slide.export({ format: "layout" });
  await fs.writeFile(`${OUT_DIR}/${stem}.layout.json`, await layout.text());
}
await writeBlob(`${OUT_DIR}/montage.webp`, await deck.export({ format: "webp", montage: true, scale: 1 }));
const pptx = await PresentationFile.exportPptx(deck);
await pptx.save(FINAL);
console.log(JSON.stringify({ final: FINAL, slides: deck.slides.items.length, preview: OUT_DIR }, null, 2));
