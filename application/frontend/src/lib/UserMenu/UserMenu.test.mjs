// Drives the users page the way an admin and a non-admin use it.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const FORBIDDEN = "Only admin can perform this request";

let users = [];
let isAdmin = true;
let calls = [];

async function stubFetch(url, opts = {}) {
  const path = String(url).split("/api/v1/")[1]?.split("?")[0];
  const body = opts.body ? JSON.parse(opts.body) : null;
  calls.push({ path, body });
  if (path === "getUsers") {
    return isAdmin
      ? jsonResponse({ users: users.map((u) => ({ ...u })), error: null })
      : jsonResponse({ error: FORBIDDEN }, 403);
  }
  if (path === "createUser") {
    if (body.login === "forbidden") return jsonResponse({ error: FORBIDDEN }, 403);
    if (users.some((u) => u.username === body.login)) {
      return jsonResponse({ error: "User is already exists" });
    }
    users.push({ username: body.login, editable: true });
    return jsonResponse({ error: null });
  }
  if (path === "deleteUser") {
    users = users.filter((u) => u.username !== body.login);
    return jsonResponse({ error: null });
  }
  if (path === "getHosts") return jsonResponse([]);
  return jsonResponse({ error: null });
}

function storeValue(store) {
  let value;
  store.subscribe((v) => (value = v))();
  return value;
}

main(async () => {
  const outfile = await bundleComponent("test/usersEntry.js");
  installDom({ fetchImpl: stubFetch });
  const bundle = await importBundle(outfile);
  const { window } = installDom({ fetchImpl: stubFetch });
  const doc = window.document;

  const rows = () =>
    [...doc.querySelectorAll(".userTable tbody tr")].map((tr) =>
      tr.querySelector("td").textContent.trim()
    );
  const rowOf = (name) =>
    [...doc.querySelectorAll(".userTable tbody tr")].find(
      (tr) => tr.querySelector("td").textContent.trim() === name
    );
  const bodiesOf = (path) => calls.filter((c) => c.path === path).map((c) => c.body);
  const form = () => doc.querySelector(".userCreateForm");
  const buttonIn = (root, title) =>
    [...root.querySelectorAll("button")].find((b) => b.textContent.trim() === title);
  const type = (input, value) => {
    input.value = value;
    input.dispatchEvent(new window.Event("input", { bubbles: true }));
  };
  const pressEnter = (input) =>
    input.dispatchEvent(new window.KeyboardEvent("keydown", { key: "Enter", bubbles: true }));

  async function usersPage(seed, { admin = true } = {}) {
    installDom({ fetchImpl: stubFetch });
    users = seed.map((username) => ({ username, editable: username !== "admin" }));
    isAdmin = admin;
    calls = [];
    window.history.replaceState({}, "", `${bundle.changeKey}/users`);
    const app = bundle.mount(bundle.Main, { target: doc.body, props: {} });
    await settle();
    return {
      async done() {
        bundle.unmount(app);
        await settle();
      },
    };
  }

  async function openCreateForm() {
    doc.querySelector(".addUserButton").click();
    await settle();
    assert.ok(form(), "the + button should open the create form");
    return form().querySelectorAll("input");
  }

  // Create: validation, success, reopen, Enter, server errors.
  {
    const page = await usersPage(["admin", "alice"]);
    assert.deepEqual(rows(), ["admin", "alice"]);

    let [login, password] = await openCreateForm();
    buttonIn(form(), "Create").click();
    await settle();
    assert.deepEqual(bodiesOf("createUser"), []);
    assert.ok(form().querySelector(".error")?.textContent.trim(), "empty fields gave no message");

    type(login, "  bob ");
    type(password, " pw ");
    buttonIn(form(), "Create").click();
    await settle();
    assert.deepEqual(bodiesOf("createUser"), [{ login: "bob", password: " pw " }]);
    console.log(`  after create: ${rows().length} row(s)`);
    assert.deepEqual(rows(), ["admin", "alice", "bob"]);
    assert.ok(rowOf("bob").querySelector("button"), "the new row has no Edit/Delete buttons");
    assert.equal(form(), null, "the create form stayed open after success");
    assert.equal(storeValue(bundle.toastIsVisible), true, "no toast after create");
    assert.equal(storeValue(bundle.toast).tittle, "Success");

    [login, password] = await openCreateForm();
    assert.equal(login.value, "", "the form kept the previous login");
    assert.equal(password.value, "", "the form kept the previous password");
    type(login, "bob");
    type(password, "pw");
    pressEnter(password);
    await settle();
    assert.equal(bodiesOf("createUser").length, 2, "Enter did not submit the create form");
    assert.ok(form()?.textContent.includes("User is already exists"), "duplicate error not shown");
    assert.deepEqual(rows(), ["admin", "alice", "bob"]);

    type(login, "forbidden");
    buttonIn(form(), "Create").click();
    await settle();
    assert.ok(form()?.textContent.includes(FORBIDDEN), "a 403 from createUser was not shown");

    buttonIn(form(), "Cancel").click();
    await settle();
    assert.equal(form(), null);
    await page.done();
  }

  // Delete removes the row.
  {
    const page = await usersPage(["admin", "alice", "bob"]);
    buttonIn(rowOf("bob"), "Delete").click();
    await settle();
    buttonIn(doc.querySelector(".modalContainer"), "Delete").click();
    await settle();
    assert.deepEqual(bodiesOf("deleteUser"), [{ login: "bob" }]);
    assert.equal(doc.querySelector(".modalContainer"), null, "the delete dialog stayed open");
    assert.deepEqual(rows(), ["admin", "alice"], "the deleted user is still listed");
    await page.done();
  }

  console.log("Users page tests passed");
});
