<script>
  export let listData = [];
  export let isRowClickable = false;
  export let storeProp = {};
  let initialActive = listData && listData[0].name;
</script>

<div class="commonList">
  <ul>
    {#each listData as listEl, index}
      <li
        class="listElement {isRowClickable && 'clickable'}"
        on:click={() => {
          isRowClickable && storeProp.set && storeProp.set(listEl.name);
          initialActive = null;
        }}
      >
        <div class="header">
          <p class="name">
            {listEl.name}
          </p>
        </div>
        <div
          class="icoContainer"
          on:click={() => {
            listEl.callBack();
          }}
        >
          <i class="log log-{listEl.ico}" />
        </div>
        <div
          class="highlightedOverlay {($storeProp === listEl.name ||
            initialActive === listEl.name) &&
            'active'}"
        />
      </li>
    {/each}
  </ul>
</div>
