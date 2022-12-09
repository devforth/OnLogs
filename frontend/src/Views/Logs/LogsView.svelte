<script>
  import LogsString from "../../lib/LogsString/LogsString.svelte";
  import fetchApi from "../../utils/fetch";
  import { afterUpdate } from "svelte";
  import LogsViewHeder from "./LogsViewHeder/LogsViewHeder.svelte";
  import { store } from "../../Stores/stores";

  export let serviceName;

  let storeVal = {};

  store.subscribe((val) => (storeVal = val));
  console.log(storeVal);

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

  const timezoneOffsetSec = new Date().getTimezoneOffset() * 60;
  console.log(timezoneOffsetSec);

  function getLogLineStatus(logLine = "") {
    const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
    const statuses_warnings = ["WARN", "WARNING"];
    const statuses_other = ["DEBUG", "INFO"];
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
    webSocket = new WebSocket(
      "ws://localhost:2874/api/v1/getLogsStream?id=" + service
    );
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
  on:scroll={async () => {
    if (
      logsDiv.scrollTop >= 0 &&
      logsDiv.scrollTop < 5 &&
      !isLogsUpdating &&
      !isUploading
    ) {
      isLogsUpdating = true;
      oldScrollHeight = logsDiv.scrollHeight;
      tmpLogs = allLogs;
      const newLogs = await getLogs(serviceName, "", logLinesCount, offset);
      offset += newLogs.length;
      setTimeout(() => {
        logsDiv.scrollTop = logsDiv.scrollHeight - oldScrollHeight;
        isLogsUpdating = false;
      });
      tmpLogs = allLogs;
    }
  }}
>
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
                  new Date(logItem.slice(0, 19).replace("T", " ")).getTime()
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
              : new Date(logItem.slice(0, 19).replace("T", " ")).getSeconds() +
                timezoneOffsetSec}
            message={logItem.slice(30)}
            status={getLogLineStatus(logItem)}
          />
        {/each}
      {/await}
    {/if}
  </table>
</div>
