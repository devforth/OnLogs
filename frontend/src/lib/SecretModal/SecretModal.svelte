<script>
  import Button from "../Button/Button.svelte";
  import {
    snipetModalIsVisible,
    addHostMenuIsVisible,
    toast,
    toastIsVisible,
  } from "../../Stores/stores";
  import { clickOutside } from "../../lib/OutsideClicker/OutsideClicker.js";
  import { onMount } from "svelte";
  import FetchApi from "../../utils/fetch.js";
  import { handleKeydown } from "../../utils/functions.js";

  let token = "";
  let origin = location.origin;
  const api = new FetchApi();
  async function getSecret() {
    try {
      const data = await api.getSecret();
      if (data.token) {
        return data.token;
      }
    } catch {
      return "2345678901234567";
    }
  }

  const copyText = function (ref, cb) {
    const text = ref;
    let textToCopy = text.innerText;
    if (navigator.clipboard) {
      navigator.clipboard.writeText(textToCopy).then(() => {
        cb();
      });
    } else {
      console.log("Browser Not compatible");
    }
  };

  onMount(async () => {
    token = await getSecret();
  });
</script>

<div
  class="secretModalContainer"
  use:clickOutside
  on:click_outside={() => {
    snipetModalIsVisible.set(false);
  }}
>
  <h3 class="secretMoalTitle">Connect new host</h3>
  <div class="snippetContainer">
    <pre class="secretSnippet">docker run devforth/onlogs
-e CLIENT=true
-e HOST={origin}
-e ONLOGS_TOKEN={token} </pre>
    <div class="coppyButton">
      <Button
        icon={"log log-Copy"}
        minHeight={40}
        minWidth={55}
        border={true}
        CB={() => {
          copyText(document.querySelector("pre"), () => {
            toast.set({
              tittle: "Success",
              message: "Text has been copied",
              position: "",
              status: "debug",
            });
            toastIsVisible.set(true);
            setTimeout(() => {
              toastIsVisible.set(false);
            }, 5000);
          });
        }}
      />
    </div>
  </div>
  <div class="buttonWrapper">
    <Button
      title={"Close"}
      highlighted={true}
      minWidth={100}
      CB={() => {
        snipetModalIsVisible.set(false);
        addHostMenuIsVisible;
      }}
    />
  </div>
</div>
<div class="modalOverlay" />
<svelte:window
  on:keydown={(e) => {
    handleKeydown(e, "Escape", () => {
      snipetModalIsVisible.set(false);
    });
  }}
/>
