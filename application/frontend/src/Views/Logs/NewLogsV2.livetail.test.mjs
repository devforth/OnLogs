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
const SERVICE = "api";

const rows = [];
let seq = 0;
let sockets = [];

function nextTs() {
  return new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + seq + 1).toISOString().replace("Z", "000000Z");
}

function send(ts, msg) {
  for (const ws of sockets) {
    if (ws.readyState === 1 && ws.url.includes(`host=${HOST}&id=${SERVICE}`)) {
      ws.onmessage?.({ data: JSON.stringify([ts, msg]) });
    }
  }
}

function store(msg, ts = nextTs(), live = true) {
  seq += 1;
  const key = `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}`;
  rows.push({ ts, msg, key });
  if (live) send(ts, msg);
  return ts;
}

function page(params, getPrev) {
  const limit = Number(params.get("limit")) || 30;
  const startWith = params.get("startWith") || "";
  const keys = rows.map((r) => r.key);
  let i = startWith ? keys.findIndex((k) => k >= startWith) : getPrev ? 0 : rows.length - 1;
  if (startWith && i === -1) {
    if (getPrev) return { logs: [], last_processed_key: "", is_end: true };
    i = rows.length - 1;
  }
  const logs = [];
  let last = "";
  let isEnd = false;
  for (let counter = 0; counter < limit; ) {
    if (i < 0 || i >= rows.length) {
      isEnd = true;
      counter += 1;
      continue;
    }
    const r = rows[i];
    i += getPrev ? 1 : -1;
    if (startWith && (r.key === startWith || r.key.startsWith(startWith + " +"))) continue;
    logs.push([r.ts, r.msg, r.key]);
    last = r.key;
    counter += 1;
  }
  return { logs, last_processed_key: last, is_end: isEnd };
}

const held = [];
let holdPrev = false;
let prevRequests = 0;
let olderRequests = 0;

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  if (name === "getPrevLogs") {
    prevRequests += 1;
    if (holdPrev) await new Promise((resolve) => held.push(resolve));
    return jsonResponse(page(u.searchParams, true));
  }
  if (name === "getLogs") {
    if (u.searchParams.get("startWith")) olderRequests += 1;
    return jsonResponse(page(u.searchParams, false));
  }
  if (name === "getLogWithPrev") return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  return jsonResponse(null);
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const count = (msg) => rendered().filter((m) => m === msg).length;
const observer = (id) => globalThis.__onlogsObservers.find((h) => h.element?.id === id);
const sentinels = () =>
  [...(globalThis.__onlogsObservers || [])].filter(
    (h) => !["endOfLogs", "startOfLogs"].includes(h.element?.id)
  );

let bundle;

async function openAtBottom() {
  ({ sockets } = installDom({ fetchImpl }));
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  await new Promise((resolve) => setTimeout(resolve, 1400));
  await settle(10);
  assert.equal(rendered().length, 180, "expected three pages on screen");
  observer("endOfLogs").set(true);
  const logsEl = document.querySelector("#logs");
  Object.defineProperty(logsEl, "scrollTop", { value: 500, configurable: true, writable: true });
  logsEl.dispatchEvent(new window.Event("scroll"));
  await settle(5);
  return view;
}

async function showNearBottomSentinel(done) {
  for (const h of sentinels()) {
    h.set(true);
    await settle(8);
    if (done()) return;
    h.set(false);
    await settle(2);
  }
  assert.fail("no sentinel requested newer rows");
}

async function goToTopLoadsOlderPage() {
  for (const h of sentinels()) h.set(false);
  observer("endOfLogs").set(false);
  const logsEl = document.querySelector("#logs");
  logsEl.scrollTop = 100;
  logsEl.dispatchEvent(new window.Event("scroll"));
  await settle(5);
  const before = olderRequests;
  observer("startOfLogs").set(true);
  for (let t = 0; t < 30 && olderRequests === before; t++) {
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  observer("startOfLogs").set(false);
  return olderRequests > before;
}

main(async () => {
  ({ sockets } = installDom({ fetchImpl }));
  bundle = await importBundle(await bundleComponent("test/entry.js"));
  for (let i = 0; i < 200; i++) store(`line-${i}`);
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);

  const view = await openAtBottom();
  holdPrev = true;
  await showNearBottomSentinel(() => held.length > 0);
  store("live-line");
  await settle(5);
  holdPrev = false;
  held.shift()();
  await settle(20);
  assert.equal(count("live-line"), 1, "a line delivered live and in a page rendered twice");
  assert.equal(rendered().length, 180);
  assert.ok(await goToTopLoadsOlderPage(), "no older page after the race");
  bundle.unmount(view);

  const view2 = await openAtBottom();
  const before = prevRequests;
  await showNearBottomSentinel(() => prevRequests > before);
  holdPrev = true;
  store("burst-a");
  await settle(5);
  assert.equal(held.length, 1, "appending a line did not request newer rows");
  store("burst-b");
  await settle(5);
  store("burst-c");
  await settle(5);
  holdPrev = false;
  held.shift()();
  await settle(20);
  assert.deepEqual(rendered().slice(-3), ["burst-a", "burst-b", "burst-c"]);
  assert.equal(count("burst-b"), 1);
  assert.equal(count("burst-c"), 1);

  store("repeat");
  await settle(5);
  store("repeat");
  await settle(5);
  const sameTs = nextTs();
  store("same-ts-a", sameTs);
  await settle(5);
  store("same-ts-b", sameTs);
  await settle(5);
  assert.deepEqual(rendered().slice(-4), ["repeat", "repeat", "same-ts-a", "same-ts-b"]);
  bundle.unmount(view2);

  const dupTs = nextTs();
  store("stored-twice", dupTs, false);
  store("stored-twice", dupTs, false);
  const view3 = await openAtBottom();
  assert.equal(count("stored-twice"), 2);
  const lateTs = store("late", nextTs(), false);
  await showNearBottomSentinel(() => count("late") > 0);
  send(lateTs, "late");
  await settle(10);
  assert.equal(count("late"), 1, "a frame for a line already paged in rendered twice");
  assert.equal(rendered().length, 180);
  assert.ok(await goToTopLoadsOlderPage(), "no older page with a stored duplicate in view");
  bundle.unmount(view3);
});
