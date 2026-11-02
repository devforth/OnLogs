import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
  settle,
  jsonResponse,
  main,
} from "../../../test/harness.mjs";

let deleteResponse = () => jsonResponse({ error: null });
let hostsAfter = [{ host: "h", services: [{ serviceName: "other" }] }];

async function stubFetch(url) {
  const target = String(url);
  if (target.includes("deleteContainer")) return deleteResponse();
  if (target.includes("getHosts")) {
    return jsonResponse(hostsAfter);
  }
  return jsonResponse({ error: null });
}

function read(store) {
  let value;
  store.subscribe((v) => (value = v))();
  return value;
}

main(async () => {
  const outfile = await bundleComponent("test/entry.js");
  const { window } = installDom({ fetchImpl: stubFetch });
  const bundle = await importBundle(outfile);
  const target = window.document.body;
  const { confirmationObj, toast, toastIsVisible, lastChosenService } = bundle;

  const start = `${bundle.changeKey}/servicesettings/h/s`;
  window.history.replaceState(null, "", start);
  bundle.lastChosenHost.set("h");
  lastChosenService.set("s");
  const view = bundle.mount(bundle.ServiceSettingsGeneral, { target });
  await settle();

  async function confirmDelete() {
    [...target.querySelectorAll("button")]
      .find((b) => b.textContent.trim() === "Delete service")
      .click();
    await settle();
    assert.equal(read(confirmationObj).isVisible, true, "no confirmation shown");
    await read(confirmationObj).action();
    await settle();
  }

  // A refused delete keeps the user on the page and says why.
  deleteResponse = () =>
    jsonResponse({ error: "Only admin can perform this request" }, 403);
  await confirmDelete();
  assert.equal(window.location.pathname, start, "navigated away after a refused delete");
  assert.equal(read(lastChosenService), "s");
  assert.equal(read(confirmationObj).isVisible, true, "the dialog closed on a refused delete");
  assert.equal(read(toastIsVisible), true, "no error toast");
  assert.equal(read(toast).message, "Only admin can perform this request");
  toastIsVisible.dismiss();

  // A failure without a JSON body is reported too.
  deleteResponse = () => ({
    status: 502,
    ok: false,
    json: async () => {
      throw new SyntaxError("Unexpected token <");
    },
  });
  await confirmDelete();
  assert.equal(window.location.pathname, start);
  assert.equal(read(toastIsVisible), true, "no error toast for a non-JSON failure");
  assert.equal(read(toast).status, "Error");
  toastIsVisible.dismiss();

  // A successful delete moves on to another service.
  deleteResponse = () => jsonResponse({ error: null });
  await confirmDelete();
  assert.equal(read(lastChosenService), "other");
  assert.equal(read(confirmationObj).isVisible, false, "the dialog stayed open");
  assert.notEqual(window.location.pathname, start, "stayed on the deleted service");

  // Deleting a host's last service moves on to another host's service.
  hostsAfter = [
    { host: "h", services: [] },
    { host: "g", services: [{ serviceName: "g1" }] },
  ];
  window.history.replaceState(null, "", start);
  bundle.lastChosenHost.set("h");
  lastChosenService.set("s");
  await confirmDelete();
  assert.equal(read(bundle.lastChosenHost), "g", "stayed on the emptied host");
  assert.equal(read(lastChosenService), "g1");
  assert.equal(window.location.pathname, `${bundle.changeKey}/view/g/g1`);

  bundle.unmount(view);
  console.log("ServiceSettings General tests passed");
});
