import fs from "node:fs/promises";
import path from "node:path";
import { Presentation, PresentationFile } from "@oai/artifact-tool";

const ROOT = "D:/claude-sandbox/nms_server";
const OUT = `${ROOT}/outputs/nms_server_image_operation_manual_zh-TW.pptx`;
const TMP = `${ROOT}/outputs/ppt_work/tmp_image_manual`;
const PREVIEW = `${TMP}/preview`;

const W = 1280;
const H = 720;
const C = {
  ink: "#17202A",
  muted: "#556270",
  faint: "#F4F6F8",
  line: "#D8DEE6",
  accent: "#FF6B35",
  accentSoft: "#FFF1EA",
  blue: "#1F5FAA",
  white: "#FFFFFF",
};

const font = {
  body: "Microsoft JhengHei",
  mono: "Consolas",
};

const img = (relative) => `${ROOT}/${relative}`;

const assets = {
  mode: img("outputs/manual_screens/00_mode_selection.png"),
  login: img("outputs/manual_screens/01_login.png"),
  dashboard: img("outputs/manual_screens/02_dashboard.png"),
  devices: img("outputs/manual_screens/03_devices.png"),
  topology: img("outputs/manual_screens/04_topology.png"),
  cameras: img("outputs/manual_screens/05_cameras.png"),
  access: img("outputs/manual_screens/06_access_control.png"),
  pdu: img("outputs/manual_screens/07_pdu.png"),
  iot: img("outputs/manual_screens/08_iot.png"),
  logs: img("outputs/manual_screens/09_logs.png"),
  admin: img("outputs/manual_screens/10_admin.png"),
  monitor: img("outputs/manual_screens/11_monitor.png"),
  iotReal: img("nms-iot-page-v1246.png"),
  embed: img("nms-embed-iot-v1246.png"),
  modbusForm: img("messageImage_1781056616389.jpg"),
  modbusResult: img("messageImage_1781057114664.jpg"),
  mobileMonitor: img("monitor-mobile-qa.png"),
};

function textbox(slide, text, position, style = {}) {
  const shape = slide.shapes.add({
    geometry: "textbox",
    position,
    fill: style.fill ?? "none",
    line: { style: "solid", fill: style.line ?? "none", width: style.lineWidth ?? 0 },
  });
  shape.text = text;
  shape.text.style = {
    fontSize: style.size ?? 21,
    bold: style.bold ?? false,
    color: style.color ?? C.ink,
    typeface: style.mono ? font.mono : font.body,
    alignment: style.align ?? "left",
  };
  return shape;
}

function shape(slide, position, style = {}) {
  return slide.shapes.add({
    geometry: style.geometry ?? "rect",
    position,
    fill: style.fill ?? C.faint,
    line: { style: "solid", fill: style.line ?? C.line, width: style.lineWidth ?? 1 },
  });
}

async function exists(file) {
  return fs.stat(file).then(() => true).catch(() => false);
}

async function readImageBlob(imagePath) {
  const bytes = await fs.readFile(imagePath);
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
}

function contentType(imagePath) {
  return imagePath.toLowerCase().endsWith(".jpg") || imagePath.toLowerCase().endsWith(".jpeg")
    ? "image/jpeg"
    : "image/png";
}

async function addImage(slide, imagePath, position, alt, fit = "contain") {
  if (!(await exists(imagePath))) {
    shape(slide, position, { fill: "#F8FAFC", line: C.line });
    textbox(slide, "圖片待補", {
      left: position.left + 24,
      top: position.top + position.height / 2 - 18,
      width: position.width - 48,
      height: 36,
    }, { size: 24, bold: true, color: C.muted, align: "center" });
    return;
  }
  slide.images.add({
    blob: await readImageBlob(imagePath),
    contentType: contentType(imagePath),
    alt,
    fit,
    position,
  });
}

function header(slide, section, title, index) {
  textbox(slide, section, { left: 46, top: 30, width: 620, height: 26 }, {
    size: 15,
    bold: true,
    color: C.accent,
  });
  textbox(slide, title, { left: 46, top: 64, width: 880, height: 58 }, {
    size: 37,
    bold: true,
    color: C.ink,
  });
  shape(slide, { left: 46, top: 132, width: 1188, height: 1 }, {
    fill: C.line,
    line: C.line,
    lineWidth: 0,
  });
  textbox(slide, String(index).padStart(2, "0"), {
    left: 1160,
    top: 34,
    width: 72,
    height: 30,
  }, { size: 20, bold: true, color: C.muted, align: "right" });
}

function bulletBlock(slide, label, lines, top) {
  textbox(slide, label, { left: 842, top, width: 330, height: 28 }, {
    size: 22,
    bold: true,
    color: C.blue,
  });
  textbox(slide, lines.map((line) => `• ${line}`).join("\n"), {
    left: 842,
    top: top + 38,
    width: 350,
    height: Math.max(52, lines.length * 34),
  }, { size: 19, color: C.ink });
}

async function manualSlide(p, index, section, title, imagePath, blocks, note) {
  const slide = p.slides.add();
  slide.background.fill = C.white;
  header(slide, section, title, index);

  shape(slide, { left: 46, top: 160, width: 760, height: 470 }, {
    fill: "#FFFFFF",
    line: C.line,
    lineWidth: 1,
  });
  await addImage(slide, imagePath, { left: 56, top: 170, width: 740, height: 450 }, title);

  shape(slide, { left: 825, top: 160, width: 380, height: 470 }, {
    fill: C.faint,
    line: C.line,
    lineWidth: 1,
  });
  let top = 184;
  for (const block of blocks) {
    bulletBlock(slide, block.label, block.lines, top);
    top += 38 + Math.max(52, block.lines.length * 34) + 18;
  }
  if (note) {
    shape(slide, { left: 825, top: 648, width: 380, height: 34 }, {
      fill: C.accentSoft,
      line: "#FFD2BF",
      lineWidth: 1,
    });
    textbox(slide, note, { left: 842, top: 655, width: 350, height: 22 }, {
      size: 16,
      bold: true,
      color: C.accent,
    });
  }
}

async function cover(p) {
  const slide = p.slides.add();
  slide.background.fill = "#0B1623";
  await addImage(slide, assets.dashboard, { left: 560, top: 80, width: 640, height: 430 }, "NMS dashboard screenshot", "cover");
  shape(slide, { left: 0, top: 0, width: 1280, height: 720 }, {
    fill: "#0B1623",
    line: "#0B1623",
    lineWidth: 0,
  });
  await addImage(slide, assets.dashboard, { left: 570, top: 92, width: 640, height: 420 }, "NMS dashboard screenshot", "contain");
  textbox(slide, "NMS 圖像式操作手冊", { left: 64, top: 142, width: 520, height: 88 }, {
    size: 52,
    bold: true,
    color: C.white,
  });
  textbox(slide, "每一頁一個功能，對照畫面完成操作", { left: 68, top: 260, width: 480, height: 42 }, {
    size: 27,
    bold: true,
    color: "#FFD2BF",
  });
  textbox(slide, "適用：登入、儀表板、設備、拓樸、攝影機、門禁、PDU、IoT、紀錄、系統管理", {
    left: 68,
    top: 340,
    width: 470,
    height: 90,
  }, { size: 21, color: "#E6EEF7" });
  textbox(slide, "nms_server 使用者版", { left: 68, top: 604, width: 360, height: 32 }, {
    size: 20,
    bold: true,
    color: "#A9B7C8",
  });
}

async function intro(p) {
  const slide = p.slides.add();
  slide.background.fill = C.white;
  header(slide, "開始使用前", "這份手冊照著畫面一步一步操作", 2);
  await addImage(slide, assets.mode, { left: 64, top: 178, width: 520, height: 360 }, "Mode selection screenshot");
  const steps = [
    ["看左邊圖片", "先確認你目前所在頁面是否和圖片相同。"],
    ["看右邊文字", "依序完成操作，不需要理解系統架構。"],
    ["看完成確認", "確認畫面出現預期狀態，再進下一步。"],
    ["遇到不同畫面", "先回上一頁或重新整理，再通知管理者。"],
  ];
  steps.forEach((item, i) => {
    const top = 178 + i * 92;
    shape(slide, { left: 660, top, width: 470, height: 68 }, {
      fill: i === 0 ? C.accentSoft : C.faint,
      line: i === 0 ? "#FFD2BF" : C.line,
    });
    textbox(slide, String(i + 1), { left: 684, top: top + 14, width: 40, height: 32 }, {
      size: 26,
      bold: true,
      color: C.accent,
      align: "center",
    });
    textbox(slide, item[0], { left: 744, top: top + 10, width: 180, height: 28 }, {
      size: 22,
      bold: true,
    });
    textbox(slide, item[1], { left: 744, top: top + 40, width: 330, height: 22 }, {
      size: 17,
      color: C.muted,
    });
  });
}

const slides = [
  {
    section: "登入與進入系統",
    title: "先選擇要進入的使用模式",
    image: assets.mode,
    blocks: [
      { label: "用途", lines: ["決定要進入管理後台或監看畫面"] },
      { label: "操作", lines: ["點選 System 進入管理功能", "點選 Monitor 進入監看大螢幕"] },
      { label: "確認", lines: ["畫面切換到登入頁或監看頁"] },
    ],
    note: "第一次使用建議先進入 System",
  },
  {
    section: "登入與進入系統",
    title: "輸入帳號密碼完成登入",
    image: assets.login,
    blocks: [
      { label: "用途", lines: ["確認操作者身分與權限"] },
      { label: "操作", lines: ["輸入帳號與密碼", "若啟用 2FA，輸入驗證碼", "按下登入"] },
      { label: "確認", lines: ["成功後會進入儀表板"] },
    ],
    note: "帳號問題請找系統管理者重設",
  },
  {
    section: "日常監看",
    title: "先用儀表板確認整體狀態",
    image: assets.dashboard,
    blocks: [
      { label: "用途", lines: ["快速掌握設備在線、離線與告警"] },
      { label: "操作", lines: ["查看上方統計數字", "檢查最近告警與 Top 5 狀態"] },
      { label: "確認", lines: ["離線或告警數量沒有異常增加"] },
    ],
    note: "每天交接班先看這一頁",
  },
  {
    section: "設備管理",
    title: "在設備清單搜尋與查看設備",
    image: assets.devices,
    blocks: [
      { label: "用途", lines: ["找到指定設備並查看目前狀態"] },
      { label: "操作", lines: ["輸入名稱、IP 或類型搜尋", "檢查狀態欄是否正常", "點進設備查看詳細資料"] },
      { label: "確認", lines: ["找到正確設備且狀態可判讀"] },
    ],
    note: "先搜尋再操作，避免改錯設備",
  },
  {
    section: "設備管理",
    title: "新增單一設備到 NMS",
    image: assets.devices,
    blocks: [
      { label: "用途", lines: ["把新設備加入監控清單"] },
      { label: "操作", lines: ["按新增設備", "填寫名稱、IP、設備類型", "選擇 Ping 或 SNMP 監控方式"] },
      { label: "確認", lines: ["清單出現新設備並開始顯示狀態"] },
    ],
    note: "SNMP 資訊需和設備端設定一致",
  },
  {
    section: "設備管理",
    title: "用批次掃描快速建立多台設備",
    image: assets.devices,
    blocks: [
      { label: "用途", lines: ["一次搜尋同網段內的設備"] },
      { label: "操作", lines: ["按批次掃描", "輸入 IP 範圍或網段", "勾選要加入的設備"] },
      { label: "確認", lines: ["被勾選設備加入清單"] },
    ],
    note: "掃描前先確認網段範圍",
  },
  {
    section: "拓樸管理",
    title: "用拓樸圖查看設備連線關係",
    image: assets.topology,
    blocks: [
      { label: "用途", lines: ["看出設備彼此如何連接"] },
      { label: "操作", lines: ["開啟拓樸頁", "查看節點與連線", "點選節點確認設備資訊"] },
      { label: "確認", lines: ["設備位置與連線符合現場狀態"] },
    ],
    note: "拓樸異常通常代表連線或資料需檢查",
  },
  {
    section: "攝影機管理",
    title: "查看攝影機清單與影像狀態",
    image: assets.cameras,
    blocks: [
      { label: "用途", lines: ["確認攝影機是否在線與可播放"] },
      { label: "操作", lines: ["進入攝影機頁", "查看在線狀態", "開啟預覽或播放畫面"] },
      { label: "確認", lines: ["影像可播放且沒有中斷"] },
    ],
    note: "無影像時先確認 RTSP 或網路",
  },
  {
    section: "監看模式",
    title: "用 Monitor 畫面做大螢幕輪播",
    image: assets.monitor,
    blocks: [
      { label: "用途", lines: ["給值班人員或大螢幕持續監看"] },
      { label: "操作", lines: ["從模式選擇進入 Monitor", "確認卡片或影像有更新", "需要時切換到全螢幕"] },
      { label: "確認", lines: ["畫面可長時間顯示不中斷"] },
    ],
    note: "適合放在值班室螢幕",
  },
  {
    section: "門禁管理",
    title: "查看門禁設備與通行狀態",
    image: assets.access,
    blocks: [
      { label: "用途", lines: ["集中查看門禁設備與事件"] },
      { label: "操作", lines: ["進入門禁頁", "查看設備在線狀態", "確認最近事件或異常"] },
      { label: "確認", lines: ["門禁設備在線且事件有更新"] },
    ],
    note: "門禁異常需同步確認現場設備",
  },
  {
    section: "PDU / UPS 管理",
    title: "查看電源設備與插座狀態",
    image: assets.pdu,
    blocks: [
      { label: "用途", lines: ["掌握 PDU、UPS 與插座供電狀態"] },
      { label: "操作", lines: ["進入 PDU 頁", "查看設備電壓、電流與負載", "依權限操作插座開關"] },
      { label: "確認", lines: ["狀態回報正常且操作有記錄"] },
    ],
    note: "遠端斷電前務必確認設備影響",
  },
  {
    section: "IoT / Modbus",
    title: "在 IoT 頁查看感測器與通訊狀態",
    image: assets.iotReal,
    blocks: [
      { label: "用途", lines: ["查看 IoT 裝置、感測值與連線狀態"] },
      { label: "操作", lines: ["進入 IoT 頁", "選擇裝置或通道", "檢查數值與更新時間"] },
      { label: "確認", lines: ["數值持續更新且沒有通訊錯誤"] },
    ],
    note: "數值不變時先看更新時間",
  },
  {
    section: "IoT / Modbus",
    title: "設定 Modbus 讀取位置",
    image: assets.modbusForm,
    blocks: [
      { label: "用途", lines: ["指定要讀取的暫存器與資料長度"] },
      { label: "操作", lines: ["選擇 Function Code", "輸入 Address 與 Quantity", "儲存或執行讀取"] },
      { label: "確認", lines: ["系統沒有顯示格式錯誤"] },
    ],
    note: "Address 需依設備手冊填寫",
  },
  {
    section: "IoT / Modbus",
    title: "確認 Modbus 回傳值是否正確",
    image: assets.modbusResult,
    blocks: [
      { label: "用途", lines: ["確認設備回傳值可被 NMS 解析"] },
      { label: "操作", lines: ["執行讀取", "查看回傳資料", "比對現場儀表或設備手冊"] },
      { label: "確認", lines: ["數值合理且單位正確"] },
    ],
    note: "值異常時檢查位址、倍率與資料型態",
  },
  {
    section: "IoT / 嵌入頁",
    title: "用嵌入頁顯示指定 IoT 畫面",
    image: assets.embed,
    blocks: [
      { label: "用途", lines: ["把 IoT 畫面嵌入其他看板或頁面"] },
      { label: "操作", lines: ["取得嵌入網址", "放入 iframe 或看板系統", "確認畫面可載入"] },
      { label: "確認", lines: ["外部頁面能看到即時資料"] },
    ],
    note: "權限與網路需允許外部載入",
  },
  {
    section: "紀錄查詢",
    title: "用紀錄頁查詢操作與事件",
    image: assets.logs,
    blocks: [
      { label: "用途", lines: ["追查誰在什麼時間做了什麼事"] },
      { label: "操作", lines: ["進入紀錄頁", "選擇時間或關鍵字", "查看事件內容"] },
      { label: "確認", lines: ["找到對應時間與事件記錄"] },
    ],
    note: "處理異常前先保留紀錄",
  },
  {
    section: "報表與匯出",
    title: "匯出資料給維護或稽核使用",
    image: assets.logs,
    blocks: [
      { label: "用途", lines: ["把狀態、事件或紀錄帶出系統"] },
      { label: "操作", lines: ["先完成篩選", "按匯出或下載", "確認檔案內容"] },
      { label: "確認", lines: ["檔案包含正確期間與欄位"] },
    ],
    note: "匯出前先縮小時間範圍",
  },
  {
    section: "系統管理",
    title: "進入管理頁查看系統設定",
    image: assets.admin,
    blocks: [
      { label: "用途", lines: ["管理帳號、權限、License 與系統設定"] },
      { label: "操作", lines: ["進入管理頁", "選擇要設定的項目", "修改後儲存"] },
      { label: "確認", lines: ["設定生效且沒有錯誤提示"] },
    ],
    note: "只有管理員可以修改關鍵設定",
  },
  {
    section: "使用者與權限",
    title: "管理使用者帳號與角色",
    image: assets.admin,
    blocks: [
      { label: "用途", lines: ["控制誰可以查看或修改系統"] },
      { label: "操作", lines: ["新增或編輯使用者", "選擇 Admin、Editor 或 Viewer", "必要時重設密碼"] },
      { label: "確認", lines: ["使用者用新權限登入成功"] },
    ],
    note: "一般值班帳號建議使用 Viewer",
  },
  {
    section: "安全設定",
    title: "啟用 2FA 提高登入安全",
    image: assets.admin,
    blocks: [
      { label: "用途", lines: ["降低帳號密碼外流風險"] },
      { label: "操作", lines: ["開啟使用者安全設定", "掃描 TOTP QR Code", "輸入驗證碼完成綁定"] },
      { label: "確認", lines: ["下次登入會要求 2FA 驗證碼"] },
    ],
    note: "手機遺失時需由管理員重設",
  },
  {
    section: "License 管理",
    title: "確認 License 狀態與可用模組",
    image: assets.admin,
    blocks: [
      { label: "用途", lines: ["確認功能是否可使用"] },
      { label: "操作", lines: ["進入 License 區", "查看到期日與授權項目", "需要時匯入授權檔"] },
      { label: "確認", lines: ["授權狀態顯示正常"] },
    ],
    note: "模組鎖定時先檢查 License",
  },
  {
    section: "備份與還原",
    title: "定期備份系統資料",
    image: assets.admin,
    blocks: [
      { label: "用途", lines: ["保留設定、設備與紀錄資料"] },
      { label: "操作", lines: ["進入備份功能", "建立備份檔", "將檔案保存到指定位置"] },
      { label: "確認", lines: ["備份檔可下載且日期正確"] },
    ],
    note: "更新或大量修改前先備份",
  },
  {
    section: "日常巡檢",
    title: "每天用同一套順序檢查系統",
    image: assets.dashboard,
    blocks: [
      { label: "巡檢順序", lines: ["先看儀表板", "再看離線設備", "最後查告警與紀錄"] },
      { label: "處理原則", lines: ["先確認是否大量異常", "再處理單一設備", "完成後留下紀錄"] },
      { label: "確認", lines: ["交接班時能說明目前狀態"] },
    ],
    note: "固定順序可避免漏看異常",
  },
  {
    section: "異常排除",
    title: "遇到畫面異常時先做基本確認",
    image: assets.devices,
    blocks: [
      { label: "先確認", lines: ["重新整理頁面", "確認登入是否過期", "確認設備是否在線"] },
      { label: "再追查", lines: ["查看紀錄頁", "確認網路與設備電源", "通知管理員或維護人員"] },
      { label: "完成", lines: ["問題排除後記錄原因與時間"] },
    ],
    note: "不要在不確定時直接重啟重要設備",
  },
];

async function writeBlob(file, blob) {
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, new Uint8Array(await blob.arrayBuffer()));
}

async function main() {
  await fs.mkdir(PREVIEW, { recursive: true });
  const presentation = Presentation.create({ slideSize: { width: W, height: H } });
  await cover(presentation);
  await intro(presentation);

  let index = 3;
  for (const spec of slides) {
    await manualSlide(
      presentation,
      index,
      spec.section,
      spec.title,
      spec.image,
      spec.blocks,
      spec.note,
    );
    index += 1;
  }

  for (const [i, slide] of presentation.slides.items.entries()) {
    const stem = `slide-${String(i + 1).padStart(2, "0")}`;
    await writeBlob(path.join(PREVIEW, `${stem}.png`), await presentation.export({ slide, format: "png", scale: 1 }));
    const layout = await slide.export({ format: "layout" });
    await fs.writeFile(path.join(PREVIEW, `${stem}.layout.json`), await layout.text());
  }
  await writeBlob(path.join(PREVIEW, "montage.webp"), await presentation.export({ format: "webp", montage: true, scale: 1 }));

  const pptx = await PresentationFile.exportPptx(presentation);
  await pptx.save(OUT);
  console.log(OUT);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
