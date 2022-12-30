<script>
  import LogsString from "../../lib/LogsString/LogsString.svelte";
  import fetchApi from "../../utils/fetch";
  import { afterUpdate, onMount, onDestroy } from "svelte";
  import LogsViewHeder from "./LogsViewHeder/LogsViewHeder.svelte";
  import {
    store,
    lastChosenHost,
    lastChosenService,
  } from "../../Stores/stores";
  import ButtonToBottom from "../../lib/ButtonToBottom/ButtonToBottom.svelte";

  let startWith = "";

  let searchText = "";
  let searchOffset = 0;
  let offset = 0,
    logLinesCount = 30,
    oldScrollHeight = 0;
  let allLogs = [],
    tmpLogs = allLogs;
  let searchLogs = [];
  $: tmpSearchLogs = searchLogs;

  let webSocket = undefined;
  let isLogsUpdating = false,
    isUploading = false;

  const api = new fetchApi();
  $: logString = undefined;
  $: logsDiv = undefined;
  $: {
    (async () => {
      if (searchText !== "") {
        searchLogs = [];
        searchOffset = 0;
        startWith = "";
        const data = await getLogsWithSearch(
          $lastChosenService,
          searchText,
          logLinesCount,
          0,
          !$store.caseInSensitive,
          "",
          $lastChosenHost
        );
        simpleScrollToBottom();
      }
    })();
  }

  afterUpdate(() => {
    scrollToBottom();
  });

  let logsContEl = null;
  let buttonToBottomIsVisible = false;

  onMount(() => {
    let isScrolling = false;
    logsContEl = document.querySelector("#logs");

    logsContEl.addEventListener("scroll", () => {
      if (!isScrolling) {
        isScrolling = true;
        setTimeout(() => {
          if (
            logsContEl.scrollTop &&
            logsContEl.scrollHeight - logsContEl.scrollTop !==
              logsContEl.clientHeight
          ) {
            buttonToBottomIsVisible = true;
          } else {
            buttonToBottomIsVisible = false;
          }
          isScrolling = false;
        }, 1000);
      }
    });
  });

  const timezoneOffsetSec = new Date().getTimezoneOffset() * 60;

  function getLogLineStatus(logLine = "") {
    const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
    const statuses_warnings = ["WARN", "WARNING"];
    const statuses_other = ["DEBUG", "INFO", "ONLOGS"];
    const logLineItems = logLine.split(" ");
    var i, j;

    for (i = 0; i < logLineItems.length; i++) {
      for (j = 0; j < logLineItems.length; j++) {
        if (logLineItems[i].includes(statuses_errors[j])) {
          return "error";
        }
      }
      for (j = 0; j < logLineItems.length; j++) {
        if (logLineItems[i].includes(statuses_warnings[j])) {
          return "warn";
        }
      }
      for (j = 0; j < logLineItems.length; j++) {
        if (logLineItems[i].includes(statuses_other[j])) {
          return statuses_other[j].toLowerCase();
        }
      }
    }
    return "";
  }

  function scrollToBottom() {
    const logsCont = document.querySelector("#logs");
    const SCROLL_FINAL_GAP_PX = 20;
    const userScrolledToSpecificLoc =
      logsCont.scrollHeight - logsCont.scrollTop - logsCont.clientHeight >
      SCROLL_FINAL_GAP_PX;

    if (!userScrolledToSpecificLoc) {
      setTimeout(() => {
        logsCont.scrollTop = logsCont.scrollHeight - logsCont.clientHeight;
        oldScrollHeight = logsCont.scrollHeight;
      });
    }
  }

  function simpleScrollToBottom() {
    const el = document.querySelector("#endOfLogs");
    if (!el) {
      return;
    } else {
      el.scrollIntoView({ behavior: "smooth" });
    }
  }

  async function getLogs(
    service = "",
    search = "",
    limit = logLinesCount,
    offset = 0,
    caseSens = false
  ) {
    isUploading = true;
    const newLogs = (
      await api.getLogs(service, search, limit, offset, "", "", $lastChosenHost)
    ).reverse();
    startWith = newLogs?.at(0).at(0);
    offset += newLogs.length;
    isUploading = false;
    allLogs = [...newLogs, ...allLogs];
    console.log(allLogs, "alllogs");
    return newLogs;
  }
  async function getLogsWithSearch(serv, search, limit, offset, caseSenset) {
    isUploading = true;
    const newLogs = (
      await api.getLogs(
        serv,
        search,
        limit,
        offset,
        caseSenset,
        startWith,
        $lastChosenHost
      )
    ).reverse();
    startWith = newLogs?.at(0)?.at(0);
    searchOffset += newLogs.length;
    isUploading = false;

    searchLogs = [...newLogs, ...searchLogs];

    return newLogs;
  }

  async function getLogsStream(service = "") {
    offset = 0;
    allLogs = [];
    tmpLogs = allLogs;
    if (service.localeCompare("") === 0) {
      return;
    }
    if (webSocket !== undefined) {
      webSocket.close();
    }
    const newLogs = await getLogs(
      $lastChosenService,
      "",
      logLinesCount,
      offset
    );
    offset += newLogs.length;
    tmpLogs = allLogs;
    webSocket = new WebSocket(`${api.wsUrl}getLogsStream?id=${service}`); // maybe should move to fetch
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
</script>

<LogsViewHeder bind:searchText />
{#if allLogs.length === 0}
  <h2 class="noLogsMessage">No logs written yet</h2>
{/if}
{#if isUploading}
  <div class="lds-ellipsis">
    <div />
    <div />
    <div />
    <div />
  </div>
{/if}
<div
  id="logs"
  class="logs"
  bind:this={logsDiv}
  on:scroll={async (e) => {
    if (
      logsDiv.scrollTop === 0 &&
      logsDiv.scrollLeft === 0 &&
      !isLogsUpdating &&
      !isUploading
    ) {
      isLogsUpdating = true;
      oldScrollHeight = logsDiv.scrollHeight;
      tmpLogs = allLogs;
      if (searchText.length === 0) {
        const newLogs = await getLogs(
          $lastChosenService,
          searchText,
          logLinesCount,
          offset
        );
        offset += newLogs.length;
        setTimeout(() => {
          logsDiv.scrollTop = logsDiv.scrollHeight - oldScrollHeight;
          isLogsUpdating = false;
        });
        tmpLogs = allLogs;
      } else {
        const newLogs = await getLogsWithSearch(
          $lastChosenService,
          searchText,
          logLinesCount,
          searchOffset,
          !$store.caseInSensitive
        );

        setTimeout(() => {
          logsDiv.scrollTop = logsDiv.scrollHeight - oldScrollHeight;
          isLogsUpdating = false;
        });
      }
    }
  }}
>
  <div class="logsTableContainer">
    <table class="logsTable {$store.breakLines ? 'breakLines' : ''}">
      {#if searchText.length === 0}
        <!-- svelte-ignore empty-block -->
        {#await getLogsStream($lastChosenService) then}
          {#each tmpLogs as logItem}
            <LogsString
              bind:this={logString}
              time={$store.UTCtime
                ? logItem.at(0).slice(0, 19).replace("T", " ")
                : new Date(
                    new Date().setTime(
                      new Date(
                        logItem.at(0).slice(0, 19).replace("T", " ")
                      ).getTime() -
                        timezoneOffsetSec * 1000
                    )
                  ).toLocaleString("sv-SE")}
              message={logItem.at(1)}
              status={getLogLineStatus(logItem.at(1))}
            />
          {/each}
        {/await}
      {:else}
        <!-- svelte-ignore empty-block -->

        {#each tmpSearchLogs as logItem}
          <LogsString
            bind:this={logString}
            time={$store.UTCtime
              ? logItem.at(0).slice(0, 19).replace("T", " ")
              : new Date(
                  new Date().setTime(
                    new Date(
                      logItem.at(0).slice(0, 19).replace("T", " ")
                    ).getTime() -
                      timezoneOffsetSec * 1000
                  )
                ).toLocaleString("sv-SE")}
            message={logItem.at(1)}
            status={getLogLineStatus(logItem.at(1))}
          />
        {/each}
      {/if}
      <div id={"endOfLogs"} />
    </table>
    {#if buttonToBottomIsVisible}
      <ButtonToBottom ico={"Down"} />
    {/if}
  </div>
</div>
