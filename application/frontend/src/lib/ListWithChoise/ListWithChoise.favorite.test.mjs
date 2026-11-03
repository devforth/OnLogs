import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
  settle,
  jsonResponse,
  main,
} from "../../../test/harness.mjs";

// Each changeFavorite request waits here until the test answers it.
const pending = [];
async function fetchImpl(url) {
  if (!String(url).includes("changeFavorite")) return jsonResponse({ error: null });
  return new Promise((resolve, reject) => pending.push({ resolve, reject }));
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

  bundle.groups.set([]);
  bundle.lastChosenHost.set("host1");
  bundle.lastChosenService.set("api");
  const props = bundle.reactiveProps({
    listData: [
      { host: "host1", services: [{ serviceName: "api", isDisabled: false, isFavorite: false }] },
    ],
    listElementButton: "true",
  });
  const app = bundle.mount(bundle.ListWithChoise, { target, props });
  await settle();

  const heart = () => target.querySelector("#heartButton-0");
  const starred = () => heart().className.includes("log-Heart");
  const click = async () => {
    target.querySelector("#heartButtonCont-0").click();
    await settle();
  };
  const answer = async (response) => {
    pending.shift().resolve(response);
    await settle();
  };

  // Refused: the heart goes back and the server's reason is shown.
  await click();
  assert.equal(starred(), true, "the heart did not fill on click");
  await answer(jsonResponse({ error: "Request refused: foreign origin" }, 403));
  console.log(`  refused: starred=${starred()}, toast="${read(bundle.toast).message}"`);
  assert.equal(starred(), false, "a refused star stayed filled");
  assert.equal(read(bundle.toastIsVisible), true, "a refused star showed no toast");
  assert.equal(read(bundle.toast).message, "Request refused: foreign origin");
  bundle.toastIsVisible.dismiss();

  // A second click while the first is unanswered cannot leave the heart wrong.
  await click();
  await click();
  assert.equal(pending.length, 1, "a second click sent a second toggle before the first answered");
  assert.equal(starred(), true);
  await answer(jsonResponse({ error: "settings database is unavailable" }, 500));
  assert.equal(starred(), false, "the double click left the heart filled after a refusal");

  // Accepted: the heart stays filled.
  await click();
  await answer(jsonResponse({ error: null }));
  assert.equal(starred(), true, "an accepted star was undone");
  bundle.toastIsVisible.dismiss();

  // Network failure: undone with a generic message.
  await click();
  assert.equal(starred(), false);
  pending.shift().reject(new TypeError("Failed to fetch"));
  await settle();
  assert.equal(starred(), true, "a failed unstar stayed empty");
  assert.equal(read(bundle.toastIsVisible), true, "a failed request showed no toast");
  assert.match(read(bundle.toast).message, /Something went wrong/);
  bundle.toastIsVisible.dismiss();

  // 401: the app is on its way to the login page, so nothing is shown.
  await click();
  await answer(jsonResponse({ error: "401 - Unauthorized!" }, 401));
  assert.equal(read(bundle.toastIsVisible), false, "an ended session showed an error over the login page");

  bundle.unmount(app);
  console.log("ListWithChoise favourite tests passed");
});
