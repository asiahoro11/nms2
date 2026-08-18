import fs from "node:fs/promises";
import path from "node:path";
import { Presentation, PresentationFile } from "@oai/artifact-tool";

const ROOT = "D:/claude-sandbox/nms_server";
const OUT = `${ROOT}/outputs/nms_server_detailed_operation_manual_zh-TW.pptx`;
const TMP = `${ROOT}/outputs/ppt_work/detailed_manual`;
const PREVIEW = `${TMP}/preview`;
const W = 1280;
const H = 720;

const C = {
  ink: "#17202A",
  muted: "#5B6776",
  faint: "#F4F6F8",
  line: "#D8DEE6",
  accent: "#FF6B35",
  accentSoft: "#FFF1EA",
  blue: "#1557A6",
  white: "#FFFFFF",
  dark: "#0B1623",
};

const font = "Microsoft JhengHei";
const asset = (p) => `${ROOT}/${p}`;
const A = {
  mode: asset("outputs/manual_screens/00_mode_selection.png"),
  login: asset("outputs/manual_screens/01_login.png"),
  dashboard: asset("outputs/manual_screens/02_dashboard.png"),
  devices: asset("outputs/manual_screens/03_devices.png"),
  topology: asset("outputs/manual_screens/04_topology.png"),
  cameras: asset("outputs/manual_screens/05_cameras.png"),
  access: asset("outputs/manual_screens/06_access_control.png"),
  pdu: asset("outputs/manual_screens/07_pdu.png"),
  iot: asset("outputs/manual_screens/08_iot.png"),
  logs: asset("outputs/manual_screens/09_logs.png"),
  admin: asset("outputs/manual_screens/10_admin.png"),
  monitor: asset("outputs/manual_screens/11_monitor.png"),
  iotReal: asset("nms-iot-page-v1246.png"),
  embed: asset("nms-embed-iot-v1246.png"),
  modbusForm: asset("messageImage_1781056616389.jpg"),
  modbusResult: asset("messageImage_1781057114664.jpg"),
};

async function exists(file) {
  return fs.stat(file).then(() => true).catch(() => false);
}

async function imageBlob(file) {
  const bytes = await fs.readFile(file);
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
}

function typeOf(file) {
  return /\.(jpg|jpeg)$/i.test(file) ? "image/jpeg" : "image/png";
}

function text(slide, value, position, style = {}) {
  const box = slide.shapes.add({
    geometry: "textbox",
    position,
    fill: style.fill ?? "none",
    line: { style: "solid", fill: style.line ?? "none", width: style.lineWidth ?? 0 },
  });
  box.text = value;
  box.text.style = {
    typeface: font,
    fontSize: style.size ?? 19,
    bold: style.bold ?? false,
    color: style.color ?? C.ink,
    alignment: style.align ?? "left",
  };
  return box;
}

function rect(slide, position, style = {}) {
  return slide.shapes.add({
    geometry: style.geometry ?? "rect",
    position,
    fill: style.fill ?? C.faint,
    line: { style: "solid", fill: style.line ?? C.line, width: style.lineWidth ?? 1 },
  });
}

async function image(slide, file, position, alt, fit = "contain") {
  if (!(await exists(file))) {
    rect(slide, position, { fill: "#FAFBFC" });
    text(slide, "圖片待補", {
      left: position.left + 20,
      top: position.top + position.height / 2 - 16,
      width: position.width - 40,
      height: 32,
    }, { size: 22, bold: true, color: C.muted, align: "center" });
    return;
  }
  slide.images.add({
    blob: await imageBlob(file),
    contentType: typeOf(file),
    alt,
    fit,
    position,
  });
}

function header(slide, chapter, title, pageNo) {
  text(slide, chapter, { left: 46, top: 30, width: 650, height: 24 }, {
    size: 15,
    bold: true,
    color: C.accent,
  });
  text(slide, title, { left: 46, top: 64, width: 930, height: 58 }, {
    size: 36,
    bold: true,
    color: C.ink,
  });
  rect(slide, { left: 46, top: 132, width: 1188, height: 1 }, {
    fill: C.line,
    line: C.line,
    lineWidth: 0,
  });
  text(slide, String(pageNo).padStart(2, "0"), { left: 1160, top: 34, width: 72, height: 28 }, {
    size: 19,
    bold: true,
    color: C.muted,
    align: "right",
  });
}

function block(slide, label, lines, top) {
  text(slide, label, { left: 842, top, width: 330, height: 28 }, {
    size: 22,
    bold: true,
    color: C.blue,
  });
  text(slide, lines.map((line) => `• ${line}`).join("\n"), {
    left: 842,
    top: top + 36,
    width: 342,
    height: Math.max(46, lines.length * 30),
  }, { size: 17.5, color: C.ink });
}

async function cover(p) {
  const slide = p.slides.add();
  slide.background.fill = C.dark;
  await image(slide, A.dashboard, { left: 572, top: 86, width: 626, height: 410 }, "NMS dashboard", "contain");
  text(slide, "NMS 詳細圖像式操作手冊", { left: 64, top: 130, width: 560, height: 120 }, {
    size: 50,
    bold: true,
    color: C.white,
  });
  text(slide, "50–60 頁使用者 SOP 版", { left: 68, top: 282, width: 460, height: 44 }, {
    size: 27,
    bold: true,
    color: "#FFD2BF",
  });
  text(slide, "每頁只講一個功能或一個操作步驟，搭配畫面截圖與完成確認。", {
    left: 68,
    top: 356,
    width: 460,
    height: 80,
  }, { size: 22, color: "#E6EEF7" });
  text(slide, "nms_server 使用者版", { left: 68, top: 604, width: 360, height: 30 }, {
    size: 19,
    bold: true,
    color: "#A9B7C8",
  });
}

async function guide(p, pageNo) {
  const slide = p.slides.add();
  slide.background.fill = C.white;
  header(slide, "使用方式", "照著圖片確認畫面，再照文字完成操作", pageNo);
  await image(slide, A.mode, { left: 64, top: 178, width: 530, height: 350 }, "mode selection");
  const items = [
    ["先確認畫面", "左側截圖就是你應該看到的頁面。"],
    ["再做操作", "右側只列必要步驟，不放系統架構。"],
    ["最後確認", "完成確認通過，再進下一頁。"],
    ["遇到不同畫面", "先重新整理或回上一頁，再通知管理者。"],
  ];
  items.forEach((item, i) => {
    const top = 178 + i * 91;
    rect(slide, { left: 665, top, width: 460, height: 66 }, {
      fill: i === 0 ? C.accentSoft : C.faint,
      line: i === 0 ? "#FFD2BF" : C.line,
    });
    text(slide, String(i + 1), { left: 686, top: top + 14, width: 36, height: 30 }, {
      size: 25,
      bold: true,
      color: C.accent,
      align: "center",
    });
    text(slide, item[0], { left: 748, top: top + 9, width: 220, height: 28 }, { size: 22, bold: true });
    text(slide, item[1], { left: 748, top: top + 39, width: 330, height: 22 }, { size: 17, color: C.muted });
  });
}

async function sectionSlide(p, pageNo, chapter, title, imageFile, points) {
  const slide = p.slides.add();
  slide.background.fill = C.dark;
  await image(slide, imageFile, { left: 660, top: 108, width: 500, height: 320 }, title, "contain");
  text(slide, chapter, { left: 68, top: 112, width: 450, height: 32 }, {
    size: 20,
    bold: true,
    color: "#FFD2BF",
  });
  text(slide, title, { left: 68, top: 166, width: 530, height: 96 }, {
    size: 44,
    bold: true,
    color: C.white,
  });
  text(slide, points.map((x) => `• ${x}`).join("\n"), {
    left: 72,
    top: 315,
    width: 500,
    height: 140,
  }, { size: 22, color: "#E6EEF7" });
  text(slide, String(pageNo).padStart(2, "0"), { left: 1120, top: 610, width: 60, height: 28 }, {
    size: 18,
    bold: true,
    color: "#A9B7C8",
    align: "right",
  });
}

async function manualSlide(p, pageNo, chapter, title, imageFile, purpose, steps, confirm, note = "") {
  const slide = p.slides.add();
  slide.background.fill = C.white;
  header(slide, chapter, title, pageNo);
  rect(slide, { left: 46, top: 160, width: 760, height: 470 }, {
    fill: C.white,
    line: C.line,
  });
  await image(slide, imageFile, { left: 56, top: 170, width: 740, height: 450 }, title, "contain");
  rect(slide, { left: 825, top: 160, width: 380, height: 470 }, {
    fill: C.faint,
    line: C.line,
  });
  block(slide, "用途", purpose, 184);
  block(slide, "操作", steps, 286);
  block(slide, "完成確認", confirm, 430);
  if (note) {
    rect(slide, { left: 825, top: 648, width: 380, height: 34 }, {
      fill: C.accentSoft,
      line: "#FFD2BF",
    });
    text(slide, note, { left: 842, top: 655, width: 350, height: 22 }, {
      size: 15.5,
      bold: true,
      color: C.accent,
    });
  }
}

const pages = [
  ["登入與入口", "選擇 System 或 Monitor 模式", A.mode, ["決定要進管理後台或監看畫面"], ["點選 System 進入管理功能", "點選 Monitor 進入監看模式"], ["畫面切到登入頁或監看頁"], "一般設定請先進 System"],
  ["登入與入口", "使用帳號密碼登入系統", A.login, ["確認操作者身分與權限"], ["輸入帳號", "輸入密碼", "按下登入"], ["成功後進入儀表板"], "帳密問題請找管理員"],
  ["登入與入口", "確認目前登入角色", A.dashboard, ["知道自己能看或能改哪些功能"], ["看右上角使用者名稱", "確認角色顯示", "無權限時不要借用他人帳號"], ["角色與工作需求相符"], "權限不足請申請調整"],
  ["登入與入口", "切換系統語言", A.dashboard, ["讓畫面文字符合使用者習慣"], ["點選右上角語言選單", "選擇繁中或英文", "等待畫面重新套用"], ["選單與標題已切換語言"], "交接班建議使用同一語言"],
  ["登入與入口", "切換深色與淺色主題", A.dashboard, ["在不同值班環境下提高可讀性"], ["找到左下角主題切換", "點一下切換模式", "確認背景顏色改變"], ["畫面仍清楚可讀"], "大螢幕常用深色模式"],
  ["儀表板", "用總覽數字快速判斷系統健康", A.dashboard, ["先看設備總數、在線與離線"], ["進入儀表板", "查看上方狀態卡", "注意離線數是否增加"], ["在線、離線數量可解釋"], "每天第一步先看這裡"],
  ["儀表板", "查看設備類型分布", A.dashboard, ["確認系統收納的設備種類"], ["查看設備分布區塊", "比對現場設備類型", "發現缺少類型時回設備清單查"], ["分布符合現場設備結構"], "大量缺漏代表設備尚未建檔"],
  ["儀表板", "查看 Top 5 流量設備", A.dashboard, ["找出流量最高的設備"], ["在 Top 5 區選擇流量", "查看設備名稱與數值", "必要時點回設備清單"], ["知道哪幾台流量較高"], "尖峰時段可用來快速定位"],
  ["儀表板", "查看 Top 5 CPU 與記憶體", A.dashboard, ["找出負載較高的設備"], ["切換 CPU", "切換記憶體", "記錄高負載設備"], ["負載異常設備被找出"], "持續高負載需進一步檢查"],
  ["儀表板", "查看最近事件", A.dashboard, ["掌握剛發生的異常或操作"], ["查看最近事件區", "確認時間與事件內容", "必要時進入日誌查詢"], ["事件可追溯到來源"], "不要只看告警數字"],
  ["儀表板", "重新整理儀表板資料", A.dashboard, ["避免看到過期狀態"], ["按重新整理", "等待數字更新", "確認最近事件時間"], ["資料時間為最新"], "異常前後都建議刷新一次"],
  ["設備管理", "搜尋指定設備", A.devices, ["快速找到要處理的設備"], ["輸入名稱或 IP", "確認列表縮小", "點選正確設備"], ["只剩目標或相關設備"], "先搜尋再操作"],
  ["設備管理", "用類型篩選設備", A.devices, ["一次查看同類設備"], ["打開類型下拉選單", "選擇交換器、AP、PDU 等", "查看結果"], ["列表只顯示該類型"], "適合做同類巡檢"],
  ["設備管理", "用狀態篩選離線設備", A.devices, ["集中處理離線問題"], ["選擇離線狀態", "查看設備 IP 與類型", "安排現場檢查"], ["離線設備清單明確"], "大量離線先查網段或電源"],
  ["設備管理", "新增一台設備", A.devices, ["把新設備加入監控"], ["按新增設備", "填名稱、IP、類型", "選監控方式後儲存"], ["清單出現新設備"], "SNMP 資訊需和設備端一致"],
  ["設備管理", "設定設備基本資料", A.devices, ["讓設備容易辨識"], ["填寫設備名稱", "選擇品牌與類型", "補上位置或備註"], ["其他人能看懂設備用途"], "命名建議含區域與用途"],
  ["設備管理", "設定 Ping 監控", A.devices, ["確認設備是否在線"], ["啟用 Ping", "確認 IP 正確", "儲存後等待狀態更新"], ["狀態可顯示在線或離線"], "Ping 被防火牆擋會誤判"],
  ["設備管理", "設定 SNMP 監控", A.devices, ["讀取 CPU、記憶體與介面資訊"], ["選擇 SNMP 版本", "填 community 或 v3 帳密", "儲存並等待輪詢"], ["設備詳情出現監控數值"], "SNMP 錯誤先查設備端設定"],
  ["設備管理", "使用批次掃描", A.devices, ["快速探索同網段設備"], ["按批次掃描", "輸入 IP 範圍", "勾選要加入的設備"], ["勾選設備加入清單"], "掃描前先確認網段"],
  ["設備管理", "使用子網段群組檢視", A.devices, ["用網段整理大量設備"], ["按子網段群組", "查看 /24 分組", "點入異常網段"], ["設備依網段分群"], "適合大量設備環境"],
  ["設備管理", "批次編輯設備", A.devices, ["一次修改多台設備設定"], ["勾選多台設備", "按批次編輯", "確認要套用的欄位"], ["被勾選設備同步更新"], "只改確定一致的欄位"],
  ["設備管理", "批次刪除設備", A.devices, ["移除不再監控的設備"], ["勾選設備", "按批次刪除", "確認刪除清單"], ["設備不再出現在列表"], "刪除前先確認沒有誤選"],
  ["設備管理", "查看設備詳細狀態", A.devices, ["確認單台設備的監控資訊"], ["點選設備列", "查看狀態與最後更新", "確認是否有異常"], ["知道單台設備目前狀態"], "狀態時間比顏色更重要"],
  ["設備管理", "處理設備離線", A.devices, ["建立一致的排查順序"], ["先確認 Ping", "再查交換器連線", "最後查設備電源"], ["離線原因可被分類"], "不要先重啟重要設備"],
  ["網路拓樸", "開啟拓樸圖確認連線", A.topology, ["看出設備彼此如何連接"], ["進入網路拓樸", "查看節點與線路", "比對現場架構"], ["拓樸和現場相符"], "拓樸異常常和資料或連線有關"],
  ["網路拓樸", "切換拓樸檢視模式", A.topology, ["用不同方式閱讀網路關係"], ["切換拓樸模式", "觀察節點排列", "選擇最容易判讀的模式"], ["節點與線路不重疊難讀"], "大型網路可嘗試不同模式"],
  ["網路拓樸", "點選節點查看設備", A.topology, ["從圖上找到設備資訊"], ["點選節點", "查看名稱、IP、狀態", "需要時回設備詳情"], ["選到正確設備"], "節點名稱需和設備清單一致"],
  ["網路拓樸", "查看待加入拓樸的設備", A.topology, ["找出還沒放進圖上的設備"], ["查看側邊待加入列表", "選擇設備", "拖曳或加入拓樸"], ["設備出現在拓樸圖"], "新增設備後記得更新拓樸"],
  ["網路拓樸", "調整節點位置", A.topology, ["讓拓樸圖更容易交接"], ["拖曳節點到正確區域", "避免線路交叉", "儲存配置"], ["重新進入後位置保留"], "依樓層或區域排列較清楚"],
  ["網路拓樸", "處理拓樸連線異常", A.topology, ["確認線路是否符合現場"], ["比對 LLDP 或手動連線", "查看異常線路", "修正設備或連線資料"], ["異常線路被修正"], "修拓樸前先確認現場"],
  ["攝影機監控", "查看攝影機清單", A.cameras, ["知道有哪些攝影機被管理"], ["進入攝影機監控", "查看左側清單", "確認數量與名稱"], ["攝影機清單符合現場"], "無清單時先確認 License"],
  ["攝影機監控", "查看即時影像", A.cameras, ["確認攝影機可正常播放"], ["點選攝影機", "等待預覽載入", "觀察是否卡住或黑畫面"], ["影像可正常更新"], "無影像先查 RTSP 與網路"],
  ["攝影機監控", "新增攝影機", A.cameras, ["把新攝影機納入監看"], ["按新增攝影機", "填名稱與 RTSP", "儲存後測試播放"], ["清單出現攝影機且可播放"], "RTSP 帳密要確認正確"],
  ["攝影機監控", "編輯攝影機設定", A.cameras, ["修正名稱、串流或顯示設定"], ["按編輯", "修改欄位", "儲存後重新播放"], ["新設定生效"], "改串流前先備份原網址"],
  ["攝影機監控", "使用錄影或錄影狀態", A.cameras, ["確認重要影像是否錄製"], ["查看錄影按鈕或狀態", "依需求啟停錄影", "檢查狀態提示"], ["錄影狀態符合需求"], "錄影會消耗儲存空間"],
  ["攝影機監控", "使用分割畫面監看", A.monitor, ["同時觀看多路影像"], ["進入 Monitor", "確認多格畫面", "必要時切換全螢幕"], ["值班螢幕可持續監看"], "大螢幕建議固定頁面"],
  ["門禁管理", "查看門禁設備", A.access, ["確認門禁控制器是否在線"], ["進入門禁管理", "查看設備卡片", "確認在線狀態"], ["門禁設備狀態正常"], "門禁異常需同步現場確認"],
  ["門禁管理", "查看卡片資料", A.access, ["查詢持卡人與卡號"], ["切到卡片管理", "輸入卡號或姓名", "查看狀態與有效期"], ["找到正確卡片"], "停用卡片前要確認人員身分"],
  ["門禁管理", "查看排程申請", A.access, ["管理臨時通行時段"], ["切到排程", "篩選待審核或已核准", "查看有效日期"], ["排程狀態清楚"], "過期排程應定期清理"],
  ["門禁管理", "查看通行事件", A.access, ["追查門禁進出紀錄"], ["切到事件", "選擇事件類型", "查看時間、門、卡號"], ["找到對應通行紀錄"], "異常事件要保留時間點"],
  ["門禁管理", "處理門禁異常", A.access, ["快速分辨設備或權限問題"], ["先看設備在線", "再查卡片狀態", "最後看事件原因"], ["可判斷是設備、卡片或排程問題"], "不要只憑使用者描述判斷"],
  ["PDU / UPS", "查看 PDU/UPS 清單", A.pdu, ["確認電源設備是否在線"], ["進入 PDU/UPS", "查看設備卡片", "確認狀態與更新時間"], ["電源設備狀態可判讀"], "模組鎖定時先查 License"],
  ["PDU / UPS", "新增 PDU 或 UPS", A.pdu, ["把電源設備納入監控"], ["按新增", "填設備連線資料", "儲存後刷新"], ["清單出現新設備"], "填寫前先確認通訊協定"],
  ["PDU / UPS", "查看電壓電流與負載", A.pdu, ["避免電源過載或異常"], ["查看設備量測值", "比對額定範圍", "記錄異常數值"], ["數值在合理範圍"], "負載升高要提前處理"],
  ["PDU / UPS", "操作插座開關", A.pdu, ["遠端控制供電"], ["確認目標插座", "執行開啟、關閉或重啟", "等待狀態回報"], ["插座狀態已更新"], "斷電前務必確認影響"],
  ["PDU / UPS", "處理電源告警", A.pdu, ["降低設備斷電風險"], ["確認告警來源", "查看負載與 UPS 狀態", "通知現場或維護"], ["告警原因被記錄"], "電源告警優先級通常較高"],
  ["IoT / Modbus", "查看 IoT 總覽", A.iotReal, ["掌握 IoT 裝置與量測數"], ["進入 IoT / Modbus", "查看設備與量測數", "確認更新時間"], ["資料持續更新"], "數值不變先看時間"],
  ["IoT / Modbus", "查看線上模組", A.iot, ["確認感測器是否在線"], ["切到 Online Module", "查看模組卡片", "確認狀態與數值"], ["線上模組與現場一致"], "離線先查通訊線路"],
  ["IoT / Modbus", "新增 IoT 裝置", A.iot, ["把感測器或閘道加入系統"], ["按 Add Device", "填名稱、協定、Target", "儲存後刷新"], ["裝置出現在 Devices"], "Target 格式要依協定填寫"],
  ["IoT / Modbus", "設定 Modbus 讀取位置", A.modbusForm, ["指定要讀哪個暫存器"], ["選 Function Code", "輸入 Address", "輸入 Quantity"], ["沒有格式錯誤"], "Address 依設備手冊填寫"],
  ["IoT / Modbus", "確認 Modbus 回傳值", A.modbusResult, ["確認數值可被解析"], ["執行讀取", "查看回傳值", "比對現場儀表"], ["數值合理且單位正確"], "異常時查位址、倍率、型態"],
  ["IoT / Modbus", "測試 Gateway Ingest", A.iot, ["驗證外部資料可送進 NMS"], ["填 External ID、Metric、Value", "按 Send Ingest", "查看量測表"], ["量測資料新增"], "測試值需標示避免誤判"],
  ["IoT / Modbus", "設定 Forward Queue", A.iot, ["把資料轉送到外部系統"], ["切到 Integration", "填 Webhook URL", "儲存並必要時 Flush"], ["轉送佇列狀態正常"], "Token 欄位需保密"],
  ["IoT / Modbus", "產生 iframe 嵌入網址", A.embed, ["把 NMS 畫面放到外部看板"], ["填 Token 名稱", "選擇 View", "產生 Token 並複製 iframe"], ["外部頁面可載入畫面"], "Allowlist 要包含外部網域"],
  ["日誌與稽核", "查詢操作紀錄", A.logs, ["追查誰做了什麼操作"], ["進入日誌與稽核", "輸入關鍵字或篩選時間", "查看事件內容"], ["找到對應操作紀錄"], "處理爭議時先保留紀錄"],
  ["日誌與稽核", "查詢系統事件", A.logs, ["查看登入、錯誤與系統變更"], ["選擇事件類型", "查看時間與來源", "必要時匯出"], ["事件可追溯"], "時間範圍不要選太大"],
  ["日誌與稽核", "篩選異常事件", A.logs, ["聚焦需要處理的異常"], ["選擇錯誤或告警類型", "查看最新事件", "回到相關模組處理"], ["異常來源明確"], "先處理影響最大的事件"],
  ["日誌與稽核", "匯出稽核資料", A.logs, ["提供維護或稽核使用"], ["先完成篩選", "按匯出或下載", "開檔確認內容"], ["檔案期間與欄位正確"], "匯出前先縮小時間範圍"],
  ["系統管理", "查看系統管理首頁", A.admin, ["確認系統設定與授權入口"], ["進入系統管理", "查看上方分頁", "選擇要管理的項目"], ["能看到管理功能分頁"], "只有 Admin 可進入"],
  ["系統管理", "管理使用者帳號", A.admin, ["新增、停用或重設帳號"], ["開啟使用者管理", "新增或編輯使用者", "儲存角色與密碼"], ["使用者可用新設定登入"], "離職帳號要停用"],
  ["系統管理", "設定角色權限", A.admin, ["控制誰能查看或修改"], ["選擇 Admin、Editor、Viewer", "確認工作需求", "儲存設定"], ["權限符合職責"], "值班帳號建議 Viewer"],
  ["系統管理", "啟用或重設 2FA", A.admin, ["提高登入安全"], ["進入安全設定", "掃描 QR Code", "輸入驗證碼完成綁定"], ["下次登入需驗證碼"], "手機遺失需管理員重設"],
  ["系統管理", "查看 License 狀態", A.admin, ["確認模組是否可用"], ["進入 License 區", "查看到期日與模組", "需要時匯入授權"], ["授權狀態正常"], "模組鎖定先查這裡"],
  ["系統管理", "管理模組顯示", A.admin, ["控制哪些模組出現在側欄"], ["進入模組或功能設定", "開啟需要的模組", "回側欄確認"], ["側欄顯示符合需求"], "未授權模組仍可能被鎖定"],
  ["系統管理", "設定品牌與登入畫面", A.admin, ["讓系統符合客戶識別"], ["進入品牌設定", "上傳 Logo 或公司名稱", "儲存後刷新"], ["側欄或登入頁顯示新品牌"], "圖片尺寸需先確認"],
  ["系統管理", "備份系統資料", A.admin, ["保留設備、設定與紀錄"], ["進入備份功能", "建立備份檔", "下載保存"], ["備份檔日期正確"], "更新前一定先備份"],
  ["系統管理", "還原系統資料", A.admin, ["在故障或移機時恢復設定"], ["選擇備份檔", "確認還原範圍", "執行後重新登入"], ["設備與設定恢復"], "還原前先通知使用者"],
  ["監看模式", "使用大螢幕 Monitor", A.monitor, ["提供值班或展示使用"], ["進入 Monitor", "確認畫面更新", "開啟全螢幕"], ["畫面能長時間顯示"], "適合放在控制室"],
  ["日常巡檢", "依固定順序完成每日檢查", A.dashboard, ["避免漏看異常"], ["看儀表板", "看離線設備", "看告警與日誌"], ["交接時能說明狀態"], "固定順序比臨時亂看可靠"],
  ["日常巡檢", "交接班要記錄重點", A.logs, ["讓下一班知道目前風險"], ["記錄離線設備", "記錄告警與處理進度", "交代未完成事項"], ["下一班能接續處理"], "交接文字要含時間"],
  ["異常排除", "登入失敗時先確認基本項目", A.login, ["排除帳密或權限問題"], ["確認帳號密碼", "確認是否需要 2FA", "請管理員重設"], ["可重新登入或明確報修"], "不要共用管理員密碼"],
  ["異常排除", "設備大量離線時的排查順序", A.devices, ["快速判斷是否為共同原因"], ["先看是否同網段", "確認交換器或電源", "再查單台設備"], ["原因範圍縮小"], "大量異常先查共用設備"],
  ["異常排除", "畫面資料不更新時怎麼辦", A.dashboard, ["分辨前端顯示或後端輪詢問題"], ["重新整理頁面", "看最後更新時間", "查日誌是否有錯誤"], ["知道資料卡在哪一步"], "不要只重開瀏覽器"],
  ["異常排除", "模組被鎖定時怎麼辦", A.admin, ["確認是授權或權限造成"], ["看側欄鎖頭", "進 License 查看授權", "確認角色權限"], ["知道要處理授權或帳號"], "不要反覆新增同一模組資料"],
  ["收尾", "完成操作後留下可追溯紀錄", A.logs, ["讓維護與稽核能追蹤"], ["記錄操作時間", "記錄處理結果", "必要時匯出日誌"], ["問題有完整紀錄"], "有紀錄才容易交接與追查"],
];

async function writeBlob(file, blob) {
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, new Uint8Array(await blob.arrayBuffer()));
}

async function main() {
  await fs.mkdir(PREVIEW, { recursive: true });
  const p = Presentation.create({ slideSize: { width: W, height: H } });
  await cover(p);
  await guide(p, 2);
  await sectionSlide(p, 3, "章節總覽", "從登入到日常維運，照順序學會 NMS", A.dashboard, [
    "先學登入與儀表板",
    "再學設備、拓樸與各功能模組",
    "最後學管理設定、巡檢與排錯",
  ]);
  const quotas = {
    "登入與入口": 4,
    "儀表板": 5,
    "設備管理": 9,
    "網路拓樸": 4,
    "攝影機監控": 4,
    "門禁管理": 3,
    "PDU / UPS": 3,
    "IoT / Modbus": 7,
    "日誌與稽核": 3,
    "系統管理": 8,
    "監看模式": 1,
    "日常巡檢": 1,
    "異常排除": 4,
    "收尾": 1,
  };
  const used = {};
  const selectedPages = pages.filter((page) => {
    const chapter = page[0];
    used[chapter] = used[chapter] || 0;
    if (used[chapter] >= (quotas[chapter] ?? 0)) return false;
    used[chapter] += 1;
    return true;
  });
  let n = 4;
  for (const page of selectedPages) {
    const [chapter, title, img, purpose, steps, confirm, note] = page;
    await manualSlide(p, n, chapter, title, img, purpose, steps, confirm, note);
    n += 1;
  }

  for (const [i, slide] of p.slides.items.entries()) {
    const stem = `slide-${String(i + 1).padStart(2, "0")}`;
    await writeBlob(path.join(PREVIEW, `${stem}.png`), await p.export({ slide, format: "png", scale: 1 }));
    const layout = await slide.export({ format: "layout" });
    await fs.writeFile(path.join(PREVIEW, `${stem}.layout.json`), await layout.text());
  }
  await writeBlob(path.join(PREVIEW, "montage.webp"), await p.export({ format: "webp", montage: true, scale: 1 }));
  const pptx = await PresentationFile.exportPptx(p);
  await pptx.save(OUT);
  console.log(OUT);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
