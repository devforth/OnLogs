<script>
    import "../../lib/LogsString/LogsString.svelte"
    import LogsString from "../../lib/LogsString/LogsString.svelte";

    export let logsData = [];
</script>

<div>
    <h2 class="header">Service logs</h2>
    <p class="header">recent at bottom</p>
    <button class="header hto">
        <div class="icon">
        <i class={"log log-Eye"} />
      </div>
    </button>
    <div class="header search">
        <i class={"log log-Search"} />
        <input type="text">
    </div>
</div>
<div class="logs">
    {#each logsData as logItem}
        <LogsString message={logItem} time="TIME"></LogsString>
    {/each}
</div>