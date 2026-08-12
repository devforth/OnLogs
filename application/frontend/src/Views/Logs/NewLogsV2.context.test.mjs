import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

// Index 0 is the newest row, so a bigger index means an older line.
const ROWS = Array.from({ length: 90 }, (_, i) => {
  const nanos = String(900000000 - i * 1000).padStart(9, "0");
  const ts = `2026-02-10T12:44:03.${nanos}Z`;
  return [ts, `line-${i}`, `${ts} +1786123141845584283-${9000 - i}`];
});

const indexOfKey = (key) => ROWS.findIndex((row) => row[2] === key);
const param = (target, name) =>
  new URL(target, "http://onlogs.test").searchParams.get(name);

function stubFetch(seen) {
  return async (url) => {
    const target = String(url);
    const limit = Number(param(target, "limit") || 60);
    const startWith = param(target, "startWith") || "";

    if (target.includes("getLogs?") || target.includes("getPrevLogs?")) {
      seen.push(target);
    }

    if (target.includes("getPrevLogs?")) {
      const at = indexOfKey(startWith);
      if (at < 0) {
        return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
      }
      // Newer than the cursor, oldest first.
      const rows = ROWS.slice(Math.max(0, at - limit), at)
        .reverse()
        .map((row) => [...row]);
      return jsonResponse({
        logs: rows,
        last_processed_key: rows.at(-1)?.at(2) ?? "",
        is_end: rows.length < limit,
      });
    }

    if (target.includes("getLogs?")) {
      // Newest first, which is what a descending scan returns.
      const from = startWith ? indexOfKey(startWith) + 1 : 0;
      const rows = ROWS.slice(from, from + limit).map((row) => [...row]);
      return jsonResponse({
        logs: rows,
        last_processed_key: rows.at(-1)?.at(2) ?? "",
        is_end: rows.length < limit,
      });
    }

    return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  };
}

const messagesIn = (root, selector) =>
  [...root.querySelectorAll(selector)]
    .map((el) => el.textContent.trim())
    .filter((text) => text.startsWith("line-"));

main(async () => {
  const seen = [];
  const { window } = installDom({ fetchImpl: stubFetch(seen) });

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

  const listedBefore = messagesIn(target, "#logs .message p");
  assert.ok(listedBefore.length > 0, "no log rows rendered, nothing to anchor on");

  const buttons = [...target.querySelectorAll("#logs .contextButtonThumb")];
  assert.ok(buttons.length, "the show-context button did not render");

  // A row in the middle of the loaded page, so context exists on both sides.
  const button = buttons[Math.floor(buttons.length / 2)];
  const anchorText = button
    .closest(".logsString")
    .querySelector(".message p")
    .textContent.trim();
  const anchorIndex = Number(anchorText.replace("line-", ""));

  seen.length = 0;
  button.click();
  await settle(40);

  const box = target.querySelector("#logContext");
  assert.ok(box, "clicking show context opened no context view");

  const contextRequests = seen.filter((url) => url.includes("startWith="));
  assert.ok(
    contextRequests.length >= 2,
    `context asked for ${contextRequests.length} pages, expected one per direction`
  );
  for (const url of contextRequests) {
    assert.equal(
      param(url, "search"),
      "",
      `context carried a search filter, which is what it exists to drop: ${url}`
    );
    assert.equal(
      param(url, "status"),
      "",
      `context carried a status filter: ${url}`
    );
    assert.equal(
      param(url, "startWith"),
      ROWS[anchorIndex][2],
      `context anchored on the wrong row: ${url}`
    );
  }

  const shown = messagesIn(target, "#logContext .message p");
  const expected = [];
  for (let i = anchorIndex + 20; i >= anchorIndex - 20; i--) {
    if (i >= 0 && i < ROWS.length) {
      expected.push(`line-${i}`);
    }
  }

  console.log(`  anchor: ${anchorText}`);
  console.log(`  context rows: ${shown.at(0)} .. ${shown.at(-1)} (${shown.length})`);

  assert.deepEqual(
    shown,
    expected,
    "context did not show the surrounding lines oldest-first around the anchor"
  );
  assert.equal(
    shown.filter((line) => line === anchorText).length,
    1,
    "the anchored line is missing from its own context, or shown twice"
  );
  assert.ok(
    target.querySelector(".contextAnchor .message p").textContent.trim() ===
      anchorText,
    "the anchored line is not the one marked in the context view"
  );

  assert.deepEqual(
    messagesIn(target, "#logs .message p"),
    listedBefore,
    "opening context changed the rows behind it; the search result must survive"
  );

  // A rendered button that does nothing is worse than no button.
  const shareInContext = box.querySelector(".shareLinkButtonThumb");
  assert.ok(shareInContext, "context rows render no share-link button");
  shareInContext.click();
  await settle(20);
  console.log(`  copied from context: ${copied}`);
  assert.ok(
    copied && copied.includes("#"),
    `the context share button copied no log anchor: ${copied}`
  );

  bundle.unmount(component);
  console.log("show-context tests passed");
});
