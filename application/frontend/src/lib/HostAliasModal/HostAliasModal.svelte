<script>
  import Modal from "../Modal/Modal.svelte";
  import Button from "../Button/Button.svelte";
  import FetchApi from "../../utils/fetch.js";
  import { reloadHostAliases } from "../../utils/hostAliases.js";
  import {
    hostBeingRenamed,
    hostAliases,
    toast,
    toastIsVisible,
  } from "../../Stores/stores.js";

  const api = new FetchApi();
  // Mirrors the server's limit; the server stays the authority.
  const MAX_NAME_LENGTH = 64;

  let alias = $state("");

  $effect(() => {
    const host = $hostBeingRenamed;
    if (host) {
      alias = $hostAliases[host] ?? "";
    }
  });

  function notify(status, tittle, message) {
    toast.set({ message, tittle, status, position: "" });
    toastIsVisible.set(true);
    setTimeout(() => toastIsVisible.set(false), 5000);
  }

  function close() {
    hostBeingRenamed.set(null);
  }

  async function save() {
    const trimmed = alias.trim();
    let data;
    try {
      data = await api.setHostAlias($hostBeingRenamed, trimmed);
    } catch (e) {
      notify("error", "Error", e.message);
      return;
    }
    if (!data || data.error) {
      notify("error", "Error", data?.error || "Could not save the display name.");
      return;
    }
    await reloadHostAliases();
    close();
  }
</script>

<Modal modalIsOpen={!!$hostBeingRenamed} closeFunction={close}>
  <div class="hostAliasModal">
    <h3>Rename host</h3>
    <p class="hostAliasReal">{$hostBeingRenamed}</p>

    <input
      class="hostAliasInput"
      type="text"
      maxlength={MAX_NAME_LENGTH}
      placeholder="Display name"
      bind:value={alias}
    />
    <p class="hostAliasHint">
      Shown instead of the machine's own hostname. Logs stay stored under
      the real name. Leave empty to show the real name again.
    </p>

    <div class="hostAliasButtons">
      <Button id="hostAliasSave" title="Save" highlighted minWidth={86} CB={save} />
    </div>
  </div>
</Modal>
