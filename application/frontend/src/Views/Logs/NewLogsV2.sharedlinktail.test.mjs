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

function page(params, getPrev, include) {
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
    if (!include && startWith && (r.key === startWith || r.key.startsWith(startWith + " +"))) continue;
    logs.push([r.ts, r.msg, r.key]);
    last = r.key;
    counter += 1;
  }
  return { logs, last_processed_key: last, is_end: isEnd };
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  requests.push(name);
  if (name === "getLogWithPrev") return jsonResponse(page(u.searchParams, false, true));
  if (name === "getPrevLogs") return jsonResponse(page(u.searchParams, true, false));
  if (name === "getLogs") return jsonResponse(page(u.searchParams, false, false));
  return jsonResponse(null);
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());

function firstSkip(list) {
  for (let k = 1; k < list.length; k++) {
    if (pos.get(list[k]) !== pos.get(list[k - 1]) + 1) return [list[k - 1], list[k]];
  }
  return null;
}

let bundle;

const endOfLogs = () => globalThis.__onlogsObservers.find((h) => h.element?.id === "endOfLogs");

async function scrollDown() {
  const el = document.querySelector("#logs");
  let top = 100;
  Object.defineProperty(el, "scrollTop", { configurable: true, get: () => top });
  el.dispatchEvent(new window.Event("scroll"));
  top = 200;
  el.dispatchEvent(new window.Event("scroll"));
  await settle(5);
}

async function openSharedLinkAtBottom(stored, linkTs) {
  rows = [];
  pos = new Map();
  ({ sockets } = installDom({ fetchImpl }));
  for (let i = 0; i < stored; i++) store(`line-${i}`);
  bundle.isPending.set(false);
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);
  bundle.urlHash.set("#" + linkTs(rows));
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(60);
  const win = rendered();
  // The scroll listener is attached about a second after opening.
  await new Promise((resolve) => setTimeout(resolve, 1100));
  await scrollDown();
  endOfLogs().set(true);
  await settle(10);
  requests = [];
  return { view, win };
}

main(async () => {
  installDom({ fetchImpl });
  bundle = await importBundle(await bundleComponent("test/entry.js"));

  const cases = [
    { name: "link to an early row", stored: 300, linkTs: (r) => r[9].ts, linked: "line-9" },
    { name: "link to a removed row", stored: 300, linkTs: () => "2026-10-19T09:00:00.000000000Z" },
  ];
  for (const c of cases) {
    const { view, win } = await openSharedLinkAtBottom(c.stored, c.linkTs);
    assert.ok(win.length < 180 && win.at(-1) !== rows.at(-1).msg, `${c.name}: window should end in the past`);
    if (c.linked) assert.ok(win.includes(c.linked), `${c.name}: linked row missing`);

    store("live-1");
    await settle(30);
    // Each return to the end of the window loads the next newer page.
    for (let step = 0; rendered().at(-1) !== "live-1"; step++) {
      assert.ok(step < 10, `${c.name}: paging never reached the live line`);
      const after = rendered();
      const skip = firstSkip(after);
      assert.ok(!skip, `${c.name}: "${skip?.[0]}" is followed by "${skip?.[1]}" with stored rows hidden between`);
      assert.ok(after.includes(win.at(-1)) || step > 0, `${c.name}: the window jumped past the next page`);
      endOfLogs().set(false);
      await settle(5);
      endOfLogs().set(true);
      await settle(30);
    }
    assert.ok(!firstSkip(rendered()), `${c.name}: rows skipped before the live line`);
    assert.ok(!requests.includes("getLogs"), `${c.name}: the end of a past window reloaded the newest rows`);
    store("live-2");
    await settle(10);
    assert.equal(rendered().at(-1), "live-2", `${c.name}: the live tail did not resume at the newest row`);
    bundle.unmount(view);
  }

  const { view, win } = await openSharedLinkAtBottom(100, (r) => r[9].ts);
  assert.equal(win.at(-1), "line-99");
  store("live-1");
  await settle(30);
  assert.deepEqual(rendered(), [...win, "live-1"]);
  assert.deepEqual(requests, [], "appending below a window at the newest row reloaded it");
  bundle.unmount(view);
});
