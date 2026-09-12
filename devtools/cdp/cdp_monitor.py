# -*- coding: utf-8 -*-
"""Real-time CDP monitor for CFST-GUI WebView on device.
Prints console/log/network events filtered by keywords; runs until Ctrl+C.
"""
import json
import os
import sys
import time
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib", "cdp-lib"))
import websocket

KEYWORDS = sys.argv[1:] or ["debug", "probe", "invoke", "error", "warn"]


def fetch_targets():
    with urllib.request.urlopen("http://127.0.0.1:9222/json", timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def main():
    targets = fetch_targets()
    target = next((t for t in targets if t.get("type") == "page"), None)
    if not target:
        print("NO_TARGET")
        return
    print("ATTACH", target["id"], target["url"], flush=True)
    ws = websocket.create_connection(target["webSocketDebuggerUrl"], timeout=30, suppress_origin=True)
    mid = 0

    def send(method, params=None):
        nonlocal mid
        mid += 1
        ws.send(json.dumps({"id": mid, "method": method, "params": params or {}}))

    for dom in ["Runtime", "Console", "Log", "Network"]:
        send(dom + ".enable")
    send("Page.enable")
    print("ENABLED domains; waiting for events (Ctrl+C to stop)...", flush=True)

    while True:
        try:
            raw = ws.recv()
        except Exception as exc:
            print("WS_END", repr(exc), flush=True)
            break
        try:
            msg = json.loads(raw)
        except Exception:
            continue
        if "id" in msg or "method" not in msg:
            continue
        method = msg.get("method", "")
        params = msg.get("params", {})
        text = json.dumps(msg, ensure_ascii=False)
        lowered = text.lower()
        if not any(k.lower() in lowered for k in KEYWORDS):
            continue
        # Compact render for common event types
        if method == "Runtime.consoleAPICalled":
            args = params.get("args", [])
            vals = " | ".join(
                a.get("value") if "value" in a else (a.get("description") or a.get("type", ""))
                for a in args[:6]
            )
            print(f"[console:{params.get('type')}] {vals}", flush=True)
        elif method == "Log.entryAdded":
            e = params.get("entry", {})
            print(f"[log:{e.get('level')}] {e.get('text')} {e.get('url') or ''}", flush=True)
        elif method == "Runtime.exceptionThrown":
            d = params.get("exceptionDetails", {})
            print(f"[exception] {d.get('text')} {json.dumps(d.get('exception', {}), ensure_ascii=False)[:200]}", flush=True)
        elif method.startswith("Network."):
            print(f"[net:{method}] {json.dumps(params, ensure_ascii=False)[:300]}", flush=True)
        else:
            print(f"[{method}] {json.dumps(params, ensure_ascii=False)[:300]}", flush=True)


if __name__ == "__main__":
    main()
