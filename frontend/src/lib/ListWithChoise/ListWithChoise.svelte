<script>
  // @ts-nocheck

  import { each } from "svelte/internal";
  import { onMount } from "svelte";

  export let listData = [];
  let sortedData = [];
  import { navigate } from "svelte-routing";
  export let openHeaderIndexs = [0];
  export let activeElementName = "";
  export let customListClass = "";
  export let customListElClass = "";
  export let customActiveElClass = "";
  export let headerButton = "";
  export let listElementButton = "";
  let chosenElSettings = "";
  import {
    lastChosenHost,
    lastChosenService,
    listScrollIsVisible,
  } from "../../Stores/stores.js";
  let initialVisitcounter = 0;

  $: {
    {
      if (!initialVisitcounter && !activeElementName) {
        const chosenHost = listData[0] && listData[0].host;
        const chosenService =
          listData[0] && listData[0].services[0].serviceName;
        activeElementName = listData[0] && `${chosenHost}-${chosenService}`;

        lastChosenHost.set(chosenHost);
        lastChosenService.set(chosenService);

        navigate(`/view/${chosenHost}/${chosenService}`, { replace: true });
      }
    }
  }

  $: {
    sortedData = listData.map((h) => {
      let activeServices = h.services.filter((s) => {
        return !s.isDisabled;
      });
      let inActiveServices = h.services.filter((s) => {
        return s.isDisabled;
      });
      let newHost = {
        ...h,
        services: [...activeServices, ...inActiveServices],
      };
      return newHost;
    });
  }

  function toggleSublistVisible(i) {
    if (openHeaderIndexs.includes(i)) {
      openHeaderIndexs = openHeaderIndexs.filter((e) => e !== i);
    } else {
      openHeaderIndexs = [...openHeaderIndexs, i];
    }
  }
  function choseSublistEl(firstEl, secondEl) {
    activeElementName = `${firstEl.trim()}-${secondEl.trim()}`;

    navigate(`/view/${firstEl}/${secondEl}`, { replace: true });
  }

  onMount(() => {});
</script>

<div class="listWithChoise {$listScrollIsVisible ? 'active' : ''}">
  <ul class={customListClass}>
    {#each sortedData as listEl, index}
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
                  choseSublistEl(listEl.host, service.serviceName);
                  lastChosenHost.set(listEl.host);
                  lastChosenService.set(service.serviceName);

                  initialVisitcounter = 1;
                }}
              >
                <div class="hostRow {customListElClass}">
                  <p class={service.isDisabled ? "disabled" : ""}>
                    {service.serviceName}
                  </p>
                  {#if listElementButton}
                    <div
                      class="listElementButton"
                      on:click={() => {
                        navigate(
                          `/servicesettings/${listEl.host.trim()}/${service.serviceName.trim()}`,
                          { replace: true }
                        );

                        chosenElSettings = `${listEl.host.trim()}-${service.serviceName.trim()}`;
                      }}
                    >
                      <i class="log log-Wheel" />
                    </div>
                  {/if}
                </div>
                <div
                  class={`highlightedOverlay ${
                    `${activeElementName}` ===
                    `${listEl.host.trim()}-${service.serviceName.trim()}`
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
