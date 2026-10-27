import assert from "node:assert/strict";
import { mock } from "node:test";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

async function stubFetch(url) {
  const target = String(url);
  if (
    target.includes("getLogs") ||
    target.includes("getPrevLogs") ||
    target.includes("getLogWithPrev")
  ) {
    return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  }
  if (target.includes("getUsers")) {
    return jsonResponse({ users: [{ username: "bob", editable: true }] });
  }
  if (target.includes("getHosts")) {
    return jsonResponse([
      { host: "h", services: [{ serviceName: "s", isDisabled: false }] },
    ]);
  }
  return jsonResponse({ error: null });
}

function read(store) {
  let value;
  store.subscribe((v) => (value = v))();
  return value;
}

function buttonTitled(root, title) {
  return [...root.querySelectorAll("button")].find(
    (b) => b.textContent.trim() === title
  );
}

main(async () => {
  const outfile = await bundleComponent("test/entry.js");
  installDom({ fetchImpl: stubFetch });
  const bundle = await importBundle(outfile);
  const { toast, toastIsVisible, toastTimeoutId } = bundle;

  // --- a toast raised by a click survives that same click -----------------
  async function mountAt(path) {
    const { window } = installDom({ fetchImpl: stubFetch });
    window.history.replaceState(null, "", `${bundle.changeKey}${path}`);
    const target = window.document.body;
    const app = bundle.mount(bundle.App, {
      target,
      props: { url: `${bundle.changeKey}${path}` },
    });
    await settle();
    return { app, target, window };
  }

  let view = await mountAt("/view/h/s");
  bundle.groupBeingEdited.set(null);
  bundle.groupModalIsVisible.set(true);
  await settle();
  view.target.querySelector("#groupModalSave").click();
  await settle();
  assert.equal(read(toastIsVisible), true, "the empty group name toast was hidden by its own click");
  assert.ok(view.target.querySelector(".toastContainer"), "the toast is not rendered");

  // A later click elsewhere still closes it.
  view.target.querySelector(".groupNameInput").click();
  await settle();
  assert.equal(read(toastIsVisible), false, "a click elsewhere no longer closes the toast");
  bundle.groupModalIsVisible.set(false);
  bundle.unmount(view.app);

  view = await mountAt("/users");
  const editButton = buttonTitled(view.target, "Edit");
  assert.ok(editButton, "the users list did not render");
  editButton.closest(".buttonSpan").click();
  await settle();
  buttonTitled(view.target, "Confirm").closest(".buttonSpan").click();
  await settle();
  assert.equal(read(toastIsVisible), true, "the empty password toast was hidden by its own click");
  assert.equal(read(toast).message, "Type a new password.");
  toastIsVisible.dismiss();
  bundle.editUserOpen.set(false);
  bundle.unmount(view.app);
  await settle();

  // --- timing ---------------------------------------------------------------
  mock.timers.enable({ apis: ["setTimeout"] });
  const tick = (ms) => mock.timers.tick(ms);
  const show = (message, extra = {}) =>
    toast.set({ tittle: "Error", message, status: "error", position: "", ...extra });

  // A caller that arms its own 5 s timer for every toast it raises.
  const raiseWithTimer = (message) => {
    toastIsVisible.set(true);
    show(message);
    setTimeout(() => toastIsVisible.set(false), 5000);
  };
  raiseWithTimer("first");
  tick(3000);
  raiseWithTimer("second");
  tick(2100);
  assert.equal(read(toastIsVisible), true, "the first toast's timer hid the second toast early");
  tick(3000);
  assert.equal(read(toastIsVisible), false, "the second toast never hid");

  // A caller that arms no timer at all.
  show("plain");
  toastIsVisible.set(true);
  tick(4900);
  assert.equal(read(toastIsVisible), true);
  tick(200);
  assert.equal(read(toastIsVisible), false, "a toast without a caller timer never hid");

  // The copied-link toast: a short own timer, and a re-show when copied again.
  const showCopied = () => {
    toast.set({ tittle: "Success", message: "URL has been copied", position: "", status: "Success" });
    if (!read(toastIsVisible)) {
      toastIsVisible.set(true);
      toastTimeoutId.set(setTimeout(() => toastIsVisible.set(false), 1500));
    } else {
      clearTimeout(read(toastTimeoutId));
      toastIsVisible.set(false);
      setTimeout(() => toastIsVisible.set(true), 400);
    }
  };
  showCopied();
  tick(1400);
  assert.equal(read(toastIsVisible), true);
  tick(200);
  assert.equal(read(toastIsVisible), false, "a single copy should keep its 1.5 s toast");

  showCopied();
  tick(300);
  showCopied();
  tick(400);
  assert.equal(read(toastIsVisible), true, "the second copy did not show the toast again");
  tick(5100);
  assert.equal(read(toastIsVisible), false, "the toast of a second copy never hid");

  // An earlier toast's registered timer does not cut a newer toast short.
  showCopied();
  tick(1000);
  show("later");
  toastIsVisible.set(true);
  tick(1000);
  assert.equal(read(toastIsVisible), true, "the copy toast's timer hid a newer toast");
  tick(4100);
  assert.equal(read(toastIsVisible), false);

  // A toast that offers an action waits for the user.
  show("warning", { additionButton: { isVisible: true, title: "Continue", CB() {} } });
  toastIsVisible.set(true);
  tick(60000);
  assert.equal(read(toastIsVisible), true, "a toast with an action button hid on its own");
  toastIsVisible.dismiss();
  assert.equal(read(toastIsVisible), false, "dismiss did not hide the toast");

  // The user closing a toast hides it at once.
  show("closed");
  toastIsVisible.set(true);
  toastIsVisible.dismiss();
  assert.equal(read(toastIsVisible), false);

  mock.timers.reset();
  console.log("Toast tests passed");
});
