#!/usr/bin/env python3
"""PROBE56j — retest CPU spaces lúc này + whatvn predict signature."""
import json, re, time, urllib.request, urllib.error

UA = {"User-Agent": "Mozilla/5.0 Chrome/128"}
R = []

def http(url, data=None, timeout=60):
    req = urllib.request.Request(url, data=data, headers=UA)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, (e.read() or b"")[:2000]
    except Exception as e:
        return -1, str(e).encode()[:200]

def log(*a):
    s = " ".join(str(x) for x in a); print(s, flush=True); R.append(s)

def gradio_call(base, ep, data, wait=110):
    st, b = http(base + f"/gradio_api/call/{ep}", data=json.dumps({"data": data, "session_hash": "p56j"}).encode(), timeout=25)
    if st not in (200, 201): return f"POST HTTP {st}"
    try: ev = json.loads(b).get("event_id")
    except Exception: return f"event_id fail"
    st2, b2 = http(base + f"/gradio_api/call/{ep}/{ev}", timeout=wait)
    txt = b2.decode("utf-8", "replace")
    if "complete" in txt:
        mu = re.search(r'"url":\s*"([^"]+)"', txt)
        return f"OK ✅ {(mu.group(1) if mu else '')[:60]}"
    if "error" in txt:
        em = re.search(r'data:\s*(.*)', txt)
        return f"error → {(em.group(1) if em else '?')[:90]}"
    return f"khác {txt[:90]!r}"

log("A) retest 3 space CPU (session_hash, đầy đủ 9 tham số):")
for sid, voice in [("eagle0019/VieNeu-TTS-v3-Turbo", "Ngọc Linh"),
                   ("Tuananh20015/VieNeu-TTS-v3-Turbo", "Ngọc Lan"),
                   ("hongqminh/VieNeu-TTS", "Ngọc (nữ miền Bắc)")]:
    base = f"https://{sid.replace('/', '-')}.hf.space"
    t0 = time.time()
    if "hongqminh" in sid:
        res = gradio_call(base, "synthesize_speech", ["Xin chào thử lại.", voice, None, None, None])
    else:
        res = gradio_call(base, "synthesize", ["Xin chào thử lại.", voice, None, 0.8, 25, 0.95, 1.2, 300, 500])
    log(f"  {sid:42s} {time.time()-t0:5.1f}s {res}")

log(""); log("B) whatvn/vietnamese-tts — /info predict:")
st, b = http("https://whatvn-vietnamese-tts.hf.space/gradio_api/info", timeout=20)
if st == 200:
    ij = json.loads(b)
    for ep, spec in ij.get("named_endpoints", {}).items():
        ps = spec.get("parameters", [])
        log(f"  {ep}: {[p.get('parameter_name') for p in ps]}")
        for p in ps:
            t = str(p.get("python_type", {}).get("type", ""))
            if "Literal" in t:
                mm = re.search(r"Literal\[(.{0,220})", t)
                log(f"    {p.get('parameter_name')}: {mm.group(1) if mm else t[:200]}")
            else:
                log(f"    {p.get('parameter_name')}: {t[:60]} default={p.get('parameter_default')!r}")
    # thử predict với default
    spec = ij.get("named_endpoints", {}).get("/predict", {})
    ps = spec.get("parameters", [])
    data = []
    for p in ps:
        d = p.get("parameter_default")
        t = str(p.get("python_type", {}).get("type", ""))
        if d is not None and d != "":
            data.append(d)
        elif "str" in t:
            data.append("Xin chào thử nghiệm tiếng Việt.")
        elif "bool" in t:
            data.append(False)
        elif "float" in t or "int" in t:
            data.append(0)
        else:
            data.append(None)
    log(f"  thử predict data={data}")
    res = gradio_call("https://whatvn-vietnamese-tts.hf.space", "predict", data, wait=90)
    log(f"  predict → {res}")
else:
    log(f"  /info HTTP {st}")

with open("/home/z/my-project/scripts/probe56j.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE")
