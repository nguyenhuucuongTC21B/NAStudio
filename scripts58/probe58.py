#!/usr/bin/env python3
"""PROBE58 — User báo: giọng OD (Quang Sơn…) + nhiều giọng không dùng được,
chuỗi báo "(đã thử 0)". Mục tiêu:
A) api.vieneu.io: 4 giọng OD còn thiếu đã xoay vòng về chưa?
B) ZeroGPU template: vẫn lỗi?
C) KHÁM PHÁ 4 space CPU mới (chưa có trong chuỗi): dongnguyen95,
   nguyenduc1222, whatvn, Tuananh20015/vieneu-tts (docker) — catalog giọng
   + tổng hợp thật end-to-end. Nếu mang preset chuẩn 20-23 giọng → Quang Sơn
   / Mai Anh / Ngọc Trân / Minh Triết sống lại qua CPU không hạn mức.
"""
import json, time, urllib.request, urllib.error, os

UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) probe58/1.0"}
R = []
LOGDIR = "/home/z/my-project/download/scripts58"

def http(url, data=None, headers=None, timeout=30):
    h = dict(UA)
    if headers: h.update(headers)
    req = urllib.request.Request(url, data=data, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, (e.read() or b"")[:3000]
    except Exception as e:
        return -1, str(e).encode()[:300]

def log(*a):
    s = " ".join(str(x) for x in a); print(s, flush=True); R.append(s)

def gradio_call(base, api, data_arr, sse_timeout=150):
    payload = json.dumps({"data": data_arr}).encode()
    st, body = http(f"{base}/gradio_api/call/{api}", data=payload,
                    headers={"Content-Type": "application/json"}, timeout=30)
    if st not in (200, 201):
        return f"http_{st}", body[:150].decode("utf-8", "replace"), 0
    try:
        ev = json.loads(body).get("event_id")
    except Exception as e:
        return "parse_fail", str(e), 0
    t0 = time.time()
    st, body = http(f"{base}/gradio_api/call/{api}/{ev}", timeout=sse_timeout)
    dt = time.time() - t0
    txt = body.decode("utf-8", "replace")
    if "event: complete" in txt:
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

def inspect_space(sid):
    """Trả về (base, api_names, voice_choices) từ /config."""
    base = f"https://{sid.replace('/', '-')}.hf.space"
    st, body = http(base + "/config", timeout=25)
    if st != 200:
        # thử /gradio_api/info
        return base, None, None, f"/config HTTP {st}"
    try:
        cfg = json.loads(body)
    except Exception as e:
        return base, None, None, f"config parse: {e}"
    deps = [(i, d.get("api_name")) for i, d in enumerate(cfg.get("dependencies", []))]
    voices = []
    for comp in (cfg.get("components") or []):
        props = comp.get("props") or {}
        ch = props.get("choices")
        if isinstance(ch, list) and len(ch) >= 5:
            vals = [str(c[0] if isinstance(c, list) else c) for c in ch]
            if len(vals) > len(voices):
                voices = vals
    return base, deps, voices, "OK"

log("=" * 74); log("PROBE58 —", time.strftime("%Y-%m-%d %H:%M:%S")); log("=" * 74)

# ── A) api.vieneu.io: 4 giọng OD còn thiếu ──
log(""); log("A) api.vieneu.io — 4 giọng OD còn thiếu (xoay vòng worker?)")
for v in ["Mai Anh", "Ngọc Trân", "Quang Sơn", "Minh Triết", "Ngọc Huyền"]:
    payload = json.dumps({"text": "Xin chào, kiểm tra kết nối.", "voiceId": v}).encode()
    st, body = http("https://api.vieneu.io/api/tts/demo", data=payload,
                    headers={"Content-Type": "application/json"}, timeout=45)
    if st in (200, 201):
        try:
            j = json.loads(body)
            log(f"  {v:14s} HTTP {st} — audioBase64 {len(j.get('audioBase64',''))} chars ✅")
        except Exception:
            log(f"  {v:14s} HTTP {st} — parse lỗi")
    else:
        snip = body[:100].decode("utf-8", "replace").replace("\n", " ")
        log(f"  {v:14s} HTTP {st} — {snip}")

# ── B) ZeroGPU template + CPU backbone re-check ──
log(""); log("B) re-check nhanh ZeroGPU + CPU backbone")
for sid, api, arr in [
    ("pnnbao-ump/VieNeu-TTS-v3-Turbo", "synthesize",
     ["Xin chào, kiểm tra kết nối.", "Minh Quân Pro", None, 0.8, 25, 0.95, 1.2, 300, 256]),
    ("trangmin11101996/VieNeu-TTS-v3-Turbo", "synthesize",
     ["Xin chào, kiểm tra kết nối.", "Quang Sơn", None, 0.8, 25, 0.95, 1.2, 300, 256]),
    ("hongqminh/VieNeu-TTS", "synthesize_speech",
     ["Xin chào, kiểm tra kết nối.", "Ngọc (nữ miền Bắc)", None, None]),
    ("Tuananh20015/VieNeu-TTS-v3-Turbo", "synthesize",
     ["Xin chào, kiểm tra kết nối.", "Quang Sơn", None, 0.8, 25, 0.95, 1.2, 300, 256]),
]:
    base = f"https://{sid.replace('/', '-')}.hf.space"
    phase, info, dt = gradio_call(base, api, arr)
    tag = "✅" if phase == "complete" else "❌"
    log(f"  {tag} {sid.split('/')[0]:20s} voice={arr[1]!r:20s} → {phase} ({dt:.1f}s) {info[:70] if phase!='complete' else ''}")

# ── C) 4 space CPU mới ──
log(""); log("C) KHÁM PHÁ 4 space mới (cpu-basic, không tốn hạn mức GPU)")
NEW = [
    "dongnguyen95/VieNeu-TTS-v3-Turbo-Web",
    "nguyenduc1222/VieNeu-TTS",
    "whatvn/vietnamese-tts",
    "Tuananh20015/vieneu-tts",
]
for sid in NEW:
    base, deps, voices, note = inspect_space(sid)
    log(f"  ── {sid} [{note}]")
    if deps is None:
        continue
    log(f"     endpoints: {deps[:10]}")
    log(f"     catalog lớn nhất ({len(voices)} mục): {voices[:26]}")
    # chọn api synthesize hợp lý + dựng data theo số tham số qua /info nếu có
    st, body = http(base + "/gradio_api/info", timeout=25)
    api_name, params = None, []
    if st == 200:
        try:
            info = json.loads(body)
            named = info.get("named_endpoints", {})
            for k, v in named.items():
                if "synth" in k or "tts" in k or "speech" in k:
                    api_name = k.strip("/")
                    params = [(p.get("type", "?"), p.get("description", "")[:60])
                              for p in (v.get("parameters") or [])]
                    break
            if api_name is None and named:
                api_name = list(named.keys())[0].strip("/")
                v = named[list(named.keys())[0]]
                params = [(p.get("type", "?"), p.get("description", "")[:60])
                          for p in (v.get("parameters") or [])]
        except Exception as e:
            log(f"     /info parse lỗi: {e}")
    log(f"     /info → api_name={api_name!r} params={params}")
    if api_name and voices:
        # dựng data: tham số dạng Literal giọng → voice; str đầu → text
        voice = voices[0]
        # ưu tiên giọng chuẩn nếu có
        for pref in ["Quang Sơn", "Mai Anh", "Ngọc Trân", "Minh Triết", "Thái Sơn",
                     "Ngọc Linh", "Ngọc (nữ miền Bắc)"]:
            if pref in voices:
                voice = pref
                break
        n = len(params)
        if n == 9:
            arr = ["Xin chào, kiểm tra kết nối.", voice, None, 0.8, 25, 0.95, 1.2, 300, 256]
        elif n == 5:
            arr = ["Xin chào, kiểm tra kết nối.", voice, None, None, None]
        elif n == 4:
            arr = ["Xin chào, kiểm tra kết nối.", voice, None, None]
        elif n == 2:
            arr = ["Xin chào, kiểm tra kết nối.", voice]
        else:
            arr = None
        if arr:
            phase, info2, dt = gradio_call(base, api_name, arr)
            tag = "✅" if phase == "complete" else "❌"
            log(f"     {tag} GỌI THẬT voice={voice!r} → {phase} ({dt:.1f}s) {info2[:70] if phase!='complete' else ''}")
    elif voices:
        # không có named endpoints → thử fn_index qua queue/join (gradio cũ)
        log("     không có named_endpoints — space có thể gradio cũ, thử bỏ qua")

log(""); log("=" * 74)
os.makedirs(LOGDIR, exist_ok=True)
with open(f"{LOGDIR}/probe58.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE — log saved")
