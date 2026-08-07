<script>
  import Checkbox from "../CheckBox/Checkbox.svelte";
  let checkBoxValue = $state(true);
  import { store } from "../../Stores/stores.js";
  /**
   * @typedef {Object} Props
   * @property {string} [rowImage]
   * @property {string} [rowTitle]
   * @property {string} [iconHeight]
   * @property {boolean} [isFirst]
   * @property {string} [storeValue]
   * @property {boolean} [disableCheckbox]
   * @property {any} [titleCallBack]
   */

  /** @type {Props} */
  let {
    rowImage = "",
    rowTitle = "",
    iconHeight = "",
    isFirst = false,
    storeValue = "",
    disableCheckbox = false,
    titleCallBack = null
  } = $props();

  store.update((pv) => {
    return { ...pv };
  });
</script>

<tr
  class=" {isFirst ? 'isFirst' : ''} {titleCallBack ? 'clickable' : ''}"
  onclick={() => {
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
></i>
    </div>
  </td>
  <td><div class="dropDownRawEl text">{rowTitle}</div></td>

  <td class="dropDownRawEl"
    >{#if !disableCheckbox}<Checkbox bind:active={checkBoxValue} {storeValue} />
    {:else}
      <div class="emptyBox"></div>
    {/if}
  </td>
  <!-- </div> -->
</tr>
