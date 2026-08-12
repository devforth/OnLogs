import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
} from "../../test/harness.mjs";

installDom({});
const { FetchApi } = await importBundle(await bundleComponent("test/entry.js"));

const CURSOR = "2026-11-07T12:44:07.370000000Z +1730980000000000000-42";
const SEARCH = "a+b&status=error#frag c";

function capturing() {
  const seen = [];
  globalThis.fetch = async (url) => {
    seen.push(String(url));
    return { status: 200, ok: true, json: async () => ({ logs: [], is_end: true }) };
  };
  return seen;
}

function queryOf(url) {
  return new URLSearchParams(url.slice(url.indexOf("?") + 1));
}

async function run() {
  const api = new FetchApi();

  let seen = capturing();
  await api.getLogs({
    containerName: "my container",
    hostName: "my host",
    search: SEARCH,
    startWith: CURSOR,
    limit: 30,
    status: "error",
    caseSens: true,
  });
  let params = queryOf(seen[0]);
  assert.equal(params.get("startWith"), CURSOR, `getLogs mangled the cursor: ${seen[0]}`);
  assert.equal(params.get("search"), SEARCH, "getLogs mangled the search term");
  assert.equal(params.get("id"), "my container");
  assert.equal(params.get("host"), "my host");
  assert.equal(params.get("status"), "error");

  seen = capturing();
  await api.getPrevLogs({
    containerName: "c",
    hostName: "h",
    search: SEARCH,
    startWith: CURSOR,
  });
  params = queryOf(seen[0]);
  assert.equal(params.get("startWith"), CURSOR, `getPrevLogs mangled the cursor: ${seen[0]}`);
  assert.equal(params.get("search"), SEARCH, "getPrevLogs mangled the search term");

  seen = capturing();
  await api.getLogsWithPrev({ containerName: "c", hostName: "h", startWith: CURSOR });
  params = queryOf(seen[0]);
  assert.equal(params.get("startWith"), CURSOR, `getLogWithPrev mangled the cursor: ${seen[0]}`);

  seen = capturing();
  await api.getServiceLogsSize("host with space", "service&name");
  params = queryOf(seen[0]);
  assert.equal(params.get("host"), "host with space");
  assert.equal(params.get("service"), "service&name");

  console.log("fetch query-building tests passed");
}

await run();
