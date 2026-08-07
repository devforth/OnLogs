// listData defaults to [], and the initial-active lookup indexed [0] with no
// guard, so rendering the component with an empty list threw.
import assert from "node:assert/strict";
import { bundleComponent, installDom, importBundle, settle } from "../../../test/harness.mjs";

async function run() {
  const outfile = await bundleComponent("test/entry.js");
  const { window } = installDom({});
  const bundle = await importBundle(outfile);
  const target = window.document.body;

  const empty = bundle.mount(bundle.CommonList, { target, props: { listData: [] } });
  await settle(2);
  console.log(`  empty list rendered ${target.querySelectorAll("li").length} row(s)`);
  assert.equal(target.querySelectorAll("li").length, 0);
  bundle.unmount(empty);

  const filled = bundle.mount(bundle.CommonList, {
    target,
    props: { listData: [{ name: "alpha", ico: "X", callBack: () => {} }] },
  });
  await settle(2);
  assert.ok(target.textContent.includes("alpha"), "a populated list should still render");
  assert.ok(
    target.querySelector(".highlightedOverlay.active"),
    "the first row should start active"
  );
  bundle.unmount(filled);

  console.log("CommonList empty-list tests passed");
  process.exit(0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
