import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import * as sass from "sass";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../../test/harness.mjs";

const HOST = "prod-1";
const SERVICE = "api";
const requested = [];

async function fetchImpl(url) {
  requested.push(new URL(String(url), "http://onlogs.test").pathname.split("/").pop());
  return jsonResponse({ error: null });
}

const read = (s) => {
  let value;
  s.subscribe((v) => (value = v))();
  return value;
};

// Each rule's selector and display/left, with the media query it sits in.
function rulesOf(window, css) {
  const style = window.document.createElement("style");
  style.textContent = css;
  window.document.head.append(style);
  const out = [];
  const walk = (rules, media) => {
    for (const r of rules) {
      if (r.media) walk(r.cssRules, r.media.mediaText);
      else if (r.selectorText) out.push({ media, selector: r.selectorText, style: r.style });
    }
  };
  walk(style.sheet.cssRules, "");
  style.remove();
  return out;
}

main(async () => {
  const { window } = installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));

  // Wherever the right panel (the only other broom) is hidden, the toolbar
  // broom is shown, and it stays out of the desktop toolbar.
  const { css } = sass.compile(fileURLToPath(new URL("../../../main.scss", import.meta.url)), {
    logger: sass.Logger.silent,
  });
  const rules = rulesOf(window, css);
  const panelHidden = rules
    .filter((r) => r.selector === ".subContainerRight" && r.style.left === "-99999px")
    .map((r) => r.media);
  assert.ok(panelHidden.length >= 2, "the right panel is no longer hidden on phones and tablets");
  const broom = rules.filter((r) => r.selector === "#top-line .clearLogsButtonContainer");
  const displayAt = (media) => broom.findLast((r) => r.media === media)?.style.display;
  assert.equal(displayAt(""), "none", "the toolbar broom is not kept off the desktop toolbar");
  for (const media of panelHidden) {
    assert.equal(displayAt(media), "block", `no Clear logs entry at ${media}`);
  }

  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);
  bundle.store.update((s) => ({ ...s, deleteFromDocker: false }));
  const header = bundle.mount(bundle.LogsViewHeder, {
    target: document.body,
    props: bundle.reactiveProps({ searchText: "", searchResetVersion: 0 }),
  });
  await settle(5);

  // A delete-service action left over from the settings page must not be
  // inherited: the broom opens the clear-logs flow.
  bundle.confirmationObj.set({ action: async () => {}, message: "x", isVisible: false });
  const broomButton = document.querySelector("#top-line .clearLogsButtonContainer button");
  assert.ok(broomButton, "the log toolbar has no Clear logs button");
  broomButton.click();
  await settle(5);
  assert.equal(read(bundle.confirmationObj).isVisible, true, "the dialog did not open");
  assert.equal(read(bundle.confirmationObj).action, null, "the dialog kept another flow's action");

  const hostsWith = (running) => [
    { host: HOST, services: [{ serviceName: SERVICE, isDisabled: !running }] },
  ];
  const toggle = () => document.querySelector(".deleteModalContainer .checkboxContainer");
  const dialogText = () => document.querySelector(".deleteModalContainer").textContent;
  const deleteButton = () =>
    [...document.querySelectorAll(".deleteModalContainer button")].find((b) =>
      b.textContent.includes("Delete")
    );

  // Running container, Docker off: the switch is locked, Delete clears OnLogs
  // for the service on screen.
  let menu = bundle.mount(bundle.ConfirmationMenu, {
    target: document.body,
    props: { hosts: hostsWith(true) },
  });
  await settle(5);
  assert.match(dialogText(), /Host: prod-1/);
  assert.match(dialogText(), /Service: api, from:\s*OnLogs/);
  assert.ok(toggle().classList.contains("disabled"), "the Docker switch is usable for a running container");
  requested.length = 0;
  deleteButton().click();
  await settle(20);
  assert.deepEqual(requested.filter((n) => n.startsWith("delete")), ["deleteContainerLogs"]);
  bundle.unmount(menu);

  // Running container, saved Docker choice: never turned into an OnLogs clear.
  bundle.store.update((s) => ({ ...s, deleteFromDocker: true }));
  broomButton.click();
  await settle(5);
  menu = bundle.mount(bundle.ConfirmationMenu, {
    target: document.body,
    props: { hosts: hostsWith(true) },
  });
  await settle(5);
  assert.match(dialogText(), /from:\s*Docker/);
  assert.ok(deleteButton().disabled, "Delete is offered for a running container's Docker log");
  requested.length = 0;
  deleteButton().click();
  await settle(20);
  assert.deepEqual(requested.filter((n) => n.startsWith("delete")), [], "a running container's logs were deleted");
  bundle.unmount(menu);

  bundle.store.update((s) => ({ ...s, deleteFromDocker: false }));
  bundle.unmount(header);
  console.log("LogsViewHeder clear logs tests passed");
});
