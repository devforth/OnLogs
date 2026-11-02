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

  // A page of 128 KiB lines froze the view; the rest is one click away.
  const PREVIEW_CHARS = 5000;
  let expanded = $state(false);
  let text = $derived(message ?? "");
  let isLong = $derived(text.length > PREVIEW_CHARS);
  let collapsed = $derived(isLong && !expanded);
  // The cut must not leave half an escape sequence behind.
  let shown = $derived(
    collapsed ? text.slice(0, PREVIEW_CHARS).replace(/[\u001B\u009B][^A-Za-z]*$/, "") : text
  );
  let parsedStr = $derived(collapsed ? null : tryToParseLogString(text));
  let messageHtml = $derived(toAnsiHtml(shown));
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
    {#if isLong}<button class="showAll" onclick={() => (expanded = !expanded)}
        >{expanded
          ? "Show less"
          : `Show all (${Math.ceil((text.length - PREVIEW_CHARS) / 1024)} KB more)`}</button
      >{/if}
  </div>
</div>
