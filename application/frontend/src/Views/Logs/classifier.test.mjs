// Shared with backend/app/containerdb/classifier_test.go: both implementations
// must agree on every one of these, or the status filter hides lines the UI
// paints red.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
} from "../../../test/harness.mjs";

installDom({});
const { classifyLogLine, getLogLineStatus } = await importBundle(
  await bundleComponent("test/entry.js")
);

const ESC = "";

const CASES = [
  ["ERROR something failed", "error"],
  ["Error: connection refused", "error"],
  ["error: connection refused", "error"],
  ["ERR disk full", "error"],
  ["WARN low memory", "warn"],
  ["WARNING low memory", "warn"],
  ["warning low memory", "warn"],
  ["DEBUG entering loop", "debug"],
  ["debug entering loop", "debug"],
  ["INFO started", "info"],
  ["INFO ERROR_COUNT=0", "info"],
  ["ONLOGS: Container listening started!", "meta"],
  ["plain application output", "other"],
  ["", "other"],
  [`${ESC}[31mERROR${ESC}[0m red text`, "error"],
  ["the word information appears here", "info"],
  ["2026-02-10 INFO ready", "info"],
];

function run() {
  for (const [line, expected] of CASES) {
    assert.equal(
      classifyLogLine(line),
      expected,
      `classifyLogLine(${JSON.stringify(line)}) should be ${expected}`
    );
  }

  // The badge stays hidden for unclassified lines, as before.
  assert.equal(getLogLineStatus("plain application output"), "");
  assert.equal(getLogLineStatus("ERROR boom"), "error");

  console.log("log level classifier tests passed");
}

run();
