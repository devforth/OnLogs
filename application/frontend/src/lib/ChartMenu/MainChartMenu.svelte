<script>
  import Chart from "./Chart.svelte";
  import "chart.js/auto";
  let headerOptions = ["Per hour", "Per day", "Per month", "Per year"];
  import { lastStatisticPeriod } from "../../Stores/stores";
  import { tryToParseLogString } from "../../utils/functions";
  function getChartData() {}
</script>

<div class="chartContainer">
  <h2 class="chartTittle">Logs statistic:</h2>

  <div class="chartHeader">
    <ul class="flex spaceBetween ">
      {#each headerOptions as option}
        <li
          class="item flex {$lastStatisticPeriod === option ? 'isActive' : ''}"
          on:click={() => {
            lastStatisticPeriod.set(option);
          }}
        >
          {option}
        </li>{/each}
    </ul>
  </div>
  <Chart />
</div>
