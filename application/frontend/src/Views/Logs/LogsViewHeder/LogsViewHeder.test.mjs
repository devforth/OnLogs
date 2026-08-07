// The search input must be debounced. LogsViewHeder defines a `debounce` helper
// and never wires it up, so every keystroke starts a full reload.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  settle,
  importBundle,
} from "../../../../test/harness.mjs";

const DEBOUNCE_MS = 750;

function type(window, input, value) {
  input.value = value;
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
}

async function run() {
  const bundlePath = await bundleComponent("test/entry.js");
  const { window } = installDom({});
  const { LogsViewHeder } = await importBundle(bundlePath);

  const target = window.document.body;
  const component = new LogsViewHeder({
    target,
    props: { searchText: "", searchResetVersion: 0 },
  });

  const observed = [];
  component.$on("searchTextChanged", () => {});

  const input = target.querySelector("input[type=text]");
  assert.ok(input, "the search input should be rendered");

  // Type a word one character at a time, as a user would.
  const word = "error";
  for (let i = 1; i <= word.length; i++) {
    type(window, input, word.slice(0, i));
    await settle(2);
    observed.push(component.searchText);
  }

  console.log(`  searchText after ${word.length} keystrokes: ${JSON.stringify(observed)}`);

  assert.deepEqual(
    observed,
    new Array(word.length).fill(""),
    `each keystroke updated searchText immediately (${JSON.stringify(observed)}), ` +
      `so every character starts a full reload`
  );

  // After the debounce window it must land exactly once, on the final value.
  await new Promise((resolve) => setTimeout(resolve, DEBOUNCE_MS + 250));
  assert.equal(
    component.searchText,
    word,
    `searchText should be ${JSON.stringify(word)} after the debounce window, got ` +
      JSON.stringify(component.searchText)
  );

  component.$destroy();
  console.log("LogsViewHeder debounce tests passed");
  process.exit(0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
