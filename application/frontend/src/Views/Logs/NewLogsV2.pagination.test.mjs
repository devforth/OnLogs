// Drives every scroll interceptor in the log view and asserts the invariants
// that the duplication bug broke: the rendered window stays bounded, and no log
// line is ever on screen twice.
//
// fetchedLogs' pagination loop had an inverted comparison and so had never run;
// this exercises the newly-live path rather than shipping it unverified.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
} from "../../../test/harness.mjs";

const LIMIT = 60;

let nextIndex = 0;

// Distinct, strictly decreasing keys, as the backend emits them.
function makePage(count) {
  return Array.from({ length: count }, () => {
    const idx = nextIndex++;
    const nanos = String(999999999 - idx * 1000).padStart(9, "0");
    return [`2026-02-10T12:44:03.${nanos}Z`, `line-${idx}`];
  });
}

function limitFromUrl(url) {
  const match = /[?&]limit=(\d+)/.exec(String(url));
  return match ? Number(match[1]) : LIMIT;
}

function stubFetch(counter) {
  return async (url) => {
    const target = String(url);
    if (target.includes("getLogs?")) {
      counter.getLogs += 1;
      // Honour the requested limit, as a real backend does.
      const rows = makePage(limitFromUrl(target));
      return jsonResponse({
        logs: rows,
        last_processed_key: rows.at(-1)[0],
        is_end: false,
      });
    }
    // Nothing newer than what is already on screen.
    return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  };
}

function renderedMessages(target) {
  return [...target.querySelectorAll(".message p")]
    .map((el) => el.textContent.trim())
    .filter((text) => text.startsWith("line-"));
}

function assertInvariants(target, label) {
  const messages = renderedMessages(target);
  const unique = new Set(messages);

  assert.equal(
    unique.size,
    messages.length,
    `${label}: the same log line is on screen more than once ` +
      `(${messages.length} rows, ${unique.size} distinct)`
  );
  assert.ok(
    messages.length <= 3 * LIMIT,
    `${label}: the rendered window grew past 3x the page size ` +
      `(${messages.length} rows); it should slide, not accumulate`
  );
  return messages;
}

async function run() {
  const counter = { getLogs: 0 };
  const { window } = installDom({ fetchImpl: stubFetch(counter) });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));

  bundle.lastChosenHost.set("testhost");
  bundle.lastChosenService.set("testservice");
  bundle.isPending.set(false);

  const target = window.document.body;
  const component = bundle.mount(bundle.NewLogsV2, { target });

  await settle(40);
  // The view only arms its interceptors once initialScroll flips, on a 1s timer.
  await new Promise((resolve) => setTimeout(resolve, 1400));
  await settle(10);

  const initial = assertInvariants(target, "initial load");
  console.log(`  initial load: ${initial.length} rows, ${counter.getLogs} request(s)`);
  assert.equal(initial.length, 3 * LIMIT, "expected three pages on screen");

  // Fire every interceptor the view registered, one at a time.
  const observers = globalThis.__onlogsObservers || [];
  assert.ok(observers.length > 0, "no interceptors were registered");

  let triggered = 0;
  for (let i = 0; i < observers.length; i++) {
    const before = counter.getLogs;
    observers[i].set(true);
    await settle(10);
    observers[i].set(false);
    await settle(5);

    if (counter.getLogs !== before) {
      triggered += 1;
      const rows = assertInvariants(target, `after interceptor ${i}`);
      console.log(
        `  interceptor ${i}: +${counter.getLogs - before} request(s), ${rows.length} rows`
      );
    }
  }

  assert.ok(
    triggered > 0,
    "no interceptor triggered a fetch, so upward pagination was not exercised"
  );

  const finalRows = assertInvariants(target, "final");
  console.log(`  after ${triggered} interceptor firing(s): ${finalRows.length} rows`);

  bundle.unmount(component);
  console.log("NewLogsV2 pagination tests passed");
  process.exit(0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
