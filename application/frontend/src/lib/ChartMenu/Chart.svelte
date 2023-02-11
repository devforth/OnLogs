<script>
  import { Bar } from "svelte-chartjs";
  import {
    Chart as ChartJS,
    Title,
    Tooltip,
    Legend,
    ArcElement,
    CategoryScale,
  } from "chart.js";
  import fetchApi from "../../utils/fetch";
  import {
    lastChosenHost,
    lastChosenService,
    theme,
  } from "../../Stores/stores";
  import { emulateData } from "../../utils/functions";
  import { onMount } from "svelte";

  ChartJS.register(Title, Tooltip, Legend, ArcElement, CategoryScale);

  const api = new fetchApi();
  let colors = {
    error: "rgba(153, 44,44,1)",
    debug: "rgba(47,  145, 45,1)",
    info: "rgba(62, 132, 160,1)",
    warn: "rgba(172, 104,26,1)",
    other: "rgba(128,128,128, 1)",
  };
  let borderColors = {
    error: "rgba(153, 44,44,1)",
    debug: "rgba(47,  145, 45,1)",
    info: "rgba(62, 132, 160,1)",
    warn: "rgba(172, 104,26,1)",
    other: "rgba(128,128,128, 1)",
  };

  let options = {
    maintainAspectRatio: false,
    scales: {
      x: {
        grid: {
          display: false,
        },
      },
      y: {
        grid: {
          display: false,
        },
      },
    },
  };

  let names = ["Error", "Debug", "Info", "Warn", "Other"];
  let data = { ...getStatisticData(), options: { ...options } };

  function getStatisticData(unit, unitsAmount) {
    // const fetchedData = await api.getChartData({
    //   host: $lastChosenHost,
    //   service: $lastChosenService,

    //   unit: unit,
    //   unitsAmount,
    // });
    const fetchedData = emulateData(10);

    if (fetchedData) {
      return {
        labels: fetchedData.dates,
        datasets: names.map((e, i) => {
          return {
            label: names[i],
            data: fetchedData[`${e.charAt(0).toLowerCase()}${e.slice(1)}`],
            stack: "Stack 0",
            backgroundColor: fetchedData.dates.map((e, j) => {
              return Object.values(colors)[i];
            }),
            borderWidth: 1,
          };
        }),
      };
    }
  }
  // let data = {
  //   labels: ["1", "2", "3", "4", "5", "6", "7"],
  //   datasets: [
  //     {
  //       label: "Error",
  //       data: [5, 19, 3, 5],
  //       stack: "Stack 0",
  //       backgroundColor: [
  //         "rgba(153, 44,44,0.4)",
  //         "rgba(153, 44,44,0.4)",
  //         "rgba(153, 44,44,0.4)",
  //         "rgba(153, 44,44,0.4)",
  //       ],
  //       borderWidth: 1,
  //       borderColor: [
  //         "rgba(153, 44,44, 1)",
  //         "rgba(153, 44,44, 1)",
  //         "rgba(153, 44,44, 1)",
  //         "rgba(153, 44,44, 1)",
  //       ],
  //     },
  //     {
  //       label: "Debug",
  //       data: [7, 19, 3, 5],
  //       stack: "Stack 0",
  //       backgroundColor: [
  //         "rgba(47,  145, 45,0.4)",
  //         "rgba(47,  145, 45,0.4)",
  //         "rgba(47,  145, 45,0.4)",
  //         "rgba(47,  145, 45,0.4)",
  //       ],
  //       borderWidth: 1,
  //       borderColor: [
  //         "rgba(47,  145, 45,1)",
  //         "rgba(47,  145, 45,1)",
  //         "rgba(47,  145, 45,1)",
  //         "rgba(47,  145, 45,1)",
  //       ],
  //     },
  //     {
  //       label: "Info",
  //       data: [12, 19, 3, 5],
  //       stack: "Stack 0",
  //       backgroundColor: [
  //         "rgba(62, 132, 160,0.4)",
  //         "rgba(62, 132, 160,0.4)",
  //         "rgba(62, 132, 160,0.4)",
  //         "rgba(62, 132, 160,0.4)",
  //       ],
  //       borderWidth: 1,
  //       borderColor: [
  //         "rgba(62, 132, 160,1)",
  //         "rgba(62, 132, 160,1)",
  //         "rgba(62, 132, 160,1)",
  //         "rgba(62, 132, 160,1)",
  //       ],
  //     },
  //     {
  //       label: "Warn",
  //       data: [1, 19, 3, 5],
  //       stack: "Stack 0",
  //       backgroundColor: [
  //         "rgba(172, 104,26,0.4)",
  //         "rgba(172, 104,26,0.4)",
  //         "rgba(172, 104,26,0.4)",
  //         "rgba(172, 104,26,0.4)",
  //       ],
  //       borderWidth: 1,
  //       borderColor: [
  //         "rgba(172, 104,26, 1)",
  //         "rgba(172, 104,26, 1)",
  //         "rgba(172, 104,26, 1)",
  //         "rgba(172, 104,26, 1)",
  //       ],
  //     },
  //     {
  //       label: "All",
  //       data: [1, 19, 3, 5],
  //       stack: "Stack 0",
  //       backgroundColor: [
  //         "	rgba(128,128,128, 0.4)",
  //         "	rgba(128,128,128, 0.4)",
  //         "	rgba(128,128,128, 0.4)",
  //         "	rgba(128,128,128, 0.4)",
  //       ],
  //       borderWidth: 1,
  //       borderColor: [
  //         "rgba(128,128,128, 1)",
  //         "rgba(128,128,128, 1)",
  //         "rgba(128,128,128, 1)",
  //         "rgba(128,128,128, 1)",
  //       ],
  //     },
  //   ],
  //   options: {
  //     scales: {
  //       x: {
  //         stacked: true,
  //         grid: { color: "red" },
  //       },
  //       y: {
  //         stacked: true,
  //       },
  //     },
  //   },
  // };
</script>

<div style="height: 50vh; display:flex; justify-content:center">
  {#if data} <Bar {data} options={data.options} />{/if}
</div>
