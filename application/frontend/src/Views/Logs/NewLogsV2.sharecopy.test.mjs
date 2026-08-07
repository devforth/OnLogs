// The share button copies a link. It used to also rebuild the whole view --
// resetting the loaded logs, reopening the websocket and refetching -- which
// looked like the page reloading under you.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
} from "../../../test/harness.mjs";

const ROWS = Array.from({ length: 90 }, (_, i) => {
  const nanos = String(900000000 - i * 1000).padStart(9, "0");
  const ts = `2026-02-10T12:44:03.${nanos}Z`;
  return [ts, `line-${i}`, `${ts} +1786123141845584283-${9000 - i}`];
});

function stubFetch(counter) {
  return async (url) => {
    const target = String(url);
    if (target.includes("getLogWithPrev?")) {
      counter.withPrev += 1;
      return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
    }
    if (target.includes("getPrevLogs?")) {
      counter.getPrevLogs += 1;
      return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
    }
    if (target.includes("getLogs?")) {
      counter.getLogs += 1;
      const limit = Number(/[?&]limit=(\d+)/.exec(target)?.[1] || 60);
      const rows = ROWS.slice(0, limit).map((r) => [...r]);
      return jsonResponse({
        logs: rows,
        last_processed_key: rows.at(-1)[2],
        is_end: false,
      });
    }
    return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  };
}

function renderedMessages(target) {
  return [...target.querySelectorAll(".message p")]
    .map((el) => el.textContent.trim())
    .filter((text) => text.startsWith("line-"));
}

async function run() {
  const counter = { getLogs: 0, getPrevLogs: 0, withPrev: 0 };
  const { window } = installDom({ fetchImpl: stubFetch(counter) });

  let copied = null;
  Object.defineProperty(window.navigator, "clipboard", {
    configurable: true,
    value: { writeText: async (text) => { copied = text; } },
  });
  globalThis.navigator = window.navigator;

  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  bundle.lastChosenHost.set("testhost");
  bundle.lastChosenService.set("testservice");
  bundle.isPending.set(false);
  bundle.urlHash.set("");

  const target = window.document.body;
  const component = bundle.mount(bundle.NewLogsV2, { target });
  await settle(40);
  await new Promise((resolve) => setTimeout(resolve, 1200));
  await settle(10);

  const before = {
    rows: renderedMessages(target),
    requests: counter.getLogs + counter.getPrevLogs + counter.withPrev,
  };
  assert.ok(before.rows.length > 0, "no log rows rendered, nothing to share");

  const shareButton = target.querySelector(".shareLinkButtonThumb");
  assert.ok(shareButton, "the share-link button did not render");
  shareButton.click();
  await settle(40);

  const after = {
    rows: renderedMessages(target),
    requests: counter.getLogs + counter.getPrevLogs + counter.withPrev,
  };
  let pending = false;
  bundle.isPending.subscribe((v) => (pending = v))();

  console.log(`  copied: ${copied}`);
  console.log(
    `  requests before/after: ${before.requests}/${after.requests}, ` +
      `rows before/after: ${before.rows.length}/${after.rows.length}`
  );

  assert.ok(copied, "clicking the share button copied nothing");
  assert.ok(
    copied.includes("#"),
    `the copied link carries no log anchor: ${copied}`
  );

  assert.equal(
    after.requests,
    before.requests,
    "sharing a link refetched logs; copying must not reload the view"
  );
  assert.deepEqual(
    after.rows,
    before.rows,
    "sharing a link changed the rendered rows; copying must leave the view alone"
  );
  assert.equal(pending, false, "sharing a link put the view back into its loading state");

  bundle.unmount(component);
  console.log("share-link copy tests passed");
  process.exit(0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
