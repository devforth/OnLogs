import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

async function stubFetch(url) {
  if (String(url).includes("getSecret")) {
    return jsonResponse({ token: "tok123" });
  }
  return jsonResponse({ error: null });
}

main(async () => {
  const outfile = await bundleComponent("test/entry.js");
  const { window } = installDom({ fetchImpl: stubFetch });

  let copied = null;
  Object.defineProperty(window.navigator, "clipboard", {
    configurable: true,
    value: { writeText: async (text) => { copied = text; } },
  });
  globalThis.navigator = window.navigator;
  // jsdom has no layout, so no innerText; a <pre> renders its text as is.
  Object.defineProperty(window.HTMLElement.prototype, "innerText", {
    configurable: true,
    get() { return this.textContent; },
  });

  const bundle = await importBundle(outfile);
  const target = window.document.body;

  // A JSON log row rendered before the modal, as the log view does.
  const logRow = window.document.createElement("pre");
  logRow.textContent = '{"level":"info"}';
  target.appendChild(logRow);

  bundle.currentSnippedOption.set("Docker");
  const modal = bundle.mount(bundle.SecretModal, { target });
  await settle();

  const copyButton = target.querySelector(".coppyButton button");
  assert.ok(copyButton, "the copy button did not render");

  copyButton.click();
  await settle();
  assert.ok(copied?.includes("docker run"), `copied the wrong text: ${copied}`);
  assert.ok(copied.includes("ONLOGS_TOKEN=tok123"), `the token is missing: ${copied}`);

  [...target.querySelectorAll(".labelItem")]
    .find((el) => el.textContent.trim() === "Docker Compose")
    .click();
  await settle();
  copied = null;
  copyButton.click();
  await settle();
  assert.ok(copied?.includes("example_onlogs:"), `copied the wrong text: ${copied}`);

  bundle.unmount(modal);
  console.log("SecretModal tests passed");
});
