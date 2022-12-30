<script>
  export let discribeText = "";
  import {
    lastChosenHost,
    lastChosenService,
    theme,
  } from "../../Stores/stores";
  import FetchApi from "../../utils/fetch.js";
  import Button from "../Button/Button.svelte";
  const fetchApi = new FetchApi();
  export let isAllLogs = false;
  let logsSize = 0;
  $: {
    if (isAllLogs) {
      console.log("all");
      (async () => {
        const data = await fetchApi.getAllLogsSize();
        logsSize = data.sizeMiB;
      })();
    } else {
      console.log("service");
      (async () => {
        const data = await fetchApi.getServiceLogsSize(
          $lastChosenHost,
          $lastChosenService
        );
        logsSize = data.sizeMiB;
      })();
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
      />
    </div>
  {/if}
  <h3 class="title">{logsSize} MiB</h3>
  <p class="commonText">{discribeText}</p>
</div>
