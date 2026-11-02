// A host or service whose name contains a page name must not pick the page.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const HOSTS = [
  {
    host: "stats-01",
    services: [
      { serviceName: "statsd", isDisabled: false, isFavorite: false },
      { serviceName: "viewer", isDisabled: false, isFavorite: false },
    ],
  },
];

async function stubFetch(url) {
  const target = String(url);
  if (target.includes("getHosts")) return jsonResponse(HOSTS);
  if (target.includes("getLogs") || target.includes("getPrevLogs") || target.includes("getLogWithPrev")) {
    return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  }
  return jsonResponse({ error: null });
}

main(async () => {
  const outfile = await bundleComponent("test/entry.js");
  installDom({ fetchImpl: stubFetch });
  const bundle = await importBundle(outfile);

  async function mountAt(path) {
    const { window } = installDom({ fetchImpl: stubFetch });
    window.history.replaceState({}, "", `${bundle.changeKey}${path}`);
    bundle.lastChosenHost.set("stats-01");
    bundle.lastChosenService.set(path.split("/").pop());
    const app = bundle.mount(bundle.App, {
      target: window.document.body,
      props: { url: `${bundle.changeKey}${path}` },
    });
    await settle(20);
    return { app, document: window.document, window };
  }

  const view = await mountAt("/view/stats-01/statsd");
  assert.ok(view.document.querySelector("#logs"), "the log view is missing on /view");
  assert.equal(view.document.querySelector(".chartContainer"), null, "a stats chart is drawn on /view");
  const viewer = [...view.document.querySelectorAll(".serviceListItem")].find(
    (li) => li.querySelector("p")?.textContent.trim() === "viewer"
  );
  viewer.querySelector("p").click();
  await settle(10);
  assert.equal(
    view.window.location.pathname,
    `${bundle.changeKey}/view/stats-01/viewer`,
    "a sidebar click from a 'stats' name left the log view"
  );
  bundle.unmount(view.app);

  const stats = await mountAt("/stats/stats-01/viewer");
  assert.ok(stats.document.querySelector(".chartContainer"), "the stats chart is missing on /stats");
  assert.equal(stats.document.querySelector("#logs"), null, "a log view is drawn on /stats");
  bundle.unmount(stats.app);

  console.log("page selection tests passed");
});
