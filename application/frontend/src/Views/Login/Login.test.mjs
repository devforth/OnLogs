import assert from "node:assert/strict";
import {
  bundleComponent,
  installDom,
  jsonResponse,
  settle,
  importBundle,
  main,
} from "../../../test/harness.mjs";

main(async () => {
  const bodies = [];
  const stubFetch = async (url, opts = {}) => {
    if (String(url).includes("/login")) bodies.push(JSON.parse(opts.body));
    return jsonResponse({ error: "Wrong password or login!" });
  };
  const outfile = await bundleComponent("test/usersEntry.js");
  const { window } = installDom({ fetchImpl: stubFetch });
  const bundle = await importBundle(outfile);
  const doc = window.document;

  const app = bundle.mount(bundle.Login, { target: doc.body, props: {} });
  await settle();
  doc.getElementById("login").value = " admin ";
  doc.getElementById("password").value = " pass word ";
  doc.querySelector(".confirmButton button").click();
  await settle();

  // The login is trimmed, the password is sent exactly as typed.
  assert.deepEqual(bodies, [{ login: "admin", password: " pass word " }]);
  bundle.unmount(app);
  console.log("Login form tests passed");
});
