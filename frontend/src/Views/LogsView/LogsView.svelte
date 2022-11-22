<script>
    import "../../lib/LogsString/LogsString.svelte";
    import LogsString from "../../lib/LogsString/LogsString.svelte";
    import fetchApi from "../../utils/fetch";
    import { afterUpdate } from "svelte";

    export let serviceName = "";

    let searchText = "", tmpName = "";
    let offset = 0, logLinesCount = 30;
    let logs = [], tmpLogs = logs;
    let webSocket = undefined;
    let isLogsUpdating = false

    const api = new fetchApi();
    $: logString = undefined; // not sure if this correct
    $: logsDiv = undefined; // not sure if this correct

    afterUpdate(() => {
        if (tmpName.localeCompare(serviceName) != 0) {
            scrollToBottom();
        }
    });

    function scrollToBottom() {
        const logsCont = document.querySelector('#logs');
        const SCROLL_FINAL_GAP_PX = 20;
        const userScrolledToSpecificLoc = logsCont.scrollHeight - logsCont.scrollTop - logsCont.clientHeight > SCROLL_FINAL_GAP_PX;
        if (!userScrolledToSpecificLoc) {
            setTimeout(() => {
                logsCont.scrollTop = logsCont.scrollHeight - logsCont.clientHeight ;
            })
        }
    }

    async function getLogs(service="", search="", limit=logLinesCount, offset=0) {
        logs = [...(await api.getLogs(service, search, limit, offset)).reverse(), ...logs];
        isLogsUpdating = false;
    }

    async function getLogsStream(service="") {
        offset = 0;
        logs = [];
        tmpLogs = logs;
        if (service.localeCompare("") == 0) {
            return
        }
        if (webSocket != undefined) {
            webSocket.close()
        }

        await getLogs(service, "", logLinesCount, offset)
        tmpLogs = logs;
        webSocket = new WebSocket("ws://localhost:2874/api/v1/getLogsStream?id="+service);
        webSocket.onmessage = (event) => {
            tmpLogs = logs;
            offset++;
            logs.push(event.data);
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
    on:scroll={() => {
        if (logsDiv.scrollTop < 5 && !isLogsUpdating) {
            // logs.reverse() TODO - write smth when loading prev logs
            // logs.push("loading. Z  ")
            // logs.reverse()
            tmpLogs = logs
            isLogsUpdating = true;
            offset += logLinesCount;
            getLogs(serviceName, "", logLinesCount, offset);
            // tmpLogs = logs;
        }
    }}
    >
    {#await getLogsStream(serviceName)}
        <p>loading...</p>
    {:then}
        {#each tmpLogs as logItem}
            <LogsString
                bind:this={logString}
                time={logItem.split(".", 2)[0].replace("T", " ")}
                message={logItem.split("Z ", 2)[1] || logItem.split("+", 2)[1].slice(9)}
            />
        {/each}
    {/await}
</div>
