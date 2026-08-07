<script>
  import { onMount } from "svelte";
  /**
   * @typedef {Object} Props
   * @property {string} [ico]
   * @property {any} [callBack]
   * @property {any} [number]
   */

  /** @type {Props} */
  let { ico = "", callBack = () => {}, number = null } = $props();
</script>

<div
  class="buttonToBottomContainer buttonToBottom"
  onclick={async () => {
    await callBack();
  }}
>
  <div class="icoContainer buttonToBottom">
    <i class="log log-Down buttonToBottom"></i>
  </div>
  {#if number}<div class="buttonToBottomNumber flex buttonToBottom">
      <p>{number}</p>
    </div>
  {/if}
</div>
