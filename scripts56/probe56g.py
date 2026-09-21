#!/usr/bin/env python3
"""PROBE56g — phân tích FULL catalog vieneu.io + demo id vs name + khớp OD voices."""
import json, time, urllib.request, urllib.error

UA = {"User-Agent": "Mozilla/5.0 Chrome/128", "Origin": "https://vieneu.io", "Referer": "https://vieneu.io/"}
R = []

def http(url, data=None, headers=None, timeout=60):
    h = dict(UA)
    if headers: h.update(headers)
    req = urllib.request.Request(url, data=data, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, (e.read() or b"")[:2500]
    except Exception as e:
        return -1, str(e).encode()[:300]

def log(*a):
    s = " ".join(str(x) for x in a); print(s, flush=True); R.append(s)

st, body = http("https://api.vieneu.io/api/tts/voices?engine=v4", timeout=40)
j = json.loads(body)
voices = j.get("voices", [])
log(f"FULL catalog: HTTP {st} — {len(voices)} giọng (keys={list(voices[0].keys())})")

# thống kê region/gender
from collections import Counter
regions = Counter(v.get("region") for v in voices)
genders = Counter(v.get("gender") for v in voices)
log(f"  regions: {dict(regions)} | genders: {dict(genders)}")

# name → id map (check trùng tên)
names = {}
dups = []
for v in voices:
    n = v.get("name", "")
    if n in names: dups.append(n)
    names[n] = v.get("id")
log(f"  unique names: {len(names)} — trùng tên: {dups[:10]}")

# OD voices + template-space voices phổ biến có trong catalog?
od = ["Mai Anh", "Minh Đức", "Ngọc Trân", "Quang Sơn", "Thùy Dung", "Minh Triết", "Thái Sơn", "Ngọc Linh",
      "Ngọc Lan", "Minh Quân", "Phạm Tuyên", "Mạnh Dũng", "Trúc Ly", "Anh Khôi", "Ngọc Huyền"]
log("  giọng app → catalog vieneu.io:")
for n in od:
    vid = names.get(n)
    log(f"    {n!r:18s} → {'✅ ' + vid if vid else '❌ không có'}")

# vài giọng tin tức/kể chuyện mẫu từng vùng (để gợi ý thay thế OD nếu thiếu)
def sample(region, gender, style_kw, k=4):
    out = []
    for v in voices:
        d = (v.get("description") or "").lower()
        if v.get("region") == region and v.get("gender") == gender and style_kw in d:
            out.append(f"{v['name']} ({v['id']}) — {v.get('description')}")
            if len(out) >= k: break
    return out

log("  mẫu nam Bắc tin tức:", sample("north", "male", "tin tức", 3))
log("  mẫu nữ Bắc tin tức:", sample("north", "female", "tin tức", 3))
log("  mẫu kể chuyện nữ:", sample("north", "female", "kể chuyện", 3))
log("  mẫu kể chuyện nam:", sample("north", "male", "kể chuyện", 3))

# demo: id vs name
log(""); log("demo voiceId dạng ID vs NAME:")
t = [("id của Minh Đức", names.get("Minh Đức")), ("name 'Minh Đức'", "Minh Đức"),
     ("name 'Ngọc Lan'", "Ngọc Lan"), ("id của Ngọc Lan", names.get("Ngọc Lan"))]
for label, vid in t:
    if not vid: log(f"  {label}: None — bỏ qua"); continue
    st2, b2 = http("https://api.vieneu.io/api/tts/demo", data=json.dumps({"text": "Kiểm tra.", "voiceId": vid}).encode(), headers={"Content-Type": "application/json"}, timeout=60)
    if st2 in (200, 201):
        ab = (json.loads(b2).get("audioBase64") or "")
        log(f"  {label} ({vid!r}): HTTP {st2} — {len(ab)} chars ✅")
    else:
        log(f"  {label} ({vid!r}): HTTP {st2} — {b2[:120]!r}")

# lưu catalog để patch registry
with open("/home/z/my-project/scripts/vieneu_full_voices_v4.json", "w") as f:
    json.dump(j, f, ensure_ascii=False)
log(f"\nđã lưu catalog → scripts/vieneu_full_voices_v4.json ({len(json.dumps(j))//1024} KB)")

with open("/home/z/my-project/scripts/probe56g.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE")
