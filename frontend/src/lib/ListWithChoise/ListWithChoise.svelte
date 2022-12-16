<script>
  import { each } from "svelte/internal";
  import { onMount } from "svelte";

  export let listData = [];
  import { navigate } from "svelte-routing";
  export let openHeaderIndexs = [0];
  export let activeElementName = "";
  export let customListClass = "";
  export let customListElClass = "";
  export let customActiveElClass = "";
  export let headerButton = "";
  export let listElementButton = "";
  import { lastChosenHost, lastChosenService } from "../../Stores/stores.js";
  let initialVisitcounter = 0;
  console.log(activeElementName);

  $: {
    {
      if (!initialVisitcounter && !activeElementName) {
        const chosenHost = listData[0] && listData[0].host;
        const chosenService = listData[0] && listData[0].services[0];
        activeElementName = listData[0] && `${chosenHost}-${chosenService}`;

        lastChosenHost.set(chosenHost);
        lastChosenService.set(chosenService);

        navigate(`/view/${chosenHost}/${chosenService}`, { replace: true });
      }
    }
  }

  function toggleSublistVisible(i) {
    if (openHeaderIndexs.includes(i)) {
      openHeaderIndexs = openHeaderIndexs.filter((e) => e !== i);
    } else {
      openHeaderIndexs = [...openHeaderIndexs, i];
    }
  }
  function choseSublistEl(firstEl, secondEl) {
    activeElementName = `${firstEl}-${secondEl}`;
    navigate(`/view/${firstEl}/${secondEl}`, { replace: true });
  }

  onMount(() => {});
</script>

<div class="listWithChoise">
  <ul class={customListClass}>
    {#each listData as listEl, index}
      <li class="listElement">
        <div
          class="hostHeader"
          on:click={() => {
            toggleSublistVisible(index);
          }}
        >
          <div>
            <i class="log log-Server" />
          </div>
          <p class="hostName">
            {listEl.host}
          </p>
          {#if headerButton}<div class="headerButton">
              <i class="log log-{headerButton}" />
            </div>{/if}
        </div>
        <div
          class="dropDownList {openHeaderIndexs.includes(index)
            ? ''
            : 'visuallyHidden'}"
        >
          <ul>
            {#each listEl.services as service, i}
              <li
                class="serviceListItem  "
                id={listEl.host}
                on:click={() => {
                  choseSublistEl(listEl.host, service);
                  lastChosenHost.set(listEl.host);
                  lastChosenService.set(service);

                  initialVisitcounter = 1;
                }}
              >
                <div class="hostRow {customListElClass}">
                  <p>
                    {service}
                  </p>
                  {#if listElementButton}
                    <div class="listElementButton">
                      <i class="log log-Wheel" />
                    </div>
                  {/if}
                </div>
                <div
                  class={`highlightedOverlay ${
                    `${activeElementName}` ===
                    `${listEl.host.trim()}-${service.trim()}`
                      ? "active"
                      : ``
                  }`}
                />
              </li>{/each}
          </ul>
        </div>
      </li>
    {/each}
  </ul>
</div>
