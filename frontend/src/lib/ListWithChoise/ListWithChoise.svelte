<script>
  // @ts-nocheck

  import { each } from "svelte/internal";
  import { onMount } from "svelte";

  export let listData = [];
  let sortedData = [];
  import { navigate } from "svelte-routing";
  export let openHeaderIndexs = [0];
  let openStopedServIndexes = [];
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
  import FetchApi from "../../utils/fetch.js";
  import { changeKey } from "../../utils/changeKey.js";
  let initialVisitcounter = 0;

  const fetchApi = new FetchApi();

  $: {
    {
      if (!initialVisitcounter && !activeElementName) {
        const chosenHost = sortedData[0] && sortedData[0].host;
        const chosenService =
          sortedData[0] && sortedData[0].services[0].serviceName;
        activeElementName = sortedData[0] && `${chosenHost}-${chosenService}`;

        lastChosenHost.set(chosenHost);
        lastChosenService.set(chosenService);

        navigate(`${changeKey}/view/${chosenHost}/${chosenService}`, {
          replace: true,
        });
      }
    }
  }

  $: {
    sortedData = listData.map((h) => {
      let activeServices = h.services
        .filter((s) => {
          return !s.isDisabled;
        })
        .sort(function (a, b) {
          if (a.isFavorite < b.isFavorite) {
            return 1;
          }
          if (a.isFavorite > b.isFavorite) {
            return -1;
          }
          // a должно быть равным b
          return 0;
        });
      let inActiveServices = h.services
        .filter((s) => {
          return s.isDisabled;
        })
        .sort(function (a, b) {
          if (a.isFavorite > b.isFavorite) {
            return 1;
          }
          if (a.isFavorite < b.isFavorite) {
            return -1;
          }
          // a должно быть равным b
          return 0;
        });
      let newHost = {
        ...h,
        services: [...activeServices, ...inActiveServices],
      };
      return newHost;
    });
  }

  $: {
    if (!initialVisitcounter) {
      const openedStopedServiceIndex = sortedData
        .map((e) => {
          return e.services.map((s) => s.isDisabled && s.serviceName);
        })
        .findIndex((el) => {
          return el.includes($lastChosenService);
        });

      if (openedStopedServiceIndex !== -1) {
        openStopedServIndexes.push(openedStopedServiceIndex);
        openStopedServIndexes = [...new Set(openStopedServIndexes)];
      }
    }
  }

  async function favoriteToggle(host, service) {
    $lastChosenHost, $lastChosenService;
    console.log("click on vaorite");
    const hostIndex = listData.findIndex((h) => h.host === host);
    const serviceIndex = listData[hostIndex].services.findIndex((s) => {
      return s.serviceName === service;
    });
    if (hostIndex !== -1) {
      if (listData[hostIndex].services[serviceIndex]) {
        listData[hostIndex].services[serviceIndex].isFavorite =
          !listData[hostIndex].services[serviceIndex].isFavorite;
      }
    }
    console.log(listData);
    const data = await fetchApi.changeFavorite(host, service);
  }

  function toggleSublistVisible(i) {
    if (openHeaderIndexs.includes(i)) {
      openHeaderIndexs = openHeaderIndexs.filter((e) => e !== i);
    } else {
      openHeaderIndexs = [...openHeaderIndexs, i];
    }
  }
  function toggleArchivedVisible(i) {
    if (openStopedServIndexes.includes(i)) {
      openStopedServIndexes = openStopedServIndexes.filter((e) => e !== i);
    } else {
      openStopedServIndexes = [...openStopedServIndexes, i];
    }
  }

  function choseSublistEl(firstEl, secondEl) {
    activeElementName = `${firstEl.trim()}-${secondEl.trim()}`;

    navigate(`${changeKey}/view/${firstEl}/${secondEl}`, {
      replace: true,
    });
  }

  onMount(() => {});
</script>

<div class="listWithChoise {$listScrollIsVisible ? 'active' : ''}">
  <ul class={customListClass}>
    {#each sortedData as listEl, index}
      <li class="listElement">
        <div
          class="hostHeader clickable"
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
          <ul class="activeServices">
            {#each listEl.services as service, i}
              {#if !service.isDisabled}<li
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
                    <p
                      class={service.isDisabled ? "disabled" : ""}
                      title={service.serviceName}
                    >
                      {service.serviceName}
                    </p>
                    {#if listElementButton}
                      <div class="buttonBox flex">
                        <div
                          class="listElementButton"
                          on:click={() => {
                            navigate(
                              `${changeKey}/servicesettings/${listEl.host.trim()}/${service.serviceName.trim()}`,
                              { replace: true }
                            );

                            chosenElSettings = `${listEl.host.trim()}-${service.serviceName.trim()}`;
                          }}
                        >
                          <i class="log log-Wheel" />
                        </div>
                        <div
                          class="listElementButton"
                          on:click={() => {
                            favoriteToggle(listEl.host, service.serviceName);
                          }}
                        >
                          <i
                            class="log {service.isFavorite
                              ? 'log-Heart'
                              : 'log-EmptyHeart'}"
                          />
                        </div>
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
                </li>
              {/if}{/each}
          </ul>
          <div
            class="flex flex-start stopedServicesBox clickable inactiveServices {listEl.services.find(
              (e) => {
                return e.isDisabled;
              }
            )
              ? ''
              : 'visuallyHidden'}"
            on:click={() => {
              toggleArchivedVisible(index);
              initialVisitcounter = 1;
            }}
          >
            <i class="log log-Archive" />
            <p class="stopedServices">stoped services</p>
            <i
              class="log log-Pointer {!openStopedServIndexes.includes(index)
                ? 'rotated'
                : ''}"
            />
          </div>

          <ul
            class="activeServices inactiveServices {!openStopedServIndexes.includes(
              index
            )
              ? 'visuallyHidden'
              : ''}"
          >
            {#each listEl.services as service, i}
              {#if service.isDisabled}<li
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
                    <p
                      class={service.isDisabled ? "disabled" : ""}
                      title={service.serviceName}
                    >
                      {service.serviceName}
                    </p>
                    {#if listElementButton}
                      <div class="buttonBox flex">
                        <div
                          class="listElementButton"
                          on:click={() => {
                            navigate(
                              `${changeKey}/servicesettings/${listEl.host.trim()}/${service.serviceName.trim()}`,
                              { replace: true }
                            );

                            chosenElSettings = `${listEl.host.trim()}-${service.serviceName.trim()}`;
                          }}
                        >
                          <i class="log log-Wheel" />
                        </div>
                        <div
                          class="listElementButton"
                          on:click={() => {
                            favoriteToggle(listEl.host, service.serviceName);
                          }}
                        >
                          <i
                            class="log {service.isFavorite
                              ? 'log-Heart'
                              : 'log-EmptyHeart'}"
                          />
                        </div>
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
                </li>{/if}{/each}
          </ul>
        </div>
      </li>
    {/each}
  </ul>
</div>
