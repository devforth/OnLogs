import assert from "node:assert/strict";
import { stripAnsi, toAnsiHtml } from "./ansi.js";

function run() {
  const red = "\u001b[31mHello World\u001b[0m";
  const redHtml = toAnsiHtml(red);
  assert.ok(redHtml.includes("<span"));
  assert.ok(redHtml.includes("Hello World"));
  assert.equal(stripAnsi(red), "Hello World");

  const mixed = "prefix \u001b[33mwarn\u001b[0m suffix";
  const mixedHtml = toAnsiHtml(mixed);
  assert.ok(mixedHtml.includes("prefix "));
  assert.ok(mixedHtml.includes("<span"));
  assert.ok(mixedHtml.includes("warn"));
  assert.ok(mixedHtml.includes(" suffix"));

  const boldBlue = "\u001b[1;34mBLUE\u001b[22;39m plain";
  const boldBlueHtml = toAnsiHtml(boldBlue);
  assert.ok(boldBlueHtml.includes("<span"));
  assert.ok(boldBlueHtml.includes("plain"));

  const unsafe = '\u001b[31m<script>alert("x")</script>\u001b[0m';
  const unsafeHtml = toAnsiHtml(unsafe);
  assert.ok(!unsafeHtml.includes("<script>"));
  assert.ok(unsafeHtml.includes("&lt;script&gt;"));

  const unknown = "\u001b[999mX\u001b[0m";
  assert.ok(toAnsiHtml(unknown).includes("X"));

  console.log("ansi tests passed");
}

run();
