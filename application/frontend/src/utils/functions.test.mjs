import assert from "node:assert/strict";
import { tryToParseLogString } from "./functions.js";

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

  console.log("functions tests passed");
}

run();
