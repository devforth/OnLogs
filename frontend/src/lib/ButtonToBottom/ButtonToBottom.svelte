<script>
  import { onMount } from "svelte";
  export let ico = "";
  export let callBack = () => {};
  export let number = null;
</script>

<div
  class="buttonToBottomContainer"
  on:click={async () => {
    await callBack();
  }}
>
  <div class="icoContainer">
    <i class="log log-Down" />
  </div>
  {#if number}<div class="buttonToBottomNumber flex">
      <p>{number}</p>
    </div>
  {/if}
</div>
