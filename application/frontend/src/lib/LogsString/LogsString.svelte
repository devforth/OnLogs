<script>
  import { tryToParseLogString } from "../../utils/functions";
  import { toAnsiHtml } from "../../utils/ansi";
  import { chosenStatus } from "../../Stores/stores";
  import { store } from "../../Stores/stores.js";
  let {
    status = "",
    time = "",
    message = "",
    width = "",
    isHiglighted = false,
    // Null hides the button rather than rendering one that does nothing.
    sharedLinkCallBack = null,
    showContextCallBack = null,
    statusFilterable = true,
  } = $props();

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
      if (!statusFilterable) {
        return;
      }
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
      {#if message?.trim()?.length > 0 && showContextCallBack}
        <div
          id={`thumb-context-${time}`}
          class="contextButtonThumb"
          title="Show context"
          onclick={() => {
            showContextCallBack();
          }}
        >
          <i class="log log-Eye" id={`context-${time}`}></i>
        </div>{/if}
      {#if message?.trim()?.length > 0 && sharedLinkCallBack}
        <div
          id={`thumb-shared-${time}`}
          class="shareLinkButtonThumb"
          title="Copy link to this line"
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
