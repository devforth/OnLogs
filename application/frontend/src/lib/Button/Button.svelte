<script>
  import { createEventDispatcher } from "svelte";

  const dispatch = createEventDispatcher();

  function clickButton() {
    dispatch("onClickButton");
  }

  export let title = "";
  export let border = true;
  export let highlighted = false;
  export let minWidth = 0;
  export let minHeight = 0;
  export let icon = "";
  export let CB = () => {};
  export let iconHeight = 12;
  export let type = "button";
  export let disabled = false;
</script>

<button
  {disabled}
  on:click={(e) => {
    CB();
    return e;
  }}
  style:min-width={`${minWidth}px`}
  style:min-height={`${minHeight}px`}
  class="button block {highlighted ? 'highlighted' : ''} {border
    ? 'border'
    : ''}"
>
  <i
    style:font-size={`${iconHeight}px`}
    style:line-height={"100%"}
    class={icon ? `${icon}` : ""}
    class:iconWidthText={title && icon}
  />{title}
</button>
