process.env.TZ = "America/New_York";

import assert from "node:assert/strict";
import { main } from "../../../test/harness.mjs";
import { toChartData } from "./chartData.js";

const inUTC = (bucket, opts) => new Date(bucket).toLocaleString([], { ...opts, timeZone: "UTC" });
const DAY = { month: "short", day: "2-digit" };
const MONTH = { month: "short", year: "numeric" };
const HOUR = { hour: "2-digit", minute: "2-digit" };

main(async () => {
  const day = toChartData(
    { "2026-10-18T00:00Z": { error: 4 }, "2026-10-19T00:00Z": { error: 7 }, now: { error: 0 } },
    "day"
  );
  assert.deepEqual(
    day.labels,
    [inUTC("2026-10-18T00:00Z", DAY), inUTC("2026-10-19T00:00Z", DAY), "now"],
    "day bars west of UTC are labelled with the UTC day they count"
  );

  const month = toChartData(
    {
      "2025-12-01T00:00Z": { error: 3 },
      "2026-01-01T00:00Z": { error: 5 },
      "2026-10-01T00:00Z": { error: 9 },
      now: { error: 0 },
    },
    "month"
  );
  assert.deepEqual(
    month.labels,
    [
      inUTC("2025-12-01T00:00Z", MONTH),
      inUTC("2026-01-01T00:00Z", MONTH),
      inUTC("2026-10-01T00:00Z", MONTH),
      "now",
    ],
    "month bars west of UTC are labelled with the UTC month and year they count"
  );

  const hour = toChartData({ "2026-10-19T14:00Z": { error: 1 } }, "hour");
  assert.deepEqual(
    hour.labels,
    [new Date("2026-10-19T14:00Z").toLocaleString([], HOUR)],
    "hour bars keep showing the bucket start in the browser's zone"
  );
  assert.notEqual(hour.labels[0], inUTC("2026-10-19T14:00Z", HOUR));

  console.log("chart labels name the UTC day and month of their buckets");
});
