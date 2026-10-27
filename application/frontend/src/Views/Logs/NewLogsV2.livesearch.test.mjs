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
const ESC = "\x1b";

const ANSI = /[\x1B\x9B][[\]()#;?]*(?:(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><])/g;
const normalize = (s, caseSens) => {
  const t = s.replace(ANSI, "").split(/\s+/).filter(Boolean).join(" ");
  return caseSens ? t : t.toLowerCase();
};

const rows = [];
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

function newestPage(p) {
  const caseSens = p.get("caseSens") === "true";
  const search = normalize(p.get("search") || "", caseSens);
  const status = p.get("status") || "";
  const logs = rows
    .filter((r) => !search || normalize(r.msg, caseSens).includes(search))
    .filter((r) => !status || bundle.classifyLogLine(r.msg) === status)
    .slice(-(Number(p.get("limit")) || 30))
    .reverse()
    .map((r) => [r.ts, r.msg, r.key]);
  return { logs, last_processed_key: logs.at(-1)?.[2] || "", is_end: true };
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  if (name === "getLogs" && !u.searchParams.get("startWith")) {
    return jsonResponse(newestPage(u.searchParams));
  }
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());

let bundle;

async function openWithSearch(caseInSensitive, text) {
  ({ sockets } = installDom({ fetchImpl }));
  bundle.store.update((s) => ({ ...s, caseInSensitive }));
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  const input = document.querySelector("#top-line input[type=text]");
  input.value = text;
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
  await new Promise((resolve) => setTimeout(resolve, 900));
  await settle(40);
  globalThis.__onlogsObservers.find((o) => o.element?.id === "endOfLogs").set(true);
  await settle(5);
  return view;
}

main(async () => {
  installDom({ fetchImpl });
  bundle = await importBundle(await bundleComponent("test/entry.js"));
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);
  ["INFO boot", "ERROR: disk full", "error: retrying"].forEach(store);

  let view = await openWithSearch(true, "error");
  assert.deepEqual(rendered(), ["ERROR: disk full", "error: retrying"]);
  store("ERROR: payment failed");
  store("INFO unrelated");
  store("error: cache miss");
  await settle(10);
  assert.deepEqual(
    rendered().slice(2),
    ["ERROR: payment failed", "error: cache miss"],
    "case-insensitive search keeps live lines that differ only in case"
  );
  bundle.unmount(view);

  view = await openWithSearch(true, "error:  db");
  store(`${ESC}[31merror${ESC}[0m: db down`);
  store("error:  db slow");
  store("error:\tdb ok");
  await settle(10);
  assert.deepEqual(
    rendered().map((m) => m.replace(ANSI, "")),
    ["error: db down", "error:  db slow", "error:\tdb ok"],
    "live search ignores ANSI colour codes and runs of whitespace"
  );
  bundle.unmount(view);

  view = await openWithSearch(false, "error");
  const before = rendered();
  assert.ok(!before.includes("ERROR: disk full"));
  store("ERROR: upstream timeout");
  store("error: still here");
  await settle(10);
  assert.deepEqual(
    rendered(),
    [...before, "error: still here"],
    "case-sensitive search drops live lines that differ in case"
  );
  bundle.unmount(view);

  store("plain boot line");
  view = await openWithSearch(true, "");
  bundle.chosenStatus.set("other");
  await settle(40);
  globalThis.__onlogsObservers.find((o) => o.element?.id === "endOfLogs").set(true);
  await settle(5);
  const unclassified = rendered();
  assert.deepEqual(unclassified, ["plain boot line"]);
  store("plain line without a level");
  store("INFO classified");
  await settle(10);
  assert.deepEqual(
    rendered(),
    [...unclassified, "plain line without a level"],
    "the Other filter dropped a matching live line"
  );
  bundle.unmount(view);
  bundle.chosenStatus.set("");

  console.log("live search filter tests passed");
});
