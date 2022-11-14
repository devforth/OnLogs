<script>
    import "../../lib/LogsString/LogsString.svelte"
    import LogsString from "../../lib/LogsString/LogsString.svelte";
    import fetchApi from "../../utils/fetch";

    export let serviceName = "";
    let searchText = "";

    async function getLogs(service="", search="") {
        console.log(search)
        return await new fetchApi().getLogs(service, search)
    }
</script>

<div>
    <!-- TODO -->
    <h2 class="header">Service logs</h2>
    <p class="header">recent at bottom</p>
    <button class="header hto">
        <div class="icon">
        <i class={"log log-Eye"} />
      </div>
    </button>
    <div class="header search">
        <i class={"log log-Search"} />
        <input type="text" bind:value={searchText}>
    </div>
</div>
<div class="logs">
    {#await getLogs(serviceName, searchText)}
      <p>loading...</p>
    {:then logs}
        {#each logs as logItem}
        <!-- TODO  svelte scroll-->
        <LogsString time={logItem.slice(0,39)} message={logItem.slice(40)}></LogsString> 
        {/each}
    {:catch}
      <p>Error</p>
    {/await}
</div>