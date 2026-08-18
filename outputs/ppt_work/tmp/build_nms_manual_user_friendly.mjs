import fs from "node:fs/promises";
import path from "node:path";
import { Presentation, PresentationFile } from "@oai/artifact-tool";

const OUT = "D:/claude-sandbox/nms_server/outputs/nms_server_user_operation_manual_zh-TW.pptx";
const TMP = "D:/claude-sandbox/nms_server/outputs/ppt_work/tmp_user_friendly";
const PREVIEW = path.join(TMP, "preview");
const W = 1280;
const H = 720;

const C = {
  ink: "#172033",
  text: "#2F3747",
  muted: "#667085",
  line: "#CBD5E1",
  panel: "#F4F6F8",
  note: "#FFF7ED",
  noteLine: "#FB923C",
  good: "#0F766E",
  warn: "#B45309",
  white: "#FFFFFF",
  navy: "#1F2937",
};

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
    color: opt.color ?? C.text,
    typeface: opt.mono ? "Consolas" : "Microsoft JhengHei",
    alignment: opt.align ?? "left",
  };
  return box;
}

function rect(slide, pos, opt = {}) {
  return slide.shapes.add({
    geometry: opt.round ? "roundRect" : "rect",
    position: pos,
    fill: opt.fill ?? C.panel,
    line: { style: "solid", fill: opt.line ?? "none", width: opt.lineWidth ?? 0 },
  });
}

function hline(slide, y) {
  slide.shapes.add({
    geometry: "line",
    position: { left: 56, top: y, width: 1168, height: 0 },
    fill: "none",
    line: { style: "solid", fill: C.line, width: 1 },
  });
}

function base(p, title, section) {
  const s = p.slides.add();
  s.background.fill = C.white;
  addText(s, section, { left: 56, top: 34, width: 400, height: 28 }, { size: 16, bold: true, color: C.muted });
  addText(s, title, { left: 56, top: 74, width: 1060, height: 74 }, { size: 38, bold: true, color: C.ink });
  hline(s, 158);
  addText(s, String(p.slides.items.length).padStart(2, "0"), { left: 1160, top: 660, width: 58, height: 24 }, { size: 14, color: C.muted, align: "right" });
  return s;
}

function note(slide, text) {
  rect(slide, { left: 72, top: 586, width: 1040, height: 58 }, { fill: C.note, line: C.noteLine, lineWidth: 1 });
  addText(slide, text, { left: 96, top: 603, width: 990, height: 26 }, { size: 20, bold: true, color: C.warn });
}

function steps(slide, items, startTop = 205) {
  items.forEach((txt, i) => {
    const top = startTop + i * 72;
    rect(slide, { left: 80, top, width: 54, height: 48 }, { fill: C.ink });
    addText(slide, String(i + 1), { left: 80, top: top + 8, width: 54, height: 28 }, { size: 23, bold: true, color: C.white, align: "center" });
    addText(slide, txt, { left: 158, top: top + 7, width: 900, height: 42 }, { size: 24, color: C.text });
  });
}

function taskSlide(p, title, section, where, todo, done, caution) {
  const s = base(p, title, section);
  rect(s, { left: 72, top: 198, width: 245, height: 128 }, { fill: C.panel });
  rect(s, { left: 350, top: 198, width: 760, height: 128 }, { fill: C.panel });
  rect(s, { left: 72, top: 360, width: 498, height: 120 }, { fill: "#F0FDFA" });
  rect(s, { left: 612, top: 360, width: 498, height: 120 }, { fill: C.note });
  addText(s, "在哪裡", { left: 94, top: 218, width: 170, height: 30 }, { size: 24, bold: true, color: C.ink });
  addText(s, where, { left: 94, top: 260, width: 190, height: 46 }, { size: 21, color: C.muted });
  addText(s, "怎麼做", { left: 374, top: 218, width: 170, height: 30 }, { size: 24, bold: true, color: C.ink });
  addText(s, todo, { left: 374, top: 260, width: 695, height: 50 }, { size: 21, color: C.text });
  addText(s, "完成後會看到", { left: 96, top: 382, width: 220, height: 28 }, { size: 24, bold: true, color: C.good });
  addText(s, done, { left: 96, top: 423, width: 420, height: 36 }, { size: 20, color: C.text });
  addText(s, "注意", { left: 636, top: 382, width: 120, height: 28 }, { size: 24, bold: true, color: C.warn });
  addText(s, caution, { left: 636, top: 423, width: 420, height: 36 }, { size: 20, color: C.text });
}

function cover(p) {
  const s = p.slides.add();
  s.background.fill = C.white;
  rect(s, { left: 56, top: 44, width: 1168, height: 110 }, { fill: C.ink });
  addText(s, "NMS 使用者操作手冊", { left: 76, top: 78, width: 760, height: 50 }, { size: 44, bold: true, color: C.white });
  addText(s, "照著畫面一步一步操作，不需要懂 API 或系統架構", { left: 76, top: 220, width: 920, height: 52 }, { size: 34, bold: true, color: C.ink });
  addText(s, "適用：一般使用者、維運人員、現場工程師、管理員", { left: 76, top: 312, width: 760, height: 36 }, { size: 24, color: C.muted });
  rect(s, { left: 76, top: 450, width: 980, height: 78 }, { fill: C.note, line: C.noteLine, lineWidth: 1 });
  addText(s, "這份手冊的寫法：先告訴你去哪裡，再告訴你按什麼、填什麼、怎麼確認成功。", { left: 100, top: 472, width: 930, height: 30 }, { size: 23, bold: true, color: C.warn });
}

function useThisManual(p) {
  const s = base(p, "看不懂功能名稱時，先用任務找頁面", "使用前先看");
  const rows = [
    ["我要看設備有沒有斷線", "儀表板、設備清單、拓樸圖"],
    ["我要新增一批設備", "設備管理 → 批次掃描"],
    ["我要看攝影機或錄影", "攝影機 / NVR"],
    ["我要看門禁刷卡紀錄", "門禁管理 → 事件"],
    ["我要匯出報告", "報表或日誌中心"],
    ["我要處理授權或帳號", "系統管理"],
  ];
  rows.forEach((r, i) => {
    const top = 200 + i * 62;
    addText(s, r[0], { left: 96, top, width: 410, height: 30 }, { size: 23, bold: true, color: C.ink });
    addText(s, r[1], { left: 560, top: top + 2, width: 500, height: 28 }, { size: 22, color: C.muted });
    hline(s, top + 44);
  });
}

function login(p) {
  const s = base(p, "登入後先確認自己進到正確模式", "登入");
  steps(s, [
    "打開瀏覽器，輸入 NMS 網址，例如 http://伺服器IP:8080。",
    "選擇管理模式。若只是大螢幕監看，才選監看模式。",
    "輸入帳號與密碼。若跳出 2FA，輸入手機驗證器的 6 位數字。",
    "登入後看左側選單。看不到的功能通常是權限或授權限制。",
  ]);
  note(s, "如果第一次登入被要求改密碼，先完成改密碼再繼續操作。");
}

function dashboard(p) {
  const s = base(p, "每天第一件事：先看儀表板有沒有紅燈", "日常監控");
  steps(s, [
    "看「離線設備」數字。有增加時，先進設備清單查是哪幾台。",
    "看「最近事件」。剛斷線、告警、重啟通常會在這裡出現。",
    "切換 Top 5 的流量、CPU、Memory，找出負載最高的設備。",
    "若只是要現場展示，切到監看模式或拓樸頁，不要停在管理頁面。",
  ]);
  note(s, "儀表板是判斷方向，不是結案證據。要交付紀錄時，請到日誌或報表匯出。");
}

function deviceSearch(p) {
  taskSlide(
    p,
    "找一台設備：用搜尋、類型、狀態縮小範圍",
    "設備管理",
    "左側選單 → 設備",
    "輸入設備名稱或 IP；也可以用設備類型、在線/離線狀態篩選。",
    "清單只剩符合條件的設備；狀態欄會顯示在線或離線。",
    "如果設備很多，可以開啟子網群組，用 /24 網段查看。"
  );
}

function addOneDevice(p) {
  const s = base(p, "新增單台設備：先用 Ping，確認後再加 SNMP", "設備管理");
  steps(s, [
    "點「新增設備」。",
    "填設備名稱、IP 位址、設備類型。",
    "如果只要知道是否在線，監控方式選 Ping Only。",
    "如果要讀 CPU、介面流量、PoE，監控方式選 SNMP 並填 community 或 SNMPv3 資訊。",
    "按儲存後回到設備清單，等待下一輪輪詢或手動 Poll。",
  ], 190);
  note(s, "SNMP Write community、CLI 帳密只有需要重啟、PoE 控制、備份設定時才填。");
}

function bulkScan(p) {
  taskSlide(
    p,
    "批次掃描：一次加入一段 IP 或一個子網",
    "設備管理",
    "設備 → 批次掃描",
    "選 IP 範圍或 CIDR 子網，例如 192.168.1.0/24。可勾選 Ping fallback。",
    "掃描完成後，成功的設備會出現在設備清單。",
    "大量掃描前先確認網段正確，避免掃到不屬於此站點的設備。"
  );
}

function deviceDetail(p) {
  const s = base(p, "設備詳情頁：先看狀態，再做控制動作", "設備管理");
  const items = [
    ["看健康狀態", "CPU、記憶體、磁碟、延遲、最後上線時間。"],
    ["看介面", "介面狀態、速率、IN/OUT 流量。"],
    ["做控制", "PoE on/off/recycle、重啟、設定備份。"],
    ["開 SSH", "用 WebSSH 進入設備 CLI。"],
  ];
  items.forEach((it, i) => {
    const top = 205 + i * 82;
    rect(s, { left: 90, top, width: 270, height: 52 }, { fill: i === 2 ? C.note : C.panel });
    addText(s, it[0], { left: 114, top: top + 11, width: 220, height: 28 }, { size: 23, bold: true, color: i === 2 ? C.warn : C.ink });
    addText(s, it[1], { left: 410, top: top + 11, width: 650, height: 30 }, { size: 22, color: C.muted });
  });
  note(s, "控制動作會影響現場服務，執行前請先確認設備與連接埠。");
}

function topology(p) {
  const s = base(p, "拓樸圖用來看影響範圍，不只是漂亮圖", "拓樸");
  steps(s, [
    "進入拓樸頁，先確認是否有離線節點或紅色警示。",
    "拖曳設備到合理位置，按「儲存位置」。",
    "要新增連線時，使用手動連線，選來源設備、目標設備與介面。",
    "若設備支援 LLDP，可按自動探索更新連線。",
  ]);
  note(s, "拓樸圖位置保存後，其他使用者再次打開會看到同一個版面。");
}

function cameraAdd(p) {
  taskSlide(
    p,
    "新增攝影機：有 RTSP 就填 RTSP，沒有就用 ONVIF 找",
    "攝影機 / NVR",
    "左側選單 → 攝影機",
    "按新增攝影機；填名稱、IP、RTSP URL。若只有 ONVIF 位址，按自動取得 RTSP。",
    "攝影機清單出現該攝影機，縮圖或快照能正常顯示。",
    "攝影機功能需要授權。未授權時可看到提示，但不能新增。"
  );
}

function cameraMonitor(p) {
  const s = base(p, "監看攝影機：把攝影機拖到格子裡", "攝影機 / NVR");
  steps(s, [
    "選擇 4 格、9 格或 16 格畫面。",
    "從左側攝影機清單拖曳到中間格子。",
    "需要換位置時，再拖曳到其他格子。",
    "如果有多頁，切換頁碼查看授權上限內的其他攝影機。",
  ]);
  note(s, "如果畫面黑掉，先檢查攝影機快照；快照正常再檢查 RTSP 編碼與網路。");
}

function recording(p) {
  taskSlide(
    p,
    "查錄影：先選攝影機與日期，再用時間軸播放",
    "攝影機 / NVR",
    "攝影機 → 錄影管理",
    "選攝影機、日期，查看錄影片段；點時間軸或片段播放。",
    "播放器出現影像，可匯出或刪除指定片段。",
    "刪除錄影無法復原；大量刪除前請先確認日期與攝影機。"
  );
}

function accessDoor(p) {
  taskSlide(
    p,
    "新增門點：先建立門，再建立卡片與時段",
    "門禁",
    "左側選單 → 門禁 → 門",
    "按新增門，填門名、位置、IP、通訊協定、帳號密碼、廠牌型號。",
    "門卡片出現在門禁頁，可執行開門或關門動作。",
    "門禁操作會產生事件紀錄，請避免用測試門名混入正式環境。"
  );
}

function accessCardSchedule(p) {
  const s = base(p, "卡片與時段：誰可以在什麼時間進哪扇門", "門禁");
  steps(s, [
    "到「卡片」頁新增卡號、持有人、部門與有效期限。",
    "到「時段」頁新增通行規則，選門、日期、星期與時間。",
    "時段建立後，管理員審核通過才生效。",
    "到「事件」頁查刷卡成功、拒絕、警報、開門、關門紀錄。",
  ]);
  note(s, "卡片啟用不代表一定能通行，還要看時段規則是否通過且在有效時間內。");
}

function pdu(p) {
  taskSlide(
    p,
    "PDU/UPS 巡檢：新增後按 Poll 讀取電力狀態",
    "PDU / UPS",
    "左側選單 → PDU/UPS",
    "新增設備，填名稱、IP、SNMP port、community、設備類型。儲存後按 Poll。",
    "卡片會顯示電池容量、剩餘時間、輸入電壓、負載、告警。",
    "PDU/UPS 也需要授權；未授權時新增按鈕會停用。"
  );
}

function iotTcp(p) {
  const s = base(p, "新增 Modbus TCP 感測點：一個量測值就是一筆設備", "IoT / Modbus");
  steps(s, [
    "進入 IoT / Modbus，按新增設備。",
    "選 Modbus TCP，填設備名稱、Host/IP、Port，通常 Port 是 502。",
    "填 Unit ID、Register address、Function Code、Data type。",
    "填 Scale、Offset、Metric，例如 temperature、humidity、voltage。",
    "儲存後按 Poll，確認讀值是否正確。",
  ], 190);
  note(s, "溫度、濕度、電壓等每個量測點都可獨立建立，方便設定不同 register。");
}

function iotSerial(p) {
  taskSlide(
    p,
    "新增 Modbus RTU / RS485：先確認序列埠參數",
    "IoT / Modbus",
    "IoT / Modbus → 新增設備 → RTU 或 RS485",
    "填 Serial port、Baud rate、Data bits、Parity、Stop bits，再填 Unit ID 與 Register。",
    "感測卡片會顯示最新值與最後更新時間。",
    "序列埠錯誤時通常完全讀不到值；先確認 COM 或 /dev/ttyUSB 名稱。"
  );
}

function iotForward(p) {
  const s = base(p, "IoT Forwarder：外部系統離線時資料會先排隊", "IoT / Modbus");
  steps(s, [
    "在 IoT 設定區打開 Forwarder。",
    "填外部系統接收 URL、Token、Batch size。",
    "按儲存後，NMS 會把量測資料送到該 URL。",
    "若外部系統離線，資料會留在 Queue；恢復後可手動 Flush。",
  ]);
  note(s, "Token 儲存在後端，設定頁不會再把完整 Token 顯示出來。");
}

function logs(p) {
  const s = base(p, "查問題：先從日誌中心篩選，再匯出證據", "日誌中心");
  steps(s, [
    "進入日誌中心，先選日誌類型：系統、設備、稽核、設定變更。",
    "使用日期、設備、操作者、狀態或關鍵字篩選。",
    "需要交付時，匯出 CSV、JSON 或證據包。",
    "處理完成後，可標記 review 或 ack，讓下一位同仁知道狀態。",
  ]);
  note(s, "稽核日誌看的是誰做了什麼；設備日誌看的是設備發生了什麼。");
}

function reports(p) {
  taskSlide(
    p,
    "匯出報表：用來交付盤點、巡檢與 SLA 證明",
    "報表",
    "左側選單 → 日誌或報表匯出按鈕",
    "選報表類型與格式。常用有設備、Inventory、Health、Availability、SLA、Camera、PDU、Access Control。",
    "瀏覽器會下載 CSV 或 PDF 檔案。",
    "報表會遮蔽敏感欄位，例如密碼、SNMP community、完整授權金鑰。"
  );
}

function users(p) {
  const s = base(p, "管理帳號：只給使用者需要的權限", "系統管理");
  steps(s, [
    "進入系統管理 → 使用者。",
    "新增帳號時設定角色。一般查看用 Viewer，需要維護設備才給 Editor。",
    "密碼至少 8 碼，需包含大小寫與數字或符號。",
    "使用者離職或不再需要操作時，先停用帳號。",
  ]);
  note(s, "Admin 可以做高風險操作，帳號不要共用。");
}

function license(p) {
  taskSlide(
    p,
    "功能被鎖住時，先看 License 狀態",
    "系統管理",
    "系統管理 → License",
    "查看目前設備數、授權上限、到期日、啟用功能。需要加功能時貼上 license key 啟用。",
    "成功後左側選單鎖頭會消失，功能按鈕可使用。",
    "Machine ID 要提供給授權產生端；不要手動修改。"
  );
}

function backup(p) {
  const s = base(p, "備份還原：更新前一定先備份", "系統管理");
  steps(s, [
    "進入系統管理 → 備份還原。",
    "一般環境可匯出系統備份；敏感環境使用加密備份。",
    "還原前先做 readiness 檢查。",
    "還原完成後重新登入，確認設備、使用者、License 與模組資料。"
  ]);
  note(s, "加密備份的密碼遺失後無法還原，請交給管理者保管。");
}

function alerts(p) {
  taskSlide(
    p,
    "告警通知：設定後一定要按測試",
    "系統管理",
    "系統管理 → 告警",
    "開啟告警總開關，設定 Email、Webhook、Syslog 等通道，填完後按測試。",
    "測試通過後，收件端會收到一則測試通知。",
    "如果測試失敗，先檢查 SMTP、Webhook URL、防火牆與憑證。"
  );
}

function embed(p) {
  const s = base(p, "嵌入外部入口：用 Embed token，不要用管理員帳號", "系統整合");
  steps(s, [
    "進入 IoT / Modbus 的整合設定區。",
    "建立 Embed token，選 view，例如 dashboard、topology、devices、iot。",
    "設定有效時間，產生 iframe 程式碼。",
    "把客戶入口網域加入 iframe allowlist。",
    "不再使用時，從 token 清單撤銷。",
  ], 184);
  note(s, "Embed token 是唯讀用途。不要把 Admin JWT 放在網址或 iframe 裡。");
}

function api(p) {
  const s = base(p, "外部系統讀資料：優先用 network-snapshot", "系統整合");
  addText(s, "適合外部儀表板一次取得總覽資料。", { left: 86, top: 205, width: 760, height: 34 }, { size: 26, bold: true, color: C.ink });
  rect(s, { left: 92, top: 270, width: 920, height: 76 }, { fill: C.panel });
  addText(s, "GET /api/v1/integrations/network-snapshot?include=dashboard,devices,topology", { left: 116, top: 294, width: 860, height: 26 }, { size: 18, mono: true });
  steps(s, [
    "先登入取得 JWT token。",
    "外部系統以 Authorization: Bearer <token> 呼叫 API。",
    "若只要設備清單，可用 include 與 limit 減少資料量。",
    "不要直接讀資料庫欄位，API 才是對外合約。",
  ], 390);
}

function mobile(p) {
  taskSlide(
    p,
    "手機 App 只是監控入口，資料仍以 NMS 伺服器為準",
    "行動端",
    "手機 App 登入頁",
    "填站點名稱、NMS 網址、帳號密碼；若啟用 2FA，再輸入驗證碼。",
    "App 顯示 Dashboard 與設備狀態。",
    "第一版以查看為主，控制動作仍建議在管理後台執行。"
  );
}

function dailyChecklist(p) {
  const s = base(p, "每日巡檢照這 6 件事做", "每日 SOP");
  const items = [
    "看儀表板離線數與最近事件。",
    "進設備清單確認離線設備與最後上線時間。",
    "進拓樸圖看是否影響核心或重要區域。",
    "查看攝影機、PDU/UPS、IoT 是否有授權模組告警。",
    "查日誌中心，確認異常是否已 ack 或 review。",
    "需要交付時匯出報表或證據包。",
  ];
  items.forEach((it, i) => {
    const top = 196 + i * 62;
    addText(s, "□", { left: 96, top, width: 38, height: 30 }, { size: 26, color: C.good });
    addText(s, it, { left: 148, top: top + 1, width: 850, height: 30 }, { size: 24, color: C.text });
  });
}

function troubleshoot(p) {
  const s = base(p, "使用者最常卡住的地方", "故障排除");
  const rows = [
    ["看不到功能", "通常是角色不夠、License 未啟用、模組被關閉。"],
    ["設備一直離線", "確認 IP、Ping、防火牆、SNMP community、設備是否真的開機。"],
    ["攝影機黑畫面", "先測 snapshot，再檢查 RTSP、ONVIF、帳密與編碼。"],
    ["IoT 沒資料", "確認 register、Unit ID、序列埠、scale，以及是否按 Poll。"],
    ["報表下載空白", "先放寬日期或篩選條件，再重新匯出。"],
  ];
  rows.forEach((r, i) => {
    const top = 196 + i * 76;
    rect(s, { left: 76, top, width: 250, height: 46 }, { fill: C.note });
    addText(s, r[0], { left: 98, top: top + 9, width: 200, height: 26 }, { size: 22, bold: true, color: C.warn });
    addText(s, r[1], { left: 366, top: top + 9, width: 720, height: 28 }, { size: 22, color: C.text });
  });
}

function glossary(p) {
  const s = base(p, "常見名詞用人話看", "附錄");
  const rows = [
    ["SNMP", "讓 NMS 讀設備狀態與流量的協定。"],
    ["Community", "SNMP v1/v2c 的讀取密碼。"],
    ["RTSP", "攝影機影像串流網址。"],
    ["ONVIF", "幫忙探索攝影機與取得 RTSP 的標準。"],
    ["Modbus Register", "感測器數值所在的地址。"],
    ["Embed token", "讓外部網頁唯讀顯示 NMS 視圖的短期鑰匙。"],
  ];
  rows.forEach((r, i) => {
    const top = 190 + i * 64;
    addText(s, r[0], { left: 100, top, width: 230, height: 30 }, { size: 24, bold: true, color: C.ink });
    addText(s, r[1], { left: 360, top: top + 2, width: 680, height: 30 }, { size: 22, color: C.muted });
  });
}

function close(p) {
  const s = p.slides.add();
  s.background.fill = C.white;
  addText(s, "結論：照任務走，不用背功能名稱", { left: 72, top: 104, width: 1000, height: 70 }, { size: 46, bold: true, color: C.ink });
  addText(s, "使用者只要記得三件事：先看儀表板、再進對應模組、最後用日誌或報表留下紀錄。", { left: 72, top: 230, width: 960, height: 90 }, { size: 32, color: C.text });
  rect(s, { left: 72, top: 452, width: 1030, height: 76 }, { fill: C.note, line: C.noteLine, lineWidth: 1 });
  addText(s, "現場交付時，建議搭配真實帳號與測試設備示範一次。", { left: 100, top: 476, width: 920, height: 30 }, { size: 24, bold: true, color: C.warn });
}

async function writeBlob(file, blob) {
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, new Uint8Array(await blob.arrayBuffer()));
}

async function main() {
  await fs.mkdir(PREVIEW, { recursive: true });
  const p = Presentation.create({ slideSize: { width: W, height: H } });
  cover(p);
  useThisManual(p);
  login(p);
  dashboard(p);
  deviceSearch(p);
  addOneDevice(p);
  bulkScan(p);
  deviceDetail(p);
  topology(p);
  cameraAdd(p);
  cameraMonitor(p);
  recording(p);
  accessDoor(p);
  accessCardSchedule(p);
  pdu(p);
  iotTcp(p);
  iotSerial(p);
  iotForward(p);
  logs(p);
  reports(p);
  users(p);
  license(p);
  backup(p);
  alerts(p);
  embed(p);
  api(p);
  mobile(p);
  dailyChecklist(p);
  troubleshoot(p);
  glossary(p);
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
