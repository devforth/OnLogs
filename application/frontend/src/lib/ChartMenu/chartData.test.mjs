import assert from "node:assert/strict";
import { main } from "../../../test/harness.mjs";
import { orderBuckets, toChartData, totalLines, LEVELS } from "./chartData.js";

const RESPONSE = {
  "2026-08-08T14:00Z": { debug: 1, error: 2, info: 3, meta: 4, other: 5, warn: 6 },
  "2026-08-08T05:00Z": { debug: 0, error: 0, info: 0, meta: 1, other: 0, warn: 0 },
  now: { debug: 0, error: 7, info: 0, meta: 0, other: 0, warn: 0 },
};

main(async () => {
  assert.deepEqual(
    orderBuckets(RESPONSE),
    ["2026-08-08T05:00Z", "2026-08-08T14:00Z", "now"],
    "buckets sort oldest-first with now pinned to the right edge"
  );

  assert.deepEqual(orderBuckets({}), [], "an empty response yields no buckets");
  assert.deepEqual(
    orderBuckets({ now: {} }),
    ["now"],
    "a response holding only the live interval still charts it"
  );

  const data = toChartData(RESPONSE, "hour");

  assert.equal(
    data.datasets.length,
    6,
    "every severity gets a series -- meta is the only non-zero level on many containers"
  );
  assert.deepEqual(
    data.datasets.map((d) => d.label),
    ["Error", "Warn", "Info", "Debug", "Meta", "Other"]
  );

  const meta = data.datasets.find((d) => d.label === "Meta");
  assert.deepEqual(meta.data, [1, 4, 0], "series values follow the bucket order");

  const error = data.datasets.find((d) => d.label === "Error");
  assert.deepEqual(error.data, [0, 2, 7], "the live interval is the last point");

  assert.equal(data.labels.at(-1), "now", "the live interval is labelled, not timestamped");
  assert.equal(data.labels.length, 3);
  assert.ok(
    !data.labels.slice(0, -1).includes("1"),
    "labels are real timestamps, not the placeholder the emulated data used"
  );

  const sparse = toChartData({ "2026-08-08T14:00Z": { error: 3 } }, "hour");
  for (const series of sparse.datasets) {
    assert.ok(
      series.data.every((n) => Number.isFinite(n)),
      `${series.label} produced a non-numeric point from a sparse bucket`
    );
  }

  // 21 in the 14:00 bucket + 1 in the 05:00 bucket + 7 live.
  assert.equal(totalLines(RESPONSE), 29, "totals sum every level across every bucket");
  assert.equal(totalLines({}), 0, "an empty response reports nothing to draw");
  assert.equal(
    totalLines({ "2026-08-08T14:00Z": { debug: 0, error: 0 } }),
    0,
    "all-zero buckets count as nothing to draw, so the empty state shows"
  );

  assert.equal(LEVELS.length, 6);
  console.log(`  ${data.labels.length} buckets, ${data.datasets.length} series, ${totalLines(RESPONSE)} lines`);
  console.log("chart data mapping tests passed");
});
