<script>
  export let searchText = "";
  import Button from "../../../lib/Button/Button.svelte";
  import DropDown from "../../../lib/DropDown/DropDown.svelte";
  let dropDownIsVisible = false;
  function dropDownToggle() {
    dropDownIsVisible = !dropDownIsVisible;
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
  <div style:position={"relative"}>
    <Button
      icon={"log log-Eye"}
      minWidth={55}
      minHeight={40}
      iconHeight={20}
      CB={dropDownToggle}
    />

    <DropDown DropDownIsvisible={dropDownIsVisible} />
  </div>
  <div class="header search">
    <i class={"log log-Search"} />
    <input type="text" bind:value={searchText} />
  </div>
</div>
