(async () => {
  const p = window.Capacitor.Plugins.Cfst;
  if (!p) return { error: "no plugin" };
  const r = await p.Invoke({ command: "config.load", payload_json: "{}" });
  const obj = typeof r === "string" ? JSON.parse(r) : r;
  const snap = (obj && obj.data && (obj.data.config_snapshot || obj.data.config || obj.data)) || null;
  const pr = snap && snap.probe;
  return {
    debug: pr ? pr.debug : null,
    capture_enabled: pr ? pr.capture_enabled : null,
    capture_address: pr ? pr.capture_address : null,
    log_mode: pr ? pr.log_mode : null,
    log_verbosity: pr ? pr.log_verbosity : null,
    log_format: pr ? pr.log_format : null
  };
})()
