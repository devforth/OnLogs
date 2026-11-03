// Search keeps working on lines long enough to be shown as a preview.
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
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + rows.length + 1).toISOString().replace("Z", "000000Z");
  rows.push({ ts, msg, key: `${ts} +${1792400000000000000n + BigInt(rows.length)}-${rows.length}` });
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  if (name === "getLogs" && !u.searchParams.get("startWith")) {
    const search = (u.searchParams.get("search") || "").toLowerCase();
    const logs = rows
      .filter((r) => !search || r.msg.toLowerCase().includes(search))
      .slice(-(Number(u.searchParams.get("limit")) || 30))
      .reverse()
      .map((r) => [r.ts, r.msg, r.key]);
    return jsonResponse({ logs, last_processed_key: logs.at(-1)?.[2] || "", is_end: true });
  }
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

main(async () => {
  installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  store("INFO short needle one");
  store("INFO head needle " + "x".repeat(20000) + " mid needle " + "y".repeat(20000) + " end needle");
  store("INFO only late " + "z".repeat(20000) + " late needle");
  store("INFO short two");
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set("prod-1");
  bundle.lastChosenService.set("api");
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  const input = document.querySelector("#top-line input[type=text]");
  input.value = "needle";
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
  await new Promise((resolve) => setTimeout(resolve, 900));
  await settle(40);

  const row = (marker) => [...document.querySelectorAll("#logs .message")].find((m) => m.textContent.includes(marker));
  const marks = (marker) => row(marker).querySelectorAll(".searchedText").length;

  // A match only past the preview opens the line whole, highlighted.
  assert.ok(row("late needle"), "a matching row hides its only match");
  assert.equal(marks("only late"), 1);

  // A match in the preview keeps it collapsed, and toggling keeps highlights.
  assert.equal(marks("head needle"), 1);
  row("head needle").querySelector(".showAll").click();
  await settle(20);
  assert.equal(marks("head needle"), 3, "Show all dropped or missed the highlights");
  row("head needle").querySelector(".showAll").click();
  await settle(20);
  assert.equal(marks("head needle"), 1, "Show less dropped the highlight");

  bundle.unmount(view);
  console.log("long line search tests passed");
});
