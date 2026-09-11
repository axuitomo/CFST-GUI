# -*- coding: utf-8 -*-
"""CDP origin-stripping tunnel proxy.

Chrome/Edge DevTools frontend -> ws://127.0.0.1:9335/... -> WebView ws://127.0.0.1:9222/...
The forward leg sends NO Origin header (WebView only accepts empty origin),
and HTTP endpoints (/json, /json/version) are relayed as-is.
"""
import asyncio
import http
import json
import os
import sys
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib", "cdp-lib"))
import websockets

LISTEN_HOST = "127.0.0.1"
LISTEN_PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 9335
TARGET_PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 9222
TARGET_BASE = f"http://127.0.0.1:{TARGET_PORT}"
TARGET_WS_BASE = f"ws://127.0.0.1:{TARGET_PORT}"


def fetch_http(path):
    with urllib.request.urlopen(TARGET_BASE + path, timeout=5) as r:
        return r.status, dict(r.getheaders()), r.read()


async def process_request(connection, request):
    """Relay plain HTTP requests (e.g. /json, /json/version) to the real endpoint."""
    path = request.path
    if path.startswith("/"):
        try:
            status, headers, body = await asyncio.to_thread(fetch_http, path)
        except Exception as exc:
            return connection.respond(
                http.HTTPStatus.BAD_GATEWAY,
                json.dumps({"error": str(exc)}),
                [("Content-Type", "application/json")],
            )
        extra = [(k, v) for k, v in headers.items() if k.lower() in ("content-type", "cache-control")]
        return connection.respond(http.HTTPStatus(status), body.decode("utf-8", "replace"), extra)
    return None


async def ws_handler(connection):
    """Bidirectional frame relay between the browser and the WebView CDP endpoint."""
    path = connection.request.path
    target_uri = TARGET_WS_BASE + path
    print(f"[proxy] ws {path} -> {target_uri}", flush=True)
    try:
        async with websockets.connect(target_uri, origin=None, close_timeout=2) as real:
            async def fwd_browser_to_real():
                async for message in connection:
                    await real.send(message)

            async def fwd_real_to_browser():
                async for message in real:
                    await connection.send(message)

            await asyncio.gather(fwd_browser_to_real(), fwd_real_to_browser())
    except Exception as exc:
        print(f"[proxy] ws error {path}: {type(exc).__name__} {str(exc)[:120]}", flush=True)


async def main():
    async with websockets.serve(
        ws_handler,
        LISTEN_HOST,
        LISTEN_PORT,
        process_request=process_request,
        max_size=2**26,
    ):
        print(f"[proxy] listening on {LISTEN_HOST}:{LISTEN_PORT} -> {TARGET_BASE}", flush=True)
        await asyncio.Future()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
