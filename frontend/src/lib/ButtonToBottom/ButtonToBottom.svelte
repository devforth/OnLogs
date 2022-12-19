<script>
  import { onMount } from "svelte";
  export let ico = "";
  export let callBack = () => {};
  onMount(() => {
    setTimeout(() => {
      console.dir(
        document.querySelector(".buttonToBottomContainer").parentElement
          .scrollTop
      );
    }, 4000);
  });
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
