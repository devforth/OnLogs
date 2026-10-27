import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
  settle,
  jsonResponse,
  main,
} from "../../../test/harness.mjs";

let sizeResponse = () => jsonResponse({}, 401);

async function stubFetch(url) {
  if (String(url).includes("getSizeBy")) return sizeResponse();
  return jsonResponse({ error: null });
}

main(async () => {
  const outfile = await bundleComponent("test/entry.js");
  const { window } = installDom({ fetchImpl: stubFetch });
  const bundle = await importBundle(outfile);
  const target = window.document.body;

  const failures = [];
  process.on("unhandledRejection", (err) => failures.push(err));

  bundle.lastChosenHost.set("h");
  bundle.lastChosenService.set("s");
  const all = bundle.mount(bundle.LogsSize, { target, props: { isAllLogs: true } });
  const one = bundle.mount(bundle.LogsSize, { target });
  await settle();
  assert.deepEqual(failures.map(String), [], "a signed-out size request threw");

  const sizes = () => [...target.querySelectorAll(".title")].map((h) => h.textContent.trim());
  assert.deepEqual(sizes(), ["0 MiB", "0 MiB"]);

  // A refused request keeps the last known size.
  bundle.unmount(all);
  bundle.unmount(one);
  sizeResponse = () => jsonResponse({ sizeMiB: 12 });
  const shown = bundle.mount(bundle.LogsSize, { target });
  await settle();
  assert.deepEqual(sizes(), ["12 MiB"]);
  sizeResponse = () => jsonResponse({ error: "Only admin can perform this request" }, 403);
  bundle.lastChosenService.set("t");
  await settle();
  assert.deepEqual(sizes(), ["12 MiB"], "an error response replaced the size");
  assert.deepEqual(failures.map(String), []);

  bundle.unmount(shown);
  console.log("LogsSize tests passed");
});
