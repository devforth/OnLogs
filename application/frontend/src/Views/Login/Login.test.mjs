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
  let reply = () => jsonResponse({ error: "Wrong password or login!" });
  const stubFetch = async (url, opts = {}) => {
    if (String(url).includes("/login")) bodies.push(JSON.parse(opts.body));
    return reply();
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

  // A refusal that is not about the credentials says so.
  const shown = () => doc.querySelector(".loginForm")?.textContent ?? "";
  for (const [status, error] of [
    [429, "Too many failed login attempts. Try again later."],
    [403, "Cross-origin request rejected"],
  ]) {
    reply = () => jsonResponse({ error }, status);
    doc.querySelector(".confirmButton button").click();
    await settle();
    assert.ok(shown().includes(error), `a ${status} was shown as "${shown().match(/Wrong[^!]*!/)?.[0]}"`);
  }
  bundle.unmount(app);
  console.log("Login form tests passed");
});
