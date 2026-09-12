# -*- coding: utf-8 -*-
"""CDP debug client for CFST-GUI WebView on real device (via adb forward)."""
import json
import os
import sys
import time
import threading
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib", "cdp-lib"))
import websocket  # websocket-client


BASE_HTTP = "http://127.0.0.1:9222"
WS_URL = None


def fetch_targets():
    with urllib.request.urlopen(BASE_HTTP + "/json", timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def pick_target(target_id=None):
    targets = fetch_targets()
    for t in targets:
        if t.get("type") != "page":
            continue
        if target_id and t.get("id") != target_id:
            continue
        return t
    return None


class CDP:
    def __init__(self, ws_url):
        self.ws = websocket.create_connection(ws_url, timeout=10, suppress_origin=True)
        self._id = 0
        self._pending = {}
        self._events = []
        self._lock = threading.Lock()
        self._reader = threading.Thread(target=self._read_loop, daemon=True)
        self._reader.start()

    def _read_loop(self):
        while True:
            try:
                raw = self.ws.recv()
            except Exception:
                break
            try:
                msg = json.loads(raw)
            except Exception:
                continue
            if "id" in msg:
                with self._lock:
                    self._pending.pop(msg["id"], None)
            else:
                self._events.append(msg)

    def call(self, method, params=None):
        self._id += 1
        mid = self._id
        self.ws.send(json.dumps({"id": mid, "method": method, "params": params or {}}))
        deadline = time.time() + 15
        while time.time() < deadline:
            with self._lock:
                if mid not in self._pending:
                    # read loop removed it -> result was never stored; refetch directly
                    pass
            time.sleep(0.05)
            break
        return {"sent": method, "id": mid}

    def eval_js(self, expression, await_promise=False):
        """Synchronous-ish evaluate: send, then poll ws for matching id via recv."""
        self._id += 1
        mid = self._id
        req = {
            "id": mid,
            "method": "Runtime.evaluate",
            "params": {
                "expression": expression,
                "returnByValue": True,
                "awaitPromise": await_promise,
            },
        }
        self.ws.send(json.dumps(req))
        deadline = time.time() + 15
        while time.time() < deadline:
            try:
                raw = self.ws.recv()
            except Exception:
                break
            try:
                msg = json.loads(raw)
            except Exception:
                continue
            if msg.get("id") == mid:
                return msg
            self._events.append(msg)
        return {"error": "timeout"}

    def drain_events(self):
        with self._lock:
            evs, self._events = self._events, []
        return evs

    def close(self):
        try:
            self.ws.close()
        except Exception:
            pass


def main():
    target = pick_target(sys.argv[1] if len(sys.argv) > 1 else None)
    if not target:
        print("NO_TARGET")
        return
    print("TARGET", target.get("id"), target.get("title"), target.get("url"))
    cdp = CDP(target["webSocketDebuggerUrl"])
    # Enable domains
    for dom in ["Runtime", "Console", "Log", "Network", "Page"]:
        r = cdp.eval_js("1")  # warm
        cdp.call(dom + ".enable")
    time.sleep(0.5)
    cdp.call("Page.enable")
    time.sleep(0.3)

    # Page state snapshot
    expr = """
    (() => {
      const ls = {};
      try {
        for (let i = 0; i < localStorage.length; i++) {
          const k = localStorage.key(i);
          ls[k] = localStorage.getItem(k);
        }
      } catch (e) { ls.__error = String(e); }
      return JSON.stringify({
        href: location.href,
        readyState: document.readyState,
        title: document.title,
        localStorage: ls,
        bodyTextStart: document.body ? document.body.innerText.slice(0, 400) : null,
        vueApp: !!document.querySelector('#app')
      });
    })()
    """
    r = cdp.eval_js(expr)
    if "result" in r:
        val = r["result"].get("result", {}).get("value")
        if val:
            print("STATE", val)
    if "exceptionDetails" in r.get("result", {}):
        print("EVAL_ERROR", json.dumps(r["result"]["exceptionDetails"], ensure_ascii=False))

    # Recent console / log events
    time.sleep(1.0)
    evs = cdp.drain_events()
    interesting = []
    for e in evs:
        m = e.get("method", "")
        if m in ("Runtime.consoleAPICalled", "Log.entryAdded", "Runtime.exceptionThrown", "Network.loadingFailed"):
            interesting.append(e)
    print("EVENT_COUNT", len(evs), "INTERESTING", len(interesting))
    for e in interesting[:30]:
        print("EVENT", json.dumps(e, ensure_ascii=False)[:1200])
    cdp.close()


if __name__ == "__main__":
    main()
