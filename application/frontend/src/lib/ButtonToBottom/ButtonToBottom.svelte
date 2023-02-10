<script>
  import { onMount } from "svelte";
  export let ico = "";
  export let callBack = () => {};
  export let number = null;
</script>

<div
  class="buttonToBottomContainer buttonToBottom"
  on:click={async () => {
    await callBack();
  }}
>
  <div class="icoContainer buttonToBottom">
    <i class="log log-Down buttonToBottom" />
  </div>
  {#if number}<div class="buttonToBottomNumber flex buttonToBottom">
      <p>{number}</p>
    </div>
  {/if}
</div>
