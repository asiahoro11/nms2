import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "file:///C:/Users/YoYoAcer/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/.pnpm/playwright-core@1.61.1/node_modules/playwright-core/index.mjs";

const BASE = "http://10.100.100.11:8080";
const OUT = "D:/claude-sandbox/nms_server/outputs/host_10_100_100_11_screens";
await fs.mkdir(OUT, { recursive: true });

const browser = await chromium.launch({
  headless: true,
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
});
const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
page.setDefaultTimeout(15000);

async function shot(name) {
  await page.screenshot({ path: path.join(OUT, `${name}.png`), fullPage: false });
}

async function waitApp() {
  await page.waitForTimeout(1200);
  await page.waitForLoadState("domcontentloaded", { timeout: 15000 }).catch(() => {});
}

async function clickIfVisible(selector) {
  const loc = page.locator(selector).first();
  if (await loc.count()) {
    if (await loc.isVisible().catch(() => false)) {
      await loc.click().catch(() => {});
      await page.waitForTimeout(700);
      return true;
    }
  }
  return false;
}

async function gotoHash(hash, name) {
  await page.goto(`${BASE}/#${hash}`, { waitUntil: "domcontentloaded", timeout: 30000 });
  await waitApp();
  await shot(name);
}

await page.goto(BASE, { waitUntil: "domcontentloaded", timeout: 30000 });
await waitApp();
await shot("00_entry");

const bodyText = await page.locator("body").innerText({ timeout: 5000 }).catch(() => "");
if (bodyText.includes("管理模式") || bodyText.includes("System") || bodyText.includes("Management Server")) {
  await clickIfVisible("text=進入系統");
  await clickIfVisible("text=System");
  await clickIfVisible("text=管理模式");
  await waitApp();
  await shot("01_mode_selected");
}

const userCandidates = [
  'input[name="username"]',
  "input#username",
  'input[type="text"]',
  'input[placeholder*="帳號"]',
  'input[placeholder*="Username"]',
  'input[placeholder*="username"]',
];
const passCandidates = [
  'input[name="password"]',
  "input#password",
  'input[type="password"]',
  'input[placeholder*="密碼"]',
  'input[placeholder*="Password"]',
  'input[placeholder*="password"]',
];

let userFilled = false;
for (const selector of userCandidates) {
  const loc = page.locator(selector).first();
  if ((await loc.count()) && (await loc.isVisible().catch(() => false))) {
    await loc.fill("demoadmin");
    userFilled = true;
    break;
  }
}
let passFilled = false;
for (const selector of passCandidates) {
  const loc = page.locator(selector).first();
  if ((await loc.count()) && (await loc.isVisible().catch(() => false))) {
    await loc.fill("demoadmin123");
    passFilled = true;
    break;
  }
}
await shot("02_login_filled");
if (userFilled && passFilled) {
  await clickIfVisible('button[type="submit"]');
  await clickIfVisible("button:has-text('登入')");
  await clickIfVisible("button:has-text('Login')");
  await page.waitForTimeout(3000);
}
await shot("03_after_login");

const pages = [
  ["dashboard", "10_dashboard"],
  ["devices", "20_devices"],
  ["topology", "30_topology"],
  ["cameras", "40_cameras"],
  ["access-control", "50_access_control"],
  ["pdu", "60_pdu"],
  ["iot", "70_iot"],
  ["logs", "80_logs"],
  ["admin", "90_admin"],
];

for (const [hash, name] of pages) {
  await gotoHash(hash, name);
}

await gotoHash("iot", "71_iot_overview");
await clickIfVisible("button:has-text('Online Module')");
await clickIfVisible("button:has-text('線上')");
await shot("72_iot_online_modules");
await clickIfVisible("button:has-text('Devices')");
await clickIfVisible("button:has-text('裝置')");
await shot("73_iot_devices");
await clickIfVisible("button:has-text('Integration')");
await clickIfVisible("button:has-text('整合')");
await shot("74_iot_integration");

await gotoHash("access-control", "51_access_doors");
await clickIfVisible("button:has-text('卡片')");
await shot("52_access_cards");
await clickIfVisible("button:has-text('排程')");
await shot("53_access_schedules");
await clickIfVisible("button:has-text('事件')");
await shot("54_access_events");

await gotoHash("logs", "81_logs_overview");
await clickIfVisible("button:has-text('匯出')");
await shot("82_logs_export");

await gotoHash("admin", "91_admin_overview");
const adminTabs = [
  ["使用者", "92_admin_users"],
  ["授權", "93_admin_license"],
  ["安全", "94_admin_security"],
  ["品牌", "95_admin_branding"],
  ["備份", "96_admin_backup"],
  ["模組", "97_admin_modules"],
];
for (const [label, name] of adminTabs) {
  await clickIfVisible(`button:has-text('${label}')`);
  await clickIfVisible(`a:has-text('${label}')`);
  await shot(name);
}

await page.goto(`${BASE}/monitor.html`, { waitUntil: "domcontentloaded", timeout: 30000 }).catch(async () => {
  await page.goto(`${BASE}/monitor`, { waitUntil: "domcontentloaded", timeout: 30000 });
});
await waitApp();
await shot("99_monitor");

const state = await page.evaluate(() => ({
  title: document.title,
  url: location.href,
  text: document.body?.innerText?.slice(0, 1000) ?? "",
}));
await fs.writeFile(path.join(OUT, "capture_state.json"), JSON.stringify(state, null, 2), "utf8");
console.log(JSON.stringify({ out: OUT, final: state }, null, 2));
await browser.close();
