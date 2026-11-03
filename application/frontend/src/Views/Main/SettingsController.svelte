<script>
  import { onMount } from "svelte";
  import { store, toast, toastIsVisible } from "../../Stores/stores";
  import fetchApi from "../../utils/fetch";

  const apiFetch = new fetchApi();
  // Last settings the server accepted; setting them back skips the save, so no loop.
  let saved = $store;
  let latest = 0;
  async function saveSettings() {
    const sent = $store;
    if (!initialSettingsGetted || sent === saved) {
      return;
    }
    const mine = ++latest;
    let data;
    try {
      data = await apiFetch.updateSettings(sent);
    } catch {
      data = { error: "Something went wrong. Please try again later" };
    }
    if (data && !data.error) {
      saved = sent;
    }
    // null: the session ended; a newer change has its own answer coming.
    if (!data?.error || mine !== latest) {
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
