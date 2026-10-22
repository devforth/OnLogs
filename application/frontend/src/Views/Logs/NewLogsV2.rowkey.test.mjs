import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const T = "2026-10-19T09:00:00.000000000Z";
const ROWS = [];
const add = (ts, msg) => {
  const row = [ts, msg, `${ts} +17924000000000000${10 + ROWS.length}-${ROWS.length + 1}`];
  ROWS.push(row);
  return row;
};
add("2026-10-19T08:59:59.000000000Z", "ready");
add(T, "job 41");
const job42 = add(T, "job 42");
add(T, "job 43");
add("2026-10-19T09:00:01.000000000Z", "idle");

function page(params, newer, include) {
  const limit = Number(params.get("limit")) || 60;
  const startWith = params.get("startWith") || "";
  let i = startWith ? ROWS.findIndex((r) => r[2] >= startWith) : newer ? 0 : ROWS.length - 1;
  if (i === -1) i = newer ? ROWS.length : ROWS.length - 1;
  const logs = [];
  for (; i >= 0 && i < ROWS.length && logs.length < limit; i += newer ? 1 : -1) {
    const key = ROWS[i][2];
    if (!include && startWith && (key === startWith || key.startsWith(startWith + " +"))) continue;
    logs.push([...ROWS[i]]);
  }
  return { logs, last_processed_key: logs.at(-1)?.[2] ?? "", is_end: true };
}

const withPrevCursors = [];
async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  if (name === "getLogWithPrev") {
    withPrevCursors.push(u.searchParams.get("startWith"));
    return jsonResponse(page(u.searchParams, false, true));
  }
  if (name === "getPrevLogs") return jsonResponse(page(u.searchParams, true, false));
  if (name === "getLogs") return jsonResponse(page(u.searchParams, false, false));
  return jsonResponse(null);
}

const messages = (selector) =>
  [...document.querySelectorAll(selector)].map((el) => el.textContent.trim());
const rowOf = (msg) =>
  [...document.querySelectorAll("#logs .logsString")].find(
    (el) => el.querySelector(".message p")?.textContent.trim() === msg
  );

let bundle;
async function open(hash) {
  const { window, sockets } = installDom({ fetchImpl });
  bundle.urlHash.set(hash);
  const view = bundle.mount(bundle.NewLogsV2, { target: window.document.body });
  await settle(40);
  await new Promise((resolve) => setTimeout(resolve, 1200));
  await settle(10);
  return { view, sockets };
}

main(async () => {
  const { window } = installDom({ fetchImpl });
  let copied = "";
  Object.defineProperty(window.navigator, "clipboard", {
    configurable: true,
    value: { writeText: async (text) => { copied = text; } },
  });
  globalThis.navigator = window.navigator;
  bundle = await importBundle(await bundleComponent("test/entry.js"));
  bundle.isPending.set(false);
  bundle.lastChosenHost.set("testhost");
  bundle.lastChosenService.set("testservice");

  let { view } = await open("");
  rowOf("job 42").querySelector(".shareLinkButtonThumb").click();
  await settle(10);
  assert.ok(copied && !/\s/.test(copied), `the copied link breaks at whitespace: ${copied}`);
  const hash = new URL(copied).hash;
  assert.equal(decodeURIComponent(hash.slice(1)), job42[2], `the link does not name the shared row: ${copied}`);
  bundle.unmount(view);

  ({ view } = await open(hash));
  assert.equal(withPrevCursors.at(-1), job42[2]);
  assert.deepEqual(messages("#logs .message p"), ["ready", "job 41", "job 42", "job 43", "idle"]);
  assert.deepEqual(messages(".chosen .message p"), ["job 42"]);
  bundle.unmount(view);

  let sockets;
  ({ view, sockets } = await open(""));
  globalThis.__onlogsObservers.find((o) => o.element?.id === "endOfLogs").set(true);
  await settle(5);
  const ts = "2026-10-19T09:00:05.000000000Z";
  for (const seq of [1, 2]) {
    sockets.at(-1).onmessage({ data: JSON.stringify([ts, "tick", `${ts} +1792400000000000100-${seq}`]) });
    await settle(5);
  }
  assert.deepEqual(messages("#logs .message p").slice(-2), ["tick", "tick"], "a distinct live row was dropped");
  bundle.unmount(view);
});
