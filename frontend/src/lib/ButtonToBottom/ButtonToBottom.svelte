<script>
  import { onMount } from "svelte";
  export let ico = "";
  export let callBack = () => {};

  function scrollToBottom() {
    const el = document.querySelector("#endOfScroll");
    console.log(el);
    if (!el) {
      return;
    } else {
      el.scrollIntoView({ behavior: "smooth" });
    }
  }
</script>

<div class="buttonToBottomContainer" on:click={scrollToBottom}>
  <div class="icoContainer">
    <i class="log log-Down" />
  </div>
</div>
<div id="endOfScroll" />
