import assert from "node:assert/strict";
import {
  applySharedLogUrl,
  buildSharedLogUrl,
  getSharedLogHash,
} from "./shareLink.js";

const timestamp = "2026-03-09T08:00:00.123456789Z";

assert.equal(
  getSharedLogHash(timestamp),
  "#2026-03-09T08:00:00.123456789Z"
);

assert.equal(
  getSharedLogHash(`#${timestamp}`),
  "#2026-03-09T08:00:00.123456789Z"
);

assert.equal(
  buildSharedLogUrl(
    "http://localhost:5173/view/host/service",
    timestamp
  ),
  `http://localhost:5173/view/host/service#${timestamp}`
);

assert.equal(
  buildSharedLogUrl(
    "http://localhost:5173/view/host/service#old-timestamp",
    timestamp
  ),
  `http://localhost:5173/view/host/service#${timestamp}`
);

const historyCalls = [];
const historyMock = {
  replaceState(...args) {
    historyCalls.push(args);
  },
};

const result = applySharedLogUrl(
  "http://localhost:5173/view/host/service#stale",
  timestamp,
  historyMock
);

assert.deepEqual(result, {
  nextUrl: `http://localhost:5173/view/host/service#${timestamp}`,
  hash: `#${timestamp}`,
});

assert.deepEqual(historyCalls, [
  [null, "", `http://localhost:5173/view/host/service#${timestamp}`],
]);

console.log("share link tests passed");
