import assert from "node:assert/strict";
import { installDom, main } from "../../../test/harness.mjs";

main(async () => {
  installDom({ fetchImpl: async () => new Response("null") });
  const { clickOutside } = await import("./OutsideClicker.js");
  document.body.innerHTML = `<div id="modal"><button id="inside"></button></div>
    <div class="toastContainer"><button id="toastClose"></button></div><button id="page"></button>`;
  const modal = document.getElementById("modal");
  let outside = 0;
  modal.addEventListener("click_outside", () => outside++);
  const action = clickOutside(modal);

  document.getElementById("inside").click();
  assert.equal(outside, 0);
  document.getElementById("toastClose").click();
  assert.equal(outside, 0, "closing a toast also closed the modal under it");
  document.getElementById("page").click();
  assert.equal(outside, 1);

  action.destroy();
  console.log("OutsideClicker tests passed");
});
