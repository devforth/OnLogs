<script>
    import { onDestroy, onMount } from "svelte";
  import {
    lastChosenHost,
    lastChosenService,
    lastStatsPeriod,
    chosenStatus
  } from "../../Stores/stores.js";
  import fetchApi from "../../utils/fetch";

  let data = {};
  const api = new fetchApi();
  let intervalId;

  function setPeriod(p) {
    lastStatsPeriod.set(p);
  }

  async function updateData() {
    if ($lastChosenHost && $lastChosenService) {
      data = await api.getStats({
        period: $lastStatsPeriod,
        service: $lastChosenService,
        host: $lastChosenHost,
      });
    }
  }

  onMount(() => {
    updateData();
    intervalId = setInterval(updateData, 1000);
  });

  onDestroy(() => {
    clearInterval(intervalId);
  });

  $: {
    (async () => {
      if ($lastChosenHost && $lastChosenService) {
        data = await api.getStats({
          period: $lastStatsPeriod,
          service: $lastChosenService,
          host: $lastChosenHost,
        });
      }
    })();
  }
</script>

<div class="statsContainer">
  <div class="flex spaceBetween ">
    <i
      class="log log-Chart "
      on:click={() => {
        // navigate(
        //   `${changeKey}/stats/${$lastChosenHost}/${$lastChosenService}`,
        //   {
        //     replace: true,
        //   }
        // );
      }}
      title="Counter updates every 1 min since OnLogs started. So, it may cause some asynchrony."
    />
    <div class=" timeSpan flex spaceBetween">
      <div
        class={$lastStatsPeriod === 2 ? "active" : ""}
        on:click={() => {
          setPeriod(2);
        }}
      >
        1hr
      </div>
      <div
        class={$lastStatsPeriod === 48 ? "active" : ""}
        on:click={() => {
          setPeriod(48);
        }}
      >
        1d
      </div>
      <div
        class={$lastStatsPeriod === 336 ? "active" : ""}
        on:click={() => {
          setPeriod(336);
        }}
      >
        1w
      </div>
      <div
        class={$lastStatsPeriod === 1344 ? "active" : ""}
        on:click={() => {
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
        on:click={async () => {
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
