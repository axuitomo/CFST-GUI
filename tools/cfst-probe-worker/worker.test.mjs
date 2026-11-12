import assert from "node:assert/strict";
import test from "node:test";

import worker, { probeData, renderTrace } from "./worker.js";

// 等价于 CFST-GUI internal/task/colo.go 的 (?im)^colo=([a-z0-9]{3})\s*$
const coloPattern = /^colo=([a-z0-9]{3})\s*$/im;

function fakeRequest(path, { method = "GET", ip = "203.0.113.7", cf = {} } = {}) {
  const request = new Request("https://probe.example.com" + path, {
    method,
    headers: ip ? { "CF-Connecting-IP": ip } : {},
  });
  Object.defineProperty(request, "cf", { value: cf });
  return request;
}

const sampleCf = {
  colo: "LAX",
  country: "CN",
  region: "Hebei",
  regionCode: "HE",
  city: "Shijiazhuang",
  continent: "AS",
  asn: 4837,
  asOrganization: "China Unicom Hebei province network",
  latitude: "38.04139",
  longitude: "114.47861",
  httpProtocol: "HTTP/3",
  tlsVersion: "TLSv1.3",
};

test("trace endpoint is consumable by CFST-GUI colo extraction", async () => {
  const response = await worker.fetch(fakeRequest("/trace", { cf: sampleCf }));
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type"), /^text\/plain/);
  assert.equal(response.headers.get("cache-control"), "no-store");

  const body = await response.text();
  assert.equal(coloPattern.exec(body)?.[1], "LAX");
  assert.match(body, /^ip=203\.0\.113\.7$/m);
  assert.match(body, /^asn=4837$/m);
  assert.match(body, /^probe=cfst$/m);
});

test("json endpoint carries the probe marker and egress identity", async () => {
  const response = await worker.fetch(fakeRequest("/", { cf: sampleCf }));
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type"), /^application\/json/);

  const data = await response.json();
  assert.equal(data.probe, "cfst");
  assert.equal(data.ip, "203.0.113.7");
  assert.equal(data.ipType, "ipv4");
  assert.equal(data.colo, "LAX");
  assert.equal(data.asn, 4837);
  assert.equal(data.regionCode, "HE");
  assert.equal(data.loc, "38.0414,114.4786");
});

test("ipv6 egress is reported as ipv6", async () => {
  const request = fakeRequest("/", { ip: "2001:db8::1", cf: sampleCf });
  assert.equal(probeData(request).ipType, "ipv6");
});

test("missing cf metadata degrades without breaking the trace body", async () => {
  const data = probeData(fakeRequest("/trace", { ip: "", cf: { latitude: null, longitude: null } }));
  const body = renderTrace(data);
  assert.equal(coloPattern.exec(body), null);
  assert.equal(data.loc, "");
  assert.match(body, /^ip=$/m);
  assert.match(body, /^ts=/m);
});

test("head requests carry headers but no body", async () => {
  const response = await worker.fetch(fakeRequest("/trace", { method: "HEAD", cf: sampleCf }));
  assert.equal(response.status, 200);
  assert.equal(await response.text(), "");
  assert.equal(response.headers.get("cache-control"), "no-store");
});

test("unknown paths and write methods are rejected", async () => {
  assert.equal((await worker.fetch(fakeRequest("/nope"))).status, 404);
  assert.equal((await worker.fetch(fakeRequest("/", { method: "POST" }))).status, 405);
  assert.equal((await worker.fetch(fakeRequest("/", { method: "OPTIONS" }))).status, 204);
});
