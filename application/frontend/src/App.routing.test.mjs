// svelte-routing tells a component apart from a lazy loader with
// `c.toString().startsWith("class ")`. Svelte 5 compiles components to
// functions, so `<Route component={X} />` called X instead of rendering it.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../test/harness.mjs";

async function stubFetch(url) {
  const target = String(url);
  if (
    target.includes("getLogs") ||
    target.includes("getPrevLogs") ||
    target.includes("getLogWithPrev")
  ) {
    return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  }
  return jsonResponse({ error: null });
}

const NOT_FOUND = "404 This is not the web page";

main(async () => {
  // changeKey.js reads `location` at module scope, so the DOM has to exist
  // before the bundle is imported.
  const outfile = await bundleComponent("test/entry.js");
  installDom({ fetchImpl: stubFetch });
  const bundle = await importBundle(outfile);

  async function mountAt(path) {
    const { window } = installDom({ fetchImpl: stubFetch });
    const target = window.document.body;
    const app = bundle.mount(bundle.App, {
      target,
      props: { url: `${bundle.changeKey}${path}` },
    });
    await settle();
    return { app, target };
  }

  const matched = await mountAt("/view/test-host/test-service");
  const matchedText = matched.target.textContent;
  console.log(`  /view/:host/:service rendered 404: ${matchedText.includes(NOT_FOUND)}`);
  assert.ok(
    !matchedText.includes(NOT_FOUND),
    "/view/:host/:service fell through to the default route"
  );
  assert.ok(
    matched.target.querySelector(".contentContainer"),
    "the matched route rendered nothing"
  );
  bundle.unmount(matched.app);

  const unmatched = await mountAt("/no-such-page");
  assert.ok(
    unmatched.target.textContent.includes(NOT_FOUND),
    "the default route did not render for an unmatched path"
  );
  bundle.unmount(unmatched.app);

  console.log("App routing tests passed");
});
