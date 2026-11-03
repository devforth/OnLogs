<script>
  import { onMount } from "svelte";
  import { store, toast, toastIsVisible } from "../../Stores/stores";
  import fetchApi from "../../utils/fetch";

  const apiFetch = new fetchApi();
  // Last settings the server accepted; setting them back skips the save, so no loop.
  let saved = $store;
  // One save at a time, so answers come back in order; changes made meanwhile
  // wait in the store and go out as one save when the answer arrives.
  let saving = false;
  async function saveSettings() {
    const sent = $store;
    if (!initialSettingsGetted || saving || sent === saved) {
      return;
    }
    saving = true;
    let data;
    try {
      data = await apiFetch.updateSettings(sent);
    } catch {
      data = { error: "Something went wrong. Please try again later" };
    }
    saving = false;
    if (data && !data.error) {
      saved = sent;
    }
    // null: the session ended.
    if (!data) {
      return;
    }
    // The waiting change carries every setting, so its answer settles this one too.
    if ($store !== sent) {
      return saveSettings();
    }
    if (!data.error) {
      return;
    }
    store.set(saved);
    toast.set({ tittle: "Error", message: data.error, position: "", status: "Error" });
    toastIsVisible.set(true);
  }
  async function getSettings() {
    const data = await apiFetch.getUserSettings();
    if (data) {
      saved = data;
      store.set(data);
    }
  }

  let initialSettingsGetted = false;
  onMount(async () => {
    await getSettings();
    initialSettingsGetted = true;
  });

  $effect(() => {
    if ($store) {
      saveSettings();
    }
  });
</script>
