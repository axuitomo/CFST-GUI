# -*- coding: utf-8 -*-
"""Test WebView CDP ws acceptance with various origins + capture full DevTools errors."""
import json
import os
import sys
import time
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib", "cdp-lib"))
import websocket


def try_origin(label, origin):
    with urllib.request.urlopen("http://127.0.0.1:9222/json", timeout=5) as r:
        targets = json.loads(r.read().decode("utf-8"))
    t = next((x for x in targets if x.get("type") == "page"), None)
    if not t:
        print(label, "NO_TARGET")
        return
    kw = {}
    if origin is not None:
        kw["origin"] = origin
    else:
        kw["suppress_origin"] = True
    try:
        ws = websocket.create_connection(t["webSocketDebuggerUrl"], timeout=5, **kw)
        ws.send(json.dumps({"id": 1, "method": "Runtime.evaluate", "params": {"expression": "1+1", "returnByValue": True}}))
        while True:
            msg = json.loads(ws.recv())
            if msg.get("id") == 1:
                print(label, "OK", msg.get("result"))
                break
        ws.close()
    except Exception as e:
        print(label, "FAIL", type(e).__name__, str(e)[:200])


try_origin("no-origin (suppress)", None)
try_origin("devtools://devtools", "devtools://devtools")
try_origin("chrome-devtools://devtools", "chrome-devtools://devtools")
