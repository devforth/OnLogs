<script>
  import { clickOutside } from "../../lib/OutsideClicker/OutsideClicker.js";

  /**
   * @typedef {Object} Props
   * @property {boolean} [modalIsOpen]
   * @property {any} [storeProp]
   * @property {any} [closeFunction]
   * @property {import('svelte').Snippet} [children]
   */

  /** @type {Props} */
  let {
    modalIsOpen = false,
    storeProp = {},
    closeFunction = null,
    children
  } = $props();

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
    <div class="modalOverlay" id="modalOverlay"></div>
    <div class="modalContainer" use:clickOutside onclick_outside={closeModal}>
      <div class="closeButton" onclick={closeModal}>
        <i class="log log-Close"></i>
      </div>
      {@render children?.()}
    </div>
  </div>{/if}
<svelte:window onkeydown={handleKeydown} />
