// Drives the whole App: host polling, the Home links and the bottom panel.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const HOSTS = [{ host: "h1", services: [{ serviceName: "s1", isDisabled: false, isFavorite: false }] }];

let loggedIn = true;
async function stubFetch(url) {
  const path = String(url).split("/api/v1/")[1]?.split("?")[0];
  if (!loggedIn) return jsonResponse({ error: "unauthorized" }, 401);
  if (path === "getHosts") return jsonResponse(HOSTS);
  if (path === "getUsers") return jsonResponse({ users: [{ username: "admin", editable: false }], error: null });
  if (/getLogs|getPrevLogs|getLogWithPrev/.test(path)) {
    return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  }
  return jsonResponse({ error: null });
}

// Host polling runs every 10 s; anything still scheduled after unmount leaked.
const HOST_POLL_MS = 10000;
const polls = new Set();
const realSetInterval = globalThis.setInterval;
const realClearInterval = globalThis.clearInterval;
globalThis.setInterval = (fn, ms, ...args) => {
  const id = realSetInterval(fn, ms, ...args);
  if (ms === HOST_POLL_MS) polls.add(id);
  return id;
};
globalThis.clearInterval = (id) => {
  polls.delete(id);
  return realClearInterval(id);
};

main(async () => {
  const outfile = await bundleComponent("test/usersEntry.js");
  installDom({ fetchImpl: stubFetch });
  const bundle = await importBundle(outfile);

  async function mountAt(path, { auth = true } = {}) {
    const { window } = installDom({ fetchImpl: stubFetch });
    window.localStorage.clear();
    bundle.lastChosenHost.set("");
    bundle.lastChosenService.set("");
    loggedIn = auth;
    window.history.replaceState({}, "", `${bundle.changeKey}${path}`);
    const app = bundle.mount(bundle.App, {
      target: window.document.body,
      props: { url: `${bundle.changeKey}${path}` },
    });
    await settle();
    return { window, app };
  }
  const pathOf = (window) => window.location.pathname.slice(bundle.changeKey.length);
  const activeMenu = () => {
    let v;
    bundle.activeMenuOption.subscribe((x) => (v = x))();
    return v;
  };

  // A logged-out visit is sent to /login, and its Main must not keep polling.
  {
    const { window, app } = await mountAt("/users", { auth: false });
    assert.equal(pathOf(window), "/login");
    assert.equal(polls.size, 0, `${polls.size} host poller(s) left running on /login`);

    loggedIn = true;
    window.document.getElementById("login").value = "admin";
    window.document.getElementById("password").value = "pw";
    window.document.querySelector(".confirmButton button").click();
    await settle();
    assert.equal(pathOf(window), "/view/h1/s1");
    assert.equal(polls.size, 1, `${polls.size} host pollers after login`);
    bundle.unmount(app);
    await settle();
    assert.equal(polls.size, 0);
  }

  console.log("Main tests passed");
});
