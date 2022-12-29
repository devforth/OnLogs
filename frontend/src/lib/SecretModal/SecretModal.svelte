<script>
  import Button from "../Button/Button.svelte";
  import {
    snipetModalIsVisible,
    addHostMenuIsVisible,
    toast,
    toastIsVisible,
  } from "../../Stores/stores";
  import { onMount } from "svelte";
  let token = "";
  let origin = location.origin;
  import FetchApi from "../../utils/fetch.js";
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

<div class="secretModalContainer">
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
