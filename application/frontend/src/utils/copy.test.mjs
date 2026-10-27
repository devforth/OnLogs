import assert from "node:assert/strict";
import { installDom, settle, main } from "../../test/harness.mjs";

function read(store) {
  let value;
  store.subscribe((v) => (value = v))();
  return value;
}

main(async () => {
  const { window } = installDom();
  const { copyCustomText } = await import("./functions.js");
  const { toast, toastIsVisible } = await import("../Stores/stores.js");

  const setClipboard = (value) =>
    Object.defineProperty(window.navigator, "clipboard", { configurable: true, value });
  let execResult = true;
  let copiedByExec = null;
  window.document.execCommand = (cmd) => {
    assert.equal(cmd, "copy");
    copiedByExec = window.document.activeElement?.value ?? null;
    return execResult;
  };

  let done = 0;
  const onCopied = () => done++;

  // Plain HTTP on a non-localhost origin: no navigator.clipboard at all.
  setClipboard(undefined);
  copyCustomText("http://onlogs.lan/view/h/s#k", onCopied);
  await settle();
  assert.equal(copiedByExec, "http://onlogs.lan/view/h/s#k", "the fallback did not copy");
  assert.equal(done, 1, "success was not reported after the fallback copy");
  assert.equal(window.document.querySelector("textarea"), null, "the helper textarea was left behind");

  // The clipboard API refuses: the fallback still copies.
  copiedByExec = null;
  setClipboard({ writeText: () => Promise.reject(new Error("denied")) });
  copyCustomText("second", onCopied);
  await settle();
  assert.equal(copiedByExec, "second");
  assert.equal(done, 2);

  // Nothing can copy: the user is told.
  execResult = false;
  copyCustomText("third", onCopied);
  await settle();
  assert.equal(done, 2, "reported success although nothing was copied");
  assert.equal(read(toastIsVisible), true, "no error toast");
  assert.equal(read(toast).status, "Error");
  toastIsVisible.dismiss();

  // The clipboard API works: no fallback.
  copiedByExec = null;
  let written = null;
  setClipboard({ writeText: async (t) => { written = t; } });
  copyCustomText("fourth", onCopied);
  await settle();
  assert.equal(written, "fourth");
  assert.equal(copiedByExec, null);
  assert.equal(done, 3);

  console.log("copy tests passed");
});
