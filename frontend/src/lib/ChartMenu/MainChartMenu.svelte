<script>
  import Chart from "./Chart.svelte";
  import "chart.js/auto";
  let headerOptions = ["Per hour", "Per day", "Per month", "Per year"];
  import { lastStatisticPeriod } from "../../Stores/stores";
  import { tryToParseLogString } from "../../utils/functions";
  const testStr =
    'cscdsrgrbrbtrbtr 3#()sdfs]]{"t":{"$date":"2023-01-27T16:49:42.571+00:00"},"s":"I",  "c":"COMMAND",  "id":51803,   "ctx":"conn41628","msg":"Slow query","attr":{"type":"command","ns":"walkingbet.promo_code","command":{"aggregate":"promo_code","pipeline":[{"$lookup":{"from":"user","localField":"affiliate_id","foreignField":"_id","as":"affiliate_user"}},{"$addFields":{"user":{"$arrayElemAt":["$affiliate_user",0]}}},{"$match":{"$and":[{"type":"AFFILIATE_CODE"},{"activate_count":{"$gt":0}}]}},{"$lookup":{"from":"activate_promo_code","localField":"_id","foreignField":"promo_code_id","pipeline":[{"$match":{"created_at":{"$gte":{"$date":"2023-01-20T16:49:38.690Z"}}}},{"$count":"count"}],"as":"activated_count_arr"}},{"$addFields":{"activated_count":{"$getField":{"input":{"$arrayElemAt":["$activated_count_arr",0]},"field":"count"}}}},{"$sort":{"activated_count":-1}},{"$limit":10},{"$lookup":{"from":"transaction","let":{"user_id":"$affiliate_id"},"pipeline":[{"$match":{"$expr":{"$and":[{"$gte":["$created_at",{"$date":"2023-01-20T16:49:38.690Z"}]},{"$eq":["$user_id","$$user_id"]},{"$eq":["$type","AFFILIATE_CODE_BONUS"]}]}}},{"$group":{"_id":null,"amount":{"$sum":"$amount"}}}],"as":"affiliate_bonus"}},{"$lookup":{"from":"activate_promo_code","localField":"_id","foreignField":"promo_code_id","pipeline":[{"$match":{"created_at":{"$gte":{"$date":"2023-01-20T16:49:38.690Z"}}}},{"$lookup":{"from":"transaction","localField":"activator_bonus_tx_id","foreignField":"_id","pipeline":[{"$group":{"_id":null,"amount":{"$sum":"$amount"}}}],"as":"transaction"}},{"$addFields":{"amount_to_user":{"$getField":{"input":{"$arrayElemAt":["$transaction",0]},"field":"amount"}}}},{"$group":{"_id":null,"amount":{"$sum":{"$sum":"$transaction.amount"}}}},{"$project":{"transaction":0}}],"as":"activator_tx_bonus"}},{"$addFields":{"affiliate_bonus_amount":{"$getField":{"input":{"$arrayElemAt":["$affiliate_bonus",0]},"field":"amount"}}}},{"$addFields":{"activator_bonus_amount":{"$getField":{"input":{"$arrayElemAt":["$activator_tx_bonus",0]},"field":"amount"}}}},{"$project":{"activator_tx_bonus":0,"affiliate_bonus":0,"affiliate_user":0,"activated_count_arr":0}}],"cursor":{},"lsid":{"id":{"$uuid":"fae0b111-b94a-4959-88be-cf134b05f022"}},"$db":"walkingbet"},"planSummary":"IXSCAN { activate_count: 1 }","keysExamined":2954,"docsExamined":5935256,"hasSortStage":true,"cursorExhausted":true,"numYields":5236,"nreturned":10,"queryHash":"4A75D23C","planCacheKey":"1556829C","reslen":12392,"locks":{"Global":{"acquireCount":{"r":9574}},"Mutex":{"acquireCount":{"r":4338}}},"storage":{},"remote":"172.18.0.1:54058","protocol":"op_msg","durationMillis":3821}} fsfsgre][][][]];;];]l]l]lwelf]lwe';
</script>

<div class="chartContainer">
  <h2 class="chartTittle">Logs statistic:</h2>
  <pre style="">{@html tryToParseLogString(testStr)}</pre>
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
