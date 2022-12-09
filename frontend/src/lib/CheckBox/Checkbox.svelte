<script>
  export let active = true;
  export let storeValue = "";
  import { store } from "../../Stores/stores.js";

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
