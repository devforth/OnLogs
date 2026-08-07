<script>
  // @ts-nocheck

  import { onMount } from "svelte";

  import { navigate } from "svelte-routing";
  let openStopedServIndexes = $state([]);
  let chosenElSettings = $state("");
  import {
    activeMenuOption,
    groups,
    groupBeingEdited,
    groupModalIsVisible,
    lastChosenHost,
    lastChosenService,
    listScrollIsVisible,
  } from "../../Stores/stores.js";
  import FetchApi from "../../utils/fetch.js";
  import { changeKey } from "../../utils/changeKey.js";
  /**
   * @typedef {Object} Props
   * @property {any} [listData]
   * @property {any} [openHeaderIndexs]
   * @property {string} [activeElementName]
   * @property {string} [customListClass]
   * @property {string} [customListElClass]
   * @property {string} [customActiveElClass]
   * @property {string} [headerButton]
   * @property {string} [listElementButton]
   */

  /** @type {Props} */
  let {
    listData = $bindable([]),
    openHeaderIndexs = $bindable([0]),
    activeElementName = $bindable(""),
    customListClass = "",
    customListElClass = "",
    customActiveElClass = "",
    headerButton = "",
    listElementButton = ""
  } = $props();
  let initialVisitcounter = $state(0);

  let sortedData = $derived(
    listData.map((h) => {
      const activeServices = h.services
        .filter((s) => !s.isDisabled)
        .sort((a, b) => {
          if (a.isFavorite < b.isFavorite) return 1;
          if (a.isFavorite > b.isFavorite) return -1;
          return 0;
        });
      const inActiveServices = h.services
        .filter((s) => s.isDisabled)
        .sort((a, b) => {
          if (a.isFavorite < b.isFavorite) return 1;
          if (a.isFavorite > b.isFavorite) return -1;
          return 0;
        });
      return { ...h, services: [...activeServices, ...inActiveServices] };
    })
  );

  const fetchApi = new FetchApi();

  $effect(() => {
    {
      if (!initialVisitcounter && !activeElementName) {
        const chosenHost =
          $lastChosenHost || (sortedData[0] && sortedData[0].host);
        const chosenService =
          $lastChosenService ||
          sortedData[0]?.services?.[0]?.serviceName;
        activeElementName = sortedData[0] && `${chosenHost}-${chosenService}`;

        lastChosenHost.set(chosenHost);
        lastChosenService.set(chosenService);

        navigate(
          `${changeKey}/${
            location.pathname.includes("stats") ? "stats" : "view"
          }/${chosenHost}/${chosenService}`,
          {
            replace: true,
          }
        );
      }
    }
  });


  // Appends only when the index is new, so the effect's own write cannot
  // retrigger it forever.
  $effect(() => {
    if (initialVisitcounter) {
      return;
    }
    const openedStopedServiceIndex = sortedData
      .map((e) => e.services.map((s) => s.isDisabled && s.serviceName))
      .findIndex((el) => el.includes($lastChosenService));

    if (
      openedStopedServiceIndex !== -1 &&
      !openStopedServIndexes.includes(openedStopedServiceIndex)
    ) {
      openStopedServIndexes = [
        ...openStopedServIndexes,
        openedStopedServiceIndex,
      ];
    }
  });

  async function favoriteToggle(host, service) {
    $lastChosenHost, $lastChosenService;

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

  // The one click path a service row runs, wherever it is rendered: under its
  // host, under "stopped services", or inside a group.
  function choseService(host, serviceName, target) {
    if (target.id.includes("heart")) {
      return;
    }
    choseSublistEl(host, serviceName);
    lastChosenHost.set(host);
    lastChosenService.set(serviceName);

    if (!target.className.includes("log")) {
      activeMenuOption.set("home");
    }
    initialVisitcounter = 1;
  }

  // A member the host list no longer offers gets the stopped-service treatment.
  let groupSections = $derived(
    $groups.map((group) => ({
      name: group.name,
      members: (group.members || []).map((member) => {
        const service = listData
          .find((h) => h.host === member.host)
          ?.services?.find((s) => s.serviceName === member.service);
        return {
          host: member.host,
          serviceName: member.service,
          isDisabled: !service || service.isDisabled,
        };
      }),
    }))
  );

  // The modal owns renaming, membership and deletion, so the row needs one entry
  // point rather than three.
  function editGroup(name) {
    groupBeingEdited.set($groups.find((g) => g.name === name) || null);
    groupModalIsVisible.set(true);
  }

  // Keyed by name, so a refetch that reorders groups does not flip which is open.
  let closedGroups = $state([]);
  function toggleGroupVisible(name) {
    closedGroups = closedGroups.includes(name)
      ? closedGroups.filter((e) => e !== name)
      : [...closedGroups, name];
  }

  function choseSublistEl(firstEl, secondEl) {
    activeElementName = `${firstEl.trim()}-${secondEl.trim()}`;

    navigate(
      `${changeKey}/${
        location.pathname.includes("stats") ? "stats" : "view"
      }/${firstEl}/${secondEl}`,
      {
        replace: true,
      }
    );
  }
  function choseInitialHost() {
    listData.forEach((h, i) => {
      if (h.host === $lastChosenHost && !openHeaderIndexs.includes(i)) {
        openHeaderIndexs = [i, ...openHeaderIndexs];
      }
    });
  }

  $effect(() => {
    listData && choseInitialHost();
  });
</script>

<div class="listWithChoise {$listScrollIsVisible ? 'active' : ''}">
  {#if groupSections.length}
    <ul class="groupSection {customListClass}">
      {#each groupSections as group}
        <li class="listElement">
          <div
            class="hostHeader clickable"
            onclick={() => toggleGroupVisible(group.name)}
          >
            <div>
              <i class="log log-Group"></i>
            </div>
            <p class="hostName">
              {group.name}
            </p>
            <div
              class="headerButton groupEditButton"
              onclick={(e) => {
                e.stopPropagation();
                editGroup(group.name);
              }}
            >
              <i class="log log-Pencil"></i>
            </div>
          </div>
          <div
            class="dropDownList {closedGroups.includes(group.name)
              ? 'visuallyHidden'
              : ''}"
          >
            <ul class="activeServices">
              {#each group.members as member}
                <li
                  class="serviceListItem"
                  id={member.host}
                  onclick={({ target }) =>
                    choseService(member.host, member.serviceName, target)}
                >
                  <div class="hostRow {customListElClass}">
                    <p
                      class={member.isDisabled ? "disabled" : ""}
                      title={`${member.host}/${member.serviceName}`}
                    >
                      {member.serviceName}
                    </p>
                  </div>
                  <div
                    class={`highlightedOverlay ${
                      `${activeElementName}` ===
                      `${member.host.trim()}-${member.serviceName.trim()}`
                        ? "active"
                        : ``
                    }`}
                  ></div>
                </li>
              {/each}
            </ul>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
  <ul class={customListClass}>
    {#each sortedData as listEl, index}
      <li class="listElement">
        <div
          class="hostHeader clickable"
          onclick={({ target }) => {
            if (target.id !== headerButton) {
              toggleSublistVisible(index);
            }
          }}
        >
          <div>
            <i class="log log-Server"></i>
          </div>
          <p class="hostName">
            {listEl.host}
          </p>
          {#if headerButton}<div
              class="headerButton "
              id={headerButton}
              onclick={() => {
                console.log("clicable");
              }}
            >
              <i class="log log-{headerButton}" id={headerButton}></i>
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
                  onclick={({ target }) =>
                    choseService(listEl.host, service.serviceName, target)}
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
                          onclick={() => {
                            navigate(
                              `${changeKey}/servicesettings/${listEl.host.trim()}/${service.serviceName.trim()}`,
                              { replace: true }
                            );

                            chosenElSettings = `${listEl.host.trim()}-${service.serviceName.trim()}`;
                          }}
                        >
                          <i class="log log-Wheel"></i>
                        </div>
                        <div
                          id={`heartButtonCont-${i}`}
                          class="listElementButton"
                          onclick={() => {
                            favoriteToggle(listEl.host, service.serviceName);
                          }}
                        >
                          <i
                            id={`heartButton-${i}`}
                            class="log {service.isFavorite
                              ? 'log-Heart'
                              : 'log-EmptyHeart'}"
></i>
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
></div>
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
            onclick={() => {
              toggleArchivedVisible(index);
              initialVisitcounter = 1;
            }}
          >
            <i class="log log-Archive"></i>
            <p class="stopedServices">stopped services</p>
            <i
              class="log log-Pointer {!openStopedServIndexes.includes(index)
                ? 'rotated'
                : ''}"
></i>
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
                  onclick={({ target }) =>
                    choseService(listEl.host, service.serviceName, target)}
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
                          onclick={() => {
                            navigate(
                              `${changeKey}/servicesettings/${listEl.host.trim()}/${service.serviceName.trim()}`,
                              { replace: true }
                            );

                            chosenElSettings = `${listEl.host.trim()}-${service.serviceName.trim()}`;
                          }}
                        >
                          <i class="log log-Wheel"></i>
                        </div>
                        <div
                          id={`heartButtonContDissabled-${i}`}
                          class="listElementButton"
                          onclick={() => {
                            favoriteToggle(listEl.host, service.serviceName);
                          }}
                        >
                          <i
                            id={`heartButtonDissabled-${i}`}
                            class="log {service.isFavorite
                              ? 'log-Heart'
                              : 'log-EmptyHeart'}"
></i>
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
></div>
                </li>{/if}{/each}
          </ul>
        </div>
      </li>
    {/each}
  </ul>
</div>
