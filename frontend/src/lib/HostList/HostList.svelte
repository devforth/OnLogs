<script>
  export let hostName = "";
  export let servicesData = [];

  export let selectedName = "";

  const selectItem = (serviceName) => {
    selectedName = serviceName;
  };
</script>

<div class="hostList">
  <div class="hostListHeader">
    <div class="icon">
      <i class={"log log-Server"} />
    </div>
    <p class="listName">{hostName}</p>
  </div>

  <div class="hosts">
    <ul>
      {#if servicesData != null}
        {#each servicesData as service}
          {#if selectedName.localeCompare(service) == 0}
            <li class="selected">{service}</li>
            <!-- <i class="log log-Wheel"/> -->
          {:else}
            <li
              on:click={() => {
                selectItem(service);
              }}
            >
              {service}
            </li>
          {/if}
        {/each}
      {/if}
    </ul>
  </div>
</div>
