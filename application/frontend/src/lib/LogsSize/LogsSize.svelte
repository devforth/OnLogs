<script>
  // @ts-nocheck

  import { onMount, afterUpdate } from "svelte";
  export let discribeText = "";
  import {
    lastChosenHost,
    lastChosenService,
    theme,
    confirmationObj,
  } from "../../Stores/stores";
  import FetchApi from "../../utils/fetch.js";
  import Button from "../Button/Button.svelte";
  const fetchApi = new FetchApi();
  export let isAllLogs = false;
  let logsSize = 0;
  let fetchCount = 0;
  let updateIntervalID = null;
  let UPDATE_INTERVAL = 30000;

  async function clearLogs() {
    confirmationObj.set({ ...$confirmationObj, isVisible: true });
  }

  async function fetchAllLogs() {
    const data = await fetchApi.getAllLogsSize();
    logsSize = data.sizeMiB;
  }
  async function fetchServiceLogs() {
    if ($lastChosenService) {
      const data = await fetchApi.getServiceLogsSize(
        $lastChosenHost,
        $lastChosenService
      );
      logsSize = data.sizeMiB;
    }
  }

  async function updateDataFromInterval(cb) {
    let alreadyStarted = false;
    if (updateIntervalID) {
      clearInterval(updateIntervalID);
    }
    if (!alreadyStarted) {
      await cb();
      alreadyStarted = true;
    }
    updateIntervalID = setInterval(async () => {
      await cb();
    }, UPDATE_INTERVAL);
  }

  $: {
    if (($lastChosenHost || $lastChosenService) && !isAllLogs) {
      updateDataFromInterval(fetchServiceLogs);
    }
  }
  $: {
    if (($lastChosenHost || $lastChosenService) && isAllLogs) {
      updateDataFromInterval(fetchAllLogs);
    }
  }
</script>

<div class="logSizeContainer">
  <i class="log log-Data" />
  {#if !isAllLogs}<div class="cleanButtonContainer">
      <Button
        icon={"log log-Clean"}
        minHeight={40}
        minWidth={40}
        border={true}
        highlighted={$theme === "dark" ? true : false}
        CB={clearLogs}
      />
    </div>
  {/if}
  <h3 class="title">{logsSize} <span>MiB</span></h3>
  <p class="commonText">{discribeText}</p>
</div>
