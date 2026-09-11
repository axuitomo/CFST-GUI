# -*- coding: utf-8 -*-
"""CDP eval helper: connect and run a JS expression, print result."""
import json
import os
import sys
import time
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib", "cdp-lib"))
import websocket


def fetch_targets(port=9222):
    with urllib.request.urlopen(f"http://127.0.0.1:{port}/json", timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def main():
    # usage: cdp_eval.py <expr|@file.js> [port]
    arg = sys.argv[1]
    port = int(sys.argv[2]) if len(sys.argv) > 2 else 9222
    if arg.startswith("@") and arg[1:].endswith(".js"):
        with open(arg[1:], "r", encoding="utf-8") as f:
            expr = f.read()
    else:
        expr = arg
    targets = fetch_targets(port)
    target = next((t for t in targets if t.get("type") == "page"), None)
    if not target:
        print("NO_TARGET")
        return
    ws = websocket.create_connection(target["webSocketDebuggerUrl"], timeout=10, suppress_origin=True)
    req = {
        "id": 1,
        "method": "Runtime.evaluate",
        "params": {"expression": expr, "returnByValue": True, "awaitPromise": True},
    }
    ws.send(json.dumps(req))
    deadline = time.time() + 20
    while time.time() < deadline:
        try:
            raw = ws.recv()
        except Exception:
            break
        msg = json.loads(raw)
        if msg.get("id") == 1:
            res = msg.get("result", {})
            if "exceptionDetails" in res:
                print("EXCEPTION", json.dumps(res["exceptionDetails"], ensure_ascii=False))
            else:
                v = res.get("result", {}).get("value")
                print("VALUE", json.dumps(v, ensure_ascii=False) if not isinstance(v, str) else v)
            break
    ws.close()


if __name__ == "__main__":
    main()
