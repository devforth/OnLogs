<script>
  import { onMount } from "svelte";
  export let ico = "";
  export let callBack = () => {};
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
</div>
<div id="endOfScroll" />
