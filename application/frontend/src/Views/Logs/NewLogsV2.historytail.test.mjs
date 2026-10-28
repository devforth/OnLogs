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

let rows = [];
let pos = new Map();
let seq = 0;
let sockets = [];
let requests = [];
let slowReloads = false;

function store(msg) {
  seq += 1;
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + seq).toISOString().replace("Z", "000000Z");
  pos.set(msg, rows.length);
  rows.push({ ts, msg, key: `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}` });
  for (const ws of sockets) {
    if (ws.readyState === 1 && ws.url.includes(`host=${HOST}&id=${SERVICE}`)) {
      ws.onmessage?.({ data: JSON.stringify([ts, msg]) });
    }
  }
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
    if (startWith && r.key === startWith) continue;
    logs.push([r.ts, r.msg, r.key]);
    last = r.key;
    counter += 1;
  }
  return { logs, last_processed_key: last, is_end: isEnd };
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  const from = u.searchParams.get("startWith");
  requests.push(name === "getLogs" && !from ? "reload" : name);
  if (name === "getPrevLogs" && !from) return jsonResponse({ error: 'Need to specify "startWith"!' });
  if (name === "getLogs" && !from && slowReloads) await sleep(1500);
  if (name === "getPrevLogs") return jsonResponse(page(u.searchParams, true));
  if (name === "getLogs") return jsonResponse(page(u.searchParams, false));
  return jsonResponse(null);
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const observer = (id) => globalThis.__onlogsObservers.find((h) => h.element?.id === id);
const sentinels = () =>
  (globalThis.__onlogsObservers || []).filter((h) => h.element?.classList?.contains("observer"));
const reloads = () => requests.filter((r) => r === "reload").length;

function assertContiguous(list, label) {
  for (let k = 1; k < list.length; k++) {
    assert.equal(
      pos.get(list[k]),
      pos.get(list[k - 1]) + 1,
      `${label}: "${list[k - 1]}" is followed by "${list[k]}"`
    );
  }
}

let scrollTop = 0;
async function scroll(direction) {
  const el = document.querySelector("#logs");
  Object.defineProperty(el, "scrollTop", { configurable: true, get: () => scrollTop });
  el.dispatchEvent(new window.Event("scroll"));
  scrollTop += direction === "down" ? 100 : -100;
  el.dispatchEvent(new window.Event("scroll"));
  await settle(5);
}

let bundle;

async function open(stored) {
  rows = [];
  pos = new Map();
  scrollTop = 5000;
  ({ sockets } = installDom({ fetchImpl }));
  for (let i = 0; i < stored; i++) store(`row-${i}`);
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  // The scroll listener and the paging guard arm about a second after opening.
  await sleep(1400);
  await settle(10);
  return view;
}

main(async () => {
  installDom({ fetchImpl });
  bundle = await importBundle(await bundleComponent("test/entry.js"));

  // Reaching the end of a window paged up into history.
  let view = await open(600);
  assert.equal(rendered()[0], "row-420");
  await scroll("up");
  for (const handle of sentinels()) {
    handle.set(true);
    await settle(8);
    handle.set(false);
    await settle(2);
  }
  await sleep(100);
  await settle(10);
  assert.equal(rendered()[0], "row-360", "control: one older page");
  await scroll("down");
  observer("endOfLogs").set(true);
  await settle(5);
  requests = [];
  store("live-1");
  await settle(30);
  assert.equal(reloads(), 0, "the end of a history window reloaded the newest rows");
  assert.equal(rendered()[0], "row-420", "the end of a history window did not load the next page");
  assertContiguous(rendered(), "next page");
  store("live-2");
  await settle(30);
  assert.deepEqual(rendered().slice(-3), ["row-599", "live-1", "live-2"]);
  assertContiguous(rendered(), "head page");
  store("live-3");
  await settle(10);
  assert.equal(rendered().at(-1), "live-3", "the live tail did not resume at the head");
  assert.equal(reloads(), 0);
  bundle.unmount(view);

  // Holding the mouse button at the live tail.
  view = await open(200);
  observer("endOfLogs").set(true);
  await settle(5);
  requests = [];
  window.dispatchEvent(new window.MouseEvent("mousedown"));
  await settle(5);
  for (const msg of ["held-1", "held-2", "held-3"]) {
    store(msg);
    await settle(10);
  }
  assert.equal(reloads(), 0, "a held mouse button reloaded the page for every live line");
  window.dispatchEvent(new window.MouseEvent("mouseup"));
  await settle(30);
  assert.deepEqual(rendered().slice(-3), ["held-1", "held-2", "held-3"]);
  assertContiguous(rendered(), "after release");
  assert.ok(reloads() <= 1, `releasing the mouse sent ${reloads()} reloads`);
  bundle.unmount(view);

  // A press whose release never reaches the page: a right click's context
  // menu takes it, or the window loses focus.
  view = await open(200);
  observer("endOfLogs").set(true);
  await settle(5);
  window.dispatchEvent(new window.MouseEvent("mousedown", { button: 2 }));
  await settle(5);
  store("menu-1");
  await settle(30);
  assert.equal(rendered().at(-1), "menu-1", "a right click froze the live tail");
  // Dragging selected text ends with dragend or drop, never a mouseup.
  for (const release of ["contextmenu", "blur", "dragend", "drop"]) {
    window.dispatchEvent(new window.MouseEvent("mousedown"));
    await settle(5);
    window.dispatchEvent(new window.Event(release));
    await settle(5);
    store(`after-${release}`);
    await settle(30);
    assert.equal(rendered().at(-1), `after-${release}`, `the live tail stayed frozen after ${release}`);
  }
  assertContiguous(rendered(), "after a lost release");
  bundle.unmount(view);

  // A next-page request armed while the mouse is held fires after a filter
  // change has emptied the list.
  view = await open(600);
  await scroll("down");
  const failures = [];
  const onRejection = (e) => failures.push(String(e));
  process.on("unhandledRejection", onRejection);
  window.dispatchEvent(new window.MouseEvent("mousedown"));
  await settle(5);
  const lastRow = [...document.querySelectorAll("#logs .chosenString")].at(-1);
  sentinels().find((h) => h.element?.isConnected && lastRow.contains(h.element)).set(true);
  await settle(5);
  slowReloads = true;
  bundle.chosenStatus.set("warn");
  await sleep(1300);
  await settle(10);
  slowReloads = false;
  window.dispatchEvent(new window.MouseEvent("mouseup"));
  await sleep(600);
  await settle(10);
  process.off("unhandledRejection", onRejection);
  assert.deepEqual(failures, [], "paging from an emptied list threw");
  bundle.chosenStatus.set("");
  bundle.unmount(view);

  console.log("NewLogsV2 history tail tests passed");
});
