#!/usr/bin/env python3
"""PROBE56h — vẽ ranh giới demo vieneu.io: engine/group nào được nhận."""
import json, urllib.request, urllib.error

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
        return e.code, (e.read() or b"")[:400]
    except Exception as e:
        return -1, str(e).encode()[:300]

def log(*a):
    s = " ".join(str(x) for x in a); print(s, flush=True); R.append(s)

cat = json.load(open("/home/z/my-project/scripts/vieneu_full_voices_v4.json"))
voices = cat["voices"]
by_name = {}
for v in voices:
    by_name.setdefault(v["name"], []).append(v)

def demo(voice_id, engine=None):
    p = {"text": "Kiểm tra nhanh.", "voiceId": voice_id}
    if engine: p["engine"] = engine
    st, b = http("https://api.vieneu.io/api/tts/demo", data=json.dumps(p).encode(), headers={"Content-Type": "application/json"}, timeout=60)
    return st

# 1) examine các giọng đã test
log("A) catalog entry của các giọng đã test:")
for n in ["Adam Tốp Tốp", "Ngọc Lan", "Minh Đức", "Mai Anh", "Thái Sơn", "Ngọc Linh", "Mạnh Dũng", "Phạm Tuyên"]:
    ents = by_name.get(n, [])
    if not ents: log(f"  {n!r:16s}: không trong catalog"); continue
    for v in ents[:2]:
        log(f"  {n!r:16s}: id={v['id']!r} engine={v.get('engine')!r} group={v.get('group')!r} style={v.get('style')!r} podcast={v.get('podcast')!r}")

# 2) test các giọng template-space + vài id dạng vieneu-N
log(""); log("B) demo từng giọng:")
tests = ["Mai Anh", "Minh Đức", "Thái Sơn", "Ngọc Linh", "Mạnh Dũng", "Phạm Tuyên",
         "Quang Sơn", "Thùy Dung", "Minh Triết", "Ngọc Trân", "Adam Tốp Tốp", "Ngọc Lan"]
for n in tests:
    ents = by_name.get(n)
    if not ents:
        log(f"  {n!r:14s}: ❌ không trong catalog"); continue
    v = ents[0]
    st = demo(v["id"])
    log(f"  {n!r:14s} id={v['id']!r:22s} engine={v.get('engine')!r:6s} → HTTP {st} {'✅' if st in (200,201) else '❌'}")

# 3) test gửi kèm engine
log(""); log("C) demo kèm engine param:")
for n, eng in [("Mai Anh", "v2"), ("Thái Sơn", None), ("Ngọc Linh", None), ("Minh Triết", "v1")]:
    ents = by_name.get(n)
    if not ents: continue
    v = ents[0]
    e = eng or v.get("engine")
    st = demo(v["id"], engine=e)
    log(f"  {n!r:14s} id={v['id']!r:22s} engine={e!r:6s} → HTTP {st} {'✅' if st in (200,201) else '❌'}")

# 4) thống kê engine trong catalog
from collections import Counter
engs = Counter(v.get("engine") for v in voices)
grps = Counter(v.get("group") for v in voices)
log(""); log(f"D) catalog engines: {dict(engs)}")
log(f"   groups: {dict(grps)}")

# featured 10 engine gì?
st, b = http("https://api.vieneu.io/api/tts/voices/featured?engine=v4", timeout=25)
feat = json.loads(b)
log(""); log(f"E) featured engine=v4: {[x.get('name') for x in (feat if isinstance(feat, list) else feat.get('voices', []))]}")

with open("/home/z/my-project/scripts/probe56h.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE")
