// Opening a shared log link must land on the linked row, not on the newest page.
// NewLogsV2 runs all three of its reactive blocks in one flush, so the deep-link
// fetch and two full reloads start together and the last writer wins.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
} from "../../../test/harness.mjs";

const LINKED = "2026-02-10T09:00:05.000000000Z";

const WINDOW_ROWS = [
  [LINKED, "linked-row", LINKED + " +1-1"],
  ["2026-02-10T09:00:04.000000000Z", "window-older", "2026-02-10T09:00:04.000000000Z +1-2"],
];
const NEWEST_ROWS = [
  ["2026-02-10T18:00:00.000000000Z", "newest-page-row", "2026-02-10T18:00:00.000000000Z +1-3"],
];

function stubFetch(counter, latency) {
  return async (url) => {
    const target = String(url);
    if (latency) {
      await new Promise((resolve) => setTimeout(resolve, latency));
    }
    if (target.includes("getLogWithPrev?")) {
      counter.withPrev += 1;
      return jsonResponse({
        logs: WINDOW_ROWS.map((r) => [...r]),
        last_processed_key: WINDOW_ROWS.at(-1)[2],
        is_end: true,
      });
    }
    if (target.includes("getPrevLogs?")) {
      return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
    }
    if (target.includes("getLogs?")) {
      counter.getLogs += 1;
      return jsonResponse({
        logs: NEWEST_ROWS.map((r) => [...r]),
        last_processed_key: NEWEST_ROWS[0][2],
        is_end: true,
      });
    }
    return jsonResponse({ error: null });
  };
}

function renderedMessages(target) {
  return [...target.querySelectorAll(".message p")]
    .map((el) => el.textContent.trim())
    .filter(Boolean);
}

// Set up the DOM before the bundle is imported: fetch.js reads document.location
// at construction.
installDom({});
const bundle = await importBundle(await bundleComponent("test/entry.js"));

function openDeepLink(latency) {
  const counter = { getLogs: 0, withPrev: 0 };
  const { window } = installDom({ fetchImpl: stubFetch(counter, latency) });

  bundle.lastChosenHost.set("testhost");
  bundle.lastChosenService.set("testservice");
  bundle.isPending.set(false);
  // App.svelte publishes the hash before this component mounts.
  bundle.urlHash.set("#" + LINKED);

  const target = window.document.body;
  const component = bundle.mount(bundle.NewLogsV2, { target });
  return { component, target, counter };
}

async function run() {
  // The deep link must win regardless of how the three concurrent loads
  // interleave, so exercise a range of response latencies.
  for (const latency of [0, 5, 20]) {
    const opened = openDeepLink(latency);
    await settle(latency ? 120 : 40);

    const seen = renderedMessages(opened.target);
    console.log(
      `  latency ${latency}ms -> ${JSON.stringify(seen)} (getLogWithPrev ${opened.counter.withPrev}, getLogs ${opened.counter.getLogs})`
    );

    assert.ok(
      opened.counter.withPrev > 0,
      `latency ${latency}ms: the deep-link fetch never ran`
    );
    assert.ok(
      seen.includes("linked-row"),
      `latency ${latency}ms: the linked row is not on screen: ${JSON.stringify(seen)}`
    );
    assert.ok(
      !seen.includes("newest-page-row"),
      `latency ${latency}ms: the newest page overwrote the deep-link window: ${JSON.stringify(seen)}`
    );

    bundle.unmount(opened.component);
  }

  console.log("shared link deep-link window tests passed");
  process.exit(0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
