// A link to a line that no longer exists opens where the line was.
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
  } else if (startWith && !getPrev && rows[i].key !== startWith && !rows[i].key.startsWith(startWith + " +")) {
    // As the backend: an older page from a missing cursor starts below it.
    i -= 1;
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
  if (name === "getLogWithPrev") return jsonResponse(page(u.searchParams, false));
  if (name === "getLogs") {
    if (u.searchParams.get("startWith")) olderRequests += 1;
    return jsonResponse(page(u.searchParams, false));
  }
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

const rendered = () => [...document.querySelectorAll("#logs .message p")].map((el) => +el.textContent.trim().slice(2));

main(async () => {
  const { window } = installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  for (let i = 1; i <= 300; i++) store(`n=${i}`);
  const scrolledTo = [];
  window.Element.prototype.scrollIntoView = function () {
    scrolledTo.push(this.textContent.trim());
  };
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set("prod-1");
  bundle.lastChosenService.set("api");
  // Between rows 150 and 151, as when retention deleted the linked line.
  bundle.urlHash.set("#" + rows[149].key + "0");
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  await new Promise((resolve) => setTimeout(resolve, 1200));
  await settle(10);

  const shown = rendered();
  assert.deepEqual(shown, Array.from({ length: shown.length }, (_, i) => shown[0] + i), "rows out of order");
  assert.ok(shown.includes(150) && shown.includes(151));
  assert.ok(
    scrolledTo.some((text) => /n=150\b/.test(text)),
    `the view was not brought to where the line was: ${JSON.stringify(scrolledTo.slice(0, 3))}`
  );

  bundle.unmount(view);
  console.log("deleted link tests passed");
});
