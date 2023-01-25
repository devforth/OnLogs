<script>
  // @ts-nocheck

  import LogsString from "../../lib/LogsString/LogsString.svelte";
  import fetchApi from "../../utils/fetch";
  import { navigate } from "svelte-routing";
  import { afterUpdate, onMount, onDestroy } from "svelte";
  import LogsViewHeder from "./LogsViewHeder/LogsViewHeder.svelte";
  import IntersectionObserver from "svelte-intersection-observer";
  import {
    store,
    lastChosenHost,
    lastChosenService,
    lastLogTimestamp,
  } from "../../Stores/stores";
  import ButtonToBottom from "../../lib/ButtonToBottom/ButtonToBottom.svelte";
  import {
    getLogLineStatus,
    transformLogString,
    getLogs,
    getPrevLogs,
    scrollToBottom,
    scrollToNewLogsEnd,
    checkLastLogTimeStamp,
  } from "./functions";
  const api = new fetchApi();
  let visibleLogs = [];
  let newLogs = [];
  let previousLogs = [];
  let allLogs = [];
  let webSocket = null;
  let logsFromWS = [];
  let logsOverflow = [];

  let elements = [];
  let intersects = [];
  let endOffLogs = null;
  let endOffLogsIntersect = null;

  let initialScroll = 0;
  let lastScrollTop = 0;
  let scrollDirection = "up";
  let lastFetchActionIsFetch = true;
  let scrollFromButton = false;
  let stopLogsUnfetch = false;
  let stopFetch = false;

  //fetch params:

  let searchText = "";
  let limit = 30;
  let offset = 0;

  let caseSens = false;
  let startWith = "";
  let tmpStartWith = [];

  //functions

  function resetAllLogs() {
    allLogs = [];
    newLogs = [];
    visibleLogs = [];
    previousLogs = [];
  }
  async function getFullLogsSet() {
    if (initialScroll && logsFromWS.length && allLogs.length >= 3 * limit) {
      const data1 = await fetchedLogs(true, "0");
      if (data1.length === limit) {
        const data2 = await fetchedLogs(true, `${limit * 1}`);
        if (data2.length === limit) {
          await fetchedLogs(true, `${limit * 2}`);
        }
      }
    }
  }
  function setLastLogTimestamp() {
    lastLogTimestamp.set(new Date().getTime());
  }

  function addLogFromWS(logfromWS) {
    if (endOffLogsIntersect || allLogs.length < 3 * limit) {
      if (newLogs.length === limit) {
        newLogs.splice(0, 1);
      }
      if (visibleLogs.length === limit) {
        visibleLogs.at(0) && newLogs.push(visibleLogs.at(0));
        visibleLogs.splice(0, 1);
        previousLogs.at(0) && visibleLogs.push(previousLogs.at(0));
      }

      if (previousLogs.length === limit) {
        previousLogs.splice(0, 1);
      }
      previousLogs.push(logfromWS);
      if (allLogs.length === 3 * limit) {
        allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
      } else allLogs = [...allLogs, logfromWS];
      if (allLogs.length < 3 * limit) {
      }
    } else {
      logsFromWS = [...logsFromWS, logfromWS];
    }
  }

  function getLogsFromWS() {
    webSocket = new WebSocket(
      `${api.wsUrl}getLogsStream?host=${$lastChosenHost}&id=${$lastChosenService}`
    );
    webSocket.onmessage = (event) => {
      if (event.data !== "PING") {
        const logfromWS = JSON.parse(event.data);
        offset = offset + 1;

        if (searchText === "") {
          addLogFromWS(logfromWS);
        } else {
          if (!$store.caseInSensitive) {
            if (logfromWS[1].includes(searchText)) {
              addLogFromWS(logfromWS);
            }
          } else {
            if (logfromWS[1].toLowerCase().includes(searchText.toLowerCase())) {
              addLogFromWS(logfromWS);
            }
          }
        }
      } else {
        webSocket.send("PONG");
      }
    };
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
  function closeWS() {
    if (webSocket) {
      webSocket.close();
    }
  }

  function isInterceptorVIsible(inter, cb) {
    (async () => {
      if (inter && initialScroll) {
        if (allLogs.length >= 3 * limit) {
          const data = await cb();
        }
      }
    })();
  }

  function setInitialScroll(val) {
    initialScroll = val;
  }
  const fetchedLogs = async (doNotScroll, customOffset) => {
    if (scrollDirection === "up" && !stopFetch) {
      // if (negativeOffset === offset - limit * 3) {
      //   offset = offset - limit;
      // }
      if (!lastFetchActionIsFetch) {
        offset = offset + limit * 4;
      }
    }
    stopLogsUnfetch = false;

    const data = await getLogs({
      containerName: $lastChosenService,
      search: searchText,
      limit,
      offset: customOffset ? Number(customOffset) : searchText ? 0 : offset,
      caseSens,
      startWith,
      hostName: $lastChosenHost,
    });
    if (data.length === limit) {
      stopFetch = false;
      previousLogs = [...visibleLogs];
      visibleLogs = [...newLogs];
      newLogs = [...data];

      allLogs = [...newLogs, ...visibleLogs, ...previousLogs];

      offset = offset + limit;
      // navigate(
      //   `${location.pathname.replace(/\/offset=[0-9]+/i, `/offset=${offset}`)}`,
      //   { replace: true }
      // );
      if (searchText) {
        startWith = data.at(0).at(0);
        tmpStartWith.push(startWith);
      }

      lastFetchActionIsFetch = true;
      setTimeout(() => {
        if (!doNotScroll) {
          scrollToNewLogsEnd(".newLogsEnd");
        }
      }, 50);
    } else {
      stopFetch = true;
      logsOverflow = data;

      allLogs = [...logsOverflow, ...allLogs];
    }

    return data;
  };
  const unfetchedLogs = async () => {
    if (
      scrollDirection === "down" &&
      offset >= 0 &&
      !scrollFromButton &&
      !stopLogsUnfetch
    ) {
      if (lastFetchActionIsFetch) {
        offset = offset - limit * 4;
      }
      const data = await getPrevLogs({
        containerName: $lastChosenService,
        search: searchText,
        limit,
        offset: searchText ? 0 : offset,
        caseSens,
        startWith: allLogs.at(-1)[0],
        hostName: $lastChosenHost,
      });

      if (data.length === limit) {
        newLogs = [...visibleLogs];
        visibleLogs = [...previousLogs];

        previousLogs = [...data];

        allLogs = [...newLogs, ...visibleLogs, ...previousLogs];

        offset = offset - limit > 0 ? offset - limit : 0;

        lastFetchActionIsFetch = false;
        stopLogsUnfetch = true;
      } else {
        logsOverflow = [...data];

        if (allLogs.length === 3 * limit) {
          allLogs = [...allLogs, ...logsOverflow];
        } else {
          allLogs = logsOverflow;
        }
      }

      scrollToNewLogsEnd(".newLogsStart", true);
      // navigate(
      //   `${location.pathname.replace(/\/offset=[0-9]+/i, `/offset=${offset}`)}`,
      //   { replace: true }
      // );

      return data;
    }
  };

  $: {
    (async () => {
      if ($lastChosenHost && $lastChosenService) {
        setInitialScroll(0);
        resetAllLogs();
        resetParams();
        resetSearchParams();
        closeWS();
        for (let i = 0; i < 3; i++) {
          const data = await fetchedLogs(true);
          if (data.length !== limit) {
            break;
          }
        }

        getLogsFromWS();
        setLastLogTimestamp();
        consoleLOgs();

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
  $: {
    (async () => {
      if (endOffLogsIntersect) {
        await getFullLogsSet();

        logsFromWS = [];
      }
    })();
  }
  onMount(() => {
    const logsContEl = document.querySelector("#logs");

    logsContEl.addEventListener(
      "scroll",
      function () {
        // or window.addEventListener("scroll"....
        let st = window.pageYOffset || logsContEl.scrollTop;
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
            message={logItem?.at(1)}
            status={getLogLineStatus(logItem?.at(1))}
            isHiglighted={new Date($lastLogTimestamp).getTime() <
              new Date(logItem?.at(0)).getTime()}
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

      <IntersectionObserver
        element={endOffLogs}
        bind:intersecting={endOffLogsIntersect}
      >
        <div id="endOfLogs" bind:this={endOffLogs} />
      </IntersectionObserver>
    </table>
    {#if !endOffLogsIntersect}
      <div>
        <ButtonToBottom
          number={logsFromWS.length}
          ico={"Down"}
          callBack={async () => {
            offset = 0;
            scrollFromButton = true;
            // checkLogsFromWs();
            scrollToBottom();
            // fetchedLogs(true, 0);
            // fetchedLogs(true, limit * 1);
            // fetchedLogs(true, limit * 2);

            setTimeout(() => {
              scrollFromButton = false;
            }, 500);
          }}
        />
      </div>
    {/if}
  </div>
</div>
