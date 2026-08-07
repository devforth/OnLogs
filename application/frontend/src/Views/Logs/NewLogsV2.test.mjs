// The reported duplication bug, asserted against the real component.
//
// NewLogsV2 is conditionally rendered, so Svelte runs all three of its $: blocks
// in the same flush at init. Each one calls getFullLogsSet(), which appends to
// allLogs instead of owning it, so one page of rows renders three times.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
} from "../../../test/harness.mjs";

const PAGE = [
  ["2026-02-10T12:44:03.560000000Z", "ONLOGS: Container listening started!"],
  ["2026-02-10T12:44:07.370000000Z", "a"],
  ["2026-02-10T12:44:07.380000000Z", "b"],
];

function stubFetch(counter) {
  return async (url) => {
    const target = String(url);
    if (target.includes("getLogs?")) {
      counter.getLogs += 1;
      return jsonResponse({
        // The component reverses this array in place, so hand out a fresh copy.
        logs: PAGE.map((row) => [...row]),
        last_processed_key: PAGE[0][0],
        is_end: true,
      });
    }
    if (target.includes("getPrevLogs?") || target.includes("getLogWithPrev?")) {
      return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
    }
    return jsonResponse({ error: null });
  };
}

// A run of non-matching keys makes the backend return zero rows with
// is_end:false and last_processed_key:"", which resets the cursor to the newest
// row. The client must stop, not re-request the same empty page forever.
const RUNAWAY_CAP = 40;

function stubEmptyPageFetch(counter) {
  return async (url) => {
    const target = String(url);
    if (target.includes("getLogs?")) {
      counter.getLogs += 1;
      if (counter.getLogs > RUNAWAY_CAP) {
        // Break the loop so the test reports a count instead of hanging.
        return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
      }
      return jsonResponse({ logs: [], last_processed_key: "", is_end: false });
    }
    if (target.includes("getPrevLogs?") || target.includes("getLogWithPrev?")) {
      return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
    }
    return jsonResponse({ error: null });
  };
}

// One transient 502 used to leave a permanent spinner, infinite scroll dead in
// both directions, and the websocket connected but every line silently dropped.
function stubFailingFetch(counter) {
  return async (url) => {
    const target = String(url);
    if (target.includes("getLogs?")) {
      counter.getLogs += 1;
      return { status: 502, ok: false, json: async () => ({}) };
    }
    return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  };
}

function renderedRowCount(target) {
  return target.querySelectorAll(".chosenString").length;
}

function renderedTimestamps(target) {
  return [...target.querySelectorAll(".time p")]
    .map((el) => el.textContent.trim())
    .filter(Boolean);
}

async function mount(bundle, fetchImpl, counter) {
  const { window } = installDom({ fetchImpl });
  const { NewLogsV2, lastChosenHost, lastChosenService, isPending } = bundle;

  // A remount with the stores already populated — Logs -> Stats -> Logs. This is
  // the trigger from the ticket, and it fires all three reactive blocks at once.
  lastChosenHost.set("testhost");
  lastChosenService.set("testservice");
  isPending.set(false);

  const target = window.document.body;
  const component = new NewLogsV2({ target });
  await settle();
  return { component, target, counter };
}

async function run() {
  const bundlePath = await bundleComponent("test/entry.js");

  const counter = { getLogs: 0 };
  installDom({ fetchImpl: stubFetch(counter) });
  const bundle = await importBundle(bundlePath);

  const { component, target } = await mount(bundle, stubFetch(counter), counter);

  const rows = renderedRowCount(target);
  const timestamps = renderedTimestamps(target);

  console.log(`  backend returned ${PAGE.length} rows in ${counter.getLogs} request(s)`);
  console.log(`  component rendered ${rows} rows`);
  for (const t of timestamps) console.log(`    ${t}`);

  assert.equal(
    rows,
    PAGE.length,
    `the backend returned ${PAGE.length} rows but the view rendered ${rows} ` +
      `(${(rows / PAGE.length).toFixed(1)}x duplication)`
  );

  // Every rendered timestamp must be distinct: a duplicate is the same stored
  // row drawn twice, which is exactly the screenshot on the ticket.
  assert.equal(
    new Set(timestamps).size,
    timestamps.length,
    `duplicate rows on screen: ${JSON.stringify(timestamps)}`
  );

  component.$destroy();

  // --- an empty page must end the load, not restart it ---
  const emptyCounter = { getLogs: 0 };
  const empty = await mount(
    bundle,
    stubEmptyPageFetch(emptyCounter),
    emptyCounter
  );

  console.log(`  empty-page load issued ${emptyCounter.getLogs} request(s)`);
  assert.ok(
    emptyCounter.getLogs < RUNAWAY_CAP,
    `an empty page with is_end:false re-requested the same page ` +
      `${emptyCounter.getLogs} times; the cursor resets to the newest row and ` +
      `the client never terminates`
  );
  assert.equal(
    renderedRowCount(empty.target),
    0,
    "an empty result set should render no rows"
  );
  empty.component.$destroy();

  // --- a failed request must not wedge the view ---
  const failingCounter = { getLogs: 0 };
  const failing = await mount(
    bundle,
    stubFailingFetch(failingCounter),
    failingCounter
  );

  const pending = bundle.isPending;
  const searching = bundle.isSearching;
  let pendingValue;
  let searchingValue;
  pending.subscribe((v) => (pendingValue = v))();
  searching.subscribe((v) => (searchingValue = v))();

  console.log(`  after a 502: isPending=${pendingValue} isSearching=${searchingValue}`);
  assert.equal(pendingValue, false, "a failed request left the spinner up forever");
  assert.equal(searchingValue, false, "a failed request left the loader up forever");
  failing.component.$destroy();

  // --- destroying the view must not leave a zombie driving the shared stores ---
  const zombieCounter = { getLogs: 0 };
  const { window: zombieWindow, sockets } = installDom({
    fetchImpl: stubFetch(zombieCounter),
  });
  bundle.lastChosenHost.set("testhost");
  bundle.lastChosenService.set("testservice");
  bundle.isPending.set(false);
  const zombie = new bundle.NewLogsV2({ target: zombieWindow.document.body });
  await settle();

  assert.ok(sockets.length > 0, "the view never opened a websocket");
  zombie.$destroy();
  await settle(5);

  assert.ok(
    sockets.every((s) => s.readyState === 3),
    "destroying the view left its websocket open"
  );

  let stillFetching;
  bundle.isFeatching.subscribe((v) => (stillFetching = v))();
  assert.equal(
    stillFetching,
    false,
    "a destroyed view left the shared isFeatching store set, which blocks the live one"
  );

  console.log("NewLogsV2 duplication tests passed");
  process.exit(0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
