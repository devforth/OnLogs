import assert from "node:assert/strict";
import {
  hasSearchResetRequest,
  shouldAutoScrollLogs,
  shouldFlushBufferedLogs,
} from "./shareLinkViewState.js";

assert.equal(hasSearchResetRequest(0, 0), false);
assert.equal(hasSearchResetRequest(0, 1), true);
assert.equal(hasSearchResetRequest(3, 4), true);

assert.equal(shouldAutoScrollLogs(true, false), true);
assert.equal(shouldAutoScrollLogs(true, true), false);
assert.equal(shouldAutoScrollLogs(false, false), false);

assert.equal(shouldFlushBufferedLogs(0, false, false), false);
assert.equal(shouldFlushBufferedLogs(5, false, false), true);
assert.equal(shouldFlushBufferedLogs(5, true, false), false);
assert.equal(shouldFlushBufferedLogs(5, true, true), true);

console.log("share link view state tests passed");
