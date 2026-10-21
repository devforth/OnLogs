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

function store(service, msg) {
  seq += 1;
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + seq).toISOString().replace("Z", "000000Z");
  rows[service].push({ ts, msg, key: `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}` });
  for (const ws of sockets) {
    if (ws.readyState === 1 && ws.url.includes(`host=${HOST}&id=${service}`)) {
      ws.onmessage?.({ data: JSON.stringify([ts, msg]) });
    }
  }
}

function newestPage(service, params) {
  const limit = Number(params.get("limit")) || 30;
  const logs = rows[service].slice(-limit).reverse().map((r) => [r.ts, r.msg, r.key]);
  return { logs, last_processed_key: logs.at(-1)?.[2] ?? "", is_end: true };
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  if (u.pathname.endsWith("/getLogs")) return jsonResponse(newestPage(u.searchParams.get("id"), u.searchParams));
  return jsonResponse(null);
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const count = (msg) => rendered().filter((m) => m === msg).length;
const notLive = () => /reconnect/i.test(document.querySelector(".liveTailDown")?.textContent ?? "");
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitFor(cond, ms) {
  for (const t0 = Date.now(); Date.now() - t0 < ms; await sleep(20)) if (cond()) return true;
  return cond();
}

function drop(ws) {
  ws.readyState = 3;
  ws.onclose?.({ code: 1006 });
}

let bundle;

async function openService(service) {
  ({ sockets } = installDom({ fetchImpl }));
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(service);
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(60);
  globalThis.__onlogsObservers.find((h) => h.element?.id === "endOfLogs").set(true);
  await settle(10);
  return view;
}

main(async () => {
  installDom({ fetchImpl });
  bundle = await importBundle(await bundleComponent("test/entry.js"));
  ["boot-1", "boot-2"].forEach((m) => store("api", m));

  let view = await openService("api");
  assert.equal(sockets.length, 1);
  store("api", "live-1");
  await settle(10);
  assert.deepEqual(rendered(), ["boot-1", "boot-2", "live-1"]);

  drop(sockets[0]);
  store("api", "gap-1");
  assert.ok(await waitFor(() => sockets.length === 2, 3000), "the view did not reconnect");
  assert.ok(sockets[1].url.includes(`host=${HOST}&id=api`));
  assert.ok(notLive(), "the view should show that the tail is not live");
  sockets[1].onopen?.({});
  await settle(30);
  assert.ok(!notLive(), "the not-live notice should go once the socket is open");
  store("api", "live-2");
  await settle(30);
  assert.deepEqual(["gap-1", "live-2"].map(count), [1, 1]);
  assert.deepEqual(rendered().slice(-3), ["live-1", "gap-1", "live-2"]);

  bundle.WSisMuted.set(true);
  drop(sockets[1]);
  assert.ok(await waitFor(() => sockets.length === 3, 3000), "the view did not reconnect");
  drop(sockets[2]);
  await sleep(1300);
  assert.equal(sockets.length, 3, "the retry after a failed attempt should wait longer");
  assert.ok(await waitFor(() => sockets.length === 4, 2000), "the view stopped retrying");
  let muted;
  bundle.WSisMuted.subscribe((v) => (muted = v))();
  assert.equal(muted, true, "a reconnect unmuted the live tail");
  bundle.WSisMuted.set(false);
  bundle.unmount(view);

  view = await openService("api");
  const sentinel = globalThis.__onlogsObservers.find((h) => h.element?.id === "endOfLogs");
  sentinel.set(false);
  await settle(10);
  const before = rendered();
  drop(sockets[0]);
  store("api", "gap-2");
  assert.ok(await waitFor(() => sockets.length === 2, 3000), "the view did not reconnect");
  sockets[1].onopen?.({});
  await settle(30);
  assert.deepEqual(rendered(), before, "the reconnect reloaded under a reader scrolled up");
  sentinel.set(true);
  await settle(30);
  assert.equal(count("gap-2"), 1);
  assert.equal(rendered().at(-1), "gap-2");
  bundle.unmount(view);

  view = await openService("api");
  bundle.lastChosenService.set("web");
  await settle(60);
  assert.equal(sockets.length, 2);
  drop(sockets[0]);
  bundle.unmount(view);
  drop(sockets[1]);
  await sleep(1500);
  assert.equal(sockets.length, 2, "a deliberate close reconnected");
  assert.ok(!notLive());

  view = await openService("api");
  drop(sockets[0]);
  bundle.unmount(view);
  await sleep(1500);
  assert.equal(sockets.length, 1, "a retry ran after the view was unmounted");
});
