<script>
  import LogsString from "../../lib/LogsString/LogsString.svelte";
  import fetchApi from "../../utils/fetch";
  import { afterUpdate, onMount } from "svelte";
  import LogsViewHeder from "./LogsViewHeder/LogsViewHeder.svelte";
  import {
    store,
    lastChosenHost,
    lastChosenService,
  } from "../../Stores/stores";
  import ButtonToBottom from "../../lib/ButtonToBottom/ButtonToBottom.svelte";

  let serviceName = "";

  lastChosenService.subscribe((v) => {
    serviceName = v;
  });

  let storeVal = {};

  store.subscribe((val) => (storeVal = val));

  let searchText = "";
  let offset = 0,
    logLinesCount = 30,
    oldScrollHeight = 0;
  let allLogs = [],
    tmpLogs = allLogs;
  let webSocket = undefined;
  let isLogsUpdating = false,
    isUploading = false;

  const api = new fetchApi();
  $: logString = undefined;
  $: logsDiv = undefined;

  afterUpdate(() => {
    scrollToBottom();
  });

  let logsContEl = null;
  let buttonToBottomIsVisible = false;

  onMount(() => {
    let isScrolling = false;
    logsContEl = document.querySelector("#logs");
    const defaultScrollDiff = logsContEl.scrollTop - logsContEl.scrollHeight;
    console.log(defaultScrollDiff);
    logsContEl.addEventListener("scroll", () => {
      if (!isScrolling) {
        isScrolling = true;
        setTimeout(() => {
          if (
            logsContEl.scrollTop &&
            logsContEl.scrollHeight - logsContEl.scrollTop !==
              logsContEl.clientHeight
          ) {
            console.log(
              logsContEl.scrollHeight,
              "test",
              logsContEl.scrollTop,
              logsContEl.scrollHeight - logsContEl.scrollTop
            );
            buttonToBottomIsVisible = true;
          } else {
            buttonToBottomIsVisible = false;
            console.log(logsContEl.scrollHeight, "test", logsContEl.scrollTop);
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
    const logLineItems = logLine.slice(30).split(" ");
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

  async function getLogs(
    service = "",
    search = "",
    limit = logLinesCount,
    offset = 0
  ) {
    isUploading = true;
    const newLogs = (
      await api.getLogs(service, search, limit, offset)
    ).reverse();
    offset += newLogs.length;
    isUploading = false;
    allLogs = [...newLogs, ...allLogs];
    return newLogs;
  }

  async function getLogsStream(service = "") {
    offset = 0;
    allLogs = [];
    tmpLogs = allLogs;
    if (service.localeCompare("") == 0) {
      return;
    }
    if (webSocket != undefined) {
      webSocket.close();
    }
    const newLogs = await getLogs(serviceName, "", logLinesCount, offset);
    offset += newLogs.length;
    tmpLogs = allLogs;
    webSocket = new WebSocket(`${api.wsUrl}getLogsStream?id=${service}`); // maybe should move to fetch
    webSocket.onmessage = (event) => {
      offset++;
      allLogs.push(event.data);
      tmpLogs = allLogs;
      scrollToBottom();
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
      logsDiv.scrollTop >= 0 &&
      logsDiv.scrollTop < 5 &&
      logsDiv.scrollLeft === 0 &&
      !isLogsUpdating &&
      !isUploading
    ) {
      isLogsUpdating = true;
      oldScrollHeight = logsDiv.scrollHeight;
      tmpLogs = allLogs;
      const newLogs = await getLogs(serviceName, searchText, logLinesCount, offset);
      offset += newLogs.length;
      setTimeout(() => {
        logsDiv.scrollTop = logsDiv.scrollHeight - oldScrollHeight;
        isLogsUpdating = false;
      });
      tmpLogs = allLogs;
    }
  }}
>
  <div class="logsTableContainer">
    <table class="logsTable {storeVal.breakLines ? 'breakLines' : ''}">
      {#if searchText.length === 0}
        <!-- svelte-ignore empty-block -->
        {#await getLogsStream(serviceName) then}
          {#each tmpLogs as logItem}
            <LogsString
              bind:this={logString}
              time={storeVal.UTCtime
                ? logItem.slice(0, 19).replace("T", " ")
                : new Date(
                    new Date().setTime(
                      new Date(
                        logItem.slice(0, 19).replace("T", " ")
                      ).getTime() -
                        timezoneOffsetSec * 1000
                    )
                  ).toLocaleString("sv-SE")}
              message={logItem.slice(30)}
              status={getLogLineStatus(logItem)}
            />
          {/each}
        {/await}
      {:else}
        <!-- svelte-ignore empty-block -->
        {#await getLogs(serviceName, searchText, logLinesCount, 0) then logs}
          {#each logs as logItem}
            <LogsString
              bind:this={logString}
              time={storeVal.UTCtime
                ? logItem.slice(0, 19).replace("T", " ")
                : new Date(
                    new Date().setTime(
                      new Date(
                        logItem.slice(0, 19).replace("T", " ")
                      ).getTime() -
                        timezoneOffsetSec * 1000
                    )
                  ).toLocaleString("sv-SE")}
              message={logItem.slice(30)}
              status={getLogLineStatus(logItem)}
            />
          {/each}
        {/await}
      {/if}
    </table>
    {#if buttonToBottomIsVisible}
      <ButtonToBottom ico={"Down"} />
    {/if}
  </div>
</div>
