<script>
    import "../../lib/LogsString/LogsString.svelte";
    import LogsString from "../../lib/LogsString/LogsString.svelte";
    import fetchApi from "../../utils/fetch";
    import { afterUpdate } from "svelte";

    export let serviceName;

    let searchText = "";
    let offset = 0, logLinesCount = 30, oldScrollHeight = 0;
    let allLogs = [], tmpLogs = allLogs;
    let webSocket = undefined;
    let isLogsUpdating = false;

    const api = new fetchApi();
    $: logString = undefined;
    $: logsDiv = undefined;

    afterUpdate(() => {
        scrollToBottom();
    });

    function scrollToBottom() {
        const logsCont = document.querySelector('#logs');
        const SCROLL_FINAL_GAP_PX = 20;
        const userScrolledToSpecificLoc = logsCont.scrollHeight - logsCont.scrollTop - logsCont.clientHeight > SCROLL_FINAL_GAP_PX;
        if (!userScrolledToSpecificLoc) {
            setTimeout(() => {
                logsCont.scrollTop = logsCont.scrollHeight - logsCont.clientHeight;
                oldScrollHeight = logsCont.scrollHeight;
            })
        }
    }

    async function getLogs(service="", search="", limit=logLinesCount, offset=0) {
        const newLogs = (await api.getLogs(service, search, limit, offset)).reverse()
        allLogs = [...newLogs, ...allLogs];
        return newLogs;
    }

    async function getLogsStream(service="") {
        offset = 0;
        allLogs = [];
        tmpLogs = allLogs;
        if (service.localeCompare("") == 0) {
            return
        }
        if (webSocket != undefined) {
            webSocket.close()
        }

        await getLogs(service, "", logLinesCount, offset)
        tmpLogs = allLogs;
        webSocket = new WebSocket("ws://localhost:2874/api/v1/getLogsStream?id="+service);
        webSocket.onmessage = (event) => {
            offset++;
            allLogs.push(event.data);
            tmpLogs = allLogs;
            scrollToBottom();
        }
    }
</script>

<div id="top-line">
    <h2 class="header">Service logs</h2>
    <p class="header">recent at bottom</p>
    <button class="header show">
        <div class="icon">
            <i class={"log log-Eye"} />
        </div>
    </button>
    <div class="header search">
        <i class={"log log-Search"} />
        <input type="text" bind:value={searchText} />
    </div>
</div>
<div
    id="logs"
    class="logs"
    bind:this={logsDiv}
    on:scroll={async () => {
        const scrolledPercent = logsDiv.scrollTop/logsDiv.scrollHeight*100
        if (logsDiv.scrollTop > 1 && scrolledPercent < 0.5 && !isLogsUpdating) {
            isLogsUpdating = true;
            offset += logLinesCount;
            oldScrollHeight = logsDiv.scrollHeight;
            tmpLogs=allLogs;
            await getLogs(serviceName, "", logLinesCount, offset);
            setTimeout(() => {
                logsDiv.scrollTop = (logsDiv.scrollHeight - oldScrollHeight); //- logsDiv.scrollTop;
                isLogsUpdating = false;
            })
            tmpLogs = allLogs;
        }
    }}
    >
    {#if searchText.length == 0}
        {#await getLogsStream(serviceName)}
            <p>loading...</p>
        {:then}
            {#each tmpLogs as logItem}
                <LogsString
                    bind:this={logString}
                    time={logItem.slice(0, 19).replace("T", " ")}
                    message={logItem.slice(30)}
                />
            {/each}
        {/await}
    {:else}
        {#await getLogs(serviceName, searchText, logLinesCount, 0)}
            <p>loading...</p>
        {:then logs}
            {#each logs as logItem}
                <LogsString
                    bind:this={logString}
                    time={logItem.slice(0, 19).replace("T", " ")}
                    message={logItem.slice(30)}
                />
            {/each}
        {/await}
    {/if}
</div>
