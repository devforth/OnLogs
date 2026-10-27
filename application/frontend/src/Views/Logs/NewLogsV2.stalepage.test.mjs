import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const SERVICE = "api";
const BUFFERED = "buffered ERROR boom";

const db = { "prod-1": [], "prod-2": [] };
let seq = 0;
function store(host, msg) {
  seq += 1;
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + seq).toISOString().replace("Z", "000000Z");
  db[host].push({ ts, msg, key: `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}` });
}
for (let i = 0; i < 600; i++) store("prod-1", i % 2 ? `p1-${i} info ok` : `p1-${i} ERROR boom`);
for (let i = 0; i < 300; i++) store("prod-2", `p2-${i} info ok`);
const statusOf = (msg) => (msg.includes("ERROR") ? "error" : "info");

function readPage(p, newer, cap) {
  const rows = db[p.get("host")] || [];
  const limit = Math.min(Number(p.get("limit")) || 30, cap);
  const startWith = p.get("startWith") || "";
  const keys = rows.map((r) => r.key);
  let i = startWith ? keys.findIndex((k) => k >= startWith) : newer ? 0 : rows.length - 1;
  if (startWith && i === -1) {
    if (newer) return { logs: [], is_end: true };
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
    const r = newer ? rows[i++] : rows[i--];
    if (startWith && (r.key === startWith || r.key.startsWith(startWith + " +"))) continue;
    if (p.get("status") && statusOf(r.msg) !== p.get("status")) continue;
    logs.push([r.ts, r.msg, r.key]);
    last = r.key;
    counter += 1;
  }
  return { logs, last_processed_key: last, is_end: isEnd };
}

const held = [];
let holdCursorPages = 0;
const heldLoads = [];
let holdLoads = 0;
let cursorPageCap = Infinity;
const cursorRequests = [];
const abortError = () => new DOMException("This operation was aborted", "AbortError");

async function fetchImpl(url, init = {}) {
  if (init.signal?.aborted) throw abortError();
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  const p = u.searchParams;
  if (name !== "getLogs" && name !== "getPrevLogs") return jsonResponse(null);
  const startWith = p.get("startWith");
  if (startWith) cursorRequests.push({ host: p.get("host"), startWith });
  const payload = readPage(p, name === "getPrevLogs", startWith ? cursorPageCap : Infinity);
  if (!startWith && holdLoads > 0) {
    holdLoads -= 1;
    await new Promise((resolve) => heldLoads.push(resolve));
  }
  if (startWith && holdCursorPages > 0) {
    holdCursorPages -= 1;
    await new Promise((resolve, reject) => {
      held.push(resolve);
      init.signal?.addEventListener("abort", () => reject(abortError()), { once: true });
    });
  }
  return jsonResponse(payload);
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const rowIndex = (m) => Number(m.split(" ")[0].slice(3));
const handles = () => globalThis.__onlogsObservers || [];
const byId = (id) => handles().find((h) => h.element?.id === id);
const sentinels = () => handles().filter((h) => h.element?.classList?.contains("observer"));
const armed = async () => {
  await new Promise((resolve) => setTimeout(resolve, 1400));
  await settle(10);
};

const jumps = [];
function recordJumps() {
  window.Element.prototype.scrollIntoView = function () {
    jumps.push(`${this.className}: ${this.textContent.trim()}`);
  };
}

async function pageFromSentinel() {
  for (const handle of sentinels()) {
    const before = cursorRequests.length;
    handle.set(true);
    await settle(8);
    handle.set(false);
    await settle(2);
    if (cursorRequests.length > before) return;
  }
  throw new Error("no sentinel requested a page");
}

async function scrollDown() {
  const el = document.querySelector("#logs");
  let top = 100;
  Object.defineProperty(el, "scrollTop", { configurable: true, get: () => top });
  el.dispatchEvent(new window.Event("scroll"));
  top = 5000;
  el.dispatchEvent(new window.Event("scroll"));
  await settle(5);
}

async function openView(bundle, host, status) {
  installDom({ fetchImpl });
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(host);
  bundle.lastChosenService.set(SERVICE);
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  bundle.chosenStatus.set(status);
  await settle(40);
  await armed();
  return view;
}

async function switchHost(bundle, host) {
  bundle.lastChosenHost.set(host);
  bundle.lastChosenService.set(SERVICE);
  await settle(40);
  const rows = rendered();
  assert.ok(rows.length && rows.every((m) => m.startsWith("p2-")), "control: the switch renders the new host alone");
  return rows;
}

async function releaseAndAssertUnchanged(before, label) {
  jumps.length = 0;
  held.splice(0).forEach((resolve) => resolve());
  await settle(30);
  await new Promise((resolve) => setTimeout(resolve, 100));
  assert.deepEqual(rendered(), before, `${label}: a page of the previous selection changed the view`);
  assert.deepEqual(jumps, [], `${label}: the view scrolled for a discarded page`);
}

function assertCursorsMatchHost(label) {
  for (const r of cursorRequests) {
    assert.ok(
      db[r.host].some((row) => row.key === r.startWith),
      `${label}: asked ${r.host} for rows next to a row of another host`
    );
  }
}

async function scrollUpPageAfterHostSwitchIsDiscarded(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  cursorRequests.length = 0;
  cursorPageCap = 20;
  holdCursorPages = 1;
  await pageFromSentinel();
  const switched = await switchHost(bundle, "prod-2");
  await releaseAndAssertUnchanged(switched, "scroll up, host switch");
  cursorPageCap = Infinity;
  assertCursorsMatchHost("scroll up, host switch");

  await armed();
  await pageFromSentinel();
  await settle(20);
  const paged = rendered();
  assert.equal(paged[0], "p2-60 info ok", "scrolling up in the new view did not load older rows");
  assert.ok(paged.every((m) => m.startsWith("p2-")), "older rows of the new host mixed with another host");
  bundle.unmount(view);
}

async function scrollUpPageAfterFilterClearIsDiscarded(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  holdCursorPages = 1;
  await pageFromSentinel();
  window.dispatchEvent(new window.KeyboardEvent("keydown", { key: "Escape" }));
  await settle(40);
  const unfiltered = rendered();
  assert.ok(
    unfiltered.length === 180 && unfiltered[0] === "p1-420 ERROR boom" && unfiltered.at(-1) === "p1-599 info ok",
    "control: clearing the filter reloads the newest rows"
  );
  await releaseAndAssertUnchanged(unfiltered, "scroll up, filter cleared");
  bundle.unmount(view);
}

async function topPageAfterHostSwitchIsDiscarded(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  holdCursorPages = 1;
  const before = cursorRequests.length;
  byId("startOfLogs").set(true);
  for (let i = 0; i < 20 && cursorRequests.length === before; i++) {
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  assert.equal(held.length, 1, "the top-of-list loader did not request a page");
  byId("startOfLogs").set(false);
  const switched = await switchHost(bundle, "prod-2");
  await releaseAndAssertUnchanged(switched, "top of list, host switch");
  bundle.unmount(view);
}

async function scrollDownPageAfterHostSwitchIsDiscarded(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  await pageFromSentinel();
  await settle(20);
  assert.equal(rendered().at(-1), "p1-478 ERROR boom", "control: scrolling up left newer rows to page down to");
  await scrollDown();
  holdCursorPages = 1;
  await pageFromSentinel();
  assert.equal(held.length, 1, "scrolling down did not request a page");
  const switched = await switchHost(bundle, "prod-2");
  await releaseAndAssertUnchanged(switched, "scroll down, host switch");
  bundle.unmount(view);
}

async function returnToBottomWithHeldReload() {
  const end = byId("endOfLogs");
  end.set(false);
  await settle(5);
  seq += 1;
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + seq).toISOString().replace("Z", "000000Z");
  db["prod-1"].push({ ts, msg: BUFFERED, key: `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}` });
  window.WebSocket.instances.at(-1).onmessage({ data: JSON.stringify([ts, BUFFERED]) });
  await settle(5);
  holdLoads = 1;
  end.set(true);
  await settle(10);
  assert.equal(heldLoads.length, 1, "returning to the bottom did not reload");
  return end;
}

async function releaseReload() {
  heldLoads.splice(0).forEach((resolve) => resolve());
  await settle(30);
  const reloaded = rendered();
  assert.equal(reloaded.at(-1), BUFFERED, "control: the reload shows the buffered line");
  return reloaded;
}

async function scrollUpPageStartedDuringReloadIsDiscarded(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  const end = await returnToBottomWithHeldReload();
  holdCursorPages = 1;
  await pageFromSentinel();
  const reloaded = await releaseReload();
  await releaseAndAssertUnchanged(reloaded, "scroll up during a reload");
  end.set(false);
  bundle.unmount(view);
}

async function pageMergedJustBeforeReloadDoesNotMoveReloadedView(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  const end = await returnToBottomWithHeldReload();
  holdCursorPages = 1;
  await pageFromSentinel();
  const shown = rendered()[0];
  jumps.length = 0;
  held.splice(0).forEach((resolve) => resolve());
  for (let i = 0; i < 20 && rendered()[0] === shown; i++) await settle(1);
  assert.notEqual(rendered()[0], shown, "control: the page merged into the view being replaced");
  await releaseReload();
  await new Promise((resolve) => setTimeout(resolve, 100));
  assert.deepEqual(jumps, [], "the reloaded view scrolled for a page of the view it replaced");
  end.set(false);
  bundle.unmount(view);
}

async function scrollUpAfterReloadStillLoadsOlderRows(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  const end = await returnToBottomWithHeldReload();
  const reloaded = await releaseReload();
  await pageFromSentinel();
  await settle(20);
  assert.equal(rowIndex(rendered()[0]), rowIndex(reloaded[0]) - 120, "scrolling up after a reload did not load older rows");
  end.set(false);
  bundle.unmount(view);
}

async function scrollUpPageAfterCaseToggleIsDiscarded(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  const before = rendered();
  holdCursorPages = 1;
  await pageFromSentinel();
  bundle.store.update((s) => ({ ...s, caseInSensitive: !s.caseInSensitive }));
  await settle(20);
  await releaseAndAssertUnchanged(before, "scroll up, case toggled");
  await pageFromSentinel();
  await settle(20);
  assert.equal(rowIndex(rendered()[0]), rowIndex(before[0]) - 120, "scrolling up after the drop did not load older rows");
  bundle.store.update((s) => ({ ...s, caseInSensitive: !s.caseInSensitive }));
  bundle.unmount(view);
}

async function topSentinelSeenOnEmptyListIsDropped(bundle) {
  const view = await openView(bundle, "prod-1", "error");
  holdLoads = 10;
  bundle.chosenStatus.set("");
  await settle(10);
  assert.ok(heldLoads.length >= 1 && rendered().length === 0, "control: clearing the filter reloads");
  // While the list is empty the top sentinel is in view.
  byId("startOfLogs").set(true);
  await settle(5);
  const before = cursorRequests.length;
  holdLoads = 0;
  heldLoads.splice(0).forEach((resolve) => resolve());
  await settle(30);
  await new Promise((resolve) => setTimeout(resolve, 700));
  assert.equal(cursorRequests.length, before, "the reloaded list loaded older rows on its own");
  assert.equal(rendered().at(-1), db["prod-1"].at(-1).msg);
  bundle.unmount(view);
}

main(async () => {
  installDom({ fetchImpl });
  recordJumps();
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  await scrollUpPageAfterHostSwitchIsDiscarded(bundle);
  await scrollUpPageAfterFilterClearIsDiscarded(bundle);
  await topPageAfterHostSwitchIsDiscarded(bundle);
  await scrollDownPageAfterHostSwitchIsDiscarded(bundle);
  await scrollUpPageStartedDuringReloadIsDiscarded(bundle);
  await pageMergedJustBeforeReloadDoesNotMoveReloadedView(bundle);
  await scrollUpAfterReloadStillLoadsOlderRows(bundle);
  await scrollUpPageAfterCaseToggleIsDiscarded(bundle);
  await topSentinelSeenOnEmptyListIsDropped(bundle);
  console.log("NewLogsV2 stale page tests passed");
});
