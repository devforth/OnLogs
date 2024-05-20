<script>
  // @ts-nocheck

  import LogsString from "../../lib/LogsString/LogsString.svelte";
  import fetchApi from "../../utils/fetch";
  import { navigate } from "svelte-routing";
  import { afterUpdate, onMount, tick } from "svelte";
  import LogsViewHeder from "./LogsViewHeder/LogsViewHeder.svelte";
  import IntersectionObserver from "svelte-intersection-observer";
  import Spiner from "./Spiner.svelte";
  import Loader from "./Loader.svelte";
  import LogStringHeader from "./LogStringHeader.svelte";
  import { fade } from "svelte/transition";
  import { handleKeydown, copyCustomText } from "../../utils/functions.js";
  import { findSearchTextInLogs } from "../../Views/Logs/functions.js";

  import {
    store,
    lastChosenHost,
    lastChosenService,
    lastLogTimestamp,
    lastLogTime,
    toast,
    toastIsVisible,
    toastTimeoutId,
    chosenLogsString,
    isPending,
    urlHash,
    isFeatching,
    isSearching,
    chosenStatus,
    WSisMuted,
    manuallyUnmuted,
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
  import { debug } from "svelte/internal";
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
  let pinedBadgeTimer = null;
  let pinedBadgeIsVisible = false;

  function refreshStatus() {
    chosenStatus.set("");
  }

  async function highlightSearchText() {
    if (searchText) {
      await tick();

      findSearchTextInLogs(
        ".string, .message p",
        searchText,
        !$store.caseInSensitive
      );
    }
  }

  function setLastLogTime(timeStamp) {
    lastLogTime.set(timeStamp);
  }

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
      let total_logs_amount = 0;
      let last_key = "";
      let is_all_logs_processed = false;
      while (total_logs_amount < limit && !is_all_logs_processed) {
        isSearching.set(true);
        let data = await api.getLogs({
            containerName: $lastChosenService,
            hostName: $lastChosenHost,
            limit: limit * 3,
            search: searchText,
            caseSens: !$store.caseInSensitive,
            status: $chosenStatus,
            startWith: last_key,
        })
        last_key = data.last_processed_key;
        is_all_logs_processed = data.is_end;
        total_logs_amount += data.logs.length;

        if (initialService === $lastChosenService) {
            setLastLogTime(data.logs.reverse()?.at(0)?.at(0));
            allLogs = [...allLogs, ...data.logs];
            let allLogsCopy = [...allLogs];

            newLogs = allLogsCopy.splice(0, limit);

            visibleLogs = allLogsCopy.splice(0, limit);
            previousLogs = allLogsCopy.splice(0, limit);
        }
      }
      isSearching.set(false);
      isPending.set(false);
      autoscroll = true;
      pauseWS = false;

      logsFromWS = [];
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
      })).logs,
    ].reverse();
    let downLogs = [];
    let upperLogs = [];

    if (viewLogs.length !== limit * 2) {
      let limitDifference = limit * 2 - viewLogs.length;

      downLogs = [
        ...(await api.getPrevLogs({
          containerName: $lastChosenService,
          hostName: $lastChosenHost,
          status: $chosenStatus,
        })).logs,
      ];
    } else {
      downLogs = [
        ...(await api.getPrevLogs({
          containerName: $lastChosenService,
          limit,
          startWith: startWith,
          hostName: $lastChosenHost,
          status: $chosenStatus,
        })).logs,
      ];

      if (limit - downLogs.length) {
        upperLogs = await api.getLogs({
          containerName: $lastChosenService,
          limit: limit - downLogs.length,
          startWith: viewLogs?.at(0)[0],
          hostName: $lastChosenHost,
          status: $chosenStatus,
        }).logs;
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
    const MESSAGE_COUNT_INTERVAL = 50;
    const MESSAGE_FREQUENCY_THRESHOLD = 5;
    if (webSocket) {
      closeWS();
    }
    let messageCount = 0;
    let startTime = Date.now();
    WSisMuted.set(false);

    webSocket = new WebSocket(
      `${api.wsUrl}getLogsStream?host=${$lastChosenHost}&id=${$lastChosenService}`
    );

    webSocket.onmessage = (event) => {
      if (event.data !== "PING") {
        const logfromWS = JSON.parse(event.data);
        setLastLogTime(logfromWS[0]);

        if (!$WSisMuted || $manuallyUnmuted) {
          messageCount++;
          if (messageCount % MESSAGE_COUNT_INTERVAL === 0) {
            const elapsedSeconds = (Date.now() - startTime) / 1000;
            const frequency = messageCount / elapsedSeconds;

            if (frequency > MESSAGE_FREQUENCY_THRESHOLD && !$manuallyUnmuted) {
              WSisMuted.set(true);

              toast.set({
                tittle: "Warning",
                message:
                  "Too many messages to display logs. Continuing may be dangerous.",
                position: "",
                status: "Warning",
                additionButton: {
                  isVisible: true,
                  CB: () => {
                    manuallyUnmuted.set(true);
                    WSisMuted.set(false);
                  },
                  title: "Continue",
                },
              });
              toastIsVisible.set(true);
              
            }
          }

          if (
            $chosenStatus &&
            $chosenStatus !== logfromWS[1].split(" ")[0]?.toLowerCase()
          ) {
            return;
          } else {
            if (searchText === "") {
              addLogFromWS(logfromWS);
            } else {
              if ($store.caseInSensitive) {
                if (logfromWS[1].includes(searchText)) {
                  addLogFromWS(logfromWS);
                }
              } else {
                if (
                  logfromWS[1].toLowerCase().includes(searchText.toLowerCase())
                ) {
                  addLogFromWS(logfromWS);
                }
              }
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
    pinedDate = "";
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
          let total_logs_amount = 0;
          let total_logs = [];
          let is_all_logs_processed = false;
          let last_key = customStartWith ? customStartWith : customStartWith === 0 ? "" : allLogs.at(0)?.at(0);
          while (limit < total_logs_amount && !is_all_logs_processed) {
            isSearching.set(true);
            const data = (await getLogs({
              containerName: $lastChosenService,
              search: searchText,
              limit,
              status: $chosenStatus,
              caseSens: !$store.caseInSensitive,
              startWith: last_key,
              hostName: $lastChosenHost,
              signal,
            })).logs.reverse();
            total_logs = [...total_logs, ...data];
            total_logs_amount += data.length;
            is_all_logs_processed = data.is_end;
            last_key = data.last_processed_key;

            if (initialService === $lastChosenService) {
                if (data.length) {
                let numberOfNewLogs = data.length;

                const logsToPrevious = visibleLogs.splice(
                    visibleLogs.length - numberOfNewLogs,
                    numberOfNewLogs
                );

                const logsToVisible = newLogs.splice(
                    newLogs.length - numberOfNewLogs,
                    numberOfNewLogs
                );

                previousLogs.splice(
                    previousLogs.length - numberOfNewLogs,
                    numberOfNewLogs
                );
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
          isSearching.set(false);
        } 
          lastFetchActionIsFetch = true;
          return total_logs;
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

      let total_logs = [];
      let total_received_logs_count = 0;
      let is_all_logs_processed = false;
      let last_key = customStartWith ? customStartWith : customStartWith === 0 ? "" : allLogs.at(0)?.at(0);
        
      while (limit > total_received_logs_count && !is_all_logs_processed) {
        isSearching.set(true);
        const data = await getLogs({
          containerName: $lastChosenService,
          search: searchText,
          limit,
          status: $chosenStatus,

          caseSens: !$store.caseInSensitive,
          startWith: last_key,
          hostName: $lastChosenHost,
        });
        is_all_logs_processed = data.is_end;
        last_key = data.last_processed_key;
        total_received_logs_count += data.logs.length;
        total_logs = [...total_logs, ...data.logs.reverse()];
      }
      isSearching.set(false);
      isFeatching.set(false);

      if (initialService === $lastChosenService) {
        if (total_logs.length === limit) {
          let numberOfNewLogs = total_logs.length;
          const logsToPrevious = visibleLogs.splice(0, numberOfNewLogs);
          const logsToVisible = newLogs.splice(0, numberOfNewLogs);
          previousLogs.splice(0, numberOfNewLogs);
          newLogs = [...total_logs, ...newLogs];
          visibleLogs = [...logsToVisible, ...visibleLogs];
          previousLogs = [...logsToPrevious, ...previousLogs];
          allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
        }
        fetchedData = total_logs;
      }
    }
    return fetchedData;
  };
  const unfetchedLogs = async () => {
    if (!$isFeatching) {
      if (scrollDirection === "down" && !scrollFromButton && !stopLogsUnfetch) {
        const initialService = $lastChosenService;
        isFeatching.set(true);
        
        let last_key = allLogs.at(-1) ? allLogs.at(-1)[0] : "";
        let total_logs = [];
        let total_received_logs_count = 0;
        let is_all_logs_processed = false;
        while (total_received_logs_count < limit && !is_all_logs_processed) {
          isSearching.set(true);
          const data = await getPrevLogs({
            containerName: $lastChosenService,
            search: searchText,
            limit,
            status: $chosenStatus,

            caseSens: !$store.caseInSensitive,
            startWith: last_key,
            hostName: $lastChosenHost,
          });
          is_all_logs_processed = data.is_end;
          last_key = data.last_processed_key;
          total_logs = [...total_logs, ...data.logs];
        }
        isSearching.set(false);
        isFeatching.set(false);
        if (initialService === $lastChosenService) {
            if (total_logs.length) {
              let numberOfPrev = total_logs.length;
              const logsToVisible = previousLogs.splice(0, numberOfPrev);
              const logsToNew = visibleLogs.splice(0, numberOfPrev);
              newLogs.splice(0, numberOfPrev);
              newLogs = [...newLogs, ...logsToNew];
              visibleLogs = [...visibleLogs, ...logsToVisible];
              previousLogs = [...previousLogs, ...total_logs];
              allLogs = [...newLogs, ...visibleLogs, ...previousLogs];
              if (total_logs.length === limit) {
                scrollToNewLogsEnd(".newLogsEnd", true);
              }

              lastFetchActionIsFetch = false;
              stopLogsUnfetch = false;
            }
          return total_logs;
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
        refreshStatus();

        isPending.set(true);
        closeWS();
        await checkIfHashIsInUrl();
        addScrollLIstenersToLogs();
      }
    })();
  }

  function addScrollLIstenersToLogs() {
    let isEventOnScroll = false;

    const interval = setInterval(() => {
      const logsContEl = document.querySelector("#logs");

      if (logsContEl) {
        logsContEl.addEventListener("scroll", function () {
          let st = window.scrollY || logsContEl.scrollTop;
          if (st > lastScrollTop) {
            scrollDirection = "down";
          } else if (st != lastScrollTop) {
            scrollDirection = "up";
          }
          lastScrollTop = st <= 0 ? 0 : st; // For Mobile or negative scrolling

          pinedBadgeIsVisible = true;
          if (pinedBadgeTimer !== null) {
            clearTimeout(pinedBadgeTimer);
          }
          pinedBadgeTimer = setTimeout(function () {
            pinedBadgeIsVisible = false;
          }, 350);
        });
        isEventOnScroll = true;
      }
      if (isEventOnScroll) {
        clearInterval(interval);
        isEventOnScroll = false;
      }
    }, 1000);
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
      addScrollLIstenersToLogs();
    })();
  }

  $: {
    (async () => {
      if ($chosenStatus) {
        resetParams();
        resetAllLogs();
        isPending.set(true);
        await getFullLogsSet();
      } else {
        getFullLogsSet();
      }
      addScrollLIstenersToLogs();
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
    if ([...allLogs]) {
      highlightSearchText();
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

    window.addEventListener("resize", () => {
      const logsContEl = document.querySelector("#logs");
      if (logsContEl) {
        limit = Math.round(logsContEl.offsetHeight / 200) * 10;
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
{#if pinedDate}<div>
    {#if pinedBadgeIsVisible || endOffLogsIntersect}<div
        transition:fade={{ duration: 250 }}
        class="timeBudge pined"
      >
        {pinedDate}
      </div>{/if}
  </div>{/if}

{#if allLogs.length === 0 && !$isPending}
  <h2 class="noLogsMessage">No logs written yet</h2>
{/if}
{#if $isPending}<Spiner />{:else}
  <div id="logs" class="logs" bind:this={div}>
    <div class="logsTableContainer">
      <table class="logsTable {$store.breakLines ? 'breakLines' : ''}">
        {#if $isSearching}
        <div style="left: 50%; top: 50%; padding-bottom: 10px;" class="flex">
          <Loader />
        </div>
        {/if}
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
              class="chosenString  {$chosenLogsString === logItem?.at(0)
                ? 'chosen'
                : ''}"
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
                sharedLinkCallBack={() => {
                  let option = "";
                  // if ($chosenLogsString !== logItem?.at(0)) {
                  option = logItem?.at(0);
                  // }
                  chosenLogsString.set(option);
                  const copiedUrl = `${location.href}#${$chosenLogsString}`;
                  copyCustomText(copiedUrl, () => {
                    toast.set({
                      tittle: "Success",
                      message: "URL has been copied",
                      position: "",
                      status: "Success",
                    });
                    if (!$toastIsVisible) {
                      toastIsVisible.set(true);
                      toastTimeoutId.set(
                        setTimeout(() => {
                          toastIsVisible.set(false);
                        }, 3000)
                      );
                    } else {
                      clearTimeout($toastTimeoutId);
                      toastIsVisible.set(false);
                      setTimeout(() => {
                        toastIsVisible.set(true);
                      }, 400);
                    }
                  });
                }}
                getLogsByTagOptions={(limit, searchText)}
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
  on:keydown={(e) => {
    handleKeydown(e, "Escape", () => {
      chosenLogsString.set("");
      chosenStatus.set("");
    });
  }}
/>
