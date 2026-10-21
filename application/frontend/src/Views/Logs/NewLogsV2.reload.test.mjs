import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const HOST = "prod-1";

const rows = { api: [], web: [] };
let seq = 0;
let sockets = [];

function send(service, ts, msg) {
  for (const ws of sockets) {
    if (ws.readyState === 1 && ws.url.includes(`host=${HOST}&id=${service}`)) {
      ws.onmessage?.({ data: JSON.stringify([ts, msg]) });
    }
  }
}

function store(service, msg, { ts, live = true } = {}) {
  seq += 1;
  ts ??= `2026-10-19T10:00:00.${String(seq).padStart(3, "0")}000000Z`;
  const key = `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}`;
  rows[service].push({ ts, msg, key });
  rows[service].sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0));
  if (live) send(service, ts, msg);
  return ts;
}

function readLogs(service, getPrev, include, params) {
  const list = rows[service] || [];
  const asked = Number(params.get("limit")) || 0;
  const limit = asked <= 0 ? 30 : Math.min(asked, 1000);
  const startWith = params.get("startWith") || "";
  let i = startWith ? list.findIndex((r) => r.key >= startWith) : getPrev ? 0 : list.length - 1;
  if (i === -1 && !getPrev) i = list.length - 1;
  if (i < 0 || i >= list.length) return { logs: [], is_end: true };
  const logs = [];
  let last = "";
  let isEnd = false;
  for (let counter = 0; counter < limit; ) {
    if (i < 0 || i >= list.length) {
      isEnd = true;
      counter += 1;
      continue;
    }
    isEnd = false;
    const r = list[i];
    i += getPrev ? 1 : -1;
    if (!include && (r.key === startWith || r.key.startsWith(startWith + " +"))) continue;
    logs.push([r.ts, r.msg, r.key]);
    last = r.key;
    counter += 1;
  }
  return { logs, last_processed_key: last, is_end: isEnd };
}

const held = [];
let holdGetLogs = false;

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  const params = u.searchParams;
  const service = params.get("id");
  if (name === "getLogs") {
    const payload = readLogs(service, false, false, params);
    if (holdGetLogs) await new Promise((resolve) => held.push(resolve));
    return jsonResponse(payload);
  }
  if (name === "getPrevLogs") return jsonResponse(readLogs(service, true, false, params));
  if (name === "getLogWithPrev") return jsonResponse(readLogs(service, false, true, params));
  return jsonResponse(null);
}

async function release(n = Infinity) {
  for (let i = 0; i < n && held.length; i += 1) {
    held.shift()();
    await settle(5);
  }
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const count = (msg) => rendered().filter((m) => m === msg).length;

async function setAtBottom(value) {
  const handle = (globalThis.__onlogsObservers || []).find((h) => h.element?.id === "endOfLogs");
  assert.ok(handle, "bottom sentinel not rendered");
  handle.set(value);
  await settle(5);
}

function assertInOrder(list, expected, label) {
  const at = expected.map((m) => list.indexOf(m));
  assert.ok(
    at.every((v, i) => v >= 0 && (i === 0 || v > at[i - 1])),
    `${label}: expected ${JSON.stringify(expected)} in order, got ${JSON.stringify(list)}`
  );
  for (const m of expected) assert.equal(list.filter((x) => x === m).length, 1, `${label}: ${m} rendered twice`);
}

main(async () => {
  installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  ["w-1", "w-2"].forEach((m) => store("web", m, { live: false }));

  ["a-1", "a-2", "a-3"].forEach((m) => store("api", m));
  ({ sockets } = installDom({ fetchImpl }));
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set("api");
  holdGetLogs = true;
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  assert.ok(held.length >= 1, "the page request was not issued");
  store("api", "a-4");
  holdGetLogs = false;
  await release();
  await settle(20);
  assertInOrder(rendered(), ["a-1", "a-2", "a-3", "a-4"], "open, before the sentinel reports");
  await setAtBottom(true);
  store("api", "a-5");
  await settle(10);
  assertInOrder(rendered(), ["a-1", "a-2", "a-3", "a-4", "a-5"], "open");

  await setAtBottom(false);
  store("api", "b-1");
  await settle(5);
  holdGetLogs = true;
  await setAtBottom(true);
  await settle(10);
  assert.ok(held.length >= 1, "returning to the bottom did not reload");
  store("api", "b-2");
  holdGetLogs = false;
  await release();
  await settle(20);
  store("api", "b-3");
  await settle(10);
  assertInOrder(rendered(), ["a-5", "b-1", "b-2", "b-3"], "return to bottom");

  bundle.lastChosenService.set("web");
  await settle(40);
  await setAtBottom(true);
  holdGetLogs = true;
  bundle.lastChosenService.set("api");
  await settle(40);
  assert.ok(held.length >= 2, "switching services did not start overlapping loads");
  holdGetLogs = false;
  await release(held.length - 1);
  await settle(10);
  store("api", "c-1");
  await settle(5);
  await release();
  await settle(20);
  assertInOrder(rendered(), ["b-3", "c-1"], "service switch");
  store("api", "c-2");
  await settle(10);
  assertInOrder(rendered(), ["b-3", "c-1", "c-2"], "service switch");

  await setAtBottom(false);
  store("api", "d-0");
  await settle(5);
  holdGetLogs = true;
  const inPageTs = store("api", "d-1", { live: false });
  await setAtBottom(true);
  await settle(10);
  assert.ok(held.length >= 1, "returning to the bottom did not reload");
  send("api", inPageTs, "d-1");
  store("api", "late", { ts: "2026-10-19T09:59:59.000000000Z" });
  holdGetLogs = false;
  await release();
  await settle(20);
  assert.equal(count("d-1"), 1, "a line both in the page and live rendered twice");
  assert.equal(count("late"), 1, "a line stored during the reload with an older timestamp is missing");

  await setAtBottom(false);
  store("api", "e-0");
  await settle(5);
  holdGetLogs = true;
  await setAtBottom(true);
  await settle(10);
  assert.equal(held.length, 1, "returning to the bottom did not reload");
  store("api", "e-api");
  const linkTs = rows.web.at(-1).ts;
  bundle.urlHash.set("#" + linkTs);
  bundle.lastChosenService.set("web");
  await settle(60);
  assert.deepEqual(rendered(), ["w-1", "w-2"], "the shared link window was not shown");
  await setAtBottom(true);
  store("web", "e-1");
  await settle(5);
  holdGetLogs = false;
  await release();
  await settle(20);
  store("web", "e-2");
  await settle(10);
  assert.ok(!rendered().includes("e-api"), "a line for the previous service was carried over");
  assertInOrder(rendered(), ["w-1", "w-2", "e-1", "e-2"], "shared link");

  bundle.unmount(view);
});
