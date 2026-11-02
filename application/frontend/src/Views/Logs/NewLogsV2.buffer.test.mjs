// A view left scrolled up keeps up with a busy service and stays bounded.
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
  // Scrolled up: the end of the list is off screen, so live lines are buffered.
  bundle.manuallyUnmuted.set(true);
  const badge = () => document.querySelector(".buttonToBottomNumber")?.textContent.trim() ?? "";

  const started = performance.now();
  for (let i = 1; i <= 11000; i++) {
    store(`live-${i}`, true);
    if (i % 500 === 0) await settle(1);
  }
  await settle(5);
  const took = performance.now() - started;
  assert.equal(badge(), "11000", "the badge lost buffered lines");
  assert.ok(took < 5000, `buffering 11000 lines took ${took.toFixed(0)} ms`);

  document.querySelector(".buttonToBottom").click();
  await settle(40);
  const shown = [...document.querySelectorAll("#logs .message p")].map((p) => p.textContent.trim());
  assert.equal(shown.at(-1), "live-11000", "the newest line is not shown after going to the bottom");
  assert.equal(badge(), "", "the badge outlived the flush");

  bundle.manuallyUnmuted.set(false);
  bundle.unmount(view);
  console.log("live buffer tests passed");
});
