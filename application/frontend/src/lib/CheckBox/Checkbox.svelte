<script>
  import { onMount, onDestroy } from "svelte";
  export let active = true;
  export let storeValue = "";
  let initialValue = true;
  import { store } from "../../Stores/stores.js";
  let unsubscribe = () => {};

  onMount(() => {
    unsubscribe = store.subscribe((v) => (initialValue = v[storeValue]));
    active = initialValue;
  });
  onDestroy(unsubscribe);

  function handleClick() {
    active = !active;
    store.update((pv) => {
      return { ...pv, [storeValue]: active };
    });
  }
</script>

<div
  class="checkboxContainer {active ? 'active' : 'inactive'}"
  on:click={handleClick}
>
  <div class="checkboxRoll" />
</div>
