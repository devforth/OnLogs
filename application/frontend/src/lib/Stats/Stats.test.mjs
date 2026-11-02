import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

let statsCalls = 0;
let down = false;
async function fetchImpl() {
  statsCalls += 1;
  if (down) throw new TypeError("Failed to fetch");
  return jsonResponse({ info: 12, warn: 3, other: 5 });
}

main(async () => {
  const { window } = installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  bundle.lastChosenHost.set("h");
  bundle.lastChosenService.set("s");
  bundle.chosenStatus.set("");
  const stats = bundle.mount(bundle.Stats, { target: window.document.body });
  await settle(10);

  const item = (level) =>
    [...window.document.querySelectorAll(".statsItem")].find((li) => li.querySelector(`p.${level}`));
  const active = () =>
    [...window.document.querySelectorAll(".statsItem.active")].map((li) => li.querySelector("p").className);
  assert.deepEqual(active(), []);

  // The level filtering the view is marked, also for "Other", whose rows carry no badge.
  item("other").click();
  await settle(5);
  assert.deepEqual(active(), ["other"], "nothing shows which level filters the view");
  item("other").click();
  await settle(5);
  assert.deepEqual(active(), []);

  bundle.unmount(stats);

  // Polling is gentle: one request on open, none a second later, none while
  // the tab is hidden, and a master that is down throws nothing.
  const failures = [];
  const onRejection = (e) => failures.push(e);
  process.on("unhandledRejection", onRejection);
  statsCalls = 0;
  const polled = bundle.mount(bundle.Stats, { target: window.document.body });
  await new Promise((resolve) => setTimeout(resolve, 1500));
  assert.equal(statsCalls, 1, `the panel sent ${statsCalls} getStats requests in 1.5 s`);
  bundle.unmount(polled);

  Object.defineProperty(window.document, "hidden", { value: true, configurable: true });
  down = true;
  statsCalls = 0;
  const hidden = bundle.mount(bundle.Stats, { target: window.document.body });
  await new Promise((resolve) => setTimeout(resolve, 5600));
  assert.equal(statsCalls, 1, "a hidden tab kept polling");
  bundle.unmount(hidden);
  delete window.document.hidden;
  down = false;
  await settle(5);
  process.off("unhandledRejection", onRejection);
  assert.deepEqual(failures.map(String), [], "a failed stats request threw");
  console.log("Stats tests passed");
});
