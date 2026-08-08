import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  importBundle,
  settle,
  jsonResponse,
  main,
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
      { host: "host1", service: "removed" },
    ],
  },
];

function textOf(node) {
  return node.textContent.replace(/\s+/g, " ").trim();
}

main(async () => {
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

  assert.ok(
    !rows[0].querySelector("p").className.includes("disabled"),
    "a live member should not render disabled"
  );
  assert.ok(
    rows[2].querySelector("p").className.includes("disabled"),
    "a member that no longer exists on any host should render disabled"
  );

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

  assert.ok(
    textOf(target).includes("host1"),
    "the host tree should still render below the groups"
  );

  bundle.unmount(app);
  bundle.groups.set([]);

  // The active and stopped service lists render from one shared snippet; their
  // rows differ only by an id suffix, so both lists have to be checked.
  const MIXED = [
    {
      host: "host1",
      services: [
        { serviceName: "api", isDisabled: false, isFavorite: true },
        { serviceName: "old", isDisabled: true, isFavorite: false },
      ],
    },
  ];
  const tree = bundle.mount(bundle.ListWithChoise, {
    target,
    props: { listData: MIXED, listElementButton: "true" },
  });
  await settle();

  const lists = [...target.querySelectorAll("ul.activeServices")];
  assert.equal(lists.length, 2, "expected an active list and a stopped list");

  const active = [...lists[0].querySelectorAll(".serviceListItem")];
  const stopped = [...lists[1].querySelectorAll(".serviceListItem")];
  console.log(
    `  active: ${active.map((r) => textOf(r.querySelector("p")))}, ` +
      `stopped: ${stopped.map((r) => textOf(r.querySelector("p")))}`
  );
  assert.deepEqual(active.map((r) => textOf(r.querySelector("p"))), ["api"]);
  assert.deepEqual(stopped.map((r) => textOf(r.querySelector("p"))), ["old"]);

  assert.ok(
    target.querySelector("#heartButton-0"),
    "the active row lost its heart button id"
  );
  assert.ok(
    target.querySelector("#heartButtonDissabled-1"),
    "the stopped row lost its suffixed heart button id"
  );
  assert.ok(
    target.querySelector("#heartButton-0").className.includes("log-Heart"),
    "a favourited service should render a filled heart"
  );
  assert.ok(
    stopped[0].querySelector("p").className.includes("disabled"),
    "a stopped service should render disabled"
  );

  bundle.unmount(tree);
  console.log("ListWithChoise rendering tests passed");
});
