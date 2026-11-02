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

  sizeResponse = () => jsonResponse({ sizeMiB: 0.1 });
  bundle.logsCleared.update((n) => n + 1);
  await settle();
  assert.deepEqual(sizes(), ["0.1 MiB"], "the size stayed stale after Clear logs");

  bundle.unmount(shown);

  // Switching service or leaving the page while a size request is out must not
  // leave a 30 s poller behind.
  const live = new Set();
  const realSet = globalThis.setInterval;
  const realClear = globalThis.clearInterval;
  globalThis.setInterval = (fn, ms, ...rest) => {
    const id = realSet(fn, ms, ...rest);
    if (ms === 30000) live.add(id);
    return id;
  };
  globalThis.clearInterval = (id) => {
    live.delete(id);
    return realClear(id);
  };
  try {
    sizeResponse = () => new Promise((resolve) => setTimeout(() => resolve(jsonResponse({ sizeMiB: 1 })), 30));
    const slow = bundle.mount(bundle.LogsSize, { target });
    for (const name of ["a", "b", "c", "d"]) {
      bundle.lastChosenService.set(name);
      await new Promise((resolve) => setTimeout(resolve, 5));
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
    assert.equal(live.size, 1, `${live.size} size pollers run for one panel`);
    bundle.lastChosenService.set("e");
    bundle.unmount(slow);
    await new Promise((resolve) => setTimeout(resolve, 100));
    assert.equal(live.size, 0, `${live.size} size pollers outlived the panel`);
  } finally {
    live.forEach((id) => realClear(id));
    globalThis.setInterval = realSet;
    globalThis.clearInterval = realClear;
  }
  console.log("LogsSize tests passed");
});
