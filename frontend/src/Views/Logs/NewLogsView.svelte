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
  let webSocket = null;
  let logsFromWS = [];

  let elements = [];
  let logsOverflow = [];

  let intersects = [];
  let initialScroll = 0;
  let lastScrollTop = 0;
  let scrollDirection = "up";
  let lastFetchActionIsFetch = true;
  let scrollFromButton = false;

  //fetch params:

  let searchText = "";
  let limit = 30;
  let offset = 0;

  let caseSens = false;
  let startWith = "";
  let tmpStartWith = [];

  //functions
  function getLogsFromWS() {
    if (webSocket) {
      webSocket.close();
    } else {
      webSocket = new WebSocket(`${api.wsUrl}getLogsStream?id=${service}`);
      webSocket.onmessage = (event) => {
        if (event.data !== "PING") {
          const logfromWS = JSON.parse(event.data);
          allLogs.push(logfromWS);
          offset++;
          if (!$store.caseInSensitive) {
            if (searchText !== "" && logfromWS[1].includes(searchText)) {
              searchLogs = [...searchLogs, logfromWS];
              searchOffset++;
            }
          } else {
            if (
              searchText !== "" &&
              logfromWS[1].toLowerCase().includes(searchText.toLowerCase())
            ) {
              searchLogs = [...searchLogs, logfromWS];
              searchOffset++;
              console.log(tmpSearchLogs), "tmp";
            }
          }

          tmpLogs = allLogs;
          scrollToBottom();
        } else {
          webSocket.send("PONG");
        }
      };
    }
  }

  function resetParams() {
    allLogs = [];
    offset = 0;
    tmpStartWith = [];
    startWith = "";
  }

  function resetSearchParams() {
    searchText = "";
  }

  function isInterceptorVIsible(inter, cb) {
    (async () => {
      if (inter && initialScroll) {
        const data = await cb();
      }
    })();
  }

  function setInitialScroll(val) {
    initialScroll = val;
  }
  const fetchedLogs = async (doNotScroll) => {
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
      search: searchText,
      limit,
      offset: searchText ? 0 : offset,
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
      if (searchText) {
        startWith = data.at(0).at(0);
        tmpStartWith.push(startWith);
        console.log(tmpStartWith);
      }

      lastFetchActionIsFetch = true;
      setTimeout(() => {
        if (!doNotScroll) {
          scrollToNewLogsEnd(".newLogsEnd");
        }
      }, 50);
    } else {
      logsOverflow = [...data];
      allLogs = [...logsOverflow, ...allLogs];
    }

    return data;
  };
  const unfetchedLogs = async () => {
    if (scrollDirection === "down" && offset >= 0 && !scrollFromButton) {
      if (lastFetchActionIsFetch && !scrollFromButton) {
        offset = offset - limit * 4;
        if (searchText) {
          tmpStartWith.splice(-4, 4);
        }
      }
      const data = await getLogs({
        containerName: $lastChosenService,
        search: searchText,
        limit,
        offset: searchText ? 0 : offset,
        caseSens,
        startWith: tmpStartWith.pop(),
        hostName: $lastChosenHost,
      });

      if (data.length === limit) {
        newLogs = [...visibleLogs];
        visibleLogs = [...previousLogs];

        previousLogs = [...data];

        allLogs = [...newLogs, ...visibleLogs, ...previousLogs];

        offset = offset - limit;
        lastFetchActionIsFetch = false;
      } else {
        logsOverflow = [...data];
        allLogs = [...allLogs, ...logsOverflow];
      }

      scrollToNewLogsEnd(".newLogsStart", true);

      return data;
    }
  };

  //
  function consoleLOgs(t) {
    console.log(t, new Date());
  }

  $: {
    (async () => {
      if ($lastChosenHost && $lastChosenService) {
        setInitialScroll(0);
        resetParams();
        resetSearchParams();
        for (let i = 0; i < 3; i++) {
          const data = await fetchedLogs(true);
          if (!data.length === limit) {
            break;
          }
        }

        setTimeout(() => {
          scrollToBottom();
          setTimeout(() => {
            setInitialScroll(1);
          }, 1000);
        });
      }
    })();
  }

  $: {
    (async () => {
      if (searchText) {
        setInitialScroll(0);
        resetParams();

        for (let i = 0; i < 3; i++) {
          const data = await fetchedLogs(true);
          if (!data.length === limit) {
            break;
          }
        }

        setTimeout(() => {
          scrollToBottom();
          setTimeout(() => {
            setInitialScroll(1);
          }, 1000);
        });
      }
    })();
  }

  $: {
    isInterceptorVIsible(intersects[0], fetchedLogs);
  }
  $: {
    isInterceptorVIsible(intersects[1], unfetchedLogs);
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
  });
</script>

<LogsViewHeder bind:searchText />

{#if allLogs.length === 0}
  <h2 class="noLogsMessage">No logs written yet</h2>
{/if}

<div id="logs" class="logs">
  <div class="logsTableContainer">
    <table class="logsTable {$store.breakLines ? 'breakLines' : ''}">
      <div id="startOfLogs" />

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

      <div id="endOfLogs" />
    </table>
    <div class={offset <= limit * 3 ? "visuallyHidden" : ""}>
      <ButtonToBottom
        ico={"Down"}
        callBack={async () => {
          offset = 0;
          scrollFromButton = true;
          await fetchedLogs(true);
          await fetchedLogs(true);
          await fetchedLogs(true);
          scrollToBottom();

          setTimeout(() => {
            scrollFromButton = false;
          }, 2000);
        }}
      />
    </div>
  </div>
</div>
