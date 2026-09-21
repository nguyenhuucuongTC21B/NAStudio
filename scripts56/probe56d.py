#!/usr/bin/env python3
"""PROBE56d — (1) repo model pnnbao-ump còn file không; (2) search lại toàn bộ
HF spaces vieneu + probe từng space RUNNING để tìm dịch vụ sống thực."""
import json, time, re, urllib.request, urllib.error

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

# ── 1. repo model (nguồn offline weights) ──
log("=" * 70); log("1) HF model repo pnnbao-ump/VieNeu-TTS-v3-Turbo"); log("=" * 70)
for repo in ["pnnbao-ump/VieNeu-TTS-v3-Turbo", "pnnbao-ump/VieNeu-TTS"]:
    st, body = http(f"https://huggingface.co/api/models/{repo}", timeout=20)
    if st == 200:
        j = json.loads(body)
        sib = [f"{s['rfilename']}({s.get('size',0)//1024//1024}MB)" for s in j.get("siblings", [])][:20]
        log(f"  {repo}: HTTP 200 lastModified={j.get('lastModified')} files={sib}")
    else:
        log(f"  {repo}: HTTP {st} — {body[:150]!r}")

# ── 2. search spaces vieneu ──
log(""); log("=" * 70); log("2) HF search spaces 'vieneu' + 'VieNeu-TTS'"); log("=" * 70)
cands = {}
for q in ["vieneu", "VieNeu-TTS", "vietnamese tts"]:
    st, body = http(f"https://huggingface.co/api/spaces?search={urllib.parse.quote(q)}&limit=60", timeout=25)
    if st != 200: log(f"  search {q!r}: HTTP {st}"); continue
    try:
        items = json.loads(body)
    except Exception:
        continue
    for it in items:
        sid = it.get("id", "")
        if not sid: continue
        cands[sid] = it
log(f"  tổng candidate unique: {len(cands)}")
running = []
for sid, it in sorted(cands.items()):
    stage = ((it.get("runtime") or {}).get("stage")) or it.get("stage") or "?"
    sdk = it.get("sdk") or "?"
    likes = it.get("likes", 0)
    mark = ""
    if stage == "RUNNING" and sdk in ("gradio", "docker"):
        running.append((sid, sdk)); mark = "  <<< RUNNING"
    log(f"  {sid:55s} {stage:10s} {sdk:8s} likes={likes}{mark}")

# ── 3. probe từng space RUNNING (ngoài những cái đã biết) ──
log(""); log("=" * 70); log("3) probe end-to-end từng space RUNNING"); log("=" * 70)
KNOWN = {"pnnbao-ump/VieNeu-TTS-v3-Turbo", "trangmin11101996/VieNeu-TTS-v3-Turbo",
         "eagle0019/VieNeu-TTS-v3-Turbo", "xtieps/VieNeu-TTS-v3-Turbo",
         "thienan2146/VieNeu-TTS-v3-Turbo", "Smrfhdl/tts", "doremon102/VieNeu-TTS-v3-Turbo",
         "kabinz/VieNeu-TTS-v3-Turbo", "Tuananh20015/VieNeu-TTS-v3-Turbo",
         "hongqminh/VieNeu-TTS", "DevTam05/vieneu-tts", "Thomcles/yodalingua-tts-arena"}

def probe_space(sid):
    base = f"https://{sid.replace('/', '-')}.hf.space"
    st, body = http(base + "/config", timeout=18)
    if st != 200:
        return f"/config HTTP {st}"
    try:
        cfg = json.loads(body)
    except Exception as e:
        return f"config parse {e}"
    deps = cfg.get("dependencies", [])
    syn = [(i, d.get("api_name")) for i, d in enumerate(deps) if "synth" in str(d.get("api_name")) or "speech" in str(d.get("api_name"))]
    if not syn:
        return f"không có synthesize endpoint (deps={[d.get('api_name') for d in deps]})"
    # chọn endpoint: synthesize hoặc synthesize_speech hoặc đầu tiên
    epname = None
    for _, nm in syn:
        if nm == "synthesize": epname = "synthesize"; break
    if not epname:
        for _, nm in syn:
            if nm: epname = nm; break
    if not epname: return f"endpoint unnamed {[nm for _, nm in syn]}"
    # /info lấy số params + voice literal
    nparams, voices, defaults = 0, [], []
    st, info = http(base + "/gradio_api/info", timeout=18)
    if st == 200:
        try:
            ij = json.loads(info)
            spec = ij.get("named_endpoints", {}).get("/" + epname) or {}
            ps = spec.get("parameters", [])
            nparams = len(ps)
            for p in ps:
                if p.get("parameter_name") in ("voice", "voice_choice", "voice_id"):
                    lit = str(p.get("python_type", {}).get("type", ""))
                    mm = re.search(r"Literal\[(.*)\]", lit)
                    if mm: voices = re.findall(r"'([^']+)'", mm.group(1))
        except Exception:
            pass
    # dựng payload theo số params
    def pk(text, voice):
        if epname == "synthesize" and nparams >= 9:
            return ["Xin chào, thử giọng.", voice, None, 0.8, 25, 0.95, 1.2, 300, 500]
        if epname == "synthesize_speech":
            return ["Xin chào, thử giọng.", voice, None, None] if nparams == 4 else ["Xin chào, thử giọng.", voice]
        return ["Xin chào, thử giọng.", voice]
    voice = voices[0] if voices else ("Nam Minh (Nam)" if "devtam" in sid.lower() else "Ngọc Lan")
    st2, b2 = http(base + f"/gradio_api/call/{epname}", data=json.dumps({"data": pk("Xin chào, thử giọng.", voice), "session_hash": "p56d"}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
    if st2 not in (200, 201):
        return f"POST {epname} HTTP {st2}"
    try:
        ev = json.loads(b2).get("event_id")
    except Exception:
        return f"event_id parse fail {b2[:80]!r}"
    st3, b3 = http(base + f"/gradio_api/call/{epname}/{ev}", timeout=110)
    txt = b3.decode("utf-8", "replace")[:400] if isinstance(b3, bytes) else str(b3)[:400]
    if "complete" in txt:
        sz = "audio?" if ".wav" in txt or ".mp3" in txt or "url" in txt else txt[:80]
        return f"OK ✅ ep={epname} params={nparams} voices={len(voices)} → {sz[:120]}"
    if "error" in txt:
        em = re.search(r'data:\s*(.*)', txt)
        return f"error → {(em.group(1) if em else txt)[:140]}"
    return f"khác: {txt[:140]}"

for sid, sdk in running:
    tag = "(known)" if sid in KNOWN else ""
    t0 = time.time()
    try:
        res = probe_space(sid)
    except Exception as e:
        res = f"EXCEPTION {e}"
    log(f"  {sid:55s} {tag:8s} {time.time()-t0:5.1f}s  {res}")

with open("/home/z/my-project/scripts/probe56d.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE")
