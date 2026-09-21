#!/usr/bin/env python3
"""PROBE56c — (1) vieneu.io API mới: demo trên api.vieneu.io + luồng async job;
(2) eagle0019: thử session_hash + Origin/Referer headers."""
import json, time, re, base64, urllib.request, urllib.error

UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
      "Accept": "application/json, text/plain, */*",
      "Origin": "https://vieneu.io", "Referer": "https://vieneu.io/"}
R = []

def http(url, data=None, headers=None, timeout=30, method=None):
    h = dict(UA)
    if headers: h.update(headers)
    req = urllib.request.Request(url, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, (e.read() or b"")[:3000]
    except Exception as e:
        return -1, str(e).encode()[:300]

def log(*a):
    s = " ".join(str(x) for x in a); print(s, flush=True); R.append(s)

# ── A. đọc bundle đã lưu, tìm context xung quanh tts/demo + async job ──
import glob
bfiles = sorted(glob.glob("/home/z/my-project/scripts/vieneu_bundle_*.js"))
if bfiles:
    js = open(bfiles[-1], "rb").read().decode("utf-8", "replace")
    log(f"bundle: {bfiles[-1]} ({len(js)} chars)")
    for kw in ["tts/demo", "tts/stream", "v1/tts", "tts/audio"]:
        for m in re.finditer(re.escape(kw), js):
            s0 = max(0, m.start() - 260); s1 = min(len(js), m.end() + 260)
            ctx = js[s0:s1].replace("\n", " ")
            log(f"  [{kw}] …{ctx}…")
            break  # 1 context đầu mỗi từ khoá

# ── B. thử demo trên api.vieneu.io ──
log(""); log("B) demo API trên api.vieneu.io:")
for url, payload in [
    ("https://api.vieneu.io/api/tts/demo", {"text": "Xin chào, kiểm tra kết nối.", "voiceId": "Ngọc Lan"}),
    ("https://vieneu.io/api/tts/demo", {"text": "Xin chào, kiểm tra kết nối.", "voiceId": "Ngọc Lan"}),
    ("https://api.vieneu.io/api/v1/tts", {"text": "Xin chào, kiểm tra kết nối.", "voiceId": "Ngọc Lan"}),
]:
    st, body = http(url, data=json.dumps(payload).encode(), headers={"Content-Type": "application/json"}, timeout=45)
    log(f"  POST {url}: HTTP {st} — {body[:220]!r}")

# ── C. eagle0019: session_hash + headers ──
log(""); log("C) eagle0019 — thử biến thể:")
base = "https://eagle0019-VieNeu-TTS-v3-Turbo.hf.space"
variants = [
    ("session_hash", {"data": ["Xin chào.", "Ngọc Linh", None, 0.8, 25, 0.95, 1.2, 300, 256], "session_hash": "hcs56x1"}, {}),
    ("origin+session", {"data": ["Xin chào.", "Ngọc Linh", None, 0.8, 25, 0.95, 1.2, 300, 256], "session_hash": "hcs56x2"}, {"Origin": base, "Referer": base + "/"}),
    ("browser-headers", {"data": ["Xin chào.", "Ngọc Linh", None, 0.8, 25, 0.95, 1.2, 300, 256], "session_hash": "hcs56x3", "trigger_id": None}, {"Accept": "*/*", "Accept-Language": "vi-VN,vi;q=0.9,en;q=0.8", "Cache-Control": "no-cache", "Pragma": "no-cache", "Sec-Fetch-Dest": "empty", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Site": "same-origin"}),
]
for name, payload, extra in variants:
    hh = {"Content-Type": "application/json"}; hh.update(extra)
    st, body = http(base + "/gradio_api/call/synthesize", data=json.dumps(payload).encode(), headers=hh, timeout=25)
    if st not in (200, 201):
        log(f"  [{name}] POST HTTP {st} — {body[:120]!r}"); continue
    ev = json.loads(body).get("event_id")
    st2, body2 = http(base + f"/gradio_api/call/synthesize/{ev}", timeout=120)
    txt = body2.decode("utf-8", "replace") if isinstance(body2, bytes) else str(body2)
    log(f"  [{name}] SSE HTTP {st2} — {txt[:200]!r}")

# ── D. retest DevTam05 + hongqminh lần nữa (xác nhận ổn định) ──
log(""); log("D) retest DevTam05:")
st, body = http("https://devtam05-vieneu-tts.hf.space/gradio_api/call/synthesize", data=json.dumps({"data": ["Kiểm tra lần hai.", "Nam Minh (Nam)"]}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
if st in (200, 201):
    ev = json.loads(body).get("event_id")
    st2, body2 = http(f"https://devtam05-vieneu-tts.hf.space/gradio_api/call/synthesize/{ev}", timeout=120)
    txt = body2.decode("utf-8", "replace")
    ok = "complete" in txt
    log(f"  DevTam05: HTTP {st2}, complete={ok} — {txt[:180]!r}")

log(""); log("E) retest hongqminh (synthes speech 4p):")
st, body = http("https://hongqminh-VieNeu-TTS.hf.space/gradio_api/call/synthesize_speech", data=json.dumps({"data": ["Kiểm tra lần hai.", "Ly (nữ miền Bắc)", None, None]}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
if st in (200, 201):
    ev = json.loads(body).get("event_id")
    st2, body2 = http(f"https://hongqminh-VieNeu-TTS.hf.space/gradio_api/call/synthesize_speech/{ev}", timeout=120)
    txt = body2.decode("utf-8", "replace")
    log(f"  hongqminh: HTTP {st2}, complete={'complete' in txt} — {txt[:180]!r}")

# ── F. ZeroGPU spaces: thử session_hash xem có thoát error:null không ──
log(""); log("F) pnnbao-ump (ZeroGPU) — thử session_hash:")
st, body = http("https://pnnbao-ump-VieNeu-TTS-v3-Turbo.hf.space/gradio_api/call/synthesize", data=json.dumps({"data": ["Xin chào.", "Minh Quân Pro", None, 0.8, 25, 0.95, 1.2, 300, 500], "session_hash": "hcs56z1"}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
if st in (200, 201):
    ev = json.loads(body).get("event_id")
    st2, body2 = http(f"https://pnnbao-ump-VieNeu-TTS-v3-Turbo.hf.space/gradio_api/call/synthesize/{ev}", timeout=60)
    txt = body2.decode("utf-8", "replace")
    log(f"  pnnbao: HTTP {st2} — {txt[:200]!r}")
else:
    log(f"  pnnbao POST HTTP {st} — {body[:120]!r}")

with open("/home/z/my-project/scripts/probe56c.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE")
