<script>
    import "../../lib/LogsString/LogsString.svelte";
    import LogsString from "../../lib/LogsString/LogsString.svelte";
    import fetchApi from "../../utils/fetch";
    import { afterUpdate } from "svelte";

    export let serviceName = "";

    let searchText = "";
    let lineWidth = "0";
    const api = new fetchApi();
    $: updElement = undefined;


    afterUpdate(() => {
        const el = document.getElementById("logs");
        el.scroll({ top: el.scrollHeight });
        lineWidth = el.scrollWidth.toString();
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
                time={logItem.split(".", 2)[0]}
                message={logItem.split("UTC ", 2)[1]}
                width={lineWidth}
            />
        {/each}
    {:catch}
        <p>Error</p>
    {/await}
</div>
