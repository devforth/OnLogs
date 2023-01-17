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

  //fetch params:

  let searchText = "";
  let limit = 30;
  let offset = 0;

  let caseSens = false;
  let startWith = "";
  let tmpStartWith = [];

  //functions
  function setLastLogTimestamp() {
    if (allLogs.at(-1)) {
      lastLogTimestamp.set(allLogs.at(-1)?.at(0));
    }
  }

  function addLogFromWS(logfromWS) {
    if (endOffLogsIntersect) {
      if (newLogs.length === limit) {
        newLogs.splice(0, 1);
      }
      newLogs.push(visibleLogs.at(0));
      if (visibleLogs.length === limit) {
        visibleLogs.splice(0, 1);
      }
      visibleLogs.push(previousLogs.at(0));
      if (previousLogs.length === limit) {
        previousLogs.splice(0, 1);
      }
      previousLogs.push(logfromWS);

      allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
    } else {
      logsFromWS = [...logsFromWS, logfromWS];
    }
  }

  function refreshAllLogs() {
    previousLogs = [];
    visibleLogs = [];
    newLogs = [];
  }
  function checkLogsFromWs() {
    if (logsFromWS.length >= 3 * limit) {
      let newAllLogs = [
        ...logsFromWS.filter((el, i) => {
          return i < 3 * limit;
        }),
      ];
      newLogs = [...newAllLogs.splice(0, limit)];
      visibleLogs = [...newAllLogs.splice(0, limit)];
      previousLogs = [...newAllLogs.splice(0, limit)];
      allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
      logsFromWS = [];

      return;
    } else {
      if (logsFromWS.length >= 2 * limit) {
        newLogs.splice(0, logsFromWS.length - 2 * limit);
        newLogs = [
          ...logsFromWS.splice(0, logsFromWS.length - 2 * limit),
          ...newLogs,
        ];
        visibleLogs = [...logsFromWS.splice(0, limit)];
        previousLogs = [...logsFromWS.splice(0, limit)];
      }
      if (logsFromWS.length >= limit) {
        const logsToNew = visibleLogs.splice(0, logsFromWS.length - limit);
        newLogs.splice(0, logsToNew.length);
        newLogs = [...newLogs, ...logsToNew];
        visibleLogs = [
          ...logsFromWS.splice(0, logsFromWS.length - limit),
          ...visibleLogs,
        ];
        previousLogs = [...logsFromWS.splice(0, limit)];
      }
      if (logsFromWS.length <= limit && logsFromWS.length >= 0) {
        const logsToVisible = previousLogs.splice(0, logsFromWS.length);
        const logsToNew = visibleLogs.splice(0, logsFromWS.length);
        newLogs.splice(0, logsToVisible.length);
        newLogs = [...newLogs, ...logsToNew];
        visibleLogs = [...visibleLogs, ...logsToVisible];
        previousLogs = [...previousLogs, ...logsFromWS];
      }
      if (newLogs.at(0) && visibleLogs.at(0) && previousLogs.at(0)) {
        allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
      }
    }

    logsFromWS = [];
  }

  function getLogsFromWS() {
    webSocket = new WebSocket(
      `${api.wsUrl}getLogsStream?id=${$lastChosenService}`
    );
    webSocket.onmessage = (event) => {
      if (event.data !== "PING") {
        const logfromWS = JSON.parse(event.data);
        offset = offset + 1;
        console.log(offset);
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
      navigate(
        `${location.pathname.replace(/\/offset=[0-9]+/i, `/offset=${offset}`)}`,
        { replace: true }
      );
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
      if (allLogs.length === 3 * limit) {
        allLogs = [...logsOverflow, ...allLogs];
      } else {
        allLogs = logsOverflow;
      }
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
        if (allLogs.length === 3 * limit) {
          allLogs = [...logsOverflow, ...allLogs];
        } else {
          allLogs = logsOverflow;
        }
      }

      scrollToNewLogsEnd(".newLogsStart", true);
      navigate(
        `${location.pathname.replace(/\/offset=[0-9]+/i, `/offset=${offset}`)}`,
        { replace: true }
      );

      return data;
    }
  };

  //
  function consoleLOgs() {
    console.log($lastLogTimestamp);
  }

  $: {
    (async () => {
      if ($lastChosenHost && $lastChosenService) {
        setInitialScroll(0);
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
  // $: {
  //   if (endOffLogsIntersect) {
  //     checkLogsFromWs();
  //   }
  // }
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
            message={logItem.at(1)}
            status={getLogLineStatus(logItem.at(1))}
            isHiglighted={new Date($lastLogTimestamp).getTime() <
              new Date(logItem.at(0)).getTime()}
          />
          {logItem[0][14]}{logItem[0][15]}{logItem[0][17]}{logItem[0][18]}
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

      <div id="endOfLogs" />
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
            fetchedLogs(true);
            fetchedLogs(true);
            fetchedLogs(true);

            setTimeout(() => {
              scrollFromButton = false;
            }, 500);
          }}
        />
      </div>
    {/if}
  </div>
</div>
