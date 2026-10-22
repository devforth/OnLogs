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
const NEEDLE = "order-7f3a9c-refund-failed";
const CAP = 500;

const dbs = {};
let seq = 0;
function put(service, msg) {
  seq += 1;
  const ts = new Date(Date.UTC(2026, 9, 19, 8, 0, 0) + seq).toISOString().replace("Z", "000000Z");
  (dbs[service] ??= []).push({
    ts,
    msg: `${msg} #${String(seq).padStart(6, "0")}`,
    key: `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}`,
  });
}
const fill = (service, n) => {
  for (let i = 0; i < n; i++) put(service, "GET /health 200");
};
const match = (service, n, tag) => {
  for (let i = 0; i < n; i++) put(service, `ERROR ${NEEDLE} ${tag}`);
};

function scan(rows, p, newer) {
  const limit = Math.min(Number(p.get("limit")) || 30, 1000);
  const startWith = p.get("startWith") || "";
  const search = (p.get("search") || "").toLowerCase();
  if (!rows.length) return { logs: [], last_processed_key: "", is_end: true };
  let i = startWith ? rows.findIndex((r) => r.key >= startWith) : newer ? 0 : rows.length - 1;
  if (i === -1) {
    if (newer) return { logs: [], is_end: true };
    i = rows.length - 1;
  }
  const logs = [];
  let isEnd = false;
  let last = "";
  let visited = "";
  let capped = false;
  for (let counter = 0, iteration = 0; counter < limit; ) {
    if (iteration >= CAP) {
      capped = true;
      break;
    }
    iteration += 1;
    if (i < 0 || i >= rows.length) {
      isEnd = true;
      counter += 1;
      continue;
    }
    isEnd = false;
    const r = rows[i];
    i += newer ? 1 : -1;
    visited = r.key;
    if (startWith && (r.key === startWith || r.key.startsWith(startWith + " +"))) continue;
    if (search && !r.msg.toLowerCase().includes(search)) continue;
    logs.push([r.ts, r.msg, r.key]);
    counter += 1;
    last = r.key;
  }
  const page = { logs, last_processed_key: capped && !last ? visited : last, is_end: isEnd };
  if (capped) page.scan_capped = true;
  return page;
}

let requests = [];
let holdNext = null;
async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  const p = u.searchParams;
  if (name !== "getLogs" && name !== "getPrevLogs") return jsonResponse(null);
  const page = scan(dbs[p.get("id")] || [], p, name === "getPrevLogs");
  requests.push({ name, search: p.get("search"), startWith: p.get("startWith") || "", capped: !!page.scan_capped, end: page.is_end, rows: page.logs.length });
  if (holdNext?.(p, page)) {
    holdNext = null;
    await new Promise((resolve) => (globalThis.__release = resolve));
  }
  return jsonResponse(page);
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const seqOf = (msg) => Number(/#(\d+)$/.exec(msg)?.[1]);
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function until(label, cond, ms = 5000) {
  const deadline = Date.now() + ms;
  while (!cond()) {
    if (Date.now() > deadline) assert.fail(`${label} (rendered ${rendered().length} rows)`);
    await sleep(20);
  }
}

let bundle;

async function openAndSearch(service) {
  installDom({ fetchImpl });
  requests = [];
  bundle.urlHash.set("");
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(service);
  bundle.isPending.set(false);
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  const input = document.querySelector("input[type=text]");
  input.value = NEEDLE;
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
  await sleep(900);
  await settle(40);
  return view;
}

function assertOrderedAndBounded(label) {
  const seqs = rendered().map(seqOf);
  assert.ok(seqs.length <= 180, `${label}: ${seqs.length} rows rendered, more than three pages`);
  for (let i = 1; i < seqs.length; i++) {
    assert.ok(seqs[i - 1] < seqs[i], `${label}: rows out of order at ${i}: #${seqs[i - 1]} before #${seqs[i]}`);
  }
}

function assertNoRepeatedCursor(label) {
  const cursors = requests.filter((r) => r.search === NEEDLE && r.startWith).map((r) => r.startWith);
  assert.equal(new Set(cursors).size, cursors.length, `${label}: a request repeated a cursor it had already sent`);
}

const holdTop = () => globalThis.__onlogsObservers.find((h) => h.element?.id === "startOfLogs").set(true);
const count = (tag) => rendered().filter((m) => m.includes(NEEDLE) && m.includes(tag)).length;

async function firstPageEmptyAndCapped() {
  match("svc-a", 1, "old");
  fill("svc-a", CAP + 100);
  const view = await openAndSearch("svc-a");
  await until("svc-a: the match behind the scan cap never showed", () => count("old") === 1);
  assert.equal(document.querySelector(".noLogsMessage"), null, "svc-a: shows 'No logs written yet'");
  assertNoRepeatedCursor("svc-a");
  bundle.unmount(view);
}

async function firstPageCappedPartial() {
  match("svc-b", 200, "old");
  fill("svc-b", 1000);
  for (let i = 0; i < 100; i++) {
    fill("svc-b", 3);
    match("svc-b", 1, "new");
  }
  const view = await openAndSearch("svc-b");
  holdTop();
  await until("svc-b: matches behind the scan cap never showed", () => count("old") > 0);
  assert.equal(count("new"), 100, "svc-b: the newest matches went missing");
  assertOrderedAndBounded("svc-b");
  assertNoRepeatedCursor("svc-b");
  bundle.unmount(view);
}

async function firstPageShortThenGap() {
  match("svc-d", 210, "old");
  fill("svc-d", 1200);
  for (let i = 0; i < 30; i++) {
    fill("svc-d", 9);
    match("svc-d", 1, "new");
  }
  const view = await openAndSearch("svc-d");
  holdTop();
  await until("svc-d: matches behind the scan cap never showed", () => count("old") > 0);
  assert.equal(count("new"), 30, "svc-d: the newest matches went missing");
  assertOrderedAndBounded("svc-d");
  assertNoRepeatedCursor("svc-d");
  bundle.unmount(view);
}

async function scrollUpThroughGap() {
  match("svc-c", 150, "old");
  fill("svc-c", 600);
  match("svc-c", 30, "mid");
  fill("svc-c", 1000);
  for (let i = 0; i < 180; i++) {
    fill("svc-c", 1);
    match("svc-c", 1, "new");
  }
  const view = await openAndSearch("svc-c");
  await until("svc-c: first load", () => count("new") === 180);
  holdTop();
  await until("svc-c: scrolling up never reached the matches behind the scan cap", () => count("old") > 0);
  await until("svc-c: scrolling up never reached the start of history", () =>
    requests.some((r) => r.search === NEEDLE && r.startWith && r.end && !r.rows)
  );
  const sent = requests.length;
  await sleep(600);
  assert.equal(requests.length, sent, "svc-c: the top loader kept asking at the start of history");
  assertOrderedAndBounded("svc-c");
  assertNoRepeatedCursor("svc-c");
  bundle.unmount(view);
}

async function walkStopsWhenSuperseded() {
  match("svc-e", 1, "old");
  fill("svc-e", CAP * 10);
  for (let i = 0; i < 180; i++) {
    match("svc-e", 1, "new");
  }
  const view = await openAndSearch("svc-e");
  await until("svc-e: first load", () => count("new") === 180);
  let held = false;
  holdNext = (p, page) => (held = Boolean(p.get("startWith") && page.scan_capped && requests.filter((r) => r.capped).length === 2));
  holdTop();
  await until("svc-e: the walk did not reach a second capped page", () => held);
  bundle.unmount(view);
  const sent = requests.length;
  globalThis.__release();
  await settle(40);
  await sleep(600);
  assert.equal(requests.length, sent, "svc-e: the walk kept scanning after the view was closed");
}

main(async () => {
  installDom({ fetchImpl });
  bundle = await importBundle(await bundleComponent("test/entry.js"));
  for (const run of [
    firstPageEmptyAndCapped,
    firstPageCappedPartial,
    firstPageShortThenGap,
    scrollUpThroughGap,
    walkStopsWhenSuperseded,
  ]) {
    await run();
    console.log(`  ${run.name}: ok`);
  }
  console.log("NewLogsV2 scan cap tests passed");
});
