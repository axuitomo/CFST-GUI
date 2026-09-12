(async () => {
  try {
    const p = window.Capacitor.Plugins.Cfst;
    if (!p) return { error: "no plugin" };
    const r = await p.Invoke({ command: "config.load", payload_json: "{}" });
    const obj = typeof r === "string" ? JSON.parse(r) : r;
    const data = (obj && obj.data) || {};
    const snap = data.config_snapshot || data.config || data;
    return {
      code: obj && obj.code,
      ok: obj && obj.ok,
      message: obj && obj.message,
      probe_debug: snap.probe,
      config_top_keys: snap ? Object.keys(snap) : []
    };
  } catch (e) { return { error: String(e) }; }
})()
