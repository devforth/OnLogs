<script>
  import Chart from "./Chart.svelte";
  import "chart.js/auto";
  let headerOptions = ["Per hour", "Per day", "Per month", "Per year"];
  import {
    lastStatisticPeriod,
    lastChosenHost,
    lastChosenService,
  } from "../../Stores/stores";
  import { tryToParseLogString } from "../../utils/functions";
  import fetchApi from "../../utils/fetch";
  import { onMount } from "svelte";
  const api = new fetchApi();
  async function getChartData(unitsAmount) {
    const data = await api.getChartData({
      host: $lastChosenHost,
      service: $lastChosenService,
      unit: $lastStatisticPeriod.split(" ")[1],
      unitsAmount,
    });
  }
  onMount(() => {
    getChartData(10);
  });
</script>

<div class="chartContainer">
  <h2 class="chartTittle">Logs statistic:</h2>

  <div class="chartHeader">
    <ul class="flex spaceBetween ">
      {#each headerOptions as option}
        <li
          class="item flex {$lastStatisticPeriod === option ? 'isActive' : ''}"
          onclick={() => {
            lastStatisticPeriod.set(option);
          }}
        >
          {option}
        </li>{/each}
    </ul>
  </div>
  <Chart />
</div>
