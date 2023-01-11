<script>
  // @ts-nocheck

  import LogsString from "../../lib/LogsString/LogsString.svelte";
  import fetchApi from "../../utils/fetch";
  import { afterUpdate, onMount, onDestroy } from "svelte";
  import LogsViewHeder from "./LogsViewHeder/LogsViewHeder.svelte";
  import IntersectionObserver from "svelte-intersection-observer";
  import {
    store,
    lastChosenHost,
    lastChosenService,
  } from "../../Stores/stores";
  import ButtonToBottom from "../../lib/ButtonToBottom/ButtonToBottom.svelte";
  import {
    getLogLineStatus,
    transformLogString,
    getLogs,
    scrollToBottom,
    scrollToNewLogsEnd,
  } from "./functions";
  const api = new fetchApi();
  let visibleLogs = [];
  let newLogs = [];
  let previousLogs = [];
  let allLogs = [];
  let logsFromWS = [];
  let searchText = "";
  let elements = {};
  let unElement = document.querySelector("#unfetch");
  let intersects = {};
  let unintersecting;
  let initialScroll = 0;
  let beforeScrollPosition = 0;

  //fetch params:

  let search = "";
  let limit = 30;
  let offset = 0;
  let caseSens = false;
  let startWith = "";

  //functions
  const fetchedLogs = async () => {
    const data = await getLogs({
      containerName: $lastChosenService,
      search,
      limit,
      offset,
      caseSens,
      startWith,
      hostName: $lastChosenHost,
    });
    if (data.length === limit) {
      previousLogs = [...visibleLogs];
      visibleLogs = [...newLogs];
      newLogs = [...data];

      allLogs = [...newLogs, ...visibleLogs, ...previousLogs];

      offset = offset + limit;
    }
    scrollToNewLogsEnd();

    return data;
  };

  const unfetchedLogs = async () => {
    const data = await getLogs({
      containerName: $lastChosenService,
      search,
      limit,
      offset: offset - limit,
      caseSens,
      startWith,
      hostName: $lastChosenHost,
    });
    if (data.length === limit) {
      previousLogs = [...visibleLogs];
      visibleLogs = [...newLogs];
      newLogs = [...data];

      allLogs = [...newLogs, ...visibleLogs, ...previousLogs];

      offset = offset - limit;
    }
    scrollToNewLogsEnd();

    return data;
  };

  //

  $: {
    (async () => {
      if ($lastChosenHost && $lastChosenService) {
        initialScroll = 0;
        const data = await fetchedLogs();
        const data2 = await fetchedLogs();

        setTimeout(() => {
          const el = document.querySelector("#endOfScroll");
          console.log(document.querySelector("#endOfLogs"));
          scrollToBottom();
          setTimeout(() => {
            initialScroll = 1;
          }, 2000);
        });
      }
    })();
  }
  $: {
    (async () => {
      if (intersecting && initialScroll) {
        const data = await fetchedLogs();
      }
    })();
  }
  $: {
    (async () => {
      if (unintersecting && initialScroll) {
        const data = await unfetchedLogs();
      }
    })();
  }
  onMount(() => {
    const logsContEl = document.querySelector("#logs");

    logsContEl.addEventListener("scroll", () => {
      element = document.querySelector("#fetch");
      unElement = document.querySelector("#unfetch");
    });
  });
</script>

<LogsViewHeder bind:searchText />
<h2>{intersecting}</h2>
<h2>{unintersecting}</h2>
{#if allLogs.length === 0}
  <h2 class="noLogsMessage">No logs written yet</h2>
{/if}
<!-- {#if isUploading}
  <div class="lds-ellipsis">
    <div />
    <div />
    <div />
    <div />
  </div>
{/if} -->
<div id="logs" class="logs">
  <div class="logsTableContainer">
    <table class="logsTable {$store.breakLines ? 'breakLines' : ''}">
      <div id="startOfLogs" />
      {#if searchText.length === 0}
        {#each allLogs as logItem, i}
          <IntersectionObserver
            element={elements[logItem]}
            bind:intersecting={intersects[]}
          >
            <div
              id={logItem[1] === newLogs.at(15)?.[1]
                ? "fetch"
                : logItem[1] === previousLogs.at(15)?.[1]
                ? "unfetch"
                : ""}
              class={logItem[1] === visibleLogs.at(-1)?.[1] ? "newLogsEnd" : ""}
            >
              <LogsString
                time={transformLogString(logItem, $store.UTCtime)}
                message={logItem.at(1)}
                status={getLogLineStatus(logItem.at(1))}
              />
            </div>
          </IntersectionObserver>
        {/each}
      {/if}
      <div id="endOfLogs" />
    </table>

    <!-- {#if buttonToBottomIsVisible}
      <ButtonToBottom ico={"Down"} />
    {/if} -->
  </div>
</div>
