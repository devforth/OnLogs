import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const HOST = "prod-1";
const SERVICE = "api";

let rows = [];
let seq = 0;
let sockets = [];
let deleteStatus = 200;
const requested = [];
const REFUSAL = "Stop the container first: Docker sends no new lines once a running container's log is cleared";

function store(msg) {
  seq += 1;
  const ts = new Date(Date.UTC(2026, 9, 19, 10, 0, 0) + seq).toISOString().replace("Z", "000000Z");
  rows.push({ ts, msg, key: `${ts} +${1792400000000000000n + BigInt(seq)}-${seq}` });
  for (const ws of sockets) {
    if (ws.readyState === 1 && ws.url.includes(`host=${HOST}&id=${SERVICE}`)) {
      ws.onmessage?.({ data: JSON.stringify([ts, msg]) });
    }
  }
}

async function fetchImpl(url) {
  const u = new URL(String(url), "http://onlogs.test");
  const name = u.pathname.split("/").pop();
  requested.push(name);
  if (name === "deleteContainerLogs") {
    if (deleteStatus === 403) return jsonResponse({ error: "Only admin can perform this request" }, 403);
    if (deleteStatus === 401) return jsonResponse({ error: "401 - Unauthorized!" }, 401);
    rows = [];
    return jsonResponse({ error: null });
  }
  if (name === "deleteDockerLogs") {
    return jsonResponse({ error: REFUSAL });
  }
  if (name === "getLogs" && !u.searchParams.get("startWith")) {
    const logs = rows
      .slice(-(Number(u.searchParams.get("limit")) || 30))
      .reverse()
      .map((r) => [r.ts, r.msg, r.key]);
    return jsonResponse({ logs, last_processed_key: logs.at(-1)?.[2] || "", is_end: true });
  }
  return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
}

const rendered = () =>
  [...document.querySelectorAll("#logs .message p")].map((el) => el.textContent.trim());
const read = (s) => {
  let value;
  s.subscribe((v) => (value = v))();
  return value;
};

main(async () => {
  ({ sockets } = installDom({ fetchImpl }));
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  ["old-1", "old-2", "old-3"].forEach(store);
  bundle.urlHash.set("");
  bundle.isPending.set(false);
  bundle.chosenStatus.set("");
  bundle.lastChosenHost.set(HOST);
  bundle.lastChosenService.set(SERVICE);
  bundle.store.update((s) => ({ ...s, deleteFromDocker: false }));
  const view = bundle.mount(bundle.NewLogsV2, { target: document.body });
  await settle(40);
  globalThis.__onlogsObservers.find((h) => h.element?.id === "endOfLogs").set(true);
  await settle(5);
  assert.deepEqual(rendered(), ["old-1", "old-2", "old-3"]);

  bundle.confirmationObj.set({ action: null, message: "", isVisible: true });
  const menu = bundle.mount(bundle.ConfirmationMenu, { target: document.body });
  await settle(5);
  const deleteButton = () =>
    [...document.querySelectorAll(".deleteModalContainer button")].find((b) =>
      b.textContent.includes("Delete")
    );

  deleteStatus = 403;
  deleteButton().click();
  await settle(20);
  assert.equal(read(bundle.toast).message, "Only admin can perform this request", "the server's refusal was replaced");
  assert.equal(read(bundle.confirmationObj).isVisible, true);
  assert.deepEqual(rendered(), ["old-1", "old-2", "old-3"]);

  bundle.toastIsVisible.dismiss();
  deleteStatus = 401;
  deleteButton().click();
  await settle(20);
  assert.equal(read(bundle.toastIsVisible), false, "an ended session showed an error over the login page");
  assert.equal(read(bundle.confirmationObj).isVisible, false, "the dialog waits behind the login page");
  bundle.confirmationObj.set({ action: null, message: "", isVisible: true });
  await settle(5);

  bundle.store.update((s) => ({ ...s, deleteFromDocker: true }));
  await settle(5);
  deleteButton().click();
  await settle(20);
  assert.equal(read(bundle.toast).message, REFUSAL);
  assert.equal(read(bundle.confirmationObj).isVisible, true);
  bundle.store.update((s) => ({ ...s, deleteFromDocker: false }));
  await settle(5);

  deleteStatus = 200;
  deleteButton().click();
  await settle(20);
  assert.equal(read(bundle.confirmationObj).isVisible, false);
  assert.deepEqual(rendered(), [], "cleared rows are still shown");
  store("after-clear");
  await settle(10);
  assert.deepEqual(rendered(), ["after-clear"]);

  bundle.unmount(menu);

  // A running container's Docker log cannot be cleared, and the dialog says why.
  const hostsWith = (running) => [{ host: HOST, services: [{ serviceName: SERVICE, isDisabled: !running }] }];
  const toggle = () => document.querySelector(".deleteModalContainer .checkboxContainer");
  const dialogText = () => document.querySelector(".deleteModalContainer").textContent;
  const openMenu = async (props) => {
    bundle.confirmationObj.set({ action: null, message: "", isVisible: true });
    const menu = bundle.mount(bundle.ConfirmationMenu, { target: document.body, props });
    await settle(5);
    return menu;
  };

  // Saved preference OFF: the switch cannot be turned on, Delete clears OnLogs.
  bundle.store.update((s) => ({ ...s, deleteFromDocker: false }));
  let runningMenu = await openMenu({ hosts: hostsWith(true) });
  assert.ok(toggle().classList.contains("disabled"), "the Docker switch is usable for a running container");
  assert.match(dialogText(), /Not available while the\s+container is running/);
  toggle().click();
  await settle(2);
  assert.ok(toggle().classList.contains("inactive"), "a click turned the disabled switch on");
  requested.length = 0;
  deleteButton().click();
  await settle(20);
  assert.deepEqual(requested.filter((n) => n.startsWith("delete")), ["deleteContainerLogs"]);
  bundle.unmount(runningMenu);

  // Saved preference ON: never silently switched to an OnLogs clear.
  bundle.store.update((s) => ({ ...s, deleteFromDocker: true }));
  runningMenu = await openMenu({ hosts: hostsWith(true) });
  assert.ok(toggle().classList.contains("active"), "the saved Docker choice is not shown");
  assert.match(dialogText(), /from:\s*Docker/, "the dialog names the wrong target");
  assert.match(dialogText(), /turn this\s+off/);
  assert.ok(deleteButton().disabled, "Delete is offered for a running container's Docker log");
  requested.length = 0;
  deleteButton().click();
  await settle(20);
  assert.deepEqual(requested.filter((n) => n.startsWith("delete")), [], "a running container's logs were deleted");
  toggle().click();
  await settle(5);
  assert.ok(toggle().classList.contains("disabled"), "the switch can be turned back on");
  assert.match(dialogText(), /from:\s*OnLogs/);
  assert.ok(!deleteButton().disabled);
  bundle.unmount(runningMenu);

  // The running state is read when the dialog opens, not on every refresh.
  bundle.store.update((s) => ({ ...s, deleteFromDocker: true }));
  const live = bundle.reactiveProps({ hosts: hostsWith(false) });
  bundle.confirmationObj.set({ action: null, message: "", isVisible: true });
  const stoppedMenu = bundle.mount(bundle.ConfirmationMenu, { target: document.body, props: live });
  await settle(5);
  assert.ok(!toggle().classList.contains("disabled"), "the Docker switch is disabled for a stopped container");
  assert.ok(toggle().classList.contains("active"));
  live.hosts = hostsWith(true);
  await settle(5);
  assert.ok(!deleteButton().disabled, "the open dialog changed its target on a host refresh");
  assert.match(dialogText(), /from:\s*Docker/);
  bundle.unmount(stoppedMenu);

  // Deleting a service is not about Docker logs at all.
  bundle.confirmationObj.set({ action: async () => {}, message: "OnLogs (the service and all its logs).", isVisible: true });
  const serviceMenu = bundle.mount(bundle.ConfirmationMenu, { target: document.body, props: { hosts: hostsWith(true) } });
  await settle(5);
  assert.match(dialogText(), /from:\s*OnLogs \(the service and all its logs\)\./);
  assert.equal(dialogText().match(/will be lost/g).length, 1, "the warning shows more than once");
  assert.equal(toggle(), null, "the Delete service dialog offers the Docker switch");
  assert.doesNotMatch(dialogText(), /Stop the container first/);
  bundle.unmount(serviceMenu);
  bundle.store.update((s) => ({ ...s, deleteFromDocker: false }));
  bundle.unmount(view);
  console.log("ConfirmationMenu tests passed");
});
