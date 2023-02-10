<script>
  import Checkbox from "../CheckBox/Checkbox.svelte";
  export let rowImage = "";
  export let rowTitle = "";
  export let iconHeight = "";
  export let isFirst = false;
  export let storeValue = "";
  export let disableCheckbox = false;
  export let titleCallBack = null;
  let checkBoxValue = true;
  import { store } from "../../Stores/stores.js";

  store.update((pv) => {
    return { ...pv };
  });
</script>

<tr
  class=" {isFirst ? 'isFirst' : ''} {titleCallBack ? 'clickable' : ''}"
  on:click={() => {
    if (titleCallBack) {
      titleCallBack();
    }
  }}
>
  <!-- <div class="rowContainer {isLast ? 'isLast' : ''}"> -->
  <td>
    <div class="dropDownRawEl ico">
      <i
        style:font-size={`${iconHeight}px`}
        style:line-height={"100%"}
        class={rowImage ? `${rowImage}` : ""}
      />
    </div>
  </td>
  <td><div class="dropDownRawEl text">{rowTitle}</div></td>

  <td class="dropDownRawEl"
    >{#if !disableCheckbox}<Checkbox bind:active={checkBoxValue} {storeValue} />
    {:else}
      <div class="emptyBox" />
    {/if}
  </td>
  <!-- </div> -->
</tr>
