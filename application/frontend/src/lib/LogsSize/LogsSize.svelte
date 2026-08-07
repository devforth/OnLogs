<script>
  // @ts-nocheck

  import { onMount, onDestroy, } from "svelte";
  import {
    lastChosenHost,
    lastChosenService,
    theme,
    confirmationObj,
  } from "../../Stores/stores";
  import FetchApi from "../../utils/fetch.js";
  import Button from "../Button/Button.svelte";
  const fetchApi = new FetchApi();
  /**
   * @typedef {Object} Props
   * @property {string} [discribeText]
   * @property {boolean} [isAllLogs]
   */

  /** @type {Props} */
  let { discribeText = "", isAllLogs = false } = $props();
  let logsSize = $state(0);
  let fetchCount = 0;
  let updateIntervalID = null;
  let UPDATE_INTERVAL = 30000;

  async function clearLogs() {
    // Explicitly null: spreading the previous state would inherit whichever
    // action was set last, e.g. "delete this service".
    confirmationObj.set({ ...$confirmationObj, action: null, isVisible: true });
  }

  onDestroy(() => {
    clearInterval(updateIntervalID);
  });

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

  $effect(() => {
    if (($lastChosenHost || $lastChosenService) && !isAllLogs) {
      updateDataFromInterval(fetchServiceLogs);
    }
  });
  $effect(() => {
    if (($lastChosenHost || $lastChosenService) && isAllLogs) {
      updateDataFromInterval(fetchAllLogs);
    }
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
