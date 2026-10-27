<script>
  // @ts-nocheck

  import { onMount, untrack } from "svelte";

  import { navigate } from "svelte-routing";
  // Host names, not indexes: a host added or removed shifts every index.
  let openStopedHosts = $state([]);
  let openHosts = $state([]);
  import {
    activeMenuOption,
    groups,
    groupBeingEdited,
    groupModalIsVisible,
    hostAliases,
    hostBeingRenamed,
    lastChosenHost,
    lastChosenService,
    listScrollIsVisible,
  } from "../../Stores/stores.js";
  import FetchApi from "../../utils/fetch.js";
  import { changeKey } from "../../utils/changeKey.js";
  let {
    listData = $bindable([]),
    activeElementName = $bindable(""),
    customListClass = "",
    customListElClass = "",
    customActiveElClass = "",
    listElementButton = "",
    section = "logs"
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


  // Appends only when the host is new, so the effect's own write cannot
  // retrigger it forever.
  $effect(() => {
    if (initialVisitcounter) {
      return;
    }
    const openedStopedHost = sortedData.find((e) =>
      e.services.some((s) => s.isDisabled && s.serviceName === $lastChosenService)
    )?.host;

    if (openedStopedHost && !openStopedHosts.includes(openedStopedHost)) {
      openStopedHosts = [...openStopedHosts, openedStopedHost];
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

  function toggleSublistVisible(host) {
    if (openHosts.includes(host)) {
      openHosts = openHosts.filter((e) => e !== host);
    } else {
      openHosts = [...openHosts, host];
    }
  }
  function toggleArchivedVisible(host) {
    if (openStopedHosts.includes(host)) {
      openStopedHosts = openStopedHosts.filter((e) => e !== host);
    } else {
      openStopedHosts = [...openStopedHosts, host];
    }
  }

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
  let autoOpenedHost = "";
  let firstHostOpened = false;

  function choseInitialHost() {
    // The first host starts open.
    if (!firstHostOpened && listData[0]) {
      firstHostOpened = true;
      if (!openHosts.includes(listData[0].host)) {
        openHosts = [...openHosts, listData[0].host];
      }
    }
    const host = $lastChosenHost;
    if (!host || autoOpenedHost === host) {
      return;
    }
    if (!listData.some((h) => h.host === host)) {
      return;
    }

    autoOpenedHost = host;
    if (!openHosts.includes(host)) {
      openHosts = [host, ...openHosts];
    }
  }

  function openSection(event, host, serviceName, section) {
    event.stopPropagation();
    initialVisitcounter = 1;
    lastChosenHost.set(host);
    lastChosenService.set(serviceName);
    navigate(
      `${changeKey}/${section}/${host.trim()}/${serviceName.trim()}`,
      { replace: true }
    );
  }

  $effect(() => {
    listData;
    $lastChosenHost;
    untrack(() => choseInitialHost());
  });
</script>

{#snippet serviceRow(host, service, i, idSuffix)}
  {@const isCurrent =
    `${activeElementName}` === `${host.trim()}-${service.serviceName.trim()}`}
  <li
    class="serviceListItem  "
    id={host}
    onclick={({ target }) => choseService(host, service.serviceName, target)}
  >
    <div class="hostRow {customListElClass}">
      <p class={service.isDisabled ? "disabled" : ""} title={service.serviceName}>
        {service.serviceName}
      </p>
      {#if listElementButton}
        <div class="buttonBox flex">
          <div
            class="listElementButton"
            title={isCurrent && section === "stats"
              ? "Back to logs"
              : "Log statistics"}
            onclick={(e) =>
              openSection(
                e,
                host,
                service.serviceName,
                isCurrent && section === "stats" ? "view" : "stats"
              )}
          >
            <i
              class="log {isCurrent && section === 'stats'
                ? 'log-ChartFilled'
                : 'log-Chart'}"
            ></i>
          </div>
          <div
            class="listElementButton"
            title={isCurrent && section === "settings"
              ? "Back to logs"
              : "Service settings"}
            onclick={(e) =>
              openSection(
                e,
                host,
                service.serviceName,
                isCurrent && section === "settings" ? "view" : "servicesettings"
              )}
          >
            <i
              class="log {isCurrent && section === 'settings'
                ? 'log-WheelFilled'
                : 'log-Wheel'}"
            ></i>
          </div>
          <div
            id={`heartButtonCont${idSuffix}-${i}`}
            class="listElementButton"
            onclick={() => favoriteToggle(host, service.serviceName)}
          >
            <i
              id={`heartButton${idSuffix}-${i}`}
              class="log {service.isFavorite ? 'log-Heart' : 'log-EmptyHeart'}"
            ></i>
          </div>
        </div>
      {/if}
    </div>
    <div class="highlightedOverlay {isCurrent ? 'active' : ''}"></div>
  </li>
{/snippet}

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
    {#each sortedData as listEl}
      <li class="listElement">
        <div class="hostHeader clickable" onclick={() => toggleSublistVisible(listEl.host)}>
          <div>
            <i class="log log-Server"></i>
          </div>
          <p class="hostName" title={listEl.host}>
            {$hostAliases[listEl.host] || listEl.host}
          </p>
          <div
            class="headerButton"
            title="Rename host"
            onclick={(e) => {
              e.stopPropagation();
              hostBeingRenamed.set(listEl.host);
            }}
          >
            <i class="log log-Pencil"></i>
          </div>
          <i
            class="log log-Pointer hostToggle {openHosts.includes(listEl.host)
              ? ''
              : 'rotated'}"
          ></i>
        </div>
        <div
          class="dropDownList {openHosts.includes(listEl.host)
            ? ''
            : 'visuallyHidden'}"
        >
          <ul class="activeServices">
            {#each listEl.services as service, i}
              {#if !service.isDisabled}{@render serviceRow(listEl.host, service, i, "")}{/if}
            {/each}
          </ul>
        </div>
        <div class={openHosts.includes(listEl.host) ? "" : "visuallyHidden"}>
          <div
            class="flex flex-start stopedServicesBox clickable inactiveServices {listEl.services.find(
              (e) => {
                return e.isDisabled;
              }
            )
              ? ''
              : 'visuallyHidden'}"
            onclick={() => {
              toggleArchivedVisible(listEl.host);
              initialVisitcounter = 1;
            }}
          >
            <i class="log log-Archive"></i>
            <p class="stopedServices">stopped services</p>
            <i
              class="log log-Pointer {!openStopedHosts.includes(listEl.host)
                ? 'rotated'
                : ''}"
></i>
          </div>

          <ul
            class="activeServices inactiveServices {!openStopedHosts.includes(
              listEl.host
            )
              ? 'visuallyHidden'
              : ''}"
          >
            {#each listEl.services as service, i}
              {#if service.isDisabled}{@render serviceRow(listEl.host, service, i, "Dissabled")}{/if}
            {/each}
          </ul>
        </div>
      </li>
    {/each}
  </ul>
</div>
