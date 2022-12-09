<script>
  export let searchText = "";
  import Button from "../../../lib/Button/Button.svelte";
  import DropDown from "../../../lib/DropDown/DropDown.svelte";
  import { clickOutside } from "../../../lib/OutsideClicker/OutsideClicker.js";
  let dropDownIsVisible = false;
  function dropDownToggle() {
    dropDownIsVisible = !dropDownIsVisible;
  }
  function handleClickOutside() {
    if (dropDownIsVisible) {
      dropDownIsVisible = false;
    }
  }
</script>

<div id="top-line">
  <h2 class="header">Service logs</h2>
  <p class="header">recent at bottom</p>
  <!-- <button class="header show">
        <div class="icon">
            <i class={"log log-Eye"} />
        </div>
    </button> -->
  <div
    style:position={"relative"}
    use:clickOutside
    on:click_outside={handleClickOutside}
  >
    <Button
      icon={"log log-Eye"}
      minWidth={55}
      minHeight={40}
      iconHeight={20}
      CB={dropDownToggle}
    />
    {#if dropDownIsVisible}
      <DropDown />
    {/if}
  </div>
  <div class="header search">
    <i class={"log log-Search"} />
    <input type="text" bind:value={searchText} />
  </div>
</div>
