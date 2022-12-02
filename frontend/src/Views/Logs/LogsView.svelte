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
    let isLogsUpdating = false, isUploading = false;

    const api = new fetchApi();
    $: logString = undefined;
    $: logsDiv = undefined;

    afterUpdate(() => {
        scrollToBottom();
    });

    function getLogLineStatus(logLine="") {
        const statuses = ["error", "debug", "warn", "info"]
        const status = logLine.slice(30).split(" ", 1)[0]
        if (statuses.includes(status.toLowerCase())) {
            return status
        }
        return "debug"
    }

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
        isUploading = true;
        const newLogs = (await api.getLogs(service, search, limit, offset)).reverse()
        offset += newLogs.length;
        isUploading = false;
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
        const newLogs = await getLogs(serviceName, "", logLinesCount, offset);
        offset += newLogs.length;
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
    <!-- <button class="header show">
        <div class="icon">
            <i class={"log log-Eye"} />
        </div>
    </button> -->
    <div class="header search">
        <i class={"log log-Search"} />
        <input type="text" bind:value={searchText} />
    </div>
</div>
{#if allLogs.length == 0}
    <h2 class="noLogsMessage">No logs written yet</h2>
{/if}
{#if isUploading}
    <div class="lds-ellipsis"><div></div><div></div><div></div><div></div></div>
{/if}
<div
    id="logs"
    class="logs"
    bind:this={logsDiv}
    on:scroll={async () => {
        if (logsDiv.scrollTop >= 0 && logsDiv.scrollTop < 5 && !isLogsUpdating) {
            isLogsUpdating = true;
            oldScrollHeight = logsDiv.scrollHeight;
            tmpLogs=allLogs;
            const newLogs = await getLogs(serviceName, "", logLinesCount, offset);
            offset += newLogs.length;
            setTimeout(() => {
                logsDiv.scrollTop = (logsDiv.scrollHeight - oldScrollHeight);
                isLogsUpdating = false;
            })
            tmpLogs = allLogs;
        }
    }}
    >
    {#if searchText.length == 0}
        <!-- svelte-ignore empty-block -->
        {#await getLogsStream(serviceName)}
        {:then}
            {#each tmpLogs as logItem}
                <LogsString
                    bind:this={logString}
                    time={logItem.slice(0, 19).replace("T", " ")}
                    message={logItem.slice(30)}
                    status={getLogLineStatus(logItem)}
                />
            {/each}
        {/await}
    {:else}
        <!-- svelte-ignore empty-block -->
        {#await getLogs(serviceName, searchText, logLinesCount, 0)}
        {:then logs}
            {#each logs as logItem}
                <LogsString
                    bind:this={logString}
                    time={logItem.slice(0, 19).replace("T", " ")}
                    message={logItem.slice(30)}
                    status={getLogLineStatus(logItem)}
                />
            {/each}
        {/await}
    {/if}
</div>
