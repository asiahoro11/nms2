import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "file:///C:/Users/YoYoAcer/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/.pnpm/playwright-core@1.61.1/node_modules/playwright-core/index.mjs";

const OUT = "D:/claude-sandbox/nms_server/outputs/demo_manual_screens";
await fs.mkdir(OUT, { recursive: true });

const browser = await chromium.launch({
  headless: false,
  executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
  args: ["--start-maximized"],
});
const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
page.setDefaultTimeout(20000);
const logs = [];
page.on("console", (msg) => logs.push({ type: msg.type(), text: msg.text() }));
page.on("pageerror", (err) => logs.push({ type: "pageerror", text: err.message }));
page.on("requestfailed", (req) => logs.push({ type: "requestfailed", url: req.url(), text: req.failure()?.errorText }));

await page.goto("https://nms.yts.idv.tw/", { waitUntil: "domcontentloaded", timeout: 30000 });
await page.waitForTimeout(3000);
await page.screenshot({ path: path.join(OUT, "00_login_page.png") });

await page.waitForFunction(() => {
  const text = document.body?.innerText || "";
  return !text.includes("正在執行安全驗證") && !document.title.includes("請稍候");
}, null, { timeout: 180000 }).catch(() => {});

await page.waitForTimeout(2000);
await page.screenshot({ path: path.join(OUT, "01_after_cloudflare.png") });

const state = await page.evaluate(() => ({
  url: location.href,
  title: document.title,
  bodyText: document.body?.innerText?.slice(0, 1000) ?? "",
  html: document.documentElement?.outerHTML?.slice(0, 2000) ?? "",
  inputs: Array.from(document.querySelectorAll("input")).map((el) => ({
    type: el.type,
    id: el.id,
    name: el.getAttribute("name"),
    placeholder: el.getAttribute("placeholder"),
  })),
  buttons: Array.from(document.querySelectorAll("button")).map((el) => el.innerText || el.getAttribute("aria-label") || el.id),
}));

console.log(JSON.stringify({ state, logs: logs.slice(-30) }, null, 2));
if (state.inputs.length >= 2 || state.bodyText.includes("登入") || state.bodyText.toLowerCase().includes("login")) {
  const userInput = page.locator('input[type="text"], input[name="username"], input#username, input[placeholder*="帳號"], input[placeholder*="Username"], input[placeholder*="username"]').first();
  const passInput = page.locator('input[type="password"], input[name="password"], input#password, input[placeholder*="密碼"], input[placeholder*="Password"], input[placeholder*="password"]').first();
  await userInput.fill("demoadmin");
  await passInput.fill("demoadmin123");
  await page.locator('button[type="submit"], button:has-text("登入"), button:has-text("Login"), input[type="submit"]').first().click();
  await page.waitForTimeout(5000);
  await page.screenshot({ path: path.join(OUT, "02_after_login.png") });
  console.log(JSON.stringify({ afterLoginTitle: await page.title(), afterLoginUrl: page.url() }, null, 2));
}
await browser.close();
