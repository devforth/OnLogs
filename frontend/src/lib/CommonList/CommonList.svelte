<script>
  export let listData = [];
</script>

<div class="commonList">
  <ul>
    {#each listData as listEl, index}
      <li class="listElement">
        <div class="header" on:click={() => {}}>
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
        <div class="highlightedOverlay" />
      </li>
    {/each}
  </ul>
</div>
