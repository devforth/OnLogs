<script>
    import { onDestroy, onMount } from "svelte";
  import {
    lastChosenHost,
    lastChosenService,
    lastStatsPeriod,
    chosenStatus
  } from "../../Stores/stores.js";
  import fetchApi from "../../utils/fetch";

  let data = $state({});
  const api = new fetchApi();
  let intervalId;

  function setPeriod(p) {
    lastStatsPeriod.set(p);
  }

  // Each poll re-sums the whole period on the server, so it is gentle: every
  // 5 s, never while the tab is hidden or a request is still out.
  const POLL_INTERVAL = 5000;
  let latest = 0;
  let inFlight = false;

  async function updateData() {
    if (!$lastChosenHost || !$lastChosenService) {
      return;
    }
    const request = ++latest;
    inFlight = true;
    try {
      const result = await api.getStats({
        period: $lastStatsPeriod,
        service: $lastChosenService,
        host: $lastChosenHost,
      });
      // An answer for a previous service or period must not overwrite this one.
      if (request === latest) {
        data = result;
      }
    } catch {
      // The master is unreachable: keep the last numbers.
    } finally {
      if (request === latest) {
        inFlight = false;
      }
    }
  }

  onMount(() => {
    intervalId = setInterval(() => {
      if (!document.hidden && !inFlight) {
        updateData();
      }
    }, POLL_INTERVAL);
  });

  onDestroy(() => {
    clearInterval(intervalId);
  });

  $effect(() => {
    updateData();
  });
</script>

<div class="statsContainer">
  <div class="flex spaceBetween ">
    <i
      class="log log-Chart "
      onclick={() => {
        // navigate(
        //   `${changeKey}/stats/${$lastChosenHost}/${$lastChosenService}`,
        //   {
        //     replace: true,
        //   }
        // );
      }}
      title="Counter updates every 1 min since OnLogs started. So, it may cause some asynchrony."
></i>
    <div class=" timeSpan flex spaceBetween">
      <div
        class={$lastStatsPeriod === 2 ? "active" : ""}
        onclick={() => {
          setPeriod(2);
        }}
      >
        1hr
      </div>
      <div
        class={$lastStatsPeriod === 48 ? "active" : ""}
        onclick={() => {
          setPeriod(48);
        }}
      >
        1d
      </div>
      <div
        class={$lastStatsPeriod === 336 ? "active" : ""}
        onclick={() => {
          setPeriod(336);
        }}
      >
        1w
      </div>
      <div
        class={$lastStatsPeriod === 1344 ? "active" : ""}
        onclick={() => {
          setPeriod(1344);
        }}
      >
        1m
      </div>
    </div>
  </div>
  <h4 class="statsTittle">
    Top {Object.keys(data).length ? Object.keys(data).length : ""} levels:
  </h4>
  <ul>
    {#each Object.entries(data).sort((a, b) => {
      if (a[1] > b[1]) {
        return -1;
      }
      if (a[1] < b[1]) {
        return 1;
      }
    }) as [key, name]}
      <li class="flex spaceBetween statsItem"
        class:active={$chosenStatus === key}
        onclick={async () => {
            if ($chosenStatus !== key) {
            chosenStatus.set(key);
            } else {
            chosenStatus.set("");
            }
        }}>
        <p class={key}>{key.charAt(0).toUpperCase() + key.slice(1)}</p>
        <p>{name}</p>
      </li>
    {/each}
  </ul>
</div>
