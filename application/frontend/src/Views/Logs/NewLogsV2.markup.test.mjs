import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const NGINX_504 =
  "<html>\r\n<head><title>504 Gateway Time-out</title></head>\r\n<body>\r\n" +
  "<center><h1>504 Gateway Time-out</h1></center>\r\n<hr><center>nginx</center>\r\n" +
  "</body>\r\n</html>";

const MARKUP_LINES = [
  NGINX_504,
  'upstream said <img src=x onerror="globalThis.markupRan = true">',
  "\u001b[31m<b>coloured</b>\u001b[0m <script>globalThis.markupRan = true</script>",
  '<i>before</i> {"body":"<h1>in json</h1>"} <u>after</u>',
];

const SHOWN_AS_TEXT = [
  "<h1>504 Gateway Time-out</h1>",
  "<img src=x onerror=",
  "<b>coloured</b>",
  "<script>",
  "<i>before</i>",
  "<u>after</u>",
  "<h1>in json</h1>",
];

const RENDERED_TAGS = new Set(["P", "PRE", "SPAN"]);

let seq = 0;
function freshRow(message) {
  seq += 1;
  const ts = `2026-02-10T12:00:00.${String(seq).padStart(9, "0")}Z`;
  return [ts, message, `${ts} +1-${seq}`];
}
const freshPage = () => MARKUP_LINES.map(freshRow);

function stubFetch(counter) {
  return async (url) => {
    const target = String(url);
    if (
      target.includes("getLogWithPrev?") ||
      target.includes("getPrevLogs?") ||
      target.includes("getLogs?")
    ) {
      if (target.includes("getLogWithPrev?")) counter.withPrev += 1;
      const logs = freshPage();
      return jsonResponse({
        logs,
        last_processed_key: logs.at(-1)[2],
        is_end: true,
      });
    }
    return jsonResponse({ error: null });
  };
}

function assertInert(root, scope, label) {
  const messages = [...root.querySelectorAll(`${scope} .message`)];
  assert.ok(
    messages.length >= MARKUP_LINES.length,
    `${label}: only ${messages.length} log rows rendered`
  );
  for (const el of root.querySelectorAll(`${scope} .message *`)) {
    assert.ok(
      RENDERED_TAGS.has(el.tagName),
      `${label}: log text became a <${el.tagName.toLowerCase()}> element`
    );
    for (const attr of el.attributes) {
      assert.ok(
        !attr.name.startsWith("on"),
        `${label}: log text added a ${attr.name} handler`
      );
    }
  }
  const text = messages.map((el) => el.textContent).join("\n");
  for (const fragment of SHOWN_AS_TEXT) {
    assert.ok(text.includes(fragment), `${label}: ${fragment} is not shown as text`);
  }
  assert.equal(globalThis.markupRan, undefined, `${label}: log text ran script`);
}

installDom({});
const bundle = await importBundle(await bundleComponent("test/entry.js"));

function mountView(hash) {
  const counter = { withPrev: 0 };
  const { window, sockets } = installDom({ fetchImpl: stubFetch(counter) });
  bundle.lastChosenHost.set("testhost");
  bundle.lastChosenService.set("testservice");
  bundle.isPending.set(false);
  bundle.urlHash.set(hash);
  const target = window.document.body;
  const component = bundle.mount(bundle.NewLogsV2, { target });
  return { window, sockets, target, component, counter };
}

main(async () => {
  const view = mountView("");
  const { window, target } = view;
  await settle(40);

  assertInert(target, "#logs", "list with JSON transform");
  assert.ok(target.querySelector("#logs .message pre"), "the JSON line was not transformed");

  bundle.store.update((s) => ({ ...s, transformJson: false }));
  await settle(5);
  assert.equal(target.querySelector("#logs .message pre"), null);
  assertInert(target, "#logs", "list without JSON transform");
  bundle.store.update((s) => ({ ...s, transformJson: true }));
  await settle(5);

  const bottom = globalThis.__onlogsObservers.find((o) => o.element?.id === "endOfLogs");
  assert.ok(bottom, "the end-of-logs observer was not registered");
  bottom.set(true);
  await settle(5);
  const live = freshRow("live tail <h2>pushed</h2> <img src=x onerror=1>");
  view.sockets.at(-1).onmessage({ data: JSON.stringify(live) });
  await settle(10);
  const liveRow = [...target.querySelectorAll("#logs .message")].find((el) =>
    el.textContent.includes("live tail")
  );
  assert.ok(liveRow, "the live line was not rendered");
  assert.ok(liveRow.textContent.includes("<h2>pushed</h2>"), "live markup not shown as text");
  assertInert(target, "#logs", "live tail");
  bottom.set(false);
  await settle(5);

  const search = target.querySelector(".search input");
  search.value = "<h1>504 Gateway";
  search.dispatchEvent(new window.Event("input", { bubbles: true }));
  await new Promise((resolve) => setTimeout(resolve, 800));
  await settle(40);

  const hits = [...target.querySelectorAll("#logs .searchedText")];
  assert.ok(hits.length, "search found nothing to highlight");
  assert.ok(
    hits.every((el) => el.textContent === "<h1>504 Gateway"),
    "search highlighting did not keep the matched markup as text"
  );
  assertInert(target, "#logs", "search highlighting");

  target.querySelector("#logs .contextButtonThumb").click();
  await settle(40);
  assert.ok(target.querySelector("#logContext"), "the context view did not open");
  assert.ok(
    target.querySelector("#logContext .searchedText"),
    "the context view did not highlight the search"
  );
  assertInert(target, "#logContext", "context view");
  bundle.unmount(view.component);

  const linked = mountView("#2026-02-10T12:00:00.000000001Z");
  await settle(60);
  assert.ok(linked.counter.withPrev > 0, "the shared-link fetch never ran");
  assertInert(linked.target, "#logs", "shared link");
  bundle.unmount(linked.component);

  console.log("log markup renders as text in every view");
});
