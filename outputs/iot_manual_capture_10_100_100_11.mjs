import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "file:///C:/Users/YoYoAcer/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/.pnpm/playwright-core@1.61.1/node_modules/playwright-core/index.mjs";

const BASE = "http://10.100.100.11:8080";
const OUT = "D:/claude-sandbox/nms_server/outputs/iot_manual_screens";
const USERNAME = "yoyo";
const PASSWORD = "P@ssw0rd";
await fs.mkdir(OUT, { recursive: true });

const browser = await chromium.launch({
  headless: true,
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
});
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
page.setDefaultTimeout(12000);

async function shot(name) {
  await page.screenshot({ path: path.join(OUT, `${name}.png`), fullPage: false, timeout: 30000 });
}

async function state(name) {
  const data = await page.evaluate(() => ({
    url: location.href,
    title: document.title,
    text: document.body?.innerText?.slice(0, 5000) || "",
    activeTab: document.querySelector("#iot .iot-nav-tab.active")?.innerText || "",
    visibleInputs: Array.from(document.querySelectorAll("#iot input, #iot select, #iot textarea"))
      .filter((el) => {
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      })
      .map((el) => ({
        id: el.id,
        tag: el.tagName,
        type: el.getAttribute("type"),
        placeholder: el.getAttribute("placeholder"),
        value: el.value,
      })),
  }));
  await fs.writeFile(path.join(OUT, `${name}.json`), JSON.stringify(data, null, 2), "utf8");
}

async function waitApp() {
  await page.waitForLoadState("domcontentloaded", { timeout: 15000 }).catch(() => {});
  await page.waitForTimeout(1500);
}

async function loginToken() {
  const response = await fetch(`${BASE}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: USERNAME, password: PASSWORD }),
  });
  const json = await response.json();
  if (!json.success) throw new Error(json.error || "login failed");
  return json.data;
}

const login = await loginToken();
await page.addInitScript((data) => {
  sessionStorage.setItem("nms_token", data.token);
  sessionStorage.setItem("nms_user", JSON.stringify(data.user));
  sessionStorage.setItem("nms_expires", String(data.expires_at));
  sessionStorage.setItem("nms_session_active", "true");
  sessionStorage.setItem("nms_active_page", "iot");
  localStorage.setItem("nms_lang", "zh-TW");
}, login);
await page.goto(`${BASE}/?restored=true#iot`, { waitUntil: "domcontentloaded", timeout: 30000 });
await waitApp();
await page.waitForFunction(() => document.querySelector("#iot")?.classList.contains("active"), null, { timeout: 15000 }).catch(() => {});

await shot("10_iot_overview_page");
await state("10_iot_overview_page");

async function clickButtonText(text) {
  const loc = page.locator(`button:visible:has-text("${text}")`).first();
  if (await loc.count()) {
    await loc.click({ timeout: 5000 });
    await page.waitForTimeout(1000);
  }
}

async function switchIotTab(name) {
  await page.evaluate((tabName) => {
    document.querySelectorAll("#iot .iot-tab-pane").forEach((pane) => pane.classList.remove("active"));
    document.querySelectorAll("#iot .iot-nav-tab").forEach((btn) => btn.classList.remove("active"));
    const pane = document.querySelector(`#iot-tab-${tabName}`);
    if (pane) pane.classList.add("active");
    const buttons = Array.from(document.querySelectorAll("#iot .iot-nav-tab"));
    const indexMap = { overview: 0, online_modules: 1, devices: 2, integration: 3 };
    if (buttons[indexMap[tabName]]) buttons[indexMap[tabName]].classList.add("active");
  }, name);
  await page.waitForTimeout(800);
}

await switchIotTab("online_modules");
await shot("11_iot_online_module_page");
await state("11_iot_online_module_page");

await switchIotTab("devices");
await shot("12_iot_devices_page");
await state("12_iot_devices_page");

await switchIotTab("integration");
await shot("13_iot_integration_page");
await state("13_iot_integration_page");

await page.evaluate(() => {
  if (typeof window.iotOpenAddModal === "function") {
    window.iotOpenAddModal();
  } else {
    document.querySelector("#iot-add-modal").style.display = "flex";
  }
});
await page.waitForTimeout(800);
await shot("14_iot_add_device_tcp_modal");
await state("14_iot_add_device_tcp_modal");

await clickButtonText("Modbus RTU");
await shot("15_iot_add_device_rtu_modal");
await state("15_iot_add_device_rtu_modal");

await clickButtonText("Modbus RS485");
await shot("16_iot_add_device_rs485_modal");
await state("16_iot_add_device_rs485_modal");

await clickButtonText("RTU over TCP");
await shot("17_iot_add_device_rtu_tcp_modal");
await state("17_iot_add_device_rtu_tcp_modal");

console.log(JSON.stringify({ out: OUT, user: login.user.username, finalUrl: page.url() }, null, 2));
await browser.close();
