// A view opened on a few rows keeps every live line once it fills up.
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
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + rows.length + 1).toISOString().replace("Z", "000000Z");
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

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());

main(async () => {
  ({ sockets } = installDom({ fetchImpl }));
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  for (let i = 1; i <= 18; i++) store(`n=${i}`, false);
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  assert.equal(rendered().length, 18);
  globalThis.__onlogsObservers.find((h) => h.element?.id === "endOfLogs").set(true);
  // This feed is faster than the too-many-messages guard allows.
  bundle.manuallyUnmuted.set(true);
  await settle(5);

  for (let i = 19; i <= 200; i++) {
    store(`n=${i}`, true);
    await settle(1);
  }
  await settle(10);

  const shown = rendered();
  const expected = rows.map((r) => r.msg).slice(-shown.length);
  assert.deepEqual(shown, expected, "the live window lost rows from its middle");
  assert.equal(shown.length, 180);
  bundle.unmount(view);
  bundle.manuallyUnmuted.set(false);
  console.log("live window growth tests passed");
});
