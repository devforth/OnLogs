<script>
  // @ts-nocheck

  import {
    lastChosenHost,
    lastChosenService,
    theme,
    confirmationObj,
    logsCleared,
  } from "../../Stores/stores";
  import FetchApi from "../../utils/fetch.js";
  import Button from "../Button/Button.svelte";
  const fetchApi = new FetchApi();
  let { discribeText = "", isAllLogs = false } = $props();
  let logsSize = $state(0);
  let fetchCount = 0;
  let UPDATE_INTERVAL = 30000;

  async function clearLogs() {
    // Explicitly null: spreading the previous state would inherit whichever
    // action was set last, e.g. "delete this service".
    confirmationObj.set({ ...$confirmationObj, action: null, isVisible: true });
  }

  async function fetchAllLogs() {
    const data = await fetchApi.getAllLogsSize();
    logsSize = data?.sizeMiB ?? logsSize;
  }
  async function fetchServiceLogs() {
    if ($lastChosenService) {
      const data = await fetchApi.getServiceLogsSize(
        $lastChosenHost,
        $lastChosenService
      );
      logsSize = data?.sizeMiB ?? logsSize;
    }
  }

  // The poller starts synchronously and the teardown stops it: started after
  // an awaited fetch, a switch or logout meanwhile left one running for good.
  $effect(() => {
    $logsCleared;
    if (!($lastChosenHost || $lastChosenService)) {
      return;
    }
    const cb = isAllLogs ? fetchAllLogs : fetchServiceLogs;
    cb();
    const id = setInterval(cb, UPDATE_INTERVAL);
    return () => clearInterval(id);
  });
</script>

<div class="logSizeContainer">
  <i class="log log-Data"></i>
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
