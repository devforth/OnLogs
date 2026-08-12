<script>
  import Chart from "./Chart.svelte";
  import {
    lastStatisticPeriod,
    lastChosenHost,
    lastChosenService,
  } from "../../Stores/stores";
  import fetchApi from "../../utils/fetch";
  import { PERIODS, totalLines } from "./chartData.js";

  const UNITS_BACK = 12;
  const api = new fetchApi();
  const full = new Intl.NumberFormat();

  let { host: routeHost = "", service: routeService = "" } = $props();

  let buckets = $state({});
  let unit = $derived($lastStatisticPeriod);
  let chartHost = $derived(routeHost || $lastChosenHost);
  let chartService = $derived(routeService || $lastChosenService);

  $effect(() => {
    const host = chartHost;
    const service = chartService;
    const currentUnit = unit;
    if (!host || !service) {
      return;
    }
    (async () => {
      const data = await api.getChartData({
        host,
        service,
        unit: currentUnit,
        unitsAmount: UNITS_BACK,
      });
      buckets = data && !data.error ? data : {};
    })();
  });

  let total = $derived(totalLines(buckets));
  let span = $derived(
    { hour: `${UNITS_BACK} hours`, day: `${UNITS_BACK} days`, month: `${UNITS_BACK} months` }[unit]
  );
</script>

<div class="chartContainer">
  <div class="chartHeader">
    <div class="chartHeading">
      <h2 class="chartTittle">{chartService}</h2>
      <p class="chartSubtitle">
        {full.format(total)} log lines in the last {span}
      </p>
    </div>

    <div class="timeSpan flex">
      {#each PERIODS as [value, label]}
        <div
          class={unit === value ? "active" : ""}
          onclick={() => lastStatisticPeriod.set(value)}
        >
          {label}
        </div>
      {/each}
    </div>
  </div>

  <Chart {buckets} {unit} />
</div>
