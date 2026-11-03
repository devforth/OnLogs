import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
  settle,
  jsonResponse,
  main,
} from "../../../test/harness.mjs";

const SAVED = {
  UTCtime: false,
  breakLines: true,
  caseInSensitive: true,
  transformJson: true,
  deleteFromDocker: false,
};

// Each updateUserSettings request waits here until the test answers it.
const saves = [];
async function fetchImpl(url, init) {
  const name = String(url).split("/").pop();
  if (name === "getUserSettings") return jsonResponse(SAVED);
  if (name !== "updateUserSettings") return jsonResponse({ error: null });
  return new Promise((resolve, reject) =>
    saves.push({ body: JSON.parse(init.body), resolve, reject })
  );
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
  await settle();
  assert.deepEqual(read(bundle.store), SAVED, "the saved settings were not loaded");
  assert.equal(saves.length, 0, "loading the settings saved them back");

  bundle.mount(bundle.Checkbox, { target, props: { storeValue: "UTCtime" } });
  await settle();
  const box = () => target.querySelector(".checkboxContainer");
  const toggle = async () => {
    box().click();
    await settle();
  };
  const answer = async (response) => {
    saves.shift().resolve(response);
    await settle();
  };
  assert.ok(box().classList.contains("inactive"));

  // Refused: the switch and the store go back and the reason is shown.
  await toggle();
  assert.equal(saves.length, 1);
  assert.equal(saves[0].body.UTCtime, true);
  assert.ok(box().classList.contains("active"));
  await answer(jsonResponse({ error: "Request refused: foreign origin" }, 403));
  console.log(`  refused: UTCtime=${read(bundle.store).UTCtime}, toast="${read(bundle.toast).message}"`);
  assert.equal(read(bundle.store).UTCtime, false, "a refused setting stayed in the store");
  assert.ok(box().classList.contains("inactive"), "a refused setting stayed switched on");
  assert.equal(read(bundle.toastIsVisible), true, "a refused setting showed no toast");
  assert.equal(read(bundle.toast).message, "Request refused: foreign origin");
  assert.equal(saves.length, 0, "undoing a refused setting saved it again");
  bundle.toastIsVisible.dismiss();

  // Accepted, then refused: back to the accepted value, not the loaded one.
  await toggle();
  await answer(jsonResponse({ error: null }));
  bundle.store.update((s) => ({ ...s, breakLines: false }));
  await settle();
  await answer(jsonResponse({ error: "settings database is unavailable" }, 500));
  assert.deepEqual(read(bundle.store), { ...SAVED, UTCtime: true }, "the store did not return to the last saved settings");
  assert.equal(saves.length, 0);
  bundle.toastIsVisible.dismiss();

  // One save at a time: a refusal with a newer change waiting sends that
  // change instead of undoing it.
  const newest = { ...SAVED, UTCtime: true, breakLines: false, transformJson: false };
  bundle.store.update((s) => ({ ...s, breakLines: false }));
  await settle();
  bundle.store.update((s) => ({ ...s, transformJson: false }));
  await settle();
  assert.equal(saves.length, 1, "a save went out before the previous one was answered");
  await answer(jsonResponse({ error: "stale refusal" }, 500));
  assert.deepEqual(read(bundle.store), newest, "a stale refusal undid a newer change");
  assert.equal(read(bundle.toastIsVisible), false, "a stale refusal showed a toast");
  assert.equal(saves.length, 1);
  assert.deepEqual(saves[0].body, newest, "the waiting change was not sent");
  await answer(jsonResponse({ error: null }));
  assert.equal(saves.length, 0);

  // Network failure: undone with a generic message.
  await toggle();
  saves.shift().reject(new TypeError("Failed to fetch"));
  await settle();
  assert.equal(read(bundle.store).UTCtime, true, "a failed save stayed in the store");
  assert.match(read(bundle.toast).message, /Something went wrong/);
  bundle.toastIsVisible.dismiss();

  // 401: the app is on its way to the login page, so nothing is shown.
  await toggle();
  await answer(jsonResponse({ error: "401 - Unauthorized!" }, 401));
  assert.equal(read(bundle.toastIsVisible), false, "an ended session showed an error over the login page");
  assert.equal(saves.length, 0);

  console.log("SettingsController tests passed");
});
