import assert from "node:assert/strict";
import {
  hasSearchResetRequest,
  createLogsViewFlags,
} from "./shareLinkViewState.js";

// Live: LogsViewHeder disarms its pending debounce when the version bumps.
assert.equal(hasSearchResetRequest(0, 0), false);
assert.equal(hasSearchResetRequest(0, 1), true);
assert.equal(hasSearchResetRequest(3, 4), true);

// The autoscroll and buffer-flush helpers were inlined at their single call
// sites; the buffering they gated is covered end to end by NewLogsV2.test.mjs.

// beginDeepLink is the only producer of the skip flags now.
const flags = createLogsViewFlags();
assert.equal(flags.isDeepLinkPending(), false);
assert.equal(flags.consumeSearchSkip(), false);
assert.equal(flags.consumeStatusSkip(), false);

flags.beginDeepLink();
assert.equal(flags.isDeepLinkPending(), true);
// One skip each: the search and status blocks retrigger once after the reset.
assert.equal(flags.consumeSearchSkip(), true);
assert.equal(flags.consumeSearchSkip(), false);
assert.equal(flags.consumeStatusSkip(), true);
assert.equal(flags.consumeStatusSkip(), false);

// Skips are independent of the pending window: it stays open until endDeepLink.
assert.equal(flags.isDeepLinkPending(), true);
flags.endDeepLink();
assert.equal(flags.isDeepLinkPending(), false);

// Instances do not share state.
const other = createLogsViewFlags();
flags.beginDeepLink();
assert.equal(other.isDeepLinkPending(), false);
assert.equal(other.consumeSearchSkip(), false);

console.log("share link view state tests passed");
