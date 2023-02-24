<script>
  import { onDestroy, onMount } from "svelte";

  import { lastLogTime, WSisMuted, manuallyUnmuted } from "../../Stores/stores";
  import { getTimeDifference } from "../../utils/functions";
  let lastLogsCheckerInterval = null;
  let componentLastLogTime = [] || "";

  $: componentLastLogTime = getTimeDifference($lastLogTime);

  let LAST_LOG_INTWRVAL = 5000;
  function checkLastLogs() {
    lastLogsCheckerInterval = setInterval(() => {
      componentLastLogTime = getTimeDifference($lastLogTime);
      console.log(componentLastLogTime);
    }, LAST_LOG_INTWRVAL);
  }

  onMount(() => {
    checkLastLogs();
  });
  onDestroy(() => {
    if (lastLogsCheckerInterval) {
      clearInterval(lastLogsCheckerInterval);
    }
  });
</script>

<div class="log-container">
  <i
    class="log log-Last {$WSisMuted ? 'WSisMuted' : ''}"
    title="Live mode"
    on:click={() => {
      WSisMuted.set(!$WSisMuted);
      manuallyUnmuted.set(!$manuallyUnmuted);
    }}
  />

  <h4 class="log-heading">Last log line:</h4>
  {#if Array.isArray(componentLastLogTime)}
    {#if componentLastLogTime?.at(0)[0] || componentLastLogTime?.at(1)[0]}
      <div class="streamInfoTimeWrapper">
        {#each componentLastLogTime as e}
          {#if e?.at(0)}
            <span class="streamInfoTime">{e?.at(0)}</span>

            <span class="streamInfoTimeShortName">{e?.at(1)}</span>{/if}
        {/each}
      </div>
    {/if}
    {#if componentLastLogTime?.at(0)[0] === 0 && componentLastLogTime?.at(1)[0] === 0}
      <span class="streamInfoTime">Recently</span>
    {/if}
    {#if componentLastLogTime?.at(0) === "" && componentLastLogTime?.at(1) === ""}
      <span class="streamInfoTime">No logs</span>
    {/if}
  {/if}
  <div class="log-time">
    <span class="log-time-ago" />
  </div>
</div>
