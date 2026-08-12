<script>
  // @ts-nocheck

  import { onMount, tick } from "svelte";
  import LogsString from "../LogsString/LogsString.svelte";
  import Loader from "../../Views/Logs/Loader.svelte";
  import fetchApi from "../../utils/fetch";
  import { store } from "../../Stores/stores.js";
  import {
    getLogLineStatus,
    transformLogString,
  } from "../../Views/Logs/functions.js";
  import { findSearchTextInLogs } from "../../utils/highlight.js";

  let {
    host = "",
    service = "",
    anchor = null,
    searchText = "",
    caseSens = false,
    shareLink = null,
  } = $props();

  const api = new fetchApi();
  const PAGE = 20;
  // The full storage key. A row that arrived over the websocket carries only its
  // timestamp, which the backend also accepts as a cursor.
  const keyOf = (row) => row?.at(2) ?? row?.at(0) ?? "";
  const linesOf = (response) =>
    Array.isArray(response?.logs) ? response.logs : [];

  let older = $state([]); // oldest first
  let newer = $state([]); // oldest first
  let isLoading = $state(true);
  let isPaging = $state(false);
  let olderDone = $state(false);
  let newerDone = $state(false);
  let olderCursor = keyOf(anchor);
  let newerCursor = keyOf(anchor);
  let box = $state();

  // No search and no status: the whole point is to see the lines the filter hid.
  async function loadOlder() {
    const response = await api.getLogs({
      containerName: service,
      hostName: host,
      limit: PAGE,
      startWith: olderCursor,
    });
    const page = linesOf(response); // newest first
    olderDone = !page.length || Boolean(response?.is_end);
    if (!page.length) {
      return;
    }
    olderCursor = response?.last_processed_key || keyOf(page.at(-1));
    older = [...page.slice().reverse(), ...older];
  }

  async function loadNewer() {
    const response = await api.getPrevLogs({
      containerName: service,
      hostName: host,
      limit: PAGE,
      startWith: newerCursor,
    });
    const page = linesOf(response); // oldest first
    newerDone = !page.length || Boolean(response?.is_end);
    if (!page.length) {
      return;
    }
    newerCursor = response?.last_processed_key || keyOf(page.at(-1));
    newer = [...newer, ...page];
  }

  function highlightSearch() {
    if (searchText) {
      findSearchTextInLogs("#logContext .message p", searchText, caseSens);
    }
  }

  // Prepending shifts everything down, so the row the reader was looking at has
  // to be pulled back under the cursor by however much the list grew.
  async function showOlder() {
    if (isPaging || olderDone) {
      return;
    }
    isPaging = true;
    const height = box?.scrollHeight ?? 0;
    const top = box?.scrollTop ?? 0;
    await loadOlder();
    await tick();
    if (box) {
      box.scrollTop = top + (box.scrollHeight - height);
    }
    isPaging = false;
    highlightSearch();
  }

  async function showNewer() {
    if (isPaging || newerDone) {
      return;
    }
    isPaging = true;
    await loadNewer();
    isPaging = false;
    await tick();
    highlightSearch();
  }

  onMount(async () => {
    if (!keyOf(anchor)) {
      isLoading = false;
      return;
    }
    try {
      await Promise.all([loadOlder(), loadNewer()]);
    } finally {
      isLoading = false;
    }
    await tick();
    // scrollIntoView would drag the page behind the modal along with it.
    const anchorEl = box?.querySelector(".contextAnchor");
    if (box && anchorEl) {
      box.scrollTop =
        anchorEl.offsetTop - box.clientHeight / 2 + anchorEl.offsetHeight / 2;
    }
    highlightSearch();
  });
</script>

<div class="logContext">
  <div class="logContextHead">
    <h3>Context</h3>
    <p>{service} &middot; {host}</p>
  </div>

  {#if isLoading}
    <div class="logContextLoading"><Loader /></div>
  {:else}
    <div class="logContextBox" id="logContext" bind:this={box}>
      <button
        class="logContextMore"
        disabled={olderDone || isPaging}
        onclick={showOlder}
      >
        {olderDone ? "No older lines" : `Load ${PAGE} older`}
      </button>

      {#each older as row (keyOf(row))}
        <LogsString
          time={transformLogString(row, $store.UTCtime)}
          message={row?.at(1)}
          status={getLogLineStatus(row?.at(1))}
          statusFilterable={false}
          sharedLinkCallBack={shareLink ? () => shareLink(row) : null}
        />
      {/each}

      <div class="contextAnchor">
        <LogsString
          time={transformLogString(anchor, $store.UTCtime)}
          message={anchor?.at(1)}
          status={getLogLineStatus(anchor?.at(1))}
          statusFilterable={false}
          sharedLinkCallBack={shareLink ? () => shareLink(anchor) : null}
        />
      </div>

      {#each newer as row (keyOf(row))}
        <LogsString
          time={transformLogString(row, $store.UTCtime)}
          message={row?.at(1)}
          status={getLogLineStatus(row?.at(1))}
          statusFilterable={false}
          sharedLinkCallBack={shareLink ? () => shareLink(row) : null}
        />
      {/each}

      <button
        class="logContextMore"
        disabled={newerDone || isPaging}
        onclick={showNewer}
      >
        {newerDone ? "No newer lines" : `Load ${PAGE} newer`}
      </button>
    </div>
  {/if}
</div>
