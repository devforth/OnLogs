import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
  settle,
  jsonResponse,
  main,
} from "../../../test/harness.mjs";

const LOADED = {
  UTCtime: false,
  breakLines: true,
  caseInSensitive: true,
  transformJson: true,
  deleteFromDocker: false,
};

// The server applies each save as it arrives; the test decides when the
// answers come back.
let server = { ...LOADED };
let refuse = () => false;
let sent = 0;
const answers = [];
async function fetchImpl(url, init) {
  const name = String(url).split("/").pop();
  if (name === "getUserSettings") return jsonResponse({ ...server });
  if (name !== "updateUserSettings") return jsonResponse({ error: null });
  sent += 1;
  const body = JSON.parse(init.body);
  const refused = refuse(body);
  if (!refused) server = body;
  const response = refused
    ? jsonResponse({ error: "settings database is unavailable" }, 500)
    : jsonResponse({ error: null });
  return new Promise((resolve) => answers.push(() => resolve(response)));
}

async function answerNewestFirst() {
  while (answers.length) {
    answers.pop()();
    await settle();
  }
}

const read = (s) => {
  let value;
  s.subscribe((v) => (value = v))();
  return value;
};

main(async () => {
  const { window } = installDom({ fetchImpl });
  const bundle = await importBundle(await bundleComponent("test/entry.js"));
  const target = window.document.body;
  const change = async (patch) => {
    bundle.store.update((s) => ({ ...s, ...patch }));
    await settle();
  };
  const scenario = async (name, body) => {
    server = { ...LOADED };
    sent = 0;
    const controller = bundle.mount(bundle.SettingsController, { target });
    await settle();
    await body();
    const ui = read(bundle.store);
    console.log(`  ${name}: UI ${JSON.stringify(ui)} | server ${JSON.stringify(server)} | saves ${sent}`);
    assert.deepEqual(ui, server, `${name}: the switches disagree with the server`);
    bundle.unmount(controller);
    bundle.toastIsVisible.dismiss();
  };

  // A refused later change must not undo an earlier one the server accepted.
  await scenario("refused after accepted", async () => {
    refuse = (s) => s.breakLines === false;
    await change({ UTCtime: true });
    await change({ breakLines: false });
    await answerNewestFirst();
    assert.equal(server.UTCtime, true);
    assert.equal(read(bundle.toastIsVisible), true, "the refusal showed no toast");
    assert.equal(sent, 2, "a save was repeated");
  });

  // Accepted answers that arrive out of order must not leave an older state
  // for the next refusal to return to.
  await scenario("out-of-order answers", async () => {
    refuse = (s) => s.caseInSensitive === false;
    await change({ UTCtime: true });
    await change({ transformJson: false });
    await answerNewestFirst();
    await change({ caseInSensitive: false });
    await answerNewestFirst();
    assert.equal(server.transformJson, false);
    assert.equal(read(bundle.toastIsVisible), true, "the refusal showed no toast");
  });

  console.log("SettingsController order tests passed");
});
