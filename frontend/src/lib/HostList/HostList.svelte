<script>
  export let hostName = "";
  export let servicesData = [];
  export let marginTop = "0";
  export let marginBottom = "0";
  export let marginLeft = "0";
  export let marginRight = "0";
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
          <!-- {#if service["isLast"] == true}
            <li class="selected">{service["serviceName"]}<i class="log log-Wheel"/></li>
          {:else} -->
            <li>{service}</li>
            <li class="selected">{service}</li>
          <!-- {/if} -->
          {/each}
        {/if}
    </ul>
  </div>
</div>
