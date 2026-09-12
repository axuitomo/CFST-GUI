# -*- coding: utf-8 -*-
"""Check Edge DevTools page console/log events."""
import json
import os
import sys
import time
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib", "cdp-lib"))
import websocket


def main():
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9333
    with urllib.request.urlopen(f"http://127.0.0.1:{port}/json", timeout=5) as r:
        targets = json.loads(r.read().decode("utf-8"))
    t = next((x for x in targets if "devtools" in (x.get("url") or "").lower()), None)
    print("DEVTOOLS_TARGET", bool(t))
    if not t:
        return
    ws = websocket.create_connection(t["webSocketDebuggerUrl"], timeout=10, suppress_origin=True)
    mid = 0
    for dom in ["Runtime", "Console", "Log"]:
        mid += 1
        ws.send(json.dumps({"id": mid, "method": dom + ".enable"}))
        time.sleep(0.3)
    time.sleep(2)
    events = []
    while True:
        try:
            raw = ws.recv()
        except Exception:
            break
        try:
            msg = json.loads(raw)
        except Exception:
            continue
        if msg.get("method") in ("Runtime.consoleAPICalled", "Log.entryAdded", "Runtime.exceptionThrown"):
            events.append(msg)
    print("EVENTS", len(events))
    for e in events[:25]:
        m = e.get("method")
        p = e.get("params", {})
        if m == "Runtime.consoleAPICalled":
            args = p.get("args", [])
            text = " ".join(str(a.get("value") or a.get("description") or "") for a in args[:5])
        elif m == "Log.entryAdded":
            text = p.get("entry", {}).get("text", "")
        else:
            text = json.dumps(p, ensure_ascii=False)[:300]
        print(f"[{m}] {text[:400]}")
    ws.close()


if __name__ == "__main__":
    main()
