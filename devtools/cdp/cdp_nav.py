# -*- coding: utf-8 -*-
"""Navigate an Edge/Chrome target to a URL via CDP, then list targets."""
import json
import os
import sys
import time
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib", "cdp-lib"))
import websocket


def get(url, timeout=5):
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return json.loads(r.read().decode("utf-8"))


def main():
    port = sys.argv[1] if len(sys.argv) > 1 else "9333"
    url = sys.argv[2] if len(sys.argv) > 2 else ""
    targets = get(f"http://127.0.0.1:{port}/json")
    pages = [t for t in targets if t.get("type") == "page"]
    if not pages:
        print("NO_PAGE_TARGET")
        return
    target = pages[0]
    ws = websocket.create_connection(target["webSocketDebuggerUrl"], timeout=10, suppress_origin=True)
    req = {"id": 1, "method": "Page.navigate", "params": {"url": url}}
    ws.send(json.dumps(req))
    deadline = time.time() + 10
    while time.time() < deadline:
        try:
            raw = ws.recv()
        except Exception:
            break
        msg = json.loads(raw)
        if msg.get("id") == 1:
            print("NAVIGATE_RESULT", json.dumps(msg.get("result", {}), ensure_ascii=False))
            break
    ws.close()
    time.sleep(3)
    try:
        after = get(f"http://127.0.0.1:{port}/json")
    except Exception as e:
        print("REFRESH_ERROR", repr(e))
        return
    for t in after:
        if t.get("type") == "page" or "devtools" in (t.get("url") or "").lower():
            print("TARGET", t.get("type"), "|", t.get("title"), "|", t.get("url"))
            if "devtools" in (t.get("url") or "").lower() or "devtools" in (t.get("title") or "").lower():
                print("  WS", t.get("webSocketDebuggerUrl"))


if __name__ == "__main__":
    main()
