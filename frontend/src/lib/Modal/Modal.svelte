<script>
  import { clickOutside } from "../../lib/OutsideClicker/OutsideClicker.js";

  export let modalIsOpen = false;
  export let storeProp = {};
  export let closeFunction = null;

  function handleKeydown(e) {
    if (e.key === "Escape") {
      closeModal();
    }
  }
  function closeModal() {
    (storeProp.set && storeProp.set(false)) ||
      (closeFunction && closeFunction());
  }
</script>

{#if modalIsOpen}
  <div>
    <div class="modalOverlay" id="modalOverlay" />
    <div class="modalContainer" use:clickOutside on:click_outside={closeModal}>
      <div class="closeButton" on:click={closeModal}>
        <i class="log log-Close" />
      </div>
      <slot />
    </div>
  </div>{/if}
<svelte:window on:keydown={handleKeydown} />
