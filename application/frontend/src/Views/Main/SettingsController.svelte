<script>
  import { onMount } from "svelte";
  import { store } from "../../Stores/stores";
  import fetchApi from "../../utils/fetch";

  const apiFetch = new fetchApi();
  async function saveSettings() {
    if (initialSettingsGetted) {
      const data = await apiFetch.updateSettings($store);
    }
  }
  async function getSettings() {
    const data = await apiFetch.getUserSettings();
    if (data) {
      store.set(data);
    }
  }

  let initialSettingsGetted = false;
  onMount(async () => {
    await getSettings();
    initialSettingsGetted = true;
  });

  $: {
    if ($store) {
      saveSettings();
    }
  }
</script>
