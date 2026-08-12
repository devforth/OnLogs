<script>
  import "chart.js/auto";
  import { Bar } from "svelte-chartjs";
  import { theme } from "../../Stores/stores";
  import { toChartData, totalLines } from "./chartData.js";

  let { buckets = {}, unit = "hour" } = $props();

  let dark = $derived($theme === "dark");
  let tick = $derived(dark ? "#8d96a3" : "#6b6883");
  let text = $derived(dark ? "#cbd2d9" : "#1b1850");
  let grid = $derived(dark ? "rgba(203, 210, 217, 0.12)" : "rgba(27, 24, 80, 0.08)");

  const compact = new Intl.NumberFormat([], { notation: "compact" });
  const full = new Intl.NumberFormat();

  let total = $derived(totalLines(buckets));

  let data = $derived({
    ...toChartData(buckets, unit),
    datasets: toChartData(buckets, unit).datasets.map((d) => ({
      ...d,
      borderWidth: 0,
      borderRadius: 3,
      borderSkipped: false,
      maxBarThickness: 44,
    })),
  });

  let options = $derived({
    maintainAspectRatio: false,
    responsive: true,
    interaction: { mode: "index", intersect: false },
    plugins: {
      legend: {
        position: "bottom",
        labels: {
          usePointStyle: true,
          pointStyle: "circle",
          boxWidth: 8,
          boxHeight: 8,
          padding: 18,
          color: tick,
          font: { size: 12 },
        },
      },
      tooltip: {
        backgroundColor: dark ? "#1f2933" : "#ffffff",
        titleColor: text,
        bodyColor: text,
        footerColor: tick,
        borderColor: grid,
        borderWidth: 1,
        padding: 12,
        cornerRadius: 8,
        boxPadding: 6,
        usePointStyle: true,
        callbacks: {
          footer: (items) =>
            "Total  " + full.format(items.reduce((n, i) => n + (i.parsed.y ?? 0), 0)),
        },
      },
    },
    scales: {
      x: {
        stacked: true,
        grid: { display: false },
        border: { display: false },
        ticks: { color: tick, font: { size: 11 } },
      },
      y: {
        stacked: true,
        beginAtZero: true,
        border: { display: false },
        grid: { color: grid, drawTicks: false },
        ticks: {
          color: tick,
          font: { size: 11 },
          padding: 10,
          maxTicksLimit: 5,
          callback: (value) => compact.format(value),
        },
      },
    },
  });
</script>

<div class="chartCanvas">
  {#if total === 0}
    <p class="chartEmpty">No statistics recorded for this period yet</p>
  {:else}
    <Bar {data} {options} />
  {/if}
</div>
