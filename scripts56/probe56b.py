#!/usr/bin/env python3
"""PROBE56b — Đào sâu: (1) vieneu.io API mới; (2) eagle0019/Tuananh20015 vì sao lỗi;
(3) hongqminh đúng endpoint; (4) Smrfhdl access_status."""
import json, time, re, urllib.request, urllib.error

UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/128 Safari/537.36"}
R = []

def http(url, data=None, headers=None, timeout=30, method=None, raw=False):
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

log("=" * 70); log("A) vieneu.io — tìm API synthesis MỚI"); log("=" * 70)
st, body = http("https://vieneu.io/", timeout=25)
log(f"  GET /: HTTP {st}, len={len(body)}")
js_bundles = re.findall(rb'src="([^"]+\.js)"', body) if st == 200 else []
log(f"  JS bundles: {[b.decode() for b in js_bundles][:5]}")
for b in js_bundles[:3]:
    u = b.decode()
    if u.startswith("//"): u = "https:" + u
    elif u.startswith("/"): u = "https://vieneu.io" + u
    st2, js = http(u, timeout=30)
    log(f"    {u}: HTTP {st2}, len={len(js)}")
    if st2 == 200:
        # tìm mọi URL api/tts
        apis = sorted(set(re.findall(rb'["\'](/api/[a-zA-Z0-9/_\-\.]+)["\']', js)))
        log(f"    API paths trong bundle: {[a.decode() for a in apis]}")
        tts_calls = sorted(set(re.findall(rb'["\']([^"\']*tts[^"\']*)["\']', js)))[:20]
        log(f"    tts-refs: {[t.decode()[:80] for t in tts_calls]}")
        with open(f"/home/z/my-project/scripts/vieneu_bundle_{int(time.time())}.js", "wb") as f:
            f.write(js)

log(""); log("=" * 70); log("B) eagle0019 + Tuananh20015 — vì sao CPU space lỗi null?"); log("=" * 70)
for sid in ["eagle0019/VieNeu-TTS-v3-Turbo", "Tuananh20015/VieNeu-TTS-v3-Turbo"]:
    base = f"https://{sid.replace('/', '-')}.hf.space"
    st, body = http(base + "/config", timeout=20)
    if st != 200: log(f"  [{sid}] /config HTTP {st}"); continue
    cfg = json.loads(body)
    ver = cfg.get("version")
    deps = cfg.get("dependencies", [])
    log(f"  [{sid}] gradio version={ver}")
    for i, d in enumerate(deps):
        if d.get("api_name") in ("synthesize", "stream_synthesize"):
            log(f"    dep[{i}] api_name={d.get('api_name')} triggers={d.get('trigger')} inputs={d.get('inputs')} outputs={d.get('outputs')} backend_fn={d.get('backend_fn')} js={str(d.get('js'))[:60]}")
    # /info để xem signature tham số thật
    st2, info = http(base + "/gradio_api/info", timeout=20)
    if st2 == 200:
        try:
            ij = json.loads(info)
            named = ij.get("named_endpoints", {})
            for ep, spec in named.items():
                if "synthesize" in ep and "conversation" not in ep and "stream" not in ep:
                    ps = spec.get("parameters", [])
                    log(f"    {ep}: {len(ps)} params")
                    for p in ps:
                        log(f"      - {p.get('parameter_name')}: {p.get('python_type', {}).get('type')} default={p.get('parameter_default')!r} desc={str(p.get('description'))[:50]}")
        except Exception as e:
            log(f"    /info parse lỗi: {e}")
    # thử gọi KHÔNG tham số optional (chỉ text + voice)
    st3, body3 = http(base + "/gradio_api/call/synthesize", data=json.dumps({"data": ["Xin chào.", "Ngọc Linh"]}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
    log(f"    POST 2-param: HTTP {st3} — {body3[:150]!r}")
    if st3 in (200, 201):
        try:
            ev = json.loads(body3).get("event_id")
            st4, body4 = http(base + f"/gradio_api/call/synthesize/{ev}", timeout=90)
            txt = body4.decode("utf-8", "replace")
            log(f"    SSE: HTTP {st4} — {txt[:250]!r}")
        except Exception as e:
            log(f"    SSE lỗi: {e}")

log(""); log("=" * 70); log("C) hongqminh — endpoint đúng synthesize_speech"); log("=" * 70)
base = "https://hongqminh-VieNeu-TTS.hf.space"
st, info = http(base + "/gradio_api/info", timeout=20)
if st == 200:
    try:
        ij = json.loads(info)
        for ep, spec in ij.get("named_endpoints", {}).items():
            ps = spec.get("parameters", [])
            log(f"  {ep}: {len(ps)} params: {[p.get('parameter_name') for p in ps]}")
            for p in ps:
                log(f"    - {p.get('parameter_name')}: type={p.get('python_type', {}).get('type')} default={p.get('parameter_default')!r}")
    except Exception as e:
        log(f"  /info parse lỗi: {e}")
# gọi đúng endpoint
st, body = http(base + "/gradio_api/call/synthesize_speech", data=json.dumps({"data": ["Xin chào kiểm tra.", "Ngọc (nữ miền Bắc)", None, None, None]}).encode(), headers={"Content-Type": "application/json"}, timeout=25)
log(f"  POST synthesize_speech (5p): HTTP {st} — {body[:150]!r}")
if st in (200, 201):
    try:
        ev = json.loads(body).get("event_id")
        st2, body2 = http(base + f"/gradio_api/call/synthesize_speech/{ev}", timeout=120)
        log(f"  SSE: HTTP {st2} — {body2.decode('utf-8','replace')[:250]!r}")
    except Exception as e:
        log(f"  SSE lỗi: {e}")

log(""); log("=" * 70); log("D) Smrfhdl — access_status nói gì?"); log("=" * 70)
base = "https://smrfhdl-tts.hf.space"
st, body = http(base + "/gradio_api/call/access_status", data=b'{"data": []}', headers={"Content-Type": "application/json"}, timeout=20)
log(f"  POST access_status: HTTP {st} — {body[:150]!r}")
if st in (200, 201):
    try:
        ev = json.loads(body).get("event_id")
        st2, body2 = http(base + f"/gradio_api/call/access_status/{ev}", timeout=30)
        log(f"  SSE: HTTP {st2} — {body2.decode('utf-8','replace')[:300]!r}")
    except Exception as e:
        log(f"  SSE lỗi: {e}")
# synthesize endpoint spec
st, info = http(base + "/gradio_api/info", timeout=20)
if st == 200:
    try:
        ij = json.loads(info)
        for ep, spec in ij.get("named_endpoints", {}).items():
            if ep.endswith("synthesize"):
                log(f"  {ep}: {len(spec.get('parameters', []))} params")
                for p in spec.get("parameters", []):
                    log(f"    - {p.get('parameter_name')}: default={p.get('parameter_default')!r}")
    except Exception:
        pass

with open("/home/z/my-project/scripts/probe56b.log", "w") as f:
    f.write("\n".join(R))
print("\nDONE")
