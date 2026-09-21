#!/usr/bin/env python3
"""PROBE56f — (1) vieneu.io demo: giọng nào nhận được? (OD voices + ngoài featured);
(2) hongqminh ma trận {4p,5p}×{Ngọc,Tuyên,Ly}; (3) DevTam05 giọng Hoài My."""
import json, time, urllib.request, urllib.error

UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/128",
      "Origin": "https://vieneu.io", "Referer": "https://vieneu.io/"}
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

API = "https://api.vieneu.io"

log("A) vieneu.io (api.vieneu.io) — voiceId tự do?")
test_voices = ["Adam Tốp Tốp", "Ngọc Lan", "Thái Sơn", "Ngọc Linh", "Mai Anh", "Giọng Không Tồn Tại 123"]
for v in test_voices:
    st, body = http(API + "/api/tts/demo", data=json.dumps({"text": "Kiểm tra giọng này.", "voiceId": v}).encode(), headers={"Content-Type": "application/json"}, timeout=60)
    if st in (200, 201):
        try:
            j = json.loads(body)
            ab = j.get("audioBase64") or ""
            log(f"  voiceId={v!r:28s} HTTP {st} — audioBase64 {len(ab)} chars ✅")
        except Exception as e:
            log(f"  voiceId={v!r:28s} HTTP {st} parse lỗi {e}")
    else:
        log(f"  voiceId={v!r:28s} HTTP {st} — {body[:160]!r}")

log(""); log("A2) list voices đầy đủ?")
for path in ["/api/tts/voices?engine=v4", "/api/tts/voices", "/api/tts/voices/ranking?window=7d"]:
    st, body = http(API + path, timeout=25)
    ln = len(body)
    head = body[:180]
    log(f"  GET {path}: HTTP {st} len={ln} — {head!r}")

log(""); log("B) hongqminh ma trận:")
BASE = "https://hongqminh-VieNeu-TTS.hf.space"
def hq_call(params):
    st, body = http(BASE + "/gradio_api/call/synthesize_speech", data=json.dumps({"data": params, "session_hash": "p56f"}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
    if st not in (200, 201): return f"POST HTTP {st}"
    try: ev = json.loads(body).get("event_id")
    except Exception: return f"event_id fail {body[:60]!r}"
    st2, b2 = http(BASE + f"/gradio_api/call/synthesize_speech/{ev}", timeout=110)
    txt = b2.decode("utf-8", "replace")
    if "complete" in txt:
        mu = None
        import re
        mu = re.search(r'"url":\s*"([^"]+)"', txt)
        return f"OK ✅ → {(mu.group(1) if mu else '')[:70]}"
    if "error" in txt:
        import re
        em = re.search(r'data:\s*(.*)', txt)
        return f"error → {(em.group(1) if em else '?')[:100]}"
    return f"khác {txt[:100]!r}"

for shape in (5, 4):
    for voice in ["Ngọc (nữ miền Bắc)", "Tuyên (nam miền Bắc)", "Ly (nữ miền Bắc)"]:
        params = ["Xin chào thử nghiệm.", voice, None, None] + ([None] if shape == 5 else [])
        t0 = time.time()
        res = hq_call(params)
        log(f"  {shape}p voice={voice!r:26s} {time.time()-t0:5.1f}s {res}")

log(""); log("C) DevTam05 — Hoài My (Nữ):")
st, body = http("https://devtam05-vieneu-tts.hf.space/gradio_api/call/synthesize", data=json.dumps({"data": ["Thử giọng nữ.", "Hoài My (Nữ)"], "session_hash": "p56f"}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
if st in (200, 201):
    ev = json.loads(body).get("event_id")
    st2, b2 = http(f"https://devtam05-vieneu-tts.hf.space/gradio_api/call/synthesize/{ev}", timeout=110)
    txt = b2.decode("utf-8", "replace")
    log(f"  Hoài My: HTTP {st2} — {'OK ✅' if 'complete' in txt else txt[:120]!r}")

with open("/home/z/my-project/scripts/probe56f.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE")
