import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

async function fetchImpl() {
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
  console.log("Stats tests passed");
});
