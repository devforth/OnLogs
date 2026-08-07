import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
  settle,
  jsonResponse,
} from "../../../test/harness.mjs";

const HOSTS = [
  {
    host: "host1",
    services: [
      { serviceName: "api", isDisabled: false, isFavorite: false },
      { serviceName: "web", isDisabled: false, isFavorite: false },
    ],
  },
  {
    host: "host2",
    services: [{ serviceName: "worker", isDisabled: true, isFavorite: false }],
  },
];

let calls = [];
let nextResponse = () => jsonResponse({ error: null });

async function recordingFetch(url, options = {}) {
  calls.push({
    url: String(url),
    body: options.body ? JSON.parse(options.body) : null,
  });
  return nextResponse(String(url));
}

function lastCall(fragment) {
  return [...calls].reverse().find((c) => c.url.includes(fragment));
}

function setName(window, value) {
  const input = window.document.querySelector(".groupNameInput");
  assert.ok(input, "the group name field did not render");
  input.value = value;
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
}

function check(window, host, service) {
  const box = window.document.querySelector(
    `input[type="checkbox"][data-host="${host}"][data-service="${service}"]`
  );
  assert.ok(box, `no checkbox for ${host}/${service}`);
  box.click();
  return box;
}

async function run() {
  const outfile = await bundleComponent("test/entry.js");
  const { window } = installDom({ fetchImpl: recordingFetch });
  const bundle = await importBundle(outfile);
  const target = window.document.body;

  const mountModal = async () => {
    calls = [];
    const app = bundle.mount(bundle.GroupModal, { target, props: { hosts: HOSTS } });
    await settle();
    return app;
  };

  // --- create ------------------------------------------------------------
  bundle.groupBeingEdited.set(null);
  bundle.groupModalIsVisible.set(true);
  let app = await mountModal();

  assert.ok(window.document.querySelector(".groupModal"), "the modal did not render");
  const boxes = window.document.querySelectorAll('input[type="checkbox"]');
  console.log(`  checkbox tree rendered ${boxes.length} service(s)`);
  assert.equal(boxes.length, 3, "every service on every host should be selectable");

  setName(window, "backend");
  check(window, "host1", "api");
  check(window, "host2", "worker");
  window.document.querySelector("#groupModalSave").click();
  await settle();

  const created = lastCall("createGroup");
  assert.ok(created, `createGroup was not called: ${calls.map((c) => c.url).join(", ")}`);
  assert.deepEqual(created.body, {
    name: "backend",
    members: [
      { host: "host1", service: "api" },
      { host: "host2", service: "worker" },
    ],
  });
  console.log(`  create posted ${JSON.stringify(created.body)}`);
  assert.ok(lastCall("getGroups"), "the sidebar was not refreshed after a create");
  bundle.unmount(app);

  // --- a group with no members is allowed and renders as an empty folder ---
  bundle.groupBeingEdited.set(null);
  bundle.groupModalIsVisible.set(true);
  app = await mountModal();
  setName(window, "empty folder");
  window.document.querySelector("#groupModalSave").click();
  await settle();

  const empty = lastCall("createGroup");
  assert.deepEqual(
    empty.body,
    { name: "empty folder", members: [] },
    "a group with no members should be allowed"
  );
  bundle.unmount(app);

  bundle.groups.set([{ name: "empty folder", members: [] }]);
  const list = bundle.mount(bundle.ListWithChoise, { target, props: { listData: HOSTS } });
  await settle();
  const section = target.querySelector(".groupSection");
  assert.ok(
    section && section.textContent.includes("empty folder"),
    "an empty group should still render as a folder"
  );
  assert.equal(
    section.querySelectorAll(".serviceListItem").length,
    0,
    "an empty group should render no rows"
  );
  bundle.unmount(list);
  bundle.groups.set([]);

  // --- the server is the authority ---------------------------------------
  bundle.groupBeingEdited.set(null);
  bundle.groupModalIsVisible.set(true);
  app = await mountModal();
  nextResponse = (url) =>
    url.includes("createGroup")
      ? jsonResponse({ error: 'Group "taken" already exists' }, 409)
      : jsonResponse({ error: null });

  setName(window, "taken");
  window.document.querySelector("#groupModalSave").click();
  await settle();

  let toastState = null;
  let toastVisible = false;
  bundle.toast.subscribe((v) => (toastState = v))();
  bundle.toastIsVisible.subscribe((v) => (toastVisible = v))();
  console.log(`  rejected create surfaced: ${toastState.message}`);
  assert.ok(toastVisible, "a rejected create should raise a toast");
  assert.equal(toastState.message, 'Group "taken" already exists');

  let stillOpen = false;
  bundle.groupModalIsVisible.subscribe((v) => (stillOpen = v))();
  assert.ok(stillOpen, "a rejected create should leave the modal open");
  nextResponse = () => jsonResponse({ error: null });
  bundle.unmount(app);

  // --- edit --------------------------------------------------------------
  bundle.groupBeingEdited.set({
    name: "backend",
    members: [{ host: "host1", service: "api" }],
  });
  bundle.groupModalIsVisible.set(true);
  app = await mountModal();

  const nameField = window.document.querySelector(".groupNameInput");
  assert.equal(nameField.value, "backend", "edit mode should pre-fill the name");
  const apiBox = window.document.querySelector(
    'input[type="checkbox"][data-host="host1"][data-service="api"]'
  );
  assert.ok(apiBox.checked, "edit mode should pre-check the existing members");

  setName(window, "backend renamed");
  check(window, "host1", "web");
  check(window, "host1", "api");
  window.document.querySelector("#groupModalSave").click();
  await settle();

  const updated = lastCall("updateGroup");
  assert.ok(updated, "updateGroup was not called");
  assert.deepEqual(updated.body, {
    name: "backend",
    newName: "backend renamed",
    members: [{ host: "host1", service: "web" }],
  });
  console.log(`  edit posted ${JSON.stringify(updated.body)}`);
  bundle.unmount(app);

  // --- delete ------------------------------------------------------------
  bundle.groupBeingEdited.set({
    name: "backend",
    members: [{ host: "host1", service: "api" }],
  });
  bundle.groupModalIsVisible.set(true);
  app = await mountModal();
  window.document.querySelector("#groupModalDelete").click();
  await settle();

  const deleted = lastCall("deleteGroup");
  assert.ok(deleted, "deleteGroup was not called");
  assert.deepEqual(deleted.body, { name: "backend" });
  let closed = true;
  bundle.groupModalIsVisible.subscribe((v) => (closed = v))();
  assert.equal(closed, false, "a successful delete should close the modal");
  bundle.unmount(app);

  // --- the dropdown row ---------------------------------------------------
  bundle.groupModalIsVisible.set(false);
  bundle.addHostMenuIsVisible.set(true);
  const dropdown = bundle.mount(bundle.DropDownAddHost, { target });
  await settle();

  const row = [...target.querySelectorAll("tr")].find((tr) =>
    tr.textContent.includes("Create services group")
  );
  assert.ok(row, "the Create services group row is missing");
  assert.ok(row.className.includes("clickable"), "the row should look clickable");
  row.click();
  await settle();

  let opened = false;
  let dropdownOpen = true;
  bundle.groupModalIsVisible.subscribe((v) => (opened = v))();
  bundle.addHostMenuIsVisible.subscribe((v) => (dropdownOpen = v))();
  assert.ok(opened, "the row did not open the group modal");
  assert.equal(dropdownOpen, false, "the row did not close the dropdown");
  bundle.unmount(dropdown);

  bundle.groupModalIsVisible.set(false);
  bundle.groupBeingEdited.set(null);
  console.log("GroupModal tests passed");
  process.exit(0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
