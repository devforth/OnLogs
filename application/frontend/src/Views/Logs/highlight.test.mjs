import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { toAnsiHtml } from "../../utils/ansi.js";

function setupDom(html = "") {
  const dom = new JSDOM(`<!doctype html><html><body>${html}</body></html>`, {
    url: "http://localhost:5173/",
  });

  global.window = dom.window;
  global.document = dom.window.document;
  global.NodeFilter = dom.window.NodeFilter;
  global.Node = dom.window.Node;
  global.Range = dom.window.Range;

  return dom;
}

function teardownDom() {
  delete global.window;
  delete global.document;
  delete global.NodeFilter;
  delete global.Node;
  delete global.Range;
}

function runCase(name, testFn) {
  try {
    testFn();
  } finally {
    teardownDom();
  }
}

async function run() {
  // Import after DOM globals exist.
  setupDom();
  const { findSearchTextInLogs } = await import("../../utils/highlight.js");
  teardownDom();

  // Cross-node ANSI case: "WARNING" in one ANSI span, " d" in another text node.
  runCase("cross-node ansi match", () => {
    setupDom(
      `<p class="message">${toAnsiHtml("\u001b[33mWARNING\u001b[0m detail message")}</p>`
    );
    findSearchTextInLogs(".message", "WARNING d", true);
    const matches = document.querySelectorAll(".searchedText");
    assert.equal(matches.length, 1);
    assert.equal(matches[0].textContent, "WARNING d");
  });

  // Special regex chars should be treated literally.
  runCase("literal special regex characters", () => {
    setupDom(`<p class="message">WARN (id=42) [db.*]</p>`);
    findSearchTextInLogs(".message", "[db.*]", true);
    const matches = document.querySelectorAll(".searchedText");
    assert.equal(matches.length, 1);
    assert.equal(matches[0].textContent, "[db.*]");
  });

  // Case-insensitive search should find mixed case text.
  runCase("case-insensitive matching", () => {
    setupDom(`<p class="message">Warning Detail</p>`);
    findSearchTextInLogs(".message", "warning detail", false);
    const matches = document.querySelectorAll(".searchedText");
    assert.equal(matches.length, 1);
    assert.equal(matches[0].textContent, "Warning Detail");
  });

  // Re-running with changed query should replace previous highlights.
  runCase("query replacement behavior", () => {
    setupDom(`<p class="message">WARNING detail WARNING next</p>`);
    findSearchTextInLogs(".message", "WARNING", true);
    assert.equal(document.querySelectorAll(".searchedText").length, 2);

    findSearchTextInLogs(".message", "detail", true);
    const matches = document.querySelectorAll(".searchedText");
    assert.equal(matches.length, 1);
    assert.equal(matches[0].textContent, "detail");
  });

  // ANSI + newline + DIM/BOLD sequence should still highlight across boundary.
  runCase("ansi newline dim/bold boundary", () => {
    const ansiLine =
      "\u001b[33mWARNING\u001b[0m\n\u001b[2m\u001b[1mDETAIL\u001b[0m end";
    setupDom(`<p class="message">${toAnsiHtml(ansiLine)}</p>`);
    findSearchTextInLogs(".message", "WARNING\nDETAIL", true);
    const matches = document.querySelectorAll(".searchedText");
    assert.equal(matches.length, 1);
    assert.equal(matches[0].textContent, "WARNING\nDETAIL");
  });

  // Realistic rendered ANSI HTML with nested tags and mixed whitespace.
  runCase("mixed whitespace across ansi segments", () => {
    setupDom(`
      <p class="message"><span style="color:#A50">WARNING<b> AzL4Y8oR KsTdiwHodbZ0i \tmOK2Wz aF6UXv5KjPaqfO rk4ND9eAdluoci YyBTR 1Yz7A09 uSOcF6OUYB VUBGZGjWuJ \t<span style="color:#0A0">bTNSP</span></b></span> \tn4WFZFy92 nV2IJ4SA0RPZ JjNiiH1N yOEN Cfy 3DJO5uv wL2einh eF4yPL \t1gISzRyK1JR \tajoJ m4uY6Jpk2WA HXZGfuae6pG \tvy7RFkJZL0Az \t<span style="color:#A0A">sLvq</span></p>
    `);

    findSearchTextInLogs(".message", "WARNING Az", true);
    assert.equal(document.querySelectorAll(".searchedText").length, 1);

    findSearchTextInLogs(".message", "KsTdiwHodbZ0i m", true);
    assert.equal(document.querySelectorAll(".searchedText").length, 1);

    findSearchTextInLogs(".message", "AzL4Y8oR KsTdiwHodbZ0i m", true);
    assert.equal(document.querySelectorAll(".searchedText").length, 1);
  });

  // Performance-oriented scenario: many rendered log rows.
  runCase("high-volume highlighting remains responsive", () => {
    const rowsCount = 800;
    const rows = new Array(rowsCount).fill(0).map((_, i) => {
      const payload = toAnsiHtml(
        `\u001b[33mWARNING\u001b[0m detail ${i} \u001b[36mtrace-${i}\u001b[0m`
      );
      return `<p class="message">${payload}</p>`;
    });
    setupDom(rows.join(""));

    const t0 = Date.now();
    findSearchTextInLogs(".message", "WARNING detail", true);
    const t1 = Date.now();

    const firstPassMatches = document.querySelectorAll(".searchedText");
    assert.equal(firstPassMatches.length, rowsCount);

    // Re-run same query to exercise cache/early-return path.
    const t2 = Date.now();
    findSearchTextInLogs(".message", "WARNING detail", true);
    const t3 = Date.now();
    const secondPassMatches = document.querySelectorAll(".searchedText");
    assert.equal(secondPassMatches.length, rowsCount);

    const firstPassDuration = t1 - t0;
    const secondPassDuration = t3 - t2;

    // Guardrail to catch pathological regressions while avoiding timing flakiness.
    assert.ok(
      firstPassDuration < 10000,
      `highlight first pass took too long: ${firstPassDuration}ms`
    );
    assert.ok(
      secondPassDuration <= firstPassDuration,
      `second pass (${secondPassDuration}ms) should not be slower than first pass (${firstPassDuration}ms)`
    );
  });

  console.log("highlight tests passed");
}

run();
