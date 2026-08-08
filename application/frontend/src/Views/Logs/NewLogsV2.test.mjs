import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
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

function stubOverlappingFetch(counter) {
  const rows = [
    ["2026-02-10T12:44:03.560000000Z", "row-a", "2026-02-10T12:44:03.560000000Z +1-1"],
    ["2026-02-10T12:44:03.561000000Z", "row-b", "2026-02-10T12:44:03.561000000Z +1-2"],
  ];
  return async (url) => {
    const target = String(url);
    if (target.includes("getLogs?")) {
      counter.getLogs += 1;
      return jsonResponse({
        logs: rows.map((r) => [...r]),
        last_processed_key: rows.at(-1)[2],
        is_end: false,
      });
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

async function mountView(bundle, fetchImpl, counter) {
  const { window } = installDom({ fetchImpl });
  const { NewLogsV2, lastChosenHost, lastChosenService, isPending } = bundle;

  lastChosenHost.set("testhost");
  lastChosenService.set("testservice");
  isPending.set(false);

  const target = window.document.body;
  const component = bundle.mount(NewLogsV2, { target });
  await settle();
  return { component, target, counter };
}

main(async () => {
  const bundlePath = await bundleComponent("test/entry.js");

  const counter = { getLogs: 0 };
  installDom({ fetchImpl: stubFetch(counter) });
  const bundle = await importBundle(bundlePath);

  const { component, target } = await mountView(bundle, stubFetch(counter), counter);

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

  assert.equal(
    new Set(timestamps).size,
    timestamps.length,
    `duplicate rows on screen: ${JSON.stringify(timestamps)}`
  );

  bundle.unmount(component);

  // --- an empty page must end the load, not restart it ---
  const emptyCounter = { getLogs: 0 };
  const empty = await mountView(
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
  bundle.unmount(empty.component);

  // --- a failed request must not wedge the view ---
  const failingCounter = { getLogs: 0 };
  const failing = await mountView(
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
  bundle.unmount(failing.component);

  // --- destroying the view must not leave a zombie driving the shared stores ---
  const zombieCounter = { getLogs: 0 };
  const { window: zombieWindow, sockets } = installDom({
    fetchImpl: stubFetch(zombieCounter),
  });
  bundle.lastChosenHost.set("testhost");
  bundle.lastChosenService.set("testservice");
  bundle.isPending.set(false);
  const zombie = bundle.mount(bundle.NewLogsV2, { target: zombieWindow.document.body });
  await settle();

  assert.ok(sockets.length > 0, "the view never opened a websocket");
  bundle.unmount(zombie);
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

  // --- a re-served row must not reach the screen twice ---
  const overlapCounter = { getLogs: 0 };
  const overlap = await mountView(
    bundle,
    stubOverlappingFetch(overlapCounter),
    overlapCounter
  );
  const overlapMessages = [...overlap.target.querySelectorAll(".message p")]
    .map((el) => el.textContent.trim())
    .filter((t) => t.startsWith("row-"));

  console.log(`  overlapping pages rendered: ${JSON.stringify(overlapMessages)}`);
  assert.equal(
    new Set(overlapMessages).size,
    overlapMessages.length,
    `a re-served row reached the screen twice: ${JSON.stringify(overlapMessages)}`
  );
  bundle.unmount(overlap.component);

  // --- a status filter must not silently freeze the live tail ---
  const wsCounter = { getLogs: 0 };
  const { window: wsWindow, sockets: wsSockets } = installDom({
    fetchImpl: stubFetch(wsCounter),
  });
  bundle.lastChosenHost.set("testhost");
  bundle.lastChosenService.set("testservice");
  bundle.isPending.set(false);
  const filtered = bundle.mount(bundle.NewLogsV2, { target: wsWindow.document.body });
  await settle();

  bundle.chosenStatus.set("warn");
  await settle(5);

  const socket = wsSockets.at(-1);
  assert.ok(socket, "no websocket was opened");

  // Nothing sets endOffLogsIntersect under jsdom, so a delivered line lands in
  // the buffer and shows on the scroll-to-bottom badge.
  const bufferedCount = () => {
    const badge = wsWindow.document.querySelector(".buttonToBottomNumber p");
    return badge ? Number(badge.textContent.trim()) : 0;
  };

  // A line the shared classifier calls "warn", not a bare leading token.
  socket.onmessage({
    data: JSON.stringify([
      "2026-02-10T12:44:09.000000000Z",
      "2026-02-10 WARNING disk almost full",
      "2026-02-10T12:44:09.000000000Z +9-9",
    ]),
  });
  await settle(5);

  const delivered = bufferedCount();
  console.log(`  live warn line with a "warn" filter delivered: ${delivered > 0}`);
  assert.ok(
    delivered > 0,
    "a live line the classifier calls warn was dropped while the warn filter was active"
  );
  bundle.chosenStatus.set("");
  bundle.unmount(filtered);

  console.log("NewLogsV2 duplication tests passed");
});
