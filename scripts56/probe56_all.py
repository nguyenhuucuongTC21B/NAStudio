#!/usr/bin/env python3
"""PROBE56 — Báo lỗi user: TẤT CẢ dịch vụ online đều thất bại.
Probe thật từng dịch vụ (2026-09-21) để tìm nguyên nhân gốc."""
import json, time, sys, urllib.request, urllib.error

UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) probe56/1.0"}
R = []

def http(url, data=None, headers=None, timeout=30, method=None):
    h = dict(UA)
    if headers: h.update(headers)
    req = urllib.request.Request(url, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, (e.read() or b"")[:2000]
    except Exception as e:
        return -1, str(e).encode()[:300]

def log(*a):
    s = " ".join(str(x) for x in a); print(s, flush=True); R.append(s)

SPACES = [
    ("pnnbao-ump/VieNeu-TTS-v3-Turbo", "Minh Quân Pro"),
    ("trangmin11101996/VieNeu-TTS-v3-Turbo", "Minh Quân"),
    ("eagle0019/VieNeu-TTS-v3-Turbo", "Ngọc Linh"),
    ("xtieps/VieNeu-TTS-v3-Turbo", "Phạm Tuyên"),
    ("thienan2146/VieNeu-TTS-v3-Turbo", "Adam"),
    ("Smrfhdl/tts", "Minh Đức"),
    ("doremon102/VieNeu-TTS-v3-Turbo", "Minh Quân"),
    ("kabinz/VieNeu-TTS-v3-Turbo", "Minh Quân Pro"),
    ("Tuananh20015/VieNeu-TTS-v3-Turbo", "Ngọc Lan"),
    ("hongqminh/VieNeu-TTS", "Tuyên (nam miền Bắc)"),
    ("DevTam05/vieneu-tts", "Nam Minh (Nam)"),
]

# ── 1. HF API status từng space ──
log("=" * 70); log("PHẦN 1 — HF API: runtime status từng space"); log("=" * 70)
for sid, _ in SPACES:
    st, body = http(f"https://huggingface.co/api/spaces/{sid}", timeout=20)
    if st == 200:
        try:
            j = json.loads(body)
            rt = j.get("runtime", {}) or {}
            stage = rt.get("stage", "?")
            hw = ((rt.get("hardware") or {}).get("current") or "?")
            sdk = (j.get("sdk") or "?")
            likes = j.get("likes", 0)
            log(f"  {sid:45s} stage={stage:12s} hw={hw:10s} sdk={sdk} likes={likes}")
        except Exception as e:
            log(f"  {sid:45s} HTTP 200 nhưng parse lỗi: {e}")
    else:
        log(f"  {sid:45s} HTTP {st} — {body[:120]!r}")

# ── 2. vieneu.io ──
log(""); log("=" * 70); log("PHẦN 2 — vieneu.io (dịch vụ #1)"); log("=" * 70)
st, body = http("https://vieneu.io/api/tts/voices/featured?engine=v4", timeout=25)
log(f"  featured API: HTTP {st}, len={len(body)}")
if st == 200:
    try:
        j = json.loads(body)
        vs = j if isinstance(j, list) else j.get("voices", j.get("data", []))
        log(f"  voices count: {len(vs) if isinstance(vs, list) else type(vs)}")
        if isinstance(vs, list) and vs:
            log(f"  first: {json.dumps(vs[0], ensure_ascii=False)[:200]}")
    except Exception as e:
        log(f"  parse lỗi: {e} — body[:200]={body[:200]!r}")
else:
    log(f"  body[:300]={body[:300]!r}")

# POST demo synthesis
try:
    payload = json.dumps({"text": "Xin chào, kiểm tra kết nối.", "voiceId": "Ngọc Lan"}).encode()
except Exception:
    payload = json.dumps({"text": "Xin chào, kiểm tra kết nối.", "voiceId": 1}).encode()
st, body = http("https://vieneu.io/api/tts/demo", data=payload,
                headers={"Content-Type": "application/json"}, timeout=40)
log(f"  POST /api/tts/demo: HTTP {st}, len={len(body)}")
if st not in (200, 201):
    log(f"  body[:400]={body[:400]!r}")
else:
    try:
        j = json.loads(body)
        ab = j.get("audioBase64") or ""
        log(f"  OK: audioBase64 {len(ab)} chars, mime={j.get('mimeType')}")
    except Exception as e:
        log(f"  parse lỗi: {e}; body[:200]={body[:200]!r}")

# ── 3. Gradio spaces: gọi thật 1 câu ngắn ──
log(""); log("=" * 70); log("PHẦN 3 — Gradio spaces: tổng hợp thật 1 câu ngắn"); log("=" * 70)

def probe_gradio(sid, voice, style):
    base = f"https://{sid.replace('/', '-')}.hf.space"
    # /config để xác nhận sống
    st, body = http(base + "/config", timeout=20)
    if st != 200:
        log(f"  [{sid}] /config HTTP {st} → space KHÔNG phản hồi")
        return
    # lấy fn_index + api_name của endpoint synthesize
    try:
        cfg = json.loads(body)
        deps = cfg.get("dependencies", [])
        api_names = [(i, d.get("api_name")) for i, d in enumerate(deps)]
        log(f"  [{sid}] /config OK — endpoints: {api_names}")
    except Exception as e:
        log(f"  [{sid}] /config parse lỗi: {e}")
        return
    # gọi synthesize
    if style == "p5":
        data = {"data": [voice, "Xin chào kiểm tra."]}
    elif style == "p9":
        data = {"data": ["Xin chào kiểm tra.", voice, None, 1.0, 10, 1.0, 3.0, 60, 500]}
    elif style == "p2":
        data = {"data": ["Xin chào kiểm tra.", voice]}
    else:
        data = {"data": ["Xin chào kiểm tra.", voice, None, None, None]}
    payload = json.dumps(data).encode()
    t0 = time.time()
    st, body = http(base + "/gradio_api/call/synthesize", data=payload,
                    headers={"Content-Type": "application/json"}, timeout=30)
    if st not in (200, 201):
        log(f"  [{sid}] POST call HTTP {st} — {body[:200]!r}")
        return
    try:
        ev = json.loads(body).get("event_id")
    except Exception as e:
        log(f"  [{sid}] event_id parse lỗi: {e} — {body[:150]!r}")
        return
    # SSE
    t0 = time.time()
    st, body = http(base + f"/gradio_api/call/synthesize/{ev}", timeout=120)
    dt = time.time() - t0
    txt = body.decode("utf-8", "replace") if isinstance(body, bytes) else str(body)
    lines = [l for l in txt.splitlines() if l.strip()]
    interesting = [l[:160] for l in lines if any(k in l.lower() for k in ("complete", "error", "data:", "event:"))][:8]
    log(f"  [{sid}] SSE HTTP {st} — {dt:.1f}s — {len(body)} bytes")
    for l in interesting:
        log(f"      {l}")

for sid, voice in SPACES:
    if "hongqminh" in sid: style = "p5"
    elif "devtam05" in sid: style = "p2"
    else: style = "p9"
    try:
        probe_gradio(sid, voice, style)
    except Exception as e:
        log(f"  [{sid}] EXCEPTION: {e}")

with open("/home/z/my-project/scripts/probe56_all.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE — log saved")
