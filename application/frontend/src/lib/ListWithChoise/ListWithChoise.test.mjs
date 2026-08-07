// The sidebar renders groups above the host tree, and a group member has to open
// its logs through the same click path a service row under its host uses.
import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
  settle,
  jsonResponse,
} from "../../../test/harness.mjs";

const HOSTS = [
  {
    host: "host1",
    services: [
      { serviceName: "api", isDisabled: false, isFavorite: false },
      { serviceName: "web", isDisabled: false, isFavorite: false },
    ],
  },
];

const GROUPS = [
  {
    name: "backend",
    members: [
      { host: "host1", service: "api" },
      { host: "host1", service: "web" },
      // Listed in the group but gone from every host.
      { host: "host1", service: "removed" },
    ],
  },
];

function textOf(node) {
  return node.textContent.replace(/\s+/g, " ").trim();
}

async function run() {
  const outfile = await bundleComponent("test/entry.js");
  const { window } = installDom({ fetchImpl: async () => jsonResponse({ error: null }) });
  const bundle = await importBundle(outfile);
  const target = window.document.body;

  bundle.groups.set(GROUPS);
  bundle.lastChosenHost.set("");
  bundle.lastChosenService.set("");

  const app = bundle.mount(bundle.ListWithChoise, {
    target,
    props: { listData: HOSTS },
  });
  await settle();

  const section = target.querySelector(".groupSection");
  assert.ok(section, "the Groups section did not render");
  assert.ok(
    textOf(section).includes("backend"),
    `the group heading is missing: ${textOf(section)}`
  );

  const rows = [...section.querySelectorAll(".serviceListItem")];
  const names = rows.map((row) => textOf(row.querySelector("p")));
  console.log(`  group rows: ${names.join(", ")}`);
  assert.deepEqual(
    names,
    ["api", "web", "removed"],
    "every member should render as a row under the group heading"
  );

  // A member whose service is gone gets the stopped-service treatment.
  assert.ok(
    !rows[0].querySelector("p").className.includes("disabled"),
    "a live member should not render disabled"
  );
  assert.ok(
    rows[2].querySelector("p").className.includes("disabled"),
    "a member that no longer exists on any host should render disabled"
  );

  // Clicking a member has to run the same path a host's service row runs.
  rows[1].querySelector("p").click();
  await settle();

  let chosenHost = "";
  let chosenService = "";
  bundle.lastChosenHost.subscribe((v) => (chosenHost = v))();
  bundle.lastChosenService.subscribe((v) => (chosenService = v))();

  console.log(`  after clicking "web": ${chosenHost}/${chosenService}`);
  assert.equal(chosenHost, "host1", "clicking a group member did not set the host");
  assert.equal(chosenService, "web", "clicking a group member did not set the service");
  assert.ok(
    window.location.pathname.endsWith("/view/host1/web"),
    `clicking a group member did not navigate: ${window.location.pathname}`
  );

  // The group section is a folder in the sidebar, not a replacement for it.
  assert.ok(
    textOf(target).includes("host1"),
    "the host tree should still render below the groups"
  );

  bundle.unmount(app);
  bundle.groups.set([]);
  console.log("ListWithChoise group rendering tests passed");
  process.exit(0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
