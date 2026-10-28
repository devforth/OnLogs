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
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

main(async () => {
  const { sockets } = installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set("prod-1");
  bundle.lastChosenService.set("api");

  // A view torn down while it is still opening, as the redirect after login does.
  for (let ticks = 0; ticks < 6; ticks++) {
    const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
    for (let i = 0; i < ticks; i++) await Promise.resolve();
    bundle.unmount(view);
    await settle(20);
    const open = sockets.filter((s) => s.readyState !== 3 && s.readyState !== 2);
    assert.equal(open.length, 0, `a view destroyed after ${ticks} microtask(s) left ${open.length} stream(s) open`);
  }

  console.log("NewLogsV2 unmount tests passed");
});
