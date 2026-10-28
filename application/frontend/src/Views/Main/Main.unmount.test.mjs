import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

const HOSTS = [
  { host: "edge-1", services: [{ serviceName: "bigline" }] },
  { host: "main-host", services: [{ serviceName: "worker" }] },
];

async function fetchImpl(url) {
  const name = String(url).split("/api/v1/")[1]?.split("?")[0];
  if (name === "getHosts") {
    await new Promise((resolve) => setTimeout(resolve, 30));
    return jsonResponse(HOSTS);
  }
  if (name === "getGroups") return jsonResponse([]);
  if (name === "getHostAliases") return jsonResponse({});
  if (/Logs/.test(name)) return jsonResponse({ logs: [], last_processed_key: "", is_end: true });
  return jsonResponse({ error: null });
}

const read = (s) => {
  let value;
  s.subscribe((v) => (value = v))();
  return value;
};

main(async () => {
  installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/usersEntry.js"));
  bundle.lastChosenHost.set("main-host");
  bundle.lastChosenService.set("gone");

  // The "/" page a login lands on is replaced by /view/... before its hosts arrive.
  window.history.replaceState({}, "", `${bundle.changeKey}/`);
  const page = bundle.mount(bundle.Main, { target: document.body, props: {} });
  await settle(5);
  bundle.unmount(page);
  await new Promise((resolve) => setTimeout(resolve, 80));
  await settle(5);
  assert.equal(read(bundle.lastChosenHost), "main-host", "a destroyed page changed the chosen host");
  assert.equal(read(bundle.lastChosenService), "gone", "a destroyed page changed the chosen service");

  console.log("Main unmount tests passed");
});
