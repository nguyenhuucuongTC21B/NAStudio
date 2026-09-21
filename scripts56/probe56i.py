#!/usr/bin/env python3
"""PROBE56i — dò id hoạt động cho TỪNG giọng app trên demo vieneu.io → mapping JSON."""
import json, time, urllib.request, urllib.error

UA = {"User-Agent": "Mozilla/5.0 Chrome/128", "Origin": "https://vieneu.io", "Referer": "https://vieneu.io/"}

def http(url, data=None, timeout=60):
    h = dict(UA)
    req = urllib.request.Request(url, data=data, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, (e.read() or b"")[:300]
    except Exception as e:
        return -1, str(e).encode()[:200]

def demo(vid):
    st, b = http("https://api.vieneu.io/api/tts/demo", data=json.dumps({"text": "Kiểm tra.", "voiceId": vid}).encode(), timeout=60)
    return st

cat = json.load(open("/home/z/my-project/scripts/vieneu_full_voices_v4.json"))
voices = cat["voices"]
by_name = {}
for v in voices:
    by_name.setdefault(v["name"], []).append(v)

# 25 giọng app + biến thể tên cần thử thêm
APP = ["Adam", "Phạm Tuyên", "Minh Đức", "Thanh Bình", "Ngọc Huyền", "Trúc Ly",
       "Đoan Trang", "Ngọc Linh", "Mai Anh", "Quỳnh Anh", "Quang Sơn", "Ngọc Trân",
       "Xuân Vĩnh", "Thái Sơn", "Minh Triết", "Đức Trí", "Thục Đoan", "Thùy Dung",
       "Mỹ Duyên", "Kim Thanh", "Adam bựa", "Anh Khôi", "Minh Quân Pro",
       "Thiền Tâm Đức", "Mạnh Dũng"]
EXTRA_NAMES = {"Trúc Ly": ["Ly", "Trúc Ly"], "Minh Quân Pro": ["Minh Quân Pro", "Minh Quân"]}

mapping, miss = {}, []
for name in APP:
    cands_names = EXTRA_NAMES.get(name, [name])
    cands = []
    seen = set()
    for cn in cands_names:
        for v in by_name.get(cn, []):
            if v["id"] not in seen:
                seen.add(v["id"])
                cands.append(v)
    ok = None
    for v in cands:
        st = demo(v["id"])
        print(f"  {name!r:16s} try id={v['id']!r:24s} engine={v.get('engine')} → {st}", flush=True)
        if st in (200, 201):
            ok = v["id"]; break
        time.sleep(0.4)
    if ok:
        mapping[name] = ok
    else:
        miss.append(name)
    time.sleep(0.4)

print("\n=== KẾT QUẢ MAPPING (name → id demo-OK) ===")
print(json.dumps(mapping, ensure_ascii=False, indent=1))
print(f"OK: {len(mapping)}/25 — không map được: {miss}")

json.dump({"mapping": mapping, "missing": miss, "verified": "2026-09-21"},
          open("/home/z/my-project/scripts/vieneu_voice_map.json", "w"), ensure_ascii=False, indent=1)
print("saved → scripts/vieneu_voice_map.json")
