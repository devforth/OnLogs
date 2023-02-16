<script>
  // @ts-nocheck

  import LogsString from "../../lib/LogsString/LogsString.svelte";
  import fetchApi from "../../utils/fetch";
  import { navigate } from "svelte-routing";
  import { afterUpdate, onMount, tick } from "svelte";
  import LogsViewHeder from "./LogsViewHeder/LogsViewHeder.svelte";
  import IntersectionObserver from "svelte-intersection-observer";
  import Spiner from "./Spiner.svelte";
  import LogStringHeader from "./LogStringHeader.svelte";

  import {
    store,
    lastChosenHost,
    lastChosenService,
    lastLogTimestamp,
    chosenLogsString,
    isPending,
    urlHash,
    isFeatching,
  } from "../../Stores/stores";
  import ButtonToBottom from "../../lib/ButtonToBottom/ButtonToBottom.svelte";
  import {
    getLogLineStatus,
    transformLogString,
    transformLogStringForTimeBudget,
    getLogs,
    getPrevLogs,
    scrollToBottom,
    scrollToNewLogsEnd,
    scrollToSpecificLog,
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

  let dateEls = [];
  let dateIntersects = [];
  let lastVisibleEl = null;
  let endOffLogs = null;
  let startOfLogs = null;
  let startOfLogsIntersect = null;
  let endOffLogsIntersect = null;

  let initialScroll = 0;
  let lastScrollTop = 0;
  let scrollDirection = "up";
  let pinedDate = ";";
  let lastFetchActionIsFetch = true;
  let scrollFromButton = false;
  let stopLogsUnfetch = false;
  let stopAllFetch = false;
  let mouseDownBlockFetch = false;
  let extremalScrollId = "";
  let interceptorsWait = false;
  let autoscroll = false;
  let div;
  let getFullLogsSetIsTrottle = false;
  let pauseWS = false;
  let newLogsAmount = 1;
  let controller = null;
  let signal = null;
  let topFetchIsStarted = false;

  function findLastVisibleLog() {
    let index = dateIntersects.indexOf(true);
    if (dateEls[index]) {
      pinedDate = dateEls[index].classList.value;
    }
  }

  $: {
    if (dateIntersects) {
      findLastVisibleLog();
    }
  }

  //fetch params:

  let searchText = "";
  let limit = 60;

  let startWith = "";
  let tmpStartWith = [];

  function resetAllLogs() {
    allLogs = [];
    newLogs = [];
    visibleLogs = [];
    previousLogs = [];
    logsFromWS = [];
  }

  async function getFullLogsSet() {
    if (!getFullLogsSetIsTrottle && $lastChosenService) {
      const initialService = $lastChosenService;
      pauseWS = true;
      const data = [
        ...(await api.getLogs({
          containerName: $lastChosenService,
          hostName: $lastChosenHost,
          limit: limit * 3,
          search: searchText,
          caseSens: !$store.caseInSensitive,
        })),
      ];
      if (initialService === $lastChosenService) {
        allLogs = [...data.reverse()];
        let allLogsCopy = [...allLogs];

        newLogs = allLogsCopy.splice(0, limit);

        visibleLogs = allLogsCopy.splice(0, limit);
        previousLogs = allLogsCopy.splice(0, limit);
      }
      isPending.set(false);
      autoscroll = true;
      pauseWS = false;

      logsFromWS = [];
      getFullLogsSetIsTrottle = true;
      setTimeout(() => {
        getFullLogsSetIsTrottle = false;
      }, 500);
    }
  }

  async function checkIfHashIsInUrl() {
    getLogsFromWS();
    topFetchIsStarted = true;
    let timeStamp = "";
    if ($urlHash) {
      timeStamp = $urlHash.slice(1);
      await fetchIfHashIsInUrl(timeStamp);
    } else {
      await fetchLogAfterChangeService();

      setLastLogTimestamp();
      scrollToBottom();
    }

    urlHash.set("");
    setTimeout(() => {
      setTimeout(() => {
        setInitialScroll(1);
        topFetchIsStarted = false;
      }, 1000);
    });
  }

  function setLastLogTimestamp() {
    lastLogTimestamp.set(new Date().getTime());
  }
  async function fetchLogAfterChangeService() {
    await getFullLogsSet();
    isPending.set(false);
  }
  async function fetchIfHashIsInUrl(startWith) {
    const initialService = $lastChosenService;
    const viewLogs = [
      ...(await api.getLogsWithPrev({
        containerName: $lastChosenService,
        hostName: $lastChosenHost,
        limit: limit * 2,
        startWith,
      })),
    ].reverse();
    let downLogs = [];
    let upperLogs = [];

    if (viewLogs.length !== limit * 2) {
      let limitDifference = limit * 2 - viewLogs.length;

      downLogs = [
        ...(await api.getPrevLogs({
          containerName: $lastChosenService,
          limit: limit + limitDifference,
          startWith: startWith,
          hostName: $lastChosenHost,
        })),
      ];
    } else {
      downLogs = [
        ...(await api.getPrevLogs({
          containerName: $lastChosenService,
          limit,
          startWith: startWith,
          hostName: $lastChosenHost,
        })),
      ];

      if (limit - downLogs.length) {
        upperLogs = await api.getLogs({
          containerName: $lastChosenService,
          limit: limit - downLogs.length,
          startWith: viewLogs?.at(0)[0],
          hostName: $lastChosenHost,
        });
      }
    }
    if (initialService === $lastChosenService) {
      allLogs = [...upperLogs.reverse(), ...viewLogs, ...downLogs];

      let allLogsCopy = [...allLogs];

      newLogs = allLogsCopy.splice(0, limit);

      visibleLogs = allLogsCopy.splice(0, limit);
      previousLogs = allLogsCopy.splice(0, limit);

      chosenLogsString.set(startWith);

      setLastLogTimestamp();
      await tick();
      isPending.set(false);

      urlHash.set("");
      let counter = 0;
      const intervalId = setInterval(() => {
        counter = counter + 1;
        scrollToSpecificLog(".chosen");
        if (counter > 10 || document.querySelector(".chosen")) {
          clearInterval(intervalId);
        }
      }, 500);
    }
    setTimeout(() => {
      topFetchIsStarted = false;
    }, 5000);
  }

  function addLogFromWS(logfromWS) {
    if (
      (!mouseDownBlockFetch && endOffLogsIntersect) ||
      (allLogs.length < 3 * limit && endOffLogsIntersect)
    ) {
      if (!pauseWS) {
        autoscroll = true;
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
      }
    } else {
      logsFromWS = [...logsFromWS, logfromWS];
      autoscroll = false;
    }
  }

  function getLogsFromWS() {
    if (webSocket) {
      closeWS();
    }
    webSocket = new WebSocket(
      `${api.wsUrl}getLogsStream?host=${$lastChosenHost}&id=${$lastChosenService}`
    );

    webSocket.onmessage = (event) => {
      if (event.data !== "PING") {
        const logfromWS = JSON.parse(event.data);

        if (searchText === "") {
          addLogFromWS(logfromWS);
        } else {
          if ($store.caseInSensitive) {
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
            isFeatching.set(false);
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
    if (!$isFeatching) {
      stopLogsUnfetch = false;
      controller = new AbortController();
      signal = controller.signal;
      // if (mouseDownBlockFetch) {
      //   return;
      // }
      const initialService = $lastChosenService;
      if (scrollDirection === "up") {
        isFeatching.set(true);

        try {
          const data = await getLogs({
            containerName: $lastChosenService,
            search: searchText,
            limit,

            caseSens: !$store.caseInSensitive,
            startWith: customStartWith
              ? customStartWith
              : customStartWith === 0
              ? ""
              : allLogs.at(0)?.at(0),
            hostName: $lastChosenHost,
            signal,
          });

          lastFetchActionIsFetch = true;

          if (initialService === $lastChosenService) {
            if (data.length) {
              let numberOfNewLogs = data.length;

              const logsToPrevious = visibleLogs.splice(0, numberOfNewLogs);

              const logsToVisible = newLogs.splice(0, numberOfNewLogs);

              previousLogs.splice(0, numberOfNewLogs);
              newLogs = [...data, ...newLogs];
              visibleLogs = [...logsToVisible, ...visibleLogs];
              previousLogs = [...logsToPrevious, ...previousLogs];
              previousLogs.length = limit;

              allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
            }
            if (data.length === limit) {
              setTimeout(() => {
                if (!doNotScroll) {
                  scrollToNewLogsEnd(".newLogsEnd");
                }
              }, 50);
            }
          }

          return data;
        } catch (e) {
          console.log(e);
        }
      }
    }
    isFeatching.set(false);
  };
  const fetchedTopLogs = async (customStartWith) => {
    const initialService = $lastChosenService;

    if (controller) {
      controller.abort();
      isFeatching.set(false);
    }
    let fetchedData = [];

    if (!$isFeatching && !topFetchIsStarted) {
      isFeatching.set(true);

      topFetchIsStarted = true;

      const data = await getLogs({
        containerName: $lastChosenService,
        search: searchText,
        limit,

        caseSens: !$store.caseInSensitive,
        startWith: customStartWith
          ? customStartWith
          : customStartWith === 0
          ? ""
          : allLogs.at(0)?.at(0),
        hostName: $lastChosenHost,
      });

      isFeatching.set(false);

      if (initialService === $lastChosenService) {
        if (data.length === limit) {
          console.log(data.length);
          let numberOfNewLogs = data.length;

          const logsToPrevious = visibleLogs.splice(0, numberOfNewLogs);

          const logsToVisible = newLogs.splice(0, numberOfNewLogs);

          previousLogs.splice(0, numberOfNewLogs);
          newLogs = [...data, ...newLogs];
          visibleLogs = [...logsToVisible, ...visibleLogs];
          previousLogs = [...logsToPrevious, ...previousLogs];

          allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
        }
        fetchedData = data;
      }
    }
    return fetchedData;
  };
  const unfetchedLogs = async () => {
    if (!$isFeatching) {
      if (scrollDirection === "down" && !scrollFromButton && !stopLogsUnfetch) {
        const initialService = $lastChosenService;
        isFeatching.set(true);

        const data = await getPrevLogs({
          containerName: $lastChosenService,
          search: searchText,
          limit,

          caseSens: !$store.caseInSensitive,
          startWith: allLogs.at(-1)[0],
          hostName: $lastChosenHost,
        });
        isFeatching.set(false);
        if (initialService === $lastChosenService) {
          if (data.length) {
            let numberOfPrev = data.length;
            const logsToVisible = previousLogs.splice(0, numberOfPrev);
            const logsToNew = visibleLogs.splice(0, numberOfPrev);
            newLogs.splice(0, numberOfPrev);
            newLogs = [...newLogs, ...logsToNew];
            visibleLogs = [...visibleLogs, ...logsToVisible];
            previousLogs = [...previousLogs, ...data];
            allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
            if (data.length === limit) {
              scrollToNewLogsEnd(".newLogsEnd", true);
            }

            lastFetchActionIsFetch = false;
            stopLogsUnfetch = false;
          }
          return data;
        }
      }
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

        isPending.set(true);
        closeWS();
        await checkIfHashIsInUrl();
      }
    })();
  }

  $: {
    (async () => {
      if (searchText) {
        resetParams();
        resetAllLogs();
        isPending.set(true);
        await getFullLogsSet();
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
  // $: {
  //   isInterceptorVIsible(intersects[2], fetchedLogs, mouseDownBlockFetch);
  // }
  $: {
    isInterceptorVIsible(intersects[3], unfetchedLogs, mouseDownBlockFetch);
  }
  $: {
    if (allLogs) {
    }
  }

  $: {
    (async () => {
      if (endOffLogsIntersect && logsFromWS.length) {
        logsFromWS.length && (await getFullLogsSet());
        logsFromWS = [];
      }
    })();
  }

  const checkIfScrollOnTop = () => {
    const checkIfScrollOnTopInterval = setInterval(async () => {
      if (
        startOfLogsIntersect &&
        allLogs.length >= 3 * limit &&
        !topFetchIsStarted
      ) {
        const data = await fetchedTopLogs();
        newLogsAmount = data.length;
        isFeatching.set(false);

        scrollToSpecificLog(".fetchFromStart", {
          behavior: "auto",
          block: "start",
          inline: "center",
        });

        topFetchIsStarted = false;
      }
    }, 500);
  };

  onMount(async () => {
    checkIfScrollOnTop();
    initialScroll = 1;
    let isEventOnScroll = false;
    const interval = setInterval(() => {
      const logsContEl = document.querySelector("#logs");

      if (logsContEl) {
        logsContEl.addEventListener("scroll", function () {
          let st = window.pageYOffset || logsContEl.scrollTop;
          if (st > lastScrollTop) {
            scrollDirection = "down";
          } else {
            scrollDirection = "up";
          }
          lastScrollTop = st <= 0 ? 0 : st; // For Mobile or negative scrolling
        });
        isEventOnScroll = true;
      }
      if (isEventOnScroll) {
        clearInterval(interval);
      }
    }, 1000);

    window.addEventListener("resize", () => {
      const logsContEl = document.querySelector("#logs");
      if (logsContEl) {
        limit = Math.round(logsContEl.offsetHeight / 200) * 10;
        console.log("resized", limit);
      }
    });
  });

  afterUpdate(() => {
    if (autoscroll) {
      div && div.scrollTo(0, div.scrollHeight ? div.scrollHeight : 0);
    }
    autoscroll = false;
  });
</script>

<LogsViewHeder bind:searchText />
<div><div class="timeBudge pined">{pinedDate}</div></div>

{#if allLogs.length === 0 && !$isPending}
  <h2 class="noLogsMessage">No logs written yet</h2>
{/if}
{#if $isPending}<Spiner />{:else}
  <div id="logs" class="logs" bind:this={div}>
    <div class="logsTableContainer">
      <table class="logsTable {$store.breakLines ? 'breakLines' : ''}">
        <div id="startOfLogs" />
        <IntersectionObserver
          element={startOfLogs}
          bind:intersecting={startOfLogsIntersect}
        >
          <div id="startOfLogs" bind:this={startOfLogs} />
        </IntersectionObserver>

        {#each allLogs as logItem, i}
          {#if transformLogStringForTimeBudget(logItem, $store.UTCtime) !== transformLogStringForTimeBudget(allLogs[i - 1], $store.UTCtime) && i - 1 >= 0}
            <div class="timeBudgeContainer">
              <div class="timeBadgeWrapper">
                <div class="timeBudge">
                  {transformLogStringForTimeBudget(logItem, $store.UTCtime)}
                </div>
              </div>
            </div>
          {/if}
          <div
            class="{i === limit * 1.5 - 1
              ? 'newLogsEnd'
              : i === allLogs.length - limit * 1.5
              ? 'newLogsStart'
              : i === limit / 2
              ? 'interseptor'
              : ''}{i === newLogsAmount - 1 ? 'fetchFromStart' : ''}"
            bind:this={elements[i]}
          >
            <div
              class="chosenString clickable {$chosenLogsString ===
              logItem?.at(0)
                ? 'chosen'
                : ''}"
              on:click={(e) => {
                let option = "";
                if ($chosenLogsString !== logItem?.at(0)) {
                  option = logItem?.at(0);
                }
                chosenLogsString.set(option);
              }}
            >
              {#if $chosenLogsString === logItem?.at(0)}
                <LogStringHeader />
              {/if}
              {#if i === limit / 2 - 1}
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
            <IntersectionObserver
              element={dateEls[i]}
              bind:intersecting={dateIntersects[i]}
            >
              <div
                class={transformLogStringForTimeBudget(logItem, $store.UTCtime)}
                bind:this={dateEls[i]}
              />
            </IntersectionObserver>
          </div>
        {/each}

        <IntersectionObserver
          element={endOffLogs}
          bind:intersecting={endOffLogsIntersect}
        >
          <div id="endOfLogs" bind:this={endOffLogs} />
        </IntersectionObserver>
      </table>
      {#if !endOffLogsIntersect && allLogs.length}
        <div>
          <ButtonToBottom
            number={logsFromWS.length}
            ico={"Down"}
            callBack={async () => {
              scrollFromButton = true;
              autoscroll = true;
              scrollDirection === "up";
              // logsFromWS.length && (await getFullLogsSet());

              setTimeout(() => {
                scrollFromButton = false;
              }, 500);
            }}
          />
        </div>
      {/if}
    </div>
    <div class="timeBudgeContainer" />
  </div>{/if}
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
