<script>
  let { listData = [], isRowClickable = false, storeProp = null } = $props();
</script>

<div class="commonList">
  <ul>
    {#each listData as listEl, index}
      <li
        class="listElement {isRowClickable && 'clickable'}"
        onclick={() => {
          isRowClickable && storeProp?.set && storeProp.set(listEl.name);
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
          class="highlightedOverlay {$storeProp === listEl.name && 'active'}"
></div>
      </li>
    {/each}
  </ul>
</div>
