<script>
  import Checkbox from "../CheckBox/Checkbox.svelte";
  export let rowImage = "";
  export let rowTitle = "";
  export let iconHeight = "";
  export let isLast = false;
  export let storeValue = "";
  let checkBoxValue = true;
  import { store } from "../../Stores/stores.js";

  store.update((pv) => {
    return { ...pv };
  });
</script>

<tr>
  <div class="rowContainer {isLast ? 'isLast' : ''}">
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
      ><Checkbox bind:active={checkBoxValue} {storeValue} /></td
    >
  </div>
</tr>
