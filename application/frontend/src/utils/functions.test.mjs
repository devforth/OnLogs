import assert from "node:assert/strict";
import { tryToParseLogString, getTimeDifference } from "./functions.js";

const PAYLOAD = '<img src=x onerror=alert(document.domain)>';

function assertNoMarkupReachesHtmlSink(line, label) {
  const parsed = tryToParseLogString(line);
  assert.ok(parsed, `${label}: expected the JSON to be recognised`);

  for (const [field, value] of Object.entries(parsed)) {
    if (field === "json") continue;
    assert.equal(
      typeof value === "string" && value.includes("<img"),
      false,
      `${label}: unescaped markup reaches {@html} via "${field}": ${value}`
    );
  }
  return parsed;
}

function run() {
  const keyLine = `prefix {"${PAYLOAD}":"safe value"} suffix`;
  const parsedKey = assertNoMarkupReachesHtmlSink(keyLine, "key position");

  const valueLine = `prefix {"k":"${PAYLOAD}"} suffix`;
  assertNoMarkupReachesHtmlSink(valueLine, "value position");

  const nestedLine = `{"outer":{"${PAYLOAD}":1}}`;
  assertNoMarkupReachesHtmlSink(nestedLine, "nested key");

  assert.ok(parsedKey.json, "the parsed object must be exposed for text rendering");
  const text = JSON.stringify(parsedKey.json, null, 2);
  assert.ok(
    text.includes(PAYLOAD),
    "the attacker-controlled key must still be shown to the operator, as text"
  );

  assert.equal(parsedKey.startText, "prefix ");
  assert.equal(parsedKey.endText, " suffix");

  assert.equal(tryToParseLogString("plain log line"), null);
  assert.equal(tryToParseLogString("not json { at all"), null);

  // Every row is parsed on each render, so a long JSON line must stay cheap.
  const big = "INFO payload " + JSON.stringify(Array.from({ length: 4000 }, (_, i) => ({ id: i, tags: ["a"], meta: { k: i } })));
  const started = performance.now();
  assert.equal(tryToParseLogString(big).json.length, 4000);
  const took = performance.now() - started;
  assert.ok(took < 20, `parsing a ${Math.round(big.length / 1024)} KB JSON line took ${took.toFixed(0)} ms`);

  const ago = (seconds) => new Date(Date.now() - seconds * 1000).toISOString();
  assert.deepEqual(getTimeDifference(ago(26 * 3600 + 60)), [[1, "d"], [2, "h"], [1, "m"]]);
  assert.deepEqual(getTimeDifference(ago(3 * 3600 + 120)), [[3, "h"], [2, "m"]]);
  assert.deepEqual(getTimeDifference(ago(10)), [[0, "h"], [0, "m"]]);
  assert.deepEqual(getTimeDifference(""), ["", ""]);

  console.log("functions tests passed");
}

run();
