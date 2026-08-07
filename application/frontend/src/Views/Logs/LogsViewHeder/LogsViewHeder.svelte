<script>
  

  import Button from "../../../lib/Button/Button.svelte";
  import DropDown from "../../../lib/DropDown/DropDown.svelte";
  import { clickOutside } from "../../../lib/OutsideClicker/OutsideClicker.js";
  import { hasSearchResetRequest } from "../shareLinkViewState.js";
  import { onDestroy, untrack } from "svelte";
  /**
   * @typedef {Object} Props
   * @property {string} [searchText]
   * @property {number} [searchResetVersion]
   */

  /** @type {Props} */
  let { searchText = $bindable(""), searchResetVersion = 0 } = $props();
  let dropDownIsVisible = $state(false);
  let isSearchVIsible = $state(false);
  let lastSearchResetVersion = $state(untrack(() => searchResetVersion));
  let timer = $state();
  const debounce = (v) => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      searchText = v;
    }, 750);
  };

  function dropDownToggle() {
    dropDownIsVisible = !dropDownIsVisible;
  }
  function handleClickOutside() {
    if (dropDownIsVisible) {
      dropDownIsVisible = false;
    }
  }

  $effect(() => {
    if (hasSearchResetRequest(lastSearchResetVersion, searchResetVersion)) {
      lastSearchResetVersion = searchResetVersion;
      isSearchVIsible = false;
      clearTimeout(timer);
    }
  });

  onDestroy(() => clearTimeout(timer));
</script>

<div id="top-line">
  <h2 class="header">Service logs</h2>
  <p class="header">recent at bottom</p>
  <!-- <button class="header show">
        <div class="icon">
            <i class={"log log-Eye"} />
        </div>
    </button> -->
  {#if !isSearchVIsible}
    <div
      style:position={"relative"}
      use:clickOutside
      onclick_outside={handleClickOutside}
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
    <div class="filterButtonContainer visuallyHidden">
      <Button
        icon={"log log-Filter"}
        minWidth={55}
        minHeight={40}
        iconHeight={20}
        CB={() => {}}
      />
    </div>
  {/if}
  <div class="searchButtonContainer">
    <Button
      icon={"log log-Search"}
      minWidth={55}
      minHeight={40}
      iconHeight={20}
      CB={() => {
        isSearchVIsible = !isSearchVIsible;
      }}
    />
  </div>
  <div class="header search {!isSearchVIsible && 'hidden'}">
    <div class="searchIcoContainer">
      <i class={"log log-Search"}></i>
    </div>
    <input
      type="text"
      value={searchText}
      oninput={(e) => debounce(e.target.value)}
      placeholder="Search"
    />
  </div>
</div>
