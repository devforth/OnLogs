<script>
  import { onMount, onDestroy } from "svelte";
  let initialValue = true;
  import { store } from "../../Stores/stores.js";
  let { active = $bindable(true), storeValue = "" } = $props();
  let unsubscribe = () => {};

  onMount(() => {
    unsubscribe = store.subscribe((v) => (initialValue = v[storeValue]));
    active = initialValue;
  });
  onDestroy(() => unsubscribe());

  function handleClick() {
    active = !active;
    store.update((pv) => {
      return { ...pv, [storeValue]: active };
    });
  }
</script>

<div
  class="checkboxContainer {active ? 'active' : 'inactive'}"
  onclick={handleClick}
>
  <div class="checkboxRoll"></div>
</div>
