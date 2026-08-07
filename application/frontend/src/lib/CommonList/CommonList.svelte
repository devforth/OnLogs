<script>
  import { untrack } from "svelte";

  /**
   * @typedef {Object} Props
   * @property {any} [listData]
   * @property {boolean} [isRowClickable]
   * @property {any} [storeProp]
   */

  /** @type {Props} */
  let { listData = [], isRowClickable = false, storeProp = null } = $props();
  let initialActive = $state(untrack(() => listData?.[0]?.name));
</script>

<div class="commonList">
  <ul>
    {#each listData as listEl, index}
      <li
        class="listElement {isRowClickable && 'clickable'}"
        onclick={() => {
          isRowClickable && storeProp?.set && storeProp.set(listEl.name);
          initialActive = null;
          listEl.callBack();
        }}
      >
        <div class="header">
          <p class="name">
            {listEl.name}
          </p>
        </div>
        <div class="icoContainer">
          <i class="log log-{listEl.ico}"></i>
        </div>
        <div
          class="highlightedOverlay {($storeProp === listEl.name ||
            initialActive === listEl.name) &&
            'active'}"
></div>
      </li>
    {/each}
  </ul>
</div>
