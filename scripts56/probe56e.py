#!/usr/bin/env python3
"""PROBE56e — check runtime từng space candidate + probe end-to-end cái RUNNING."""
import json, time, re, urllib.request, urllib.error
from concurrent.futures import ThreadPoolExecutor

UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/128"}
R = []

def http(url, data=None, headers=None, timeout=30):
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

CANDS = [
    "05chatgpt/vietnamese-tts", "Arrcttacsrks/VieNeu-TTS-Run-On-CPU", "anhnt1289/tts-vietnamese",
    "daviius/VieNeu-TTS", "dongnguyen95/VieNeu-TTS-v3-Turbo-Web", "doremon102/VieNeu-TTS-v3-Turbo",
    "eagle0019/VieNeu-TTS-v3-Turbo", "fdsbk204/VieNeu-TTS-0.3B", "hongqminh/VieNeu-TTS",
    "hungnk221/VieNeu-TTS", "hungthai84/VieNeuTTS-Platfrom", "kabinz/VieNeu-TTS-v3-Turbo",
    "namnguyenpat/vieneu-tts", "ndk8386/vieneu-tts-api", "ngocquy0201/vieneu-tts",
    "nguyenduc1222/VieNeu-TTS", "phan-kim-tu/VieNeu-TTS-v3-Turbo", "phucsd/vieneu-tts-kaggle",
    "pnnbao-ump/VieNeu-TTS-v3-Turbo", "Smrfhdl/tts", "Tuananh20015/VieNeu-TTS-v3-Turbo",
    "trangmin11101996/VieNeu-TTS-v3-Turbo", "vipsphi/VieNeu-TTS-0.3B", "xtieps/VieNeu-TTS-v3-Turbo",
    "thienan2146/VieNeu-TTS-v3-Turbo", "vinhzdz192/TTS-VIETNAMESE", "whatvn/vietnamese-tts",
    "DevTam05/vieneu-tts", "michsethowusu/VieNeu-TTS-Twi-Demo", "trinhvanhung/vietnamese-tts",
    "phanphuc2609/vietnamese-tts-server", "thao2005/vietnamese-tts-v1",
]

def runtime(sid):
    st, body = http(f"https://huggingface.co/api/spaces/{sid}", timeout=15)
    if st != 200:
        return sid, f"HTTP{st}", "", ""
    j = json.loads(body)
    rt = j.get("runtime", {}) or {}
    stage = rt.get("stage", "?")
    hw = (rt.get("hardware") or {}).get("current") or "?"
    sdk = j.get("sdk") or "?"
    lm = j.get("lastModified", "")
    return sid, stage, f"{sdk}/{hw}", str(lm)[:10]

log("A) runtime từng space (parallel):")
with ThreadPoolExecutor(max_workers=12) as ex:
    for sid, stage, sdkhw, lm in ex.map(runtime, CANDS):
        flag = " <<<" if stage == "RUNNING" else ""
        log(f"  {sid:48s} {stage:12s} {sdkhw:20s} {lm}{flag}")

# ── probe end-to-end các space RUNNING chưa biết ──
def probe_space(sid):
    base = f"https://{sid.replace('/', '-')}.hf.space"
    st, body = http(base + "/config", timeout=18)
    if st != 200: return f"/config HTTP {st}"
    try: cfg = json.loads(body)
    except Exception as e: return f"config parse {e}"
    deps = cfg.get("dependencies", [])
    syn = [(i, d.get("api_name")) for i, d in enumerate(deps) if d.get("api_name")]
    if not syn: return f"deps không tên: {len(deps)}"
    epname = None
    for _, nm in syn:
        if nm == "synthesize": epname = "synthesize"; break
    if not epname:
        for _, nm in syn:
            if "synth" in nm or "speech" in nm: epname = nm; break
    if not epname: return f"không syn endpoint: {[n for _, n in syn][:6]}"
    nparams, voices = 0, []
    st, info = http(base + "/gradio_api/info", timeout=18)
    if st == 200:
        try:
            ij = json.loads(info)
            spec = ij.get("named_endpoints", {}).get("/" + epname) or {}
            ps = spec.get("parameters", []); nparams = len(ps)
            for p in ps:
                if p.get("parameter_name") in ("voice", "voice_choice", "voice_id"):
                    lit = str(p.get("python_type", {}).get("type", ""))
                    mm = re.search(r"Literal\[(.*)\]", lit)
                    if mm: voices = re.findall(r"'([^']+)'", mm.group(1))
        except Exception: pass
    def pk(text, voice):
        if epname == "synthesize" and nparams >= 9:
            return [text, voice, None, 0.8, 25, 0.95, 1.2, 300, 500]
        if epname == "synthesize_speech":
            return [text, voice, None, None]
        return [text, voice]
    voice = voices[0] if voices else "Nam Minh (Nam)"
    st2, b2 = http(base + f"/gradio_api/call/{epname}", data=json.dumps({"data": pk("Xin chào thử nghiệm.", voice), "session_hash": "p56e"}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
    if st2 not in (200, 201): return f"POST HTTP {st2}"
    try: ev = json.loads(b2).get("event_id")
    except Exception: return f"event_id fail {b2[:60]!r}"
    st3, b3 = http(base + f"/gradio_api/call/{epname}/{ev}", timeout=110)
    txt = b3.decode("utf-8", "replace")
    if "complete" in txt:
        mu = re.search(r'"url":\s*"([^"]+)"', txt)
        return f"OK ✅ ep={epname} params={nparams} voices={len(voices)} → {(mu.group(1) if mu else '')[:80]}"
    if "error" in txt:
        em = re.search(r'data:\s*(.*)', txt)
        return f"error → {(em.group(1) if em else '?')[:120]}"
    return f"khác {txt[:100]!r}"

log(""); log("B) probe end-to-end space RUNNING:")
def full_check(sid):
    _, stage, _, _ = runtime(sid)
    if stage != "RUNNING": return sid, stage, "— skip (không RUNNING)"
    t0 = time.time()
    try: res = probe_space(sid)
    except Exception as e: res = f"EXCEPTION {e}"
    return sid, stage, f"{time.time()-t0:5.1f}s {res}"

with ThreadPoolExecutor(max_workers=8) as ex:
    for sid, stage, res in ex.map(full_check, CANDS):
        if "skip" in res: continue
        log(f"  {sid:48s} [{stage}] {res}")

with open("/home/z/my-project/scripts/probe56e.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE")
