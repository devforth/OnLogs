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
let seq = 0;
let sockets = [];

function store(msg) {
  seq += 1;
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + seq).toISOString().replace("Z", "000000Z");
  rows.push({ ts, msg, key: `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}` });
  for (const ws of sockets) {
    if (ws.readyState === 1 && ws.url.includes(`host=${HOST}&id=${SERVICE}`)) {
      ws.onmessage?.({ data: JSON.stringify([ts, msg]) });
    }
  }
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const p = u.searchParams;
  if (u.pathname.endsWith("/getLogs") && !p.get("startWith")) {
    const search = (p.get("search") || "").toLowerCase();
    const status = (p.get("status") || "").toUpperCase();
    const logs = rows
      .filter((r) => !search || r.msg.toLowerCase().includes(search))
      .filter((r) => !status || r.msg.startsWith(status))
      .slice(-(Number(p.get("limit")) || 30))
      .reverse()
      .map((r) => [r.ts, r.msg, r.key]);
    return jsonResponse({ logs, last_processed_key: logs.at(-1)?.[2] || "", is_end: true });
  }
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const emptyMessage = () => document.querySelector(".noLogsMessage")?.textContent.trim() ?? null;

let bundle;

// The bottom sentinel is never reported as visible: an empty pane is 0 px tall.
async function open() {
  ({ sockets } = installDom({ fetchImpl }));
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  return view;
}

async function search(text) {
  const input = document.querySelector("#top-line input[type=text]");
  input.value = text;
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
  await new Promise((resolve) => setTimeout(resolve, 900));
  await settle(40);
}

main(async () => {
  installDom({ fetchImpl });
  bundle = await importBundle(await bundleComponent("test/entry.js"));
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);

  let view = await open();
  assert.equal(emptyMessage(), "No logs written yet");
  store("INFO first line");
  await settle(10);
  assert.deepEqual(rendered(), ["INFO first line"], "an empty view kept a live line back");
  globalThis.__onlogsObservers.find((o) => o.element?.id === "endOfLogs").set(true);
  await settle(5);
  store("INFO second line");
  await settle(10);
  assert.deepEqual(rendered(), ["INFO first line", "INFO second line"]);
  bundle.unmount(view);

  view = await open();
  await search("needle");
  assert.deepEqual(rendered(), []);
  assert.equal(emptyMessage(), "No matching log lines", "an empty search result says nothing was written");
  store("INFO hay");
  store("INFO a needle");
  await settle(10);
  assert.deepEqual(rendered(), ["INFO a needle"], "a search with no matches kept a matching live line back");
  bundle.unmount(view);

  bundle.chosenStatus.set("error");
  view = await open();
  bundle.chosenStatus.set("error");
  await settle(40);
  assert.equal(emptyMessage(), "No matching log lines", "an empty status filter says nothing was written");
  bundle.unmount(view);
  bundle.chosenStatus.set("");

  console.log("NewLogsV2 empty view tests passed");
});
