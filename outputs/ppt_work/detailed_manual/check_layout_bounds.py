import json
from pathlib import Path

base = Path("outputs/ppt_work/detailed_manual/preview")
bad = []
files = sorted(base.glob("slide-*.layout.json"))

for file in files:
    data = json.loads(file.read_text(encoding="utf-8"))
    stack = [data]
    while stack:
        item = stack.pop()
        if isinstance(item, dict):
            pos = item.get("position") or item.get("bounds")
            if isinstance(pos, dict) and all(k in pos for k in ("left", "top", "width", "height")):
                left = float(pos["left"])
                top = float(pos["top"])
                width = float(pos["width"])
                height = float(pos["height"])
                if left < -1 or top < -1 or left + width > 1281 or top + height > 721:
                    bad.append((file.name, item.get("name") or item.get("id"), pos))
            stack.extend(item.values())
        elif isinstance(item, list):
            stack.extend(item)

print("layout_files", len(files))
print("out_of_bounds", len(bad))
print(bad[:5])
