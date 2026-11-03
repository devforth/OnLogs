// A reader who scrolls up is not pulled back by lines arriving in the same frame.
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
let sockets = [];

function store(msg, live) {
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + rows.length * 1000).toISOString().replace("Z", "000000Z");
  const key = `${ts} +${1792400000000000000n + BigInt(rows.length)}-${rows.length}`;
  rows.push({ ts, msg, key });
  if (!live) return;
  for (const ws of sockets) {
    if (ws.readyState === 1 && ws.url.includes(`host=${HOST}&id=${SERVICE}`)) {
      ws.onmessage?.({ data: JSON.stringify([ts, msg, key]) });
    }
  }
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  if (name === "getLogs" && !u.searchParams.get("startWith")) {
    const logs = rows
      .slice(-(Number(u.searchParams.get("limit")) || 30))
      .reverse()
      .map((r) => [r.ts, r.msg, r.key]);
    return jsonResponse({ logs, last_processed_key: logs.at(-1)?.[2] || "", is_end: true });
  }
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

main(async () => {
  ({ sockets } = installDom({ fetchImpl }));
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  for (let i = 1; i <= 200; i++) store(`n=${i}`, false);
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  await new Promise((resolve) => setTimeout(resolve, 1400));
  await settle(10);
  assert.equal(document.querySelectorAll("#logs .message p").length, 180);
  globalThis.__onlogsObservers.find((h) => h.element?.id === "endOfLogs").set(true);
  await settle(5);

  bundle.manuallyUnmuted.set(true);
  const logsEl = document.querySelector("#logs");
  let scrollTop = 9500;
  Object.defineProperty(logsEl, "scrollHeight", { configurable: true, get: () => 10000 });
  Object.defineProperty(logsEl, "clientHeight", { configurable: true, get: () => 500 });
  Object.defineProperty(logsEl, "scrollTop", { configurable: true, get: () => scrollTop, set: (v) => (scrollTop = v) });
  logsEl.scrollTo = (_x, y) => (scrollTop = Math.min(y, 10000 - 500));
  const last = () => [...document.querySelectorAll("#logs .message p")].at(-1).textContent.trim();
  const badge = () => document.querySelector(".buttonToBottomNumber")?.textContent.trim() ?? "";

  store("live-at-bottom", true);
  await settle(2);
  assert.equal(last(), "live-at-bottom", "a line at the bottom was not shown");

  // The wheel moved the view up; the observer has not reported it yet.
  scrollTop = 9000;
  for (let i = 0; i < 20; i++) store(`live-${i}`, true);
  await settle(5);
  assert.equal(scrollTop, 9000, "a live line scrolled the reader back to the bottom");
  assert.equal(last(), "live-at-bottom");
  globalThis.__onlogsObservers.find((h) => h.element?.id === "endOfLogs").set(false);
  await settle(5);
  assert.equal(badge(), "20", "the lines that arrived meanwhile are not counted");
  bundle.manuallyUnmuted.set(false);

  bundle.unmount(view);
  console.log("wheel up tests passed");
});
