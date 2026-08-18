import fs from "node:fs/promises";
import path from "node:path";

const dir = "D:/claude-sandbox/nms_server/outputs/ppt_work/iot_operation_manual/tmp/preview";
const files = (await fs.readdir(dir)).filter((f) => f.endsWith(".layout.json")).sort();
const problems = [];
for (const file of files) {
  const raw = await fs.readFile(path.join(dir, file), "utf8");
  const data = JSON.parse(raw);
  const items = [];
  function walk(v) {
    if (!v || typeof v !== "object") return;
    if (Array.isArray(v)) {
      v.forEach(walk);
      return;
    }
    if (v.bbox && Array.isArray(v.bbox)) items.push(v);
    Object.values(v).forEach(walk);
  }
  walk(data);
  for (const item of items) {
    const [x, y, w, h] = item.bbox;
    if (x < -1 || y < -1 || x + w > 1281 || y + h > 721) {
      problems.push({ file, name: item.name || item.id || item.kind, bbox: item.bbox });
    }
  }
}
console.log(JSON.stringify({ checked: files.length, problems }, null, 2));
if (problems.length) process.exitCode = 1;
