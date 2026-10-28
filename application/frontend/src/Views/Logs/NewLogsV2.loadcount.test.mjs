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

const rows = Array.from({ length: 20 }, (_, i) => {
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, i)).toISOString().replace("Z", "000000Z");
  const msg = `${i % 2 ? "WARN" : "INFO"} row-${i}${i % 3 ? "" : " needle"}`;
  return [ts, msg, `${ts} +${1792400000000000000n + BigInt(i)}-${i}`];
});

// Newest-page loads only: the requests a service open or a filter change sends.
let loads = [];

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const p = u.searchParams;
  if (u.pathname.endsWith("/getLogs") && !p.get("startWith")) {
    loads.push(`${p.get("id")} search=${p.get("search")} status=${p.get("status")}`);
    const logs = rows
      .filter((r) => r[1].includes(p.get("search") || ""))
      .filter((r) => !p.get("status") || r[1].startsWith(p.get("status").toUpperCase()))
      .slice()
      .reverse();
    return jsonResponse({ logs, last_processed_key: logs.at(-1)?.[2] || "", is_end: true });
  }
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());

main(async () => {
  installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set("api");

  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  assert.deepEqual(loads, ["api search= status="], "opening a service sent duplicate loads");
  assert.equal(rendered().length, rows.length);

  loads = [];
  const input = document.querySelector("#top-line input[type=text]");
  input.value = "needle";
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
  await new Promise((resolve) => setTimeout(resolve, 900));
  await settle(40);
  assert.deepEqual(loads, ["api search=needle status="], "a search change sent duplicate loads");

  loads = [];
  bundle.chosenStatus.set("warn");
  await settle(40);
  assert.deepEqual(loads, ["api search=needle status=warn"], "a status change sent duplicate loads");
  assert.ok(rendered().every((m) => m.startsWith("WARN") && m.includes("needle")));

  loads = [];
  bundle.lastChosenService.set("worker");
  await settle(40);
  console.log(`  service switch with a filter on: ${loads.join(", ")}`);
  assert.equal(loads.at(-1), "worker search= status=");
  assert.ok(loads.length <= 2, `a service switch sent ${loads.length} loads: ${loads.join(", ")}`);
  assert.equal(rendered().length, rows.length);

  bundle.unmount(view);
  console.log("NewLogsV2 load count tests passed");
});
