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
for (let i = 0; i < 600; i++) {
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + i).toISOString().replace("Z", "000000Z");
  rows.push({ ts, msg: `row-${i}`, key: `${ts} +${1792400000000000000n + BigInt(i)}-${i}` });
}

function readPage(p, newer) {
  const limit = Number(p.get("limit")) || 30;
  const startWith = p.get("startWith") || "";
  let i = startWith ? rows.findIndex((r) => r.key === startWith) : newer ? 0 : rows.length - 1;
  if (startWith) i += newer ? 1 : -1;
  const logs = [];
  while (logs.length < limit && i >= 0 && i < rows.length) {
    const r = newer ? rows[i++] : rows[i--];
    logs.push([r.ts, r.msg, r.key]);
  }
  return { logs, last_processed_key: logs.at(-1)?.[2] ?? "", is_end: i < 0 || i >= rows.length };
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  if (name !== "getLogs" && name !== "getPrevLogs") return jsonResponse(null);
  return jsonResponse(readPage(u.searchParams, name === "getPrevLogs"));
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const sentinels = () =>
  (globalThis.__onlogsObservers || []).filter((h) => h.element?.classList?.contains("observer"));

const jumps = [];

async function scroll(direction) {
  const el = document.querySelector("#logs");
  let top = direction === "down" ? 100 : 5000;
  Object.defineProperty(el, "scrollTop", { configurable: true, get: () => top });
  el.dispatchEvent(new window.Event("scroll"));
  top = direction === "down" ? 5000 : 100;
  el.dispatchEvent(new window.Event("scroll"));
  await settle(5);
}

async function pageFromSentinel() {
  for (const handle of sentinels()) {
    handle.set(true);
    await settle(8);
    handle.set(false);
    await settle(2);
  }
  await new Promise((resolve) => setTimeout(resolve, 100));
  await settle(10);
}

main(async () => {
  installDom({ fetchImpl });
  window.Element.prototype.scrollIntoView = function () {
    jumps.push({ className: String(this.className), text: this.textContent });
  };
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set("prod-1");
  bundle.lastChosenService.set("api");
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  await new Promise((resolve) => setTimeout(resolve, 1400));
  await settle(10);
  assert.equal(rendered()[0], "row-420", "control: the view opens on the newest 180 rows");

  await scroll("up");
  await pageFromSentinel();
  assert.equal(rendered()[0], "row-360", "control: scrolling up loads one older page");

  jumps.length = 0;
  await scroll("down");
  await pageFromSentinel();
  const shown = rendered();
  assert.equal(shown[0], "row-420", "control: scrolling down loads one newer page");
  const anchor = jumps.filter((j) => j.className.includes("newLogsEnd")).at(-1);
  assert.ok(anchor, "the view did not keep its place after loading newer rows");
  assert.ok(
    anchor.text.includes(shown[89]),
    `the view kept its place on a row of the previous window (${anchor.text.trim()}), not ${shown[89]}`
  );

  bundle.unmount(view);
  console.log("NewLogsV2 scroll anchor tests passed");
});
