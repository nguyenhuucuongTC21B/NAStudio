#!/usr/bin/env python3
"""PROBE58b — xác minh kịch bản user: "Quang Sơn" qua chuỗi sau khi pnnbao hồi phục.
 + retry nguyenduc1222 (4p/5p) + kabinz + hongqminh (flap check)."""
import json, time, urllib.request, urllib.error, os

UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) probe58b/1.0"}
R = []

def http(url, data=None, timeout=40):
    h = dict(UA)
    if data is not None:
        h["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, (e.read() or b"")[:2000]
    except Exception as e:
        return -1, str(e).encode()[:300]

def log(*a):
    s = " ".join(str(x) for x in a); print(s, flush=True); R.append(s)

def gradio_call(base, api, data_arr, sse_timeout=150):
    payload = json.dumps({"data": data_arr}).encode()
    st, body = http(f"{base}/gradio_api/call/{api}", data=payload, timeout=30)
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
        return "error", (dat[-1] if dat else "?")[:60], dt
    return "unknown", txt[:60], dt

log("PROBE58b —", time.strftime("%Y-%m-%d %H:%M:%S"))

# 1) KỊCH BẢN USER: "Quang Sơn" trên pnnbao (vừa hồi phục) + kabinz
P9 = ["Cảm ơn bạn đã kiên nhẫn chờ đợi. Chúng tôi đã khắc phục sự cố.", "Quang Sơn", None, 0.8, 25, 0.95, 1.2, 300, 256]
for sid in ["pnnbao-ump/VieNeu-TTS-v3-Turbo", "kabinz/VieNeu-TTS-v3-Turbo",
            "trangmin11101996/VieNeu-TTS-v3-Turbo"]:
    base = f"https://{sid.replace('/', '-')}.hf.space"
    phase, info, dt = gradio_call(base, "synthesize", P9)
    tag = "✅" if phase == "complete" else "❌"
    log(f"  {tag} {sid.split('/')[0]:20s} voice='Quang Sơn' → {phase} ({dt:.1f}s) {info if phase=='complete' else ''}")

# 2) Mai Anh / Ngọc Trân trên pnnbao (2 giọng OD còn lại thuộc nhóm này)
for v in ["Mai Anh", "Ngọc Trân"]:
    arr = ["Cảm ơn bạn đã kiên nhẫn chờ đợi. Chúng tôi đã khắc phục sự cố.", v, None, 0.8, 25, 0.95, 1.2, 300, 256]
    phase, info, dt = gradio_call("https://pnnbao-ump-vieneu-tts-v3-turbo.hf.space", "synthesize", arr)
    tag = "✅" if phase == "complete" else "❌"
    log(f"  {tag} pnnbao-ump            voice={v!r:14s} → {phase} ({dt:.1f}s)")

# 3) nguyenduc1222 — retry 3 lần, 4p rồi 5p (dự phòng họ giọng hongqminh)
B = "https://nguyenduc1222-vieneu-tts.hf.space"
for i, arr in enumerate([
    ["Xin chào, kiểm tra kết nối.", "Ngọc (nữ miền Bắc)", None, None],
    ["Xin chào, kiểm tra kết nối.", "Ngọc (nữ miền Bắc)", None, None, None],
    ["Xin chào, kiểm tra kết nối.", "Ngọc (nữ miền Bắc)", None, None],
]):
    phase, info, dt = gradio_call(B, "synthesize_speech", arr)
    tag = "✅" if phase == "complete" else "❌"
    log(f"  {tag} nguyenduc1222 thử#{i+1} ({len(arr)}p) → {phase} ({dt:.1f}s) {info if phase=='complete' else ''}")

# 4) hongqminh — đã hồi phục chưa?
phase, info, dt = gradio_call("https://hongqminh-vieneu-tts.hf.space", "synthesize_speech",
    ["Xin chào, kiểm tra kết nối.", "Ngọc (nữ miền Bắc)", None, None, None])
tag = "✅" if phase == "complete" else "❌"
log(f"  {tag} hongqminh voice='Ngọc (nữ miền Bắc)' → {phase} ({dt:.1f}s)")

# 5) eagle0019 — vẫn ổn?
phase, info, dt = gradio_call("https://eagle0019-vieneu-tts-v3-turbo.hf.space", "synthesize",
    ["Xin chào, kiểm tra kết nối.", "Ngọc Linh", None, 0.8, 25, 0.95, 1.2, 300, 256])
tag = "✅" if phase == "complete" else "❌"
log(f"  {tag} eagle0019 voice='Ngọc Linh' → {phase} ({dt:.1f}s)")

os.makedirs("/home/z/my-project/download/scripts58", exist_ok=True)
with open("/home/z/my-project/download/scripts58/probe58b.log", "w") as f:
    f.write("\n".join(R))
print("DONE")
