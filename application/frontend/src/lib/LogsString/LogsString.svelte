<script>
  import { tryToParseLogString } from "../../utils/functions";
  import { toAnsiHtml } from "../../utils/ansi";
  import { chosenStatus } from "../../Stores/stores";
  import fetchApi from "../../utils/fetch";

  import { store } from "../../Stores/stores.js";
  let {
    status = "",
    time = "",
    message = "",
    width = "",
    isHiglighted = false,
    sharedLinkCallBack = () => {},
    getLogsByTagOptions = {}
  } = $props();

  let activeStatus = "";
  let parsedStr = $derived(tryToParseLogString(message));
  let messageHtml = $derived(toAnsiHtml(message));
</script>

<div
  class="logsString {isHiglighted ? 'new' : ''} {message?.trim().length === 0
    ? 'emptyLogsString'
    : ''}"
  style="width: {width}px"
>
  <div
    onclick={async () => {
      if ($chosenStatus !== status) {
        chosenStatus.set(status);
      } else {
        chosenStatus.set("");
      }
    }}
    class="status {status ? status : 'hidden'} {status === $chosenStatus
      ? 'chosenStatus'
      : ''}"><p><span> ◉ </span>{status.toUpperCase()}</p></div
  >

  <div class="time row_group"
    ><p>{message?.trim()?.length > 0 ? time : ""}</p>
    <div>
      {#if message?.trim()?.length > 0}
        <div
          id={`thumb-shared-${time}`}
          class="shareLinkButtonThumb"
          onclick={() => {
            sharedLinkCallBack();
          }}
        >
          <i class="log log-ShareLink" id={`shared-${time}`}></i>
        </div>{/if}
    </div>
  </div>
  <div class="message">
    {#if !parsedStr}<p>
        {@html messageHtml}
      </p>{:else if $store.transformJson}<p>{@html toAnsiHtml(parsedStr.startText)}</p>
      <pre>{JSON.stringify(parsedStr.json, null, 2)}</pre>
      <p>{@html toAnsiHtml(parsedStr.endText)}</p>
    {:else}<p>
        {@html messageHtml}
      </p>{/if}
  </div>
</div>
