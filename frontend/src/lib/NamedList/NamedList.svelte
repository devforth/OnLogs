<script>
  export let listName = "";
  export let listIco = "";
  export let listData = [];
  export let marginTop = "0";
  export let marginBottom = "0";
  export let marginLeft = "0";
  export let marginRight = "0";
</script>

<div>
  <div
    class="namedListHeader"
    style:margin-top={`${marginTop}`}
    style:margin-bottom={`${marginBottom}`}
    style:margin-left={`${marginLeft}`}
    style:margin-right={`${marginRight}`}
  >
    <div class="icon">
      <i class={listIco ? `log log-${listIco}` : ""} />

      <p class="listName">{listName}</p>
    </div>
  </div>
  <div>
    <ul>
      {#each listData as item}
        <li>{item}</li>{/each}
    </ul>
  </div>
</div>
