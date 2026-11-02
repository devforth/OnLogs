// A very long line is shown as a preview, so a page of them stays responsive.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

main(async () => {
  const { window } = installDom({ fetchImpl: async () => jsonResponse({}) });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  const doc = window.document;
  const text = () => doc.querySelector(".message").textContent;
  const toggle = () => doc.querySelector(".message .showAll");

  const long = "INFO start " + "x".repeat(130000) + " end-marker";
  const row = bundle.mount(bundle.LogsString, { target: doc.body, props: { message: long, time: "10:00:00" } });
  await settle(5);
  assert.ok(text().length < 10000, `a ${long.length}-char line put ${text().length} chars on the page`);
  assert.ok(text().includes("INFO start"), "the preview lost the start of the line");
  assert.ok(!text().includes("end-marker"));
  assert.match(toggle().textContent, /Show all \(\d+ KB more\)/);

  toggle().click();
  await settle(5);
  assert.ok(text().includes("end-marker"), "Show all did not show the whole line");
  assert.match(toggle().textContent, /Show less/);
  toggle().click();
  await settle(5);
  assert.ok(!text().includes("end-marker"));
  bundle.unmount(row);

  // A long JSON line keeps its preview raw until expanded, then is transformed.
  bundle.store.update((s) => ({ ...s, transformJson: true }));
  const json = "INFO payload " + JSON.stringify({ items: Array.from({ length: 3000 }, (_, i) => ({ id: i })) });
  const jsonRow = bundle.mount(bundle.LogsString, { target: doc.body, props: { message: json } });
  await settle(5);
  assert.equal(doc.querySelector(".message pre"), null, "a collapsed JSON line was transformed");
  toggle().click();
  await settle(5);
  assert.ok(doc.querySelector(".message pre"), "an expanded JSON line was not transformed");
  bundle.unmount(jsonRow);
  bundle.store.update((s) => ({ ...s, transformJson: false }));

  // Short lines are untouched and get no toggle.
  const shortRow = bundle.mount(bundle.LogsString, { target: doc.body, props: { message: "INFO short" } });
  await settle(5);
  assert.equal(toggle(), null);
  assert.match(text(), /INFO short/);
  bundle.unmount(shortRow);

  console.log("LogsString tests passed");
});
