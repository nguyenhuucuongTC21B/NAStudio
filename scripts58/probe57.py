#!/usr/bin/env python3
"""PROBE57 — Vì sao exe build từ FIX54 vẫn có vài giọng đọc bình thường?
Mục tiêu: chụp hiện trạng TỪNG dịch vụ tại thời điểm này (2026-09-21 ~sau FIX56),
tái lập chính xác hành vi chuỗi FIX54 (13 bước, không lọc giọng, ResolveVoice
đổi giọng lạ thành default), từ đó giải thích hiện tượng "vài giọng đọc bình thường".
"""
import json, time, sys, urllib.request, urllib.error, base64

UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) probe57/1.0"}
R = []
LOGDIR = "/home/z/my-project/download/scripts57"

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

def gradio_call(base, api, data_arr, sse_timeout=120):
    """Trả về (phase, payload_snippet, seconds) — phase: complete|error|http_fail"""
    payload = json.dumps({"data": data_arr}).encode()
    st, body = http(f"{base}/gradio_api/call/{api}", data=payload,
                    headers={"Content-Type": "application/json"}, timeout=30)
    if st not in (200, 201):
        return f"http_{st}", body[:120].decode("utf-8", "replace"), 0
    try:
        ev = json.loads(body).get("event_id")
    except Exception as e:
        return "parse_fail", str(e), 0
    t0 = time.time()
    st, body = http(f"{base}/gradio_api/call/{api}/{ev}", timeout=sse_timeout)
    dt = time.time() - t0
    txt = body.decode("utf-8", "replace")
    if "event: complete" in txt:
        # trích url
        url = ""
        for line in txt.splitlines():
            if line.startswith("data:"):
                try:
                    d0 = json.loads(line[5:])
                    if isinstance(d0, list) and d0 and isinstance(d0[0], dict):
                        url = d0[0].get("url", "")
                except Exception:
                    pass
        return "complete", url, dt
    if "event: error" in txt:
        dat = [l[5:].strip() for l in txt.splitlines() if l.startswith("data:")]
        return "error", (dat[-1] if dat else "?")[:80], dt
    return "unknown", txt[:80], dt

def download_head(url, max_bytes=64):
    st, body = http(url, timeout=60)
    if st == 200:
        magic = body[:12]
        is_wav = magic[:4] == b"RIFF"
        is_mp3 = magic[:3] == b"ID3" or (len(magic) > 1 and magic[0] == 0xFF and (magic[1] & 0xE0) == 0xE0)
        kind = "WAV" if is_wav else ("MP3" if is_mp3 else repr(magic[:4]))
        return f"{len(body)}B magic={kind}"
    return f"HTTP {st}"

log("=" * 74)
log("PROBE57 —", time.strftime("%Y-%m-%d %H:%M:%S"))
log("Câu hỏi: vì sao FIX54 (13 bước, không lọc giọng) vẫn có vài giọng đọc bình thường?")
log("=" * 74)

# ── PHẦN 1: HF runtime status ──
SPACES = [
    "pnnbao-ump/VieNeu-TTS-v3-Turbo", "trangmin11101996/VieNeu-TTS-v3-Turbo",
    "eagle0019/VieNeu-TTS-v3-Turbo", "xtieps/VieNeu-TTS-v3-Turbo",
    "thienan2146/VieNeu-TTS-v3-Turbo", "Smrfhdl/tts",
    "doremon102/VieNeu-TTS-v3-Turbo", "kabinz/VieNeu-TTS-v3-Turbo",
    "Tuananh20015/VieNeu-TTS-v3-Turbo", "hongqminh/VieNeu-TTS",
    "DevTam05/vieneu-tts",
]
log(""); log("PHẦN 1 — HF runtime status")
for sid in SPACES:
    st, body = http(f"https://huggingface.co/api/spaces/{sid}", timeout=20)
    if st == 200:
        j = json.loads(body)
        rt = j.get("runtime", {}) or {}
        log(f"  {sid:45s} stage={rt.get('stage','?'):12s} hw={(rt.get('hardware') or {}).get('current','?')}")

# ── PHẦN 2: vieneu.io host CŨ (FIX54 bấm vào đây đầu tiên) ──
log(""); log("PHẦN 2 — vienеu.io host CŨ www.vieneu.io (bước #1 của FIX54)")
for host in ("https://www.vieneu.io", "https://vieneu.io"):
    payload = json.dumps({"text": "Xin chào, kiểm tra kết nối.", "voiceId": "Adam Tốp Tốp"}).encode()
    t0 = time.time()
    st, body = http(f"{host}/api/tts/demo", data=payload,
                    headers={"Content-Type": "application/json"}, timeout=40)
    dt = time.time() - t0
    ok = ""
    if st in (200, 201):
        try:
            j = json.loads(body)
            ok = f"audioBase64={len(j.get('audioBase64',''))} chars — VẪN SỐNG"
        except Exception:
            ok = "2xx nhưng không parse được"
    log(f"  POST {host}/api/tts/demo → HTTP {st} ({dt:.1f}s) {ok} {body[:150] if st not in (200,201) else ''!r}")

# ── PHẦN 3: api.vieneu.io — giọng nào đang được worker nhận? ──
log(""); log("PHẦN 3 — api.vieneu.io (FIX56 bước #1): snapshot giọng app đang nhận")
VOICE_TEST = ["Adam Tốp Tốp", "Ngọc Lan", "Mai Anh", "Minh Đức", "Ngọc Trân",
              "Quang Sơn", "Thùy Dung", "Minh Triết", "Thái Sơn", "Ngọc Linh",
              "Mạnh Dũng"]
avail = []
for v in VOICE_TEST:
    payload = json.dumps({"text": "Xin chào, kiểm tra kết nối.", "voiceId": v}).encode()
    st, body = http("https://api.vieneu.io/api/tts/demo", data=payload,
                    headers={"Content-Type": "application/json"}, timeout=45)
    if st in (200, 201):
        try:
            j = json.loads(body)
            avail.append(v)
            log(f"  {v:20s} HTTP {st} — audioBase64 {len(j.get('audioBase64',''))} chars ✅")
        except Exception:
            log(f"  {v:20s} HTTP {st} — parse lỗi")
    else:
        snip = body[:120].decode("utf-8", "replace").replace("\n", " ")
        log(f"  {v:20s} HTTP {st} — {snip}")
log(f"  → NHẬN ĐƯỢC ({len(avail)}/{len(VOICE_TEST)}): {avail}")

# ── PHẦN 4: các space gradio — hành vi tại thời điểm này ──
log(""); log("PHẦN 4 — gọi thật từng space (đúng giao thức app dùng)")
HQ = "https://hongqminh-vieneu-tts.hf.space"

# hongqminh: /config lấy catalog hiện tại + 3 lần gọi (2 catalog + 1 lạ)
st, body = http(HQ + "/config", timeout=20)
if st == 200:
    cfg = json.loads(body)
    deps = [(i, d.get("api_name")) for i, d in enumerate(cfg.get("dependencies", []))]
    log(f"  [hongqminh] /config OK — endpoints: {deps}")
    # tìm component Literal choices (radio voice_choice)
    lits = []
    for comp in (cfg.get("components") or []):
        props = comp.get("props") or {}
        ch = props.get("choices")
        if isinstance(ch, list) and ch and any("(nữ" in str(c) or "(nam" in str(c) for c in ch):
            lits = [str(c[0] if isinstance(c, list) else c) for c in ch]
    log(f"  [hongqminh] catalog giọng hiện tại: {lits}")
else:
    log(f"  [hongqminh] /config HTTP {st}")

for i, v in enumerate(["Ngọc (nữ miền Bắc)", "Tuyên (nam miền Bắc)", "Minh Đức"]):
    phase, info, dt = gradio_call(HQ, "synthesize_speech", ["Xin chào, kiểm tra kết nối.", v, None, None])
    extra = ""
    if phase == "complete" and info:
        extra = " | " + download_head(info)
    tag = "catalog" if i < 2 else "GIỌNG LẠ (không trong catalog)"
    log(f"  [hongqminh] {tag} voice={v!r:28s} → {phase} ({dt:.1f}s) {info[:90] if phase!='complete' else ''}{extra}")

# eagle0019 / Tuananh20015 (template p9) — còn lỗi không?
for sid, voice in [("eagle0019/VieNeu-TTS-v3-Turbo", "Ngọc Linh"),
                   ("Tuananh20015/VieNeu-TTS-v3-Turbo", "Ngọc Lan"),
                   ("pnnbao-ump/VieNeu-TTS-v3-Turbo", "Minh Quân Pro")]:
    base = f"https://{sid.replace('/', '-')}.hf.space"
    t0 = time.time()
    phase, info, dt = gradio_call(base, "synthesize",
        ["Xin chào, kiểm tra kết nối.", voice, None, 0.8, 25, 0.95, 1.2, 300, 256])
    log(f"  [{sid.split('/')[0]:14s}] voice={voice!r:18s} → {phase} ({dt:.1f}s) {info[:80] if phase!='complete' else info[:90]}")

# DevTam05 (p2) — MP3?
st, body = http("https://devtam05-vieneu-tts.hf.space/config", timeout=20)
log(f"  [DevTam05] /config HTTP {st}")
phase, info, dt = gradio_call("https://devtam05-vieneu-tts.hf.space", "synthesize",
    ["Xin chào, kiểm tra kết nối.", "Nam Minh (Nam)"])
extra = ""
if phase == "complete" and info:
    extra = " | " + download_head(info)
log(f"  [DevTam05] voice='Nam Minh (Nam)' → {phase} ({dt:.1f}s) {info[:90] if phase!='complete' else ''}{extra}")

log(""); log("=" * 74)
log("KẾT LUẬN SỐ: xem phần tổng hợp ở cuối log.")

import os
os.makedirs(LOGDIR, exist_ok=True)
with open(f"{LOGDIR}/probe57.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE — log saved to", f"{LOGDIR}/probe57.log")
