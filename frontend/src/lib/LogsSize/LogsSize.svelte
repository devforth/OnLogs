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

  async function clearLogs() {
    confirmationObj.set({
      action: async function () {
        const data = await fetchApi.cleanLogs(
          $lastChosenHost,
          $lastChosenService
        );
        if (data) {
          confirmationObj.update((pv) => {
            return { ...pv, isVisible: false };
          });
        }
        console.log("data", data);
        console.log("helo.world");
      },
      message:
        "You want to delete host service logs. This data will be lost. This action cannot be undone.",

      isVisible: true,
    });
  }

  async function fetchAllLogs() {
    const data = await fetchApi.getAllLogsSize();
    logsSize = data.sizeMiB;
  }
  async function fetchServiceLogs() {
    const data = await fetchApi.getServiceLogsSize(
      $lastChosenHost,
      $lastChosenService
    );
    logsSize = data.sizeMiB;
  }

  $: {
    if ($lastChosenService && !isAllLogs) {
      fetchServiceLogs();
    }
  }
  $: {
    if ($lastChosenHost && isAllLogs) {
      fetchAllLogs();
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
  <h3 class="title">{logsSize} MiB</h3>
  <p class="commonText">{discribeText}</p>
</div>
