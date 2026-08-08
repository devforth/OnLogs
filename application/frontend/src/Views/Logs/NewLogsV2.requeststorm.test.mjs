// A sentinel that stays in view must cost at most one request per endpoint once
// the backend has nothing left to give.
//
// The backend models the established contract: getLogs pages toward OLDER rows
// and returns them newest-first, getPrevLogs pages toward NEWER rows. The corpus
// is exactly one rendered window, so both directions are already exhausted.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const LIMIT = 60;
const CORPUS = 3 * LIMIT;

// index 0 = newest. The third element is the full storage key logKey() reads.
const ROWS = Array.from({ length: CORPUS }, (_, i) => {
  const nanos = String(999999999 - i * 1000).padStart(9, "0");
  const ts = `2026-08-07T16:14:16.${nanos}Z`;
  return [ts, `line-${i}`, `${ts} +1786119256365958477-${44159 - i}`];
});
const INDEX_OF = new Map(ROWS.map((row, i) => [row[2], i]));

const counter = { getLogs: 0, getPrevLogs: 0 };
const seen = { getLogs: new Set(), getPrevLogs: new Set() };

const fetchImpl = async (url) => {
  const target = String(url);
  const params = new URL(target, "http://onlogs.test").searchParams;
  const limit = Number(params.get("limit")) || LIMIT;
  const startWith = params.get("startWith") || "";

  if (target.includes("getPrevLogs?")) {
    counter.getPrevLogs += 1;
    seen.getPrevLogs.add(target);
    const i = INDEX_OF.get(startWith) ?? 0;
    const rows = ROWS.slice(Math.max(0, i - limit), i).reverse();
    return jsonResponse({
      logs: rows,
      last_processed_key: rows.at(-1)?.[2] ?? startWith,
      is_end: i - limit <= 0,
    });
  }

  if (target.includes("getLogs?")) {
    counter.getLogs += 1;
    seen.getLogs.add(target);
    const start = startWith ? (INDEX_OF.get(startWith) ?? -1) + 1 : 0;
    const rows = ROWS.slice(start, start + limit);
    return jsonResponse({
      logs: rows,
      last_processed_key: rows.at(-1)?.[2] ?? startWith,
      is_end: start + limit >= CORPUS,
    });
  }

  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
};

function renderedMessages(target) {
  return [...target.querySelectorAll(".message p")]
    .map((el) => el.textContent.trim())
    .filter((text) => text.startsWith("line-"));
}

// Holds each sentinel in turn until one of them fetches, and leaves it held —
// which is what a real IntersectionObserver does while the sentinel is on screen.
async function holdTheFetchingSentinel(endpoint) {
  const observers = globalThis.__onlogsObservers || [];
  assert.ok(observers.length > 0, "no interceptors were registered");

  for (const handle of observers) {
    const before = counter[endpoint];
    handle.set(true);
    await settle(6);
    if (counter[endpoint] > before) {
      return handle;
    }
    handle.set(false);
    await settle(2);
  }
  assert.fail(`no interceptor triggered ${endpoint}`);
}

async function idleRequests(endpoint, rounds) {
  const before = counter[endpoint];
  await settle(rounds);
  return counter[endpoint] - before;
}

main(async () => {
  const { window } = installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));

  bundle.lastChosenHost.set("rmalenko-XPS-15-9530");
  bundle.lastChosenService.set("crash-app");
  bundle.isPending.set(false);

  const target = window.document.body;
  const component = bundle.mount(bundle.NewLogsV2, { target });

  await settle(40);
  // The interceptors only arm once initialScroll flips, on a 1s timer.
  await new Promise((resolve) => setTimeout(resolve, 1400));
  await settle(10);

  const rows = renderedMessages(target);
  assert.equal(rows.length, CORPUS, "expected three pages on screen");
  // getFullLogsSet reverses each newest-first page, so the window is oldest-first.
  assert.equal(rows.at(0), `line-${CORPUS - 1}`);
  assert.equal(rows.at(-1), "line-0");

  // Scrolling up: fetchedLogs asks getLogs for rows older than the oldest one.
  const upSentinel = await holdTheFetchingSentinel("getLogs");
  const idleGetLogs = await idleRequests("getLogs", 60);
  console.log(
    `  getLogs while the sentinel sat still: +${idleGetLogs} request(s) over 60 idle ticks`
  );
  upSentinel.set(false);
  await settle(5);

  // Scrolling down: unfetchedLogs asks getPrevLogs for rows newer than the newest.
  const logsEl = window.document.querySelector("#logs");
  Object.defineProperty(logsEl, "scrollTop", { value: 500, configurable: true });
  logsEl.dispatchEvent(new window.Event("scroll"));
  await settle(3);

  const downSentinel = await holdTheFetchingSentinel("getPrevLogs");
  const idleGetPrevLogs = await idleRequests("getPrevLogs", 60);
  console.log(
    `  getPrevLogs while the sentinel sat still: +${idleGetPrevLogs} request(s) over 60 idle ticks`
  );
  downSentinel.set(false);
  await settle(5);

  assert.equal(
    renderedMessages(target).length,
    CORPUS,
    "empty responses must leave the rendered window alone"
  );

  bundle.unmount(component);

  assert.ok(
    idleGetLogs === 0 && idleGetPrevLogs === 0,
    `request storm: the view kept fetching with nothing changing — ` +
      `getLogs +${idleGetLogs}, getPrevLogs +${idleGetPrevLogs} over 60 idle ticks ` +
      `(${counter.getLogs} getLogs / ${seen.getLogs.size} distinct, ` +
      `${counter.getPrevLogs} getPrevLogs / ${seen.getPrevLogs.size} distinct)`
  );

  console.log("NewLogsV2 request-storm tests passed");
});
