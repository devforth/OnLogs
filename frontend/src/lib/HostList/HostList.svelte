<script>
  export let hostName = "";
  export let servicesData = [];
  export let marginTop = "0";
  export let marginBottom = "0";
  export let marginLeft = "0";
  export let marginRight = "0";

  export let selectedName = ""
  const selectItem = (serviceName) => {
    selectedName = serviceName
  }
</script>

<div>
  <div
    class="hostListHeader"
    style:margin-top={`${marginTop}`}
    style:margin-bottom={`${marginBottom}`}
    style:margin-left={`${marginLeft}`}
    style:margin-right={`${marginRight}`}
  >
    <div class="icon">
      <i class={"log log-Server"} />
      <p class="listName">{hostName}</p>
    </div>
  </div>

  <div>
    <ul>
        {#if servicesData != null}
          {#each servicesData as service}
            {#if selectedName.localeCompare(service) == 0}
              <li class="selected">{service}</li> <!-- <i class="log log-Wheel"/> -->
            {:else}
              <li on:click={() => {selectItem(service)}}>{service}</li>
            {/if}
          {/each}
        {/if}
    </ul>
  </div>
</div>
