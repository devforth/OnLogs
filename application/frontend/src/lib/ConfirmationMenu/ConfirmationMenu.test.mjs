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
let deleteStatus = 200;

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
  const name = u.pathname.split("/").pop();
  if (name === "deleteContainerLogs") {
    if (deleteStatus === 403) return jsonResponse({ error: "Only admin can perform this request" }, 403);
    rows = [];
    return jsonResponse({ error: null });
  }
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
const read = (s) => {
  let value;
  s.subscribe((v) => (value = v))();
  return value;
};

main(async () => {
  ({ sockets } = installDom({ fetchImpl }));
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  ["old-1", "old-2", "old-3"].forEach(store);
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);
  bundle.store.update((s) => ({ ...s, deleteFromDocker: false }));
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  globalThis.__onlogsObservers.find((h) => h.element?.id === "endOfLogs").set(true);
  await settle(5);
  assert.deepEqual(rendered(), ["old-1", "old-2", "old-3"]);

  bundle.confirmationObj.set({ action: null, message: "", isVisible: true });
  const menu = bundle.mount(bundle.ConfirmationMenu, { target: document.body });
  await settle(5);
  const deleteButton = () =>
    [...document.querySelectorAll(".deleteModalContainer button")].find((b) =>
      b.textContent.includes("Delete")
    );

  deleteStatus = 403;
  deleteButton().click();
  await settle(20);
  assert.equal(read(bundle.toast).message, "Only admin can perform this request", "the server's refusal was replaced");
  assert.equal(read(bundle.confirmationObj).isVisible, true);
  assert.deepEqual(rendered(), ["old-1", "old-2", "old-3"]);

  deleteStatus = 200;
  deleteButton().click();
  await settle(20);
  assert.equal(read(bundle.confirmationObj).isVisible, false);
  assert.deepEqual(rendered(), [], "cleared rows are still shown");
  store("after-clear");
  await settle(10);
  assert.deepEqual(rendered(), ["after-clear"]);

  bundle.unmount(menu);
  bundle.unmount(view);
  console.log("ConfirmationMenu tests passed");
});
