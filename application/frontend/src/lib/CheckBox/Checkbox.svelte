<script>
  import { onMount, onDestroy } from "svelte";
  import { store } from "../../Stores/stores.js";
  let { active = $bindable(true), storeValue = "", disabled = false } = $props();
  let unsubscribe = () => {};

  onMount(() => {
    unsubscribe = store.subscribe((v) => (active = v[storeValue]));
  });
  onDestroy(() => unsubscribe());

  function handleClick() {
    if (disabled) {
      return;
    }
    active = !active;
    store.update((pv) => {
      return { ...pv, [storeValue]: active };
    });
  }
</script>

<div
  class="checkboxContainer {active && !disabled ? 'active' : 'inactive'} {disabled ? 'disabled' : ''}"
  onclick={handleClick}
>
  <div class="checkboxRoll"></div>
</div>
