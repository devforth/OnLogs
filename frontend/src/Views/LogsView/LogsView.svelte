<script>
    import "../../lib/LogsString/LogsString.svelte";
    import LogsString from "../../lib/LogsString/LogsString.svelte";
    import fetchApi from "../../utils/fetch";
    import { afterUpdate } from "svelte";

    export let serviceName = "";
    let searchText = "";
    let api = new fetchApi();
    $: updElement = undefined;

    afterUpdate(() => {
        const el = document.getElementById("logs");
        console.log(el.scrollHeight);
        el.scroll({ top: el.scrollHeight });
    });

    async function getLogs(service = "", search = "", limit = 30, offset = 0) {
        return await api.getLogs(service, search, limit, offset);
    }
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
        <input type="text" bind:value={searchText} />
    </div>
</div>
<div id="logs" class="logs">
    {#await getLogs(serviceName, searchText)}
        <p>loading...</p>
    {:then logs}
        {#each logs as logItem}
            <LogsString
                bind:this={updElement}
                time={logItem.slice(0, 39)}
                message={logItem.slice(40)}
            />
        {/each}
    {:catch}
        <p>Error</p>
    {/await}
</div>
