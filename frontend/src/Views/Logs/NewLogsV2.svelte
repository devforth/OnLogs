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
  let mouseDownBlockFetch = false;
  let extremalScrollId = "";
  let interceptorsWait = false;

  //fetch params:

  let searchText = "";
  let limit = 30;

  let caseSens = false;
  let startWith = "";
  let tmpStartWith = [];

  function resetAllLogs() {
    allLogs = [];
    newLogs = [];
    visibleLogs = [];
    previousLogs = [];
  }
  async function getFullLogsSet() {
    if (initialScroll) {
      const data1 = await fetchedLogs(true, 0);
      if (data1.length === limit) {
        const data2 = await fetchedLogs(true, data1?.at(0)?.at(0));
        if (data2.length === limit) {
          await fetchedLogs(true, data2?.at(0)?.at(0));
        }
      }
      logsFromWS = [];
    }
  }
  function setLastLogTimestamp() {
    lastLogTimestamp.set(new Date().getTime());
  }

  function addLogFromWS(logfromWS) {
    if (
      (!mouseDownBlockFetch && endOffLogsIntersect) ||
      allLogs.length < 3 * limit
    ) {
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

  function isInterceptorVIsible(inter, cb, limitation) {
    (async () => {
      if (inter && initialScroll && !interceptorsWait) {
        let delay = mouseDownBlockFetch ? 1000 : 0;
        if (allLogs.length >= 3 * limit && limitation) {
          interceptorsWait = true;
          extremalScrollId = setTimeout(async () => {
            const data = await cb();
            interceptorsWait = false;
          }, delay);
        }
      }
    })();
  }

  function setInitialScroll(val) {
    initialScroll = val;
  }
  const fetchedLogs = async (doNotScroll, customStartWith) => {
    stopLogsUnfetch = false;
    // if (mouseDownBlockFetch) {
    //   return;
    // }

    const data = await getLogs({
      containerName: $lastChosenService,
      search: searchText,
      limit,

      caseSens,
      startWith: customStartWith
        ? customStartWith
        : customStartWith === 0
        ? ""
        : allLogs.at(0)?.at(0),
      hostName: $lastChosenHost,
    });
    lastFetchActionIsFetch = true;
    if (data.length) {
      let numberOfNewLogs = data.length;
      const logsToPrevious = visibleLogs.splice(0, numberOfNewLogs);
      const logsToVisible = newLogs.splice(0, numberOfNewLogs);
      previousLogs.splice(0, numberOfNewLogs);
      newLogs = [...data, ...newLogs];
      visibleLogs = [...logsToVisible, ...visibleLogs];
      previousLogs = [...logsToPrevious, ...previousLogs];

      allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
    }
    if (data.length === limit) {
      setTimeout(() => {
        if (!doNotScroll) {
          scrollToNewLogsEnd(".newLogsEnd");
        }
      }, 50);
    } else {
      stopFetch = true;
    }

    return data;
  };
  const unfetchedLogs = async () => {
    if (scrollDirection === "down" && !scrollFromButton && !stopLogsUnfetch) {
      const data = await getPrevLogs({
        containerName: $lastChosenService,
        search: searchText,
        limit,

        caseSens,
        startWith: allLogs.at(-1)[0],
        hostName: $lastChosenHost,
      });
      if (data.length) {
        let numberOfPrev = data.length;
        const logsToVisible = previousLogs.splice(0, numberOfPrev);
        const logsToNew = visibleLogs.splice(0, numberOfPrev);
        newLogs.splice(0, numberOfPrev);
        newLogs = [...newLogs, ...logsToNew];
        visibleLogs = [...visibleLogs, ...logsToVisible];
        previousLogs = [...previousLogs, ...data];
        allLogs = [...newLogs, ...visibleLogs, ...previousLogs];

        lastFetchActionIsFetch = false;
        stopLogsUnfetch = false;
      }

      if (data.length === limit) {
      } else {
        stopLogsUnfetch = true;
        logsFromWS = [];
      }

      scrollToNewLogsEnd(".newLogsStart", true);

      return data;
    }
  };

  //

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
      } else {
        getFullLogsSet();
      }
    })();
  }

  $: {
    isInterceptorVIsible(intersects[0], fetchedLogs, !mouseDownBlockFetch);
  }
  $: {
    isInterceptorVIsible(intersects[1], unfetchedLogs, !mouseDownBlockFetch);
  }
  $: {
    isInterceptorVIsible(intersects[2], fetchedLogs, mouseDownBlockFetch);
  }
  $: {
    isInterceptorVIsible(intersects[3], unfetchedLogs, mouseDownBlockFetch);
  }

  //   $: {
  //     (async () => {
  //       if (endOffLogsIntersect) {
  //         await getFullLogsSet();

  //         logsFromWS = [];
  //       }
  //     })();
  //   }
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

    window.addEventListener("resize", () => {
      limit = Math.round(logsContEl.offsetHeight / 300) * 10;
    });
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

          {#if i === 0 && allLogs.length >= 3 * limit}
            <IntersectionObserver
              element={elements[2]}
              bind:intersecting={intersects[2]}
            >
              <div class="observer" bind:this={elements[2]} />
            </IntersectionObserver>{/if}

          {#if i === allLogs.length - 1 && allLogs.length >= 3 * limit}
            <IntersectionObserver
              element={elements[3]}
              bind:intersecting={intersects[3]}
            >
              <div class="observer" bind:this={elements[3]} />
            </IntersectionObserver>{/if}
          <LogsString
            time={transformLogString(logItem, $store.UTCtime)}
            message={logItem?.at(1)}
            status={getLogLineStatus(logItem?.at(1))}
            isHiglighted={new Date($lastLogTimestamp).getTime() <
              new Date(logItem?.at(0)).getTime()}
          />
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
            scrollFromButton = true;

            scrollToBottom();
            getFullLogsSet();

            setTimeout(() => {
              scrollFromButton = false;
            }, 500);
          }}
        />
      </div>
    {/if}
  </div>
</div>
<svelte:window
  on:mousedown={(e) => {
    mouseDownBlockFetch = true;
  }}
  on:mouseup={(e) => {
    mouseDownBlockFetch = false;
    clearInterval(extremalScrollId);
    interceptorsWait = false;
  }}
/>
