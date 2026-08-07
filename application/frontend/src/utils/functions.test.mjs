import assert from "node:assert/strict";
import { tryToParseLogString } from "./functions.js";

const PAYLOAD = '<img src=x onerror=alert(document.domain)>';

function assertNoMarkupReachesHtmlSink(line, label) {
  const parsed = tryToParseLogString(line);
  assert.ok(parsed, `${label}: expected the JSON to be recognised`);

  // Whatever the component hands to {@html} must carry no live markup.
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
  // The payload sits in a KEY position, which json-to-html never escaped.
  const keyLine = `prefix {"${PAYLOAD}":"safe value"} suffix`;
  const parsedKey = assertNoMarkupReachesHtmlSink(keyLine, "key position");

  // The payload in a VALUE position was already escaped, but must stay safe.
  const valueLine = `prefix {"k":"${PAYLOAD}"} suffix`;
  assertNoMarkupReachesHtmlSink(valueLine, "value position");

  // Nested keys are the same sink.
  const nestedLine = `{"outer":{"${PAYLOAD}":1}}`;
  assertNoMarkupReachesHtmlSink(nestedLine, "nested key");

  // The content must still be displayable, as text.
  assert.ok(parsedKey.json, "the parsed object must be exposed for text rendering");
  const text = JSON.stringify(parsedKey.json, null, 2);
  assert.ok(
    text.includes(PAYLOAD),
    "the attacker-controlled key must still be shown to the operator, as text"
  );

  assert.equal(parsedKey.startText, "prefix ");
  assert.equal(parsedKey.endText, " suffix");

  // Non-JSON lines are still passed through untouched.
  assert.equal(tryToParseLogString("plain log line"), null);
  assert.equal(tryToParseLogString("not json { at all"), null);

  console.log("functions tests passed");
}

run();
