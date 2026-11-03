// An older page shorter than a full page keeps the window in order.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const rows = [];
function store(msg) {
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + rows.length * 1000).toISOString().replace("Z", "000000Z");
  rows.push({ ts, msg, key: `${ts} +${1792400000000000000n + BigInt(rows.length)}-${rows.length}` });
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

let olderRequests = 0;
async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  if (name === "getPrevLogs") return jsonResponse(page(u.searchParams, true));
  if (name === "getLogs") {
    if (u.searchParams.get("startWith")) olderRequests += 1;
    return jsonResponse(page(u.searchParams, false));
  }
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

const rendered = () => [...document.querySelectorAll("#logs .message p")].map((el) => +el.textContent.trim().slice(2));
const observer = (id) => globalThis.__onlogsObservers.find((h) => h.element?.id === id);

main(async () => {
  installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  // 210 rows: the first load shows the newest 180, the older page has only 30.
  for (let i = 1; i <= 210; i++) store(`n=${i}`);
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set("prod-1");
  bundle.lastChosenService.set("api");
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  await new Promise((resolve) => setTimeout(resolve, 1400));
  await settle(10);
  assert.deepEqual(rendered(), Array.from({ length: 180 }, (_, i) => i + 31));

  const logsEl = document.querySelector("#logs");
  logsEl.scrollTop = 100;
  logsEl.dispatchEvent(new window.Event("scroll"));
  await settle(5);
  observer("startOfLogs").set(true);
  for (let t = 0; t < 30 && olderRequests === 0; t++) {
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  observer("startOfLogs").set(false);
  await settle(20);
  assert.ok(olderRequests > 0, "no older page was requested");

  const shown = rendered();
  assert.deepEqual(shown, Array.from({ length: shown.length }, (_, i) => i + 1), `rows out of order: ${shown.slice(0, 12).join(",")} ...`);
  assert.equal(shown.length, 180);

  bundle.unmount(view);
  console.log("top page tests passed");
});
