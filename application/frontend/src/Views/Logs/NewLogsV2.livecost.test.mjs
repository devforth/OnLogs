// A live line must not reformat the dates of every row on screen.
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

  let localeCalls = 0;
  const original = Date.prototype.toLocaleString;
  Date.prototype.toLocaleString = function (...args) {
    localeCalls += 1;
    return original.apply(this, args);
  };
  try {
    for (let i = 0; i < 10; i++) {
      store(`live-${i}`, true);
      await settle(2);
    }
  } finally {
    Date.prototype.toLocaleString = original;
  }
  assert.ok(localeCalls < 10, `10 live lines made ${localeCalls} toLocaleString calls`);
  assert.equal(document.querySelectorAll("#logs .message p")[179].textContent.trim(), "live-9");

  bundle.unmount(view);
  console.log("live line cost tests passed");
});
