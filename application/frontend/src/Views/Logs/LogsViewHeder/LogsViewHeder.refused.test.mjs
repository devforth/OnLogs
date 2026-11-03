import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../../test/harness.mjs";

const saves = [];
async function fetchImpl(url) {
  const name = String(url).split("/").pop();
  if (name === "getUserSettings") {
    return jsonResponse({ UTCtime: false, breakLines: true, caseInSensitive: true, transformJson: true });
  }
  if (name !== "updateUserSettings") return jsonResponse({ error: null });
  return new Promise((resolve) => saves.push(resolve));
}

const read = (s) => {
  let value;
  s.subscribe((v) => (value = v))();
  return value;
};

main(async () => {
  const { window } = installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  const target = window.document.body;
  bundle.mount(bundle.SettingsController, { target });
  bundle.mount(bundle.LogsViewHeder, {
    target,
    props: bundle.reactiveProps({ searchText: "", searchResetVersion: 0 }),
  });
  await settle();

  const menu = () => target.querySelector("#top-line .dropDownContainer");
  target.querySelector("#top-line button").click();
  await settle();
  assert.ok(menu(), "the eye button did not open the display options");

  // The refusal's toast sits over this menu, so the menu closes under it.
  menu().querySelector(".checkboxContainer").click();
  await settle();
  assert.equal(saves.length, 1);
  saves.shift()(jsonResponse({ error: "settings database is unavailable" }, 500));
  await settle();
  assert.equal(read(bundle.toastIsVisible), true, "the refusal showed no toast");
  assert.equal(menu(), null, "the display options stayed open under the error toast");

  // Opened again while the toast is up, it stays open.
  target.querySelector("#top-line button").click();
  await settle();
  assert.ok(menu(), "the display options cannot be opened while a toast is shown");

  console.log("LogsViewHeder refused-setting tests passed");
});
