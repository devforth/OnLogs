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
  let elements = [];

  let intersects = [];
  let initialScroll = 0;
  let lastScrollTop = 0;
  let scrollDirection = "up";
  let lastFetchActionIsFetch = true;

  //fetch params:

  let search = "";
  let limit = 30;
  let offset = 0;
  let negativeOffset = 0;
  let caseSens = false;
  let startWith = "";

  //functions
  const fetchedLogs = async (needScroll) => {
    if (scrollDirection === "up") {
      // if (negativeOffset === offset - limit * 3) {
      //   offset = offset - limit;
      // }
      if (!lastFetchActionIsFetch) {
        offset = offset + limit * 4;
      }
    }
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

      lastFetchActionIsFetch = true;
    }

    setTimeout(() => {
      if (!needScroll) {
        scrollToNewLogsEnd(".newLogsEnd");
      }
    }, 50);

    return data;
  };
  const unfetchedLogs = async () => {
    if (scrollDirection === "down" && offset >= 0) {
      if (lastFetchActionIsFetch) {
        offset = offset - limit * 4;
      }
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
        newLogs = [...visibleLogs];
        visibleLogs = [...previousLogs];

        previousLogs = [...data];

        allLogs = [...newLogs, ...visibleLogs, ...previousLogs];

        offset = offset - limit;
        lastFetchActionIsFetch = false;
      }

      scrollToNewLogsEnd(".newLogsStart", true);

      return data;
    }
  };

  //

  $: {
    (async () => {
      if ($lastChosenHost && $lastChosenService) {
        initialScroll = 0;
        const data = await fetchedLogs(true);
        const data2 = await fetchedLogs(true);
        const data3 = await fetchedLogs(true);

        setTimeout(() => {
          const el = document.querySelector("#endOfScroll");
          console.log(document.querySelector("#endOfLogs"));
          scrollToBottom();
          setTimeout(() => {
            initialScroll = 1;
          }, 4000);
        });
      }
    })();
  }
  $: {
    (async () => {
      if (intersects[0] && initialScroll) {
        const data = await fetchedLogs();
      }
    })();
  }
  $: {
    (async () => {
      if (intersects[1] && initialScroll) {
        const data = await unfetchedLogs();
      }
    })();
  }
  onMount(() => {
    const logsContEl = document.querySelector("#logs");

    logsContEl.addEventListener(
      "scroll",
      function () {
        // or window.addEventListener("scroll"....
        let st = window.pageYOffset || logsContEl.scrollTop; // Credits: "https://github.com/qeremy/so/blob/master/so.dom.js#L426"
        if (st > lastScrollTop) {
          scrollDirection = "down";
        } else {
          scrollDirection = "up";
        }
        lastScrollTop = st <= 0 ? 0 : st; // For Mobile or negative scrolling
      },
      false
    );

    setTimeout(() => {
      console.log("intersects", intersects);
    }, 5000);
  });
</script>

<LogsViewHeder bind:searchText />
<h2>{intersects[1]}</h2>

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
          <div
            class={i === limit * 1.5
              ? "newLogsEnd"
              : i === allLogs.length + 1 - limit * 1.5
              ? "newLogsStart"
              : i === limit / 2
              ? "interseptor"
              : ""}
            bind:this={elements[i]}
          >
            <LogsString
              time={transformLogString(logItem, $store.UTCtime)}
              message={logItem.at(1)}
              status={getLogLineStatus(logItem.at(1))}
            />
            {#if i === limit / 2}
              <IntersectionObserver
                element={elements[0]}
                bind:intersecting={intersects[0]}
              >
                <div class="observer" bind:this={elements[0]} />
              </IntersectionObserver>{/if}
            {#if i === allLogs.length - limit / 2 && allLogs.length >= 3 * limit}
              <IntersectionObserver
                element={elements[1]}
                bind:intersecting={intersects[1]}
              >
                <div class="observer" bind:this={elements[1]} />
              </IntersectionObserver>{/if}
          </div>
        {/each}
      {/if}
      <div id="endOfLogs" />
    </table>

    <!-- {#if buttonToBottomIsVisible}
      <ButtonToBottom ico={"Down"} />
    {/if} -->
  </div>
</div>
