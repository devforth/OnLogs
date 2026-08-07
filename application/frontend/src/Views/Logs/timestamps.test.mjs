process.env.TZ = "Europe/Kyiv";

import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
} from "../../../test/harness.mjs";

installDom({});
const { transformLogString, transformLogStringForTimeBudget } =
  await importBundle(await bundleComponent("test/entry.js"));

const row = (ts) => [ts, "a message", ts + " +1-1"];

function run() {
  const summer = "2026-07-15T12:44:03.567891234Z";
  const winter = "2026-01-15T12:44:03.567891234Z";

  // Milliseconds must survive: without them, two distinct rows look identical
  // and a genuine duplicate is indistinguishable from two adjacent lines.
  assert.equal(
    transformLogString(row(summer), true),
    "12:44:03.567",
    "UTC mode truncates the millisecond"
  );
  assert.equal(
    transformLogString(row(summer), false),
    "15:44:03.567",
    "local mode loses the millisecond"
  );

  const a = transformLogString(row("2026-07-15T12:44:03.560000000Z"), false);
  const b = transformLogString(row("2026-07-15T12:44:03.561000000Z"), false);
  assert.notEqual(a, b, `rows 1ms apart render identically: ${a} == ${b}`);

  // Europe/Kyiv is UTC+3 in July and UTC+2 in January. A single offset captured
  // at module load makes everything across a DST boundary an hour wrong.
  assert.equal(
    transformLogString(row(winter), false),
    "14:44:03.567",
    "the timezone offset is not computed per timestamp"
  );

  assert.equal(transformLogStringForTimeBudget(row(summer), true), "Jul 15 2026");
  assert.equal(transformLogStringForTimeBudget(row(winter), false), "Jan 15 2026");

  // A date that falls on the previous day in UTC but the next locally.
  const lateEvening = "2026-07-15T22:30:00.000000000Z";
  assert.equal(transformLogStringForTimeBudget(row(lateEvening), true), "Jul 15 2026");
  assert.equal(transformLogStringForTimeBudget(row(lateEvening), false), "Jul 16 2026");

  // Degenerate input must not throw.
  assert.equal(transformLogString(undefined, true), "");
  assert.equal(transformLogString(["not a date", "m"], true), "");
  assert.equal(transformLogStringForTimeBudget(undefined, true), "");

  console.log("timestamp rendering tests passed");
}

run();
